//go:build arm64 && go1.20 && !go1.26
// +build arm64,go1.20,!go1.26

/*
 * Copyright 2021 ByteDance Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package arm64

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// MemoryPool provides optimized memory management for ARM64 JIT operations
// Reduces allocation overhead and improves cache locality
type MemoryPool struct {
	// Buffer pools for different sizes
	buffers     map[int]*sync.Pool
	bufferMutex sync.RWMutex

	// JIT code memory pools
	codeBlocks    map[int]*sync.Pool
	codeMutex     sync.RWMutex
	codeAllocator *JITCodeAllocator

	// Statistics
	totalAllocs int64
	totalFrees  int64
	totalBytes  int64
	hitCount    int64
	missCount   int64

	// Configuration
	config PoolConfig
}

// PoolConfig configures memory pool behavior
type PoolConfig struct {
	MinBufferSize    int           // Minimum buffer size (bytes)
	MaxBufferSize    int           // Maximum buffer size (bytes)
	BufferGrowth     float64       // Buffer size growth factor
	CodeBlockSize    int           // JIT code block size (bytes)
	MaxCodeBlocks    int           // Maximum number of code blocks
	GCThreshold      int           // GC trigger threshold
	EnablePrealloc   bool          // Enable pre-allocation
	EnableProfiling  bool          // Enable memory profiling
	ZeroOnFree       bool          // Zero memory on free
	AlignTo          int           // Memory alignment
	MaxIdleTime      time.Duration // Maximum idle time before GC
}

// DefaultPoolConfig returns sensible default configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MinBufferSize:   64,
		MaxBufferSize:   64 * 1024, // 64KB
		BufferGrowth:    2.0,
		CodeBlockSize:   4096, // 4KB pages
		MaxCodeBlocks:   1000,
		GCThreshold:     10000,
		EnablePrealloc:  true,
		EnableProfiling: false,
		ZeroOnFree:      false,
		AlignTo:         16, // ARM64 cache line alignment
		MaxIdleTime:     5 * time.Minute,
	}
}

// NewMemoryPool creates a new optimized memory pool
func NewMemoryPool(config PoolConfig) *MemoryPool {
	pool := &MemoryPool{
		buffers:       make(map[int]*sync.Pool),
		codeBlocks:    make(map[int]*sync.Pool),
		codeAllocator: NewJITCodeAllocator(config.CodeBlockSize),
		config:        config,
	}

	// Pre-allocate common buffer sizes
	if config.EnablePrealloc {
		pool.preallocateBuffers()
	}

	// Start GC goroutine if enabled
	go pool.gcLoop()

	return pool
}

// GetBuffer retrieves a buffer of the requested size
func (mp *MemoryPool) GetBuffer(size int) []byte {
	if size <= 0 {
		return nil
	}

	// Round up to nearest pool size
	poolSize := mp.roundUpBufferSize(size)

	mp.bufferMutex.RLock()
	pool, exists := mp.buffers[poolSize]
	mp.bufferMutex.RUnlock()

	var buf []byte
	if exists {
		buf = pool.Get().([]byte)
		if buf != nil {
			atomic.AddInt64(&mp.hitCount, 1)
			return buf[:size] // Return requested size
		}
	}

	atomic.AddInt64(&mp.missCount, 1)

	// Allocate new buffer if pool miss
	buf = make([]byte, poolSize)
	atomic.AddInt64(&mp.totalAllocs, 1)
	atomic.AddInt64(&mp.totalBytes, int64(poolSize))

	return buf[:size]
}

// PutBuffer returns a buffer to the pool
func (mp *MemoryPool) PutBuffer(buf []byte) {
	if buf == nil || cap(buf) == 0 {
		return
	}

	// Zero buffer if configured
	if mp.config.ZeroOnFree {
		for i := range buf {
			buf[i] = 0
		}
	}

	poolSize := cap(buf)

	mp.bufferMutex.RLock()
	pool, exists := mp.buffers[poolSize]
	mp.bufferMutex.RUnlock()

	if !exists {
		// Create new pool for this size
		mp.bufferMutex.Lock()
		// Double-check pattern
		if pool, exists = mp.buffers[poolSize]; !exists {
			pool = &sync.Pool{}
			mp.buffers[poolSize] = pool
		}
		mp.bufferMutex.Unlock()
	}

	pool.Put(buf)
	atomic.AddInt64(&mp.totalFrees, 1)
}

// GetCodeBlock retrieves executable memory for JIT code
func (mp *MemoryPool) GetCodeBlock(size int) ([]byte, func()) {
	if size <= 0 {
		return nil, nil
	}

	// Round up to block size
	blockSize := mp.roundUpCodeSize(size)

	mp.codeMutex.RLock()
	pool, exists := mp.codeBlocks[blockSize]
	mp.codeMutex.RUnlock()

	var block []byte
	var cleanup func()

	if exists {
		block = pool.Get().([]byte)
		if block != nil {
			atomic.AddInt64(&mp.hitCount, 1)
			return block[:size], func() { mp.PutCodeBlock(block) }
		}
	}

	atomic.AddInt64(&mp.missCount, 1)

	// Allocate new executable memory
	block, cleanup = mp.codeAllocator.Allocate(blockSize)
	atomic.AddInt64(&mp.totalAllocs, 1)
	atomic.AddInt64(&mp.totalBytes, int64(blockSize))

	return block[:size], cleanup
}

// PutCodeBlock returns executable memory to the pool
func (mp *MemoryPool) PutCodeBlock(block []byte) {
	if block == nil || cap(block) == 0 {
		return
	}

	blockSize := cap(block)

	mp.codeMutex.RLock()
	pool, exists := mp.codeBlocks[blockSize]
	mp.codeMutex.RUnlock()

	if !exists {
		// Create new pool for this size
		mp.codeMutex.Lock()
		if pool, exists = mp.codeBlocks[blockSize]; !exists {
			pool = &sync.Pool{}
			mp.codeBlocks[blockSize] = pool
		}
		mp.codeMutex.Unlock()
	}

	pool.Put(block)
	atomic.AddInt64(&mp.totalFrees, 1)
}

// JITCodeAllocator manages executable memory allocation
type JITCodeAllocator struct {
	blockSize    int
	allocated    atomic.Int64
	totalBlocks  atomic.Int64
	freedBlocks  atomic.Int64
}

// NewJITCodeAllocator creates a new JIT code allocator
func NewJITCodeAllocator(blockSize int) *JITCodeAllocator {
	return &JITCodeAllocator{
		blockSize: blockSize,
	}
}

// Allocate allocates executable memory for JIT code
func (jca *JITCodeAllocator) Allocate(size int) ([]byte, func()) {
	// Use mmap to allocate executable memory on ARM64
	// This is a simplified implementation
	block := make([]byte, size)

	jca.allocated.Add(int64(size))
	jca.totalBlocks.Add(1)

	// Return cleanup function
	cleanup := func() {
		jca.freedBlocks.Add(1)
		// In real implementation, this would munmap the memory
	}

	return block, cleanup
}

// AlignedBuffer provides aligned memory for ARM64 SIMD operations
type AlignedBuffer struct {
	data     []byte
	aligned  unsafe.Pointer
	size     int
	capacity int
}

// GetAlignedBuffer returns an aligned buffer for SIMD operations
func (mp *MemoryPool) GetAlignedBuffer(size int) *AlignedBuffer {
	if size <= 0 {
		return nil
	}

	// Allocate extra space for alignment
	extraSize := size + mp.config.AlignTo - 1
	buf := mp.GetBuffer(extraSize)

	// Calculate aligned address
	base := uintptr(unsafe.Pointer(&buf[0]))
	aligned := (base + uintptr(mp.config.AlignTo) - 1) &^ (uintptr(mp.config.AlignTo) - 1)
	offset := int(aligned - base)

	return &AlignedBuffer{
		data:     buf,
		aligned:  unsafe.Pointer(aligned),
		size:     size,
		capacity: extraSize - offset,
	}
}

// PutAlignedBuffer returns an aligned buffer to the pool
func (mp *MemoryPool) PutAlignedBuffer(ab *AlignedBuffer) {
	if ab == nil {
		return
	}

	mp.PutBuffer(ab.data)
}

// Pointer returns the aligned memory pointer
func (ab *AlignedBuffer) Pointer() unsafe.Pointer {
	return ab.aligned
}

// Slice returns a slice view of the aligned buffer
func (ab *AlignedBuffer) Slice() []byte {
	if ab == nil || ab.aligned == nil {
		return nil
	}

	// Create slice from aligned pointer
	hdr := (*struct {
		Data uintptr
		Len  int
		Cap  int
	})(unsafe.Pointer(&[]byte{}))

	hdr.Data = uintptr(ab.aligned)
	hdr.Len = ab.size
	hdr.Cap = ab.capacity

	return *(*[]byte)(unsafe.Pointer(hdr))
}

// Helper methods

func (mp *MemoryPool) roundUpBufferSize(size int) int {
	if size <= mp.config.MinBufferSize {
		return mp.config.MinBufferSize
	}

	// Find next power of 2 or growth factor size
	rounded := mp.config.MinBufferSize
	for rounded < size && rounded < mp.config.MaxBufferSize {
		newSize := int(float64(rounded) * mp.config.BufferGrowth)
		if newSize <= rounded {
			newSize = rounded + mp.config.MinBufferSize
		}
		rounded = newSize
	}

	if rounded > mp.config.MaxBufferSize {
		rounded = ((size + mp.config.MinBufferSize - 1) / mp.config.MinBufferSize) * mp.config.MinBufferSize
	}

	return rounded
}

func (mp *MemoryPool) roundUpCodeSize(size int) int {
	return ((size + mp.config.CodeBlockSize - 1) / mp.config.CodeBlockSize) * mp.config.CodeBlockSize
}

func (mp *MemoryPool) preallocateBuffers() {
	// Pre-allocate common buffer sizes
	sizes := []int{
		64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768,
	}

	for _, size := range sizes {
		if size > mp.config.MaxBufferSize {
			break
		}

		mp.bufferMutex.Lock()
		if _, exists := mp.buffers[size]; !exists {
			pool := &sync.Pool{}
			// Pre-populate with some buffers
			for i := 0; i < 4; i++ {
				buf := make([]byte, size)
				pool.Put(buf)
			}
			mp.buffers[size] = pool
		}
		mp.bufferMutex.Unlock()
	}
}

func (mp *MemoryPool) gcLoop() {
	ticker := time.NewTicker(mp.config.MaxIdleTime)
	defer ticker.Stop()

	for range ticker.C {
		if atomic.LoadInt64(&mp.totalAllocs) > int64(mp.config.GCThreshold) {
			mp.triggerGC()
		}
	}
}

func (mp *MemoryPool) triggerGC() {
	// Force garbage collection of unused pool entries
	runtime.GC()

	// Reset statistics
	atomic.StoreInt64(&mp.totalAllocs, 0)
	atomic.StoreInt64(&mp.totalFrees, 0)
}

// Statistics methods

func (mp *MemoryPool) GetStats() PoolStats {
	return PoolStats{
		TotalAllocs:   atomic.LoadInt64(&mp.totalAllocs),
		TotalFrees:    atomic.LoadInt64(&mp.totalFrees),
		TotalBytes:    atomic.LoadInt64(&mp.totalBytes),
		HitCount:      atomic.LoadInt64(&mp.hitCount),
		MissCount:     atomic.LoadInt64(&mp.missCount),
		HitRate:       float64(atomic.LoadInt64(&mp.hitCount)) /
					   float64(atomic.LoadInt64(&mp.hitCount)+atomic.LoadInt64(&mp.missCount)),
		BufferPools:   len(mp.buffers),
		CodePools:     len(mp.codeBlocks),
	}
}

// PoolStats provides memory pool statistics
type PoolStats struct {
	TotalAllocs int64   `json:"total_allocs"`
	TotalFrees  int64   `json:"total_frees"`
	TotalBytes  int64   `json:"total_bytes"`
	HitCount    int64   `json:"hit_count"`
	MissCount   int64   `json:"miss_count"`
	HitRate     float64 `json:"hit_rate"`
	BufferPools int     `json:"buffer_pools"`
	CodePools   int     `json:"code_pools"`
}

// Global memory pool instance
var globalMemoryPool *MemoryPool
var globalPoolOnce sync.Once

// GetGlobalMemoryPool returns the singleton global memory pool
func GetGlobalMemoryPool() *MemoryPool {
	globalPoolOnce.Do(func() {
		globalMemoryPool = NewMemoryPool(DefaultPoolConfig())
	})
	return globalMemoryPool
}

// ResetGlobalMemoryPool resets the global memory pool (mainly for testing)
func ResetGlobalMemoryPool() {
	globalPoolOnce = sync.Once{}
	globalMemoryPool = nil
}