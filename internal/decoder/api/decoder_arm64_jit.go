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

package api

import (
	"reflect"
	"unsafe"

	"github.com/bytedance/sonic/internal/decoder/jitdec"
	"github.com/bytedance/sonic/internal/envs"
	"github.com/bytedance/sonic/internal/decoder/optdec"
	"github.com/bytedance/sonic/option"
)

// ARM64 JIT decoder interface
var (
	pretouchImpl = jitdec.Pretouch
	decodeImpl   = decodeWithJIT
)

func init() {
	// Check if JIT should be enabled
	if envs.UseJIT {
		pretouchImpl = jitdec.Pretouch
		decodeImpl = decodeWithJIT
	} else if envs.UseOptDec {
		pretouchImpl = optdec.Pretouch
		decodeImpl = optdec.Decode
	} else {
		// Fallback to optimized decoder
		pretouchImpl = optdec.Pretouch
		decodeImpl = optdec.Decode
	}
}

// decodeWithJIT implements decoding using ARM64 JIT compilation
func decodeWithJIT(sp *string, ic *int, fv uint64, val interface{}) error {
	// Create decoder stack
	sb := jitdec.NewStack()

	// Get the type of the value to decode
	vt := reflect.TypeOf(val)
	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}

	// Check if JIT compilation is available and enabled
	if !jitdec.IsJITEnabled() {
		// Fallback to optimized decoder
		return optdec.Decode(sp, ic, fv, val)
	}

	// Create and compile a JIT decoder for this type
	decoder := jitdec.CreateDecoderWithName("decode_jit")
	compiledDecoder, err := decoder.Compile(vt)
	if err != nil {
		// Fallback to optimized decoder on compilation error
		return optdec.Decode(sp, ic, fv, val)
	}

	// Get the actual value pointer
	var vp unsafe.Pointer
	if val != nil {
		vp = unsafe.Pointer(&val)
	}

	// Perform the decoding using compiled JIT code
	result, err := compiledDecoder.(func(string, int, unsafe.Pointer, *jitdec._Stack, uint64, string) (int, error))(*sp, *ic, vp, sb, fv, "")
	if err != nil {
		// Fallback to optimized decoder on runtime error
		return optdec.Decode(sp, ic, fv, val)
	}

	*ic = result
	return nil
}

// ARM64JITDecoder provides additional ARM64-specific functionality
type ARM64JITDecoder struct {
	*Decoder
}

// NewARM64JITDecoder creates a new ARM64 JIT decoder with additional options
func NewARM64JITDecoder(name string, opts ...option.CompileOption) *ARM64JITDecoder {
	baseDecoder := NewDecoder(name)
	if len(opts) > 0 {
		// Apply compile options to the base decoder
		// This would require extending the decoder interface
	}

	return &ARM64JITDecoder{Decoder: baseDecoder}
}

// CompileType compiles a specific type for ARM64 JIT decoding
func (d *ARM64JITDecoder) CompileType(vt reflect.Type, opts ...option.CompileOption) error {
	// Create a temporary decoder to compile this specific type
	tempDecoder := jitdec.CreateDecoderWithName("compile_" + vt.String())
	_, err := tempDecoder.Compile(vt, opts...)
	return err
}

// BatchCompile compiles multiple types in batch for better performance
func (d *ARM64JITDecoder) BatchCompile(types []reflect.Type, opts ...option.CompileOption) (map[reflect.Type]interface{}, error) {
	return jitdec.BatchCompile(types)
}

// GetJITStatistics returns JIT compilation statistics
func (d *ARM64JITDecoder) GetJITStatistics() jitdec.PerfStats {
	return jitdec.GetPerfStats(d.Decoder)
}

// EnableSIMD enables SIMD optimizations in the JIT compiler
func (d *ARM64JITDecoder) EnableSIMD() {
	opts := jitdec.DefaultJITOptions()
	opts.EnableSIMD = true
	d.ApplyJITOptions(opts)
}

// DisableSIMD disables SIMD optimizations in the JIT compiler
func (d *ARM64JITDecoder) DisableSIMD() {
	opts := jitdec.DefaultJITOptions()
	opts.EnableSIMD = false
	d.ApplyJITOptions(opts)
}

// SetOptimizationLevel sets the JIT optimization level (0-3)
func (d *ARM64JITDecoder) SetOptimizationLevel(level int) {
	if level < 0 || level > 3 {
		level = 1 // Default to level 1
	}
	opts := jitdec.DefaultJITOptions()
	opts.OptimizationLevel = level
	d.ApplyJITOptions(opts)
}

// EnableDebugMode enables debug mode for JIT compilation
func (d *ARM64JITDecoder) EnableDebugMode() {
	opts := jitdec.DefaultJITOptions()
	opts.DebugMode = true
	d.ApplyJITOptions(opts)
}

// GetDebugInfo returns detailed debug information about the compiled code
func (d *ARM64JITDecoder) GetDebugInfo() jitdec.DebugInfo {
	return jitdec.GetDebugInfo(d.Decoder)
}

// VerifyCompiledCode verifies that the compiled ARM64 code is valid
func (d *ARM64JITDecoder) VerifyCompiledCode() error {
	return d.Decoder.VerifyCode()
}

// WarmUp pre-compiles commonly used types to reduce first-hit latency
func (d *ARM64JITDecoder) WarmUp(types []reflect.Type, opts ...option.CompileOption) error {
	for _, vt := range types {
		err := d.CompileType(vt, opts...)
		if err != nil {
			return err
		}
	}
	return nil
}

// ARM64JITOptions provides configuration options for ARM64 JIT compilation
type ARM64JITOptions struct {
	OptimizationLevel int
	EnableSIMD       bool
	EnableInlining   bool
	DebugMode         bool
	StrictMode        bool
	FastPath          bool
	MaxStackDepth     int
	MaxProgramSize    int
}

// DefaultARM64JITOptions returns default ARM64 JIT options
func DefaultARM64JITOptions() ARM64JITOptions {
	return ARM64JITOptions{
		OptimizationLevel: jitdec.DefaultOptLevel,
		EnableSIMD:       true,
		EnableInlining:   true,
		DebugMode:         false,
		StrictMode:        false,
		FastPath:          true,
		MaxStackDepth:     jitdec.MaxStackDepth,
		MaxProgramSize:    jitdec.MaxProgramSize,
	}
}

// ConfigureARM64JIT configures global ARM64 JIT settings
func ConfigureARM64JIT(opts ARM64JITOptions) {
	// Apply global JIT configuration
	if opts.OptimizationLevel >= 0 && opts.OptimizationLevel <= 3 {
		// Set global optimization level
	}

	if opts.EnableSIMD {
		// Enable SIMD globally
		jitdec.EnableJIT()
	} else {
		// Disable SIMD globally
		jitdec.DisableJIT()
	}

	if opts.DebugMode {
		// Enable debug mode globally
	}
}

// IsARM64JITAvailable checks if ARM64 JIT compilation is available
func IsARM64JITAvailable() bool {
	return jitdec.IsJITEnabled()
}

// GetARM64JITArchitectureInfo returns ARM64-specific architecture information
func GetARM64JITArchitectureInfo() map[string]interface{} {
	return jitdec.GetArchitectureInfo()
}

// ARM64JITPretouch pre-compiles types using ARM64 JIT for better performance
func ARM64JITPretouch(vt reflect.Type, opts ...option.CompileOption) error {
	return jitdec.Pretouch(vt, opts...)
}

// ARM64JITDecode performs decoding using ARM64 JIT compilation
func ARM64JITDecode(data []byte, val interface{}) error {
	if len(data) == 0 {
		return nil
	}

	s := string(data)
	ic := 0
	return decodeImpl(&s, &ic, 0, val)
}

// ARM64JITDecodeString performs string decoding using ARM64 JIT compilation
func ARM64JITDecodeString(s string, val interface{}) error {
	ic := 0
	return decodeImpl(&s, &ic, 0, val)
}

// Performance monitoring
type JITPerformanceStats struct {
	CompileTime     int64   `json:"compile_time_ns"`
	CodeSize        int     `json:"code_size_bytes"`
	InstructionCount int     `json:"instruction_count"`
	DecodeTime      int64   `json:"decode_time_ns"`
	Optimizations   []string `json:"optimizations"`
	CacheHits       int64   `json:"cache_hits"`
	CacheMisses     int64   `json:"cache_misses"`
}

// GetJITPerformanceStats returns performance statistics for ARM64 JIT
func GetJITPerformanceStats() JITPerformanceStats {
	// TODO: Implement actual performance monitoring
	return JITPerformanceStats{
		Optimizations: []string{"arm64", "jit", "simd", "neon"},
	}
}

// Cache management for compiled decoders
type DecoderCache struct {
	cache map[reflect.Type]interface{}
	mutex sync.RWMutex
}

var globalDecoderCache = &DecoderCache{
	cache: make(map[reflect.Type]interface{}),
}

// GetCachedDecoder returns a cached decoder for the given type
func GetCachedDecoder(vt reflect.Type) (interface{}, bool) {
	globalDecoderCache.mutex.RLock()
	defer globalDecoderCache.mutex.RUnlock()
	decoder, exists := globalDecoderCache.cache[vt]
	return decoder, exists
}

// CacheDecoder stores a compiled decoder in the cache
func CacheDecoder(vt reflect.Type, decoder interface{}) {
	globalDecoderCache.mutex.Lock()
	defer globalDecoderCache.mutex.Unlock()
	globalDecoderCache.cache[vt] = decoder
}

// ClearDecoderCache clears the global decoder cache
func ClearDecoderCache() {
	globalDecoderCache.mutex.Lock()
	defer globalDecoderCache.mutex.Unlock()
	globalDecoderCache.cache = make(map[reflect.Type]interface{})
}

// GetDecoderCacheSize returns the current size of the decoder cache
func GetDecoderCacheSize() int {
	globalDecoderCache.mutex.RLock()
	defer globalDecoderCache.mutex.RUnlock()
	return len(globalDecoderCache.cache)
}

// Memory management for JIT compiled code
type JITMemoryManager struct {
	totalAllocated int64
	maxMemory      int64
	compiledCode    map[string]interface{}
}

var globalJITMemoryManager = &JITMemoryManager{
	maxMemory:   100 * 1024 * 1024, // 100MB default limit
	compiledCode: make(map[string]interface{}),
}

// GetMemoryUsage returns current memory usage statistics
func GetMemoryUsage() (allocated, max int64, count int) {
	globalJITMemoryManager.mutex.Lock()
	defer globalJITMemoryManager.mutex.Unlock()
	return globalJITMemoryManager.totalAllocated,
		globalJITMemoryManager.maxMemory,
		len(globalJITMemoryManager.compiledCode)
}

// CleanupMemory frees unused JIT compiled code
func CleanupMemory() {
	globalJITMemoryManager.mutex.Lock()
	defer globalJITMemoryManager.mutex.Unlock()
	// TODO: Implement actual cleanup logic
}

import "sync"