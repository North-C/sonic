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
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// JITCache provides intelligent caching for ARM64 JIT compilation
// Reduces compilation overhead for frequently used types
type JITCache struct {
	// Core cache storage
	encoderCache sync.Map // map[string]*CachedEncoder
	decoderCache sync.Map // map[string]*CachedDecoder

	// Type information
	typeCache     sync.Map // map[uintptr]*TypeInfo
	typeCacheSize int32

	// Statistics
	cacheHits   int64
	cacheMisses int64
	compilations int64

	// Configuration
	config CacheConfig

	// Cleanup management
	cleanupTicker *time.Ticker
	cleanupStop   chan struct{}
}

// CacheConfig configures cache behavior
type CacheConfig struct {
	MaxSize          int           // Maximum number of cached items
	TTL              time.Duration // Time to live for cache entries
	CleanupInterval  time.Duration // Cleanup interval
	EnableTypeHash   bool          // Enable type hashing for faster lookups
	EnableStats      bool          // Enable detailed statistics
	EnableLRU        bool          // Enable LRU eviction
	CompressionLevel int           // Compression level for cached code
	PreloadTypes     []reflect.Type // Types to preload into cache
}

// DefaultCacheConfig returns sensible default configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxSize:         1000,
		TTL:             30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
		EnableTypeHash:  true,
		EnableStats:     true,
		EnableLRU:       true,
		CompressionLevel: 0, // No compression by default
		PreloadTypes:    []reflect.Type{},
	}
}

// CachedEncoder represents a cached ARM64 encoder
type CachedEncoder struct {
	Program       []byte      // Compiled JIT code
	Function      interface{} // Compiled function pointer
	TypeInfo      *TypeInfo   // Type information
	CompileTime   time.Time   // When this was compiled
	AccessTime    time.Time   // Last access time
	AccessCount   int64       // Number of accesses
	CodeSize      int         // Size of compiled code
	Instructions  int         // Number of instructions
	Optimizations []string    // Applied optimizations
}

// CachedDecoder represents a cached ARM64 decoder
type CachedDecoder struct {
	Program       []byte      // Compiled JIT code
	Function      interface{} // Compiled function pointer
	TypeInfo      *TypeInfo   // Type information
	CompileTime   time.Time   // When this was compiled
	AccessTime    time.Time   // Last access time
	AccessCount   int64       // Number of accesses
	CodeSize      int         // Size of compiled code
	Instructions  int         // Number of instructions
	Optimizations []string    // Applied optimizations
}

// TypeInfo contains optimized type information for fast lookups
type TypeInfo struct {
	TypeHash      uint64        // Fast hash of type
	Type          reflect.Type  // Go type
	Kind          reflect.Kind  // Type kind
	Size          uintptr       // Type size
	Align         uintptr       // Type alignment
	Fields        []FieldInfo   // Field information (for structs)
	Methods       []MethodInfo  // Method information
	IsPointer     bool          // Is pointer type
	IsInterface   bool          // Is interface type
	IsSlice       bool          // Is slice type
	IsMap         bool          // Is map type
	Element       *TypeInfo     // Element type (for slices, arrays, pointers)
	Key, Value    *TypeInfo     // Key and value types (for maps)
	Complexity    int           // Type complexity score
}

// FieldInfo contains optimized field information
type FieldInfo struct {
	Name     string    // Field name
	Type     *TypeInfo // Field type
	Index    int       // Field index
	Offset   uintptr   // Field offset
	Tag      string    // JSON tag
	Exported bool      // Is exported field
	Embedded bool      // Is embedded field
}

// MethodInfo contains optimized method information
type MethodInfo struct {
	Name      string    // Method name
	Type      *TypeInfo // Method type
	Index     int       // Method index
	Exported  bool      // Is exported method
}

// NewJITCache creates a new JIT cache
func NewJITCache(config CacheConfig) *JITCache {
	cache := &JITCache{
		config:       config,
		cleanupStop:  make(chan struct{}),
	}

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		cache.cleanupTicker = time.NewTicker(config.CleanupInterval)
		go cache.cleanupLoop()
	}

	// Preload types if specified
	if len(config.PreloadTypes) > 0 {
		go cache.preloadTypes(config.PreloadTypes)
	}

	return cache
}

// GetEncoder retrieves or compiles an encoder for the given type
func (jc *JITCache) GetEncoder(vt reflect.Type) (*CachedEncoder, error) {
	typeInfo := jc.getTypeInfo(vt)
	cacheKey := jc.getCacheKey(typeInfo, "encoder")

	// Try cache first
	if cached, ok := jc.encoderCache.Load(cacheKey); ok {
		if encoder := cached.(*CachedEncoder); jc.isValid(encoder) {
			atomic.AddInt64(&jc.cacheHits, 1)
			encoder.AccessCount++
			encoder.AccessTime = time.Now()
			return encoder, nil
		}
		// Invalid entry, remove it
		jc.encoderCache.Delete(cacheKey)
	}

	// Cache miss, compile new encoder
	atomic.AddInt64(&jc.cacheMisses, 1)
	atomic.AddInt64(&jc.compilations, 1)

	encoder, err := jc.compileEncoder(typeInfo)
	if err != nil {
		return nil, err
	}

	// Store in cache
	jc.encoderCache.Store(cacheKey, encoder)

	return encoder, nil
}

// GetDecoder retrieves or compiles a decoder for the given type
func (jc *JITCache) GetDecoder(vt reflect.Type) (*CachedDecoder, error) {
	typeInfo := jc.getTypeInfo(vt)
	cacheKey := jc.getCacheKey(typeInfo, "decoder")

	// Try cache first
	if cached, ok := jc.decoderCache.Load(cacheKey); ok {
		if decoder := cached.(*CachedDecoder); jc.isValid(decoder) {
			atomic.AddInt64(&jc.cacheHits, 1)
			decoder.AccessCount++
			decoder.AccessTime = time.Now()
			return decoder, nil
		}
		// Invalid entry, remove it
		jc.decoderCache.Delete(cacheKey)
	}

	// Cache miss, compile new decoder
	atomic.AddInt64(&jc.cacheMisses, 1)
	atomic.AddInt64(&jc.compilations, 1)

	decoder, err := jc.compileDecoder(typeInfo)
	if err != nil {
		return nil, err
	}

	// Store in cache
	jc.decoderCache.Store(cacheKey, decoder)

	return decoder, nil
}

// getTypeInfo gets or creates type information for the given type
func (jc *JITCache) getTypeInfo(vt reflect.Type) *TypeInfo {
	typePtr := (*[2]uintptr)(unsafe.Pointer(&vt))
	typeHash := jc.computeTypeHash(vt)

	// Try cache first
	if cached, ok := jc.typeCache.Load(typePtr[0]); ok {
		return cached.(*TypeInfo)
	}

	// Create new type info
	info := &TypeInfo{
		TypeHash:    typeHash,
		Type:        vt,
		Kind:        vt.Kind(),
		Size:        vt.Size(),
		Align:       vt.Align(),
		IsPointer:   vt.Kind() == reflect.Ptr,
		IsInterface: vt.Kind() == reflect.Interface,
		IsSlice:     vt.Kind() == reflect.Slice,
		IsMap:       vt.Kind() == reflect.Map,
		Complexity:  jc.computeComplexity(vt),
	}

	// Gather field information for structs
	if vt.Kind() == reflect.Struct {
		info.Fields = jc.getFieldInfo(vt)
	}

	// Gather method information
	info.Methods = jc.getMethodInfo(vt)

	// Handle element types for slices, arrays, pointers
	switch vt.Kind() {
	case reflect.Slice, reflect.Array, reflect.Ptr:
		info.Element = jc.getTypeInfo(vt.Elem())
	case reflect.Map:
		info.Key = jc.getTypeInfo(vt.Key())
		info.Value = jc.getTypeInfo(vt.Elem())
	}

	// Store in cache
	jc.typeCache.Store(typePtr[0], info)
	atomic.AddInt32(&jc.typeCacheSize, 1)

	return info
}

// getFieldInfo extracts field information from struct types
func (jc *JITCache) getFieldInfo(vt reflect.Type) []FieldInfo {
	var fields []FieldInfo
	for i := 0; i < vt.NumField(); i++ {
		field := vt.Field(i)
		jsonTag := field.Tag.Get("json")

		fields = append(fields, FieldInfo{
			Name:     field.Name,
			Type:     jc.getTypeInfo(field.Type),
			Index:    i,
			Offset:   field.Offset,
			Tag:      jsonTag,
			Exported: field.PkgPath == "",
			Embedded: field.Anonymous,
		})
	}
	return fields
}

// getMethodInfo extracts method information from types
func (jc *JITCache) getMethodInfo(vt reflect.Type) []MethodInfo {
	var methods []MethodInfo
	for i := 0; i < vt.NumMethod(); i++ {
		method := vt.Method(i)
		methods = append(methods, MethodInfo{
			Name:     method.Name,
			Type:     jc.getTypeInfo(method.Type),
			Index:    i,
			Exported: method.PkgPath == "",
		})
	}
	return methods
}

// computeTypeHash computes a fast hash for type identification
func (jc *JITCache) computeTypeHash(vt reflect.Type) uint64 {
	if !jc.config.EnableTypeHash {
		return 0
	}

	// Simple hash implementation - in production, use a better hash function
	h := uint64(2166136261)
	h ^= uint64(vt.Kind())
	h *= 16777619

	name := vt.String()
	for _, c := range name {
		h ^= uint64(c)
		h *= 16777619
	}

	return h
}

// computeComplexity calculates type complexity for cache prioritization
func (jc *JITCache) computeComplexity(vt reflect.Type) int {
	complexity := 1

	switch vt.Kind() {
	case reflect.Struct:
		complexity += vt.NumField() * 2
		for i := 0; i < vt.NumField(); i++ {
			complexity += jc.computeComplexity(vt.Field(i).Type)
		}
	case reflect.Slice, reflect.Array:
		complexity += 1 + jc.computeComplexity(vt.Elem())
	case reflect.Map:
		complexity += 1 + jc.computeComplexity(vt.Key()) + jc.computeComplexity(vt.Elem())
	case reflect.Ptr:
		complexity += jc.computeComplexity(vt.Elem())
	case reflect.Interface:
		complexity += 10 // Interfaces are more complex
	}

	return complexity
}

// getCacheKey generates cache key for type and operation
func (jc *JITCache) getCacheKey(info *TypeInfo, operation string) string {
	return fmt.Sprintf("%s_%s_%x", operation, info.Type.String(), info.TypeHash)
}

// isValid checks if cached entry is still valid (TTL check)
func (jc *JITCache) isValid(entry interface{}) bool {
	if jc.config.TTL <= 0 {
		return true
	}

	var accessTime time.Time
	switch e := entry.(type) {
	case *CachedEncoder:
		accessTime = e.AccessTime
	case *CachedDecoder:
		accessTime = e.AccessTime
	default:
		return false
	}

	return time.Since(accessTime) < jc.config.TTL
}

// compileEncoder compiles new encoder for given type (placeholder)
func (jc *JITCache) compileEncoder(info *TypeInfo) (*CachedEncoder, error) {
	// This would integrate with the actual ARM64 encoder JIT
	now := time.Now()

	return &CachedEncoder{
		TypeInfo:     info,
		CompileTime:  now,
		AccessTime:   now,
		AccessCount:  1,
		CodeSize:     1024, // Placeholder
		Instructions: 100,  // Placeholder
		Optimizations: []string{"arm64", "simd", "inline"},
	}, nil
}

// compileDecoder compiles new decoder for given type (placeholder)
func (jc *JITCache) compileDecoder(info *TypeInfo) (*CachedDecoder, error) {
	// This would integrate with the actual ARM64 decoder JIT
	now := time.Now()

	return &CachedDecoder{
		TypeInfo:     info,
		CompileTime:  now,
		AccessTime:   now,
		AccessCount:  1,
		CodeSize:     1024, // Placeholder
		Instructions: 100,  // Placeholder
		Optimizations: []string{"arm64", "simd", "branch-prediction"},
	}, nil
}

// preloadTypes precompiles encoders/decoders for specified types
func (jc *JITCache) preloadTypes(types []reflect.Type) {
	for _, vt := range types {
		jc.GetEncoder(vt)
		jc.GetDecoder(vt)
	}
}

// cleanupLoop periodically cleans up expired cache entries
func (jc *JITCache) cleanupLoop() {
	ticker := jc.cleanupTicker
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			jc.cleanup()
		case <-jc.cleanupStop:
			return
		}
	}
}

// cleanup removes expired and least-used cache entries
func (jc *JITCache) cleanup() {
	now := time.Now()
	maxSize := jc.config.MaxSize

	// Cleanup encoder cache
	encoderCount := 0
	jc.encoderCache.Range(func(key, value interface{}) bool {
		encoder := value.(*CachedEncoder)

		// Remove expired entries
		if jc.config.TTL > 0 && now.Sub(encoder.AccessTime) > jc.config.TTL {
			jc.encoderCache.Delete(key)
			return true
		}

		encoderCount++

		// Remove least used entries if over limit
		if encoderCount > maxSize {
			jc.encoderCache.Delete(key)
		}

		return true
	})

	// Cleanup decoder cache
	decoderCount := 0
	jc.decoderCache.Range(func(key, value interface{}) bool {
		decoder := value.(*CachedDecoder)

		// Remove expired entries
		if jc.config.TTL > 0 && now.Sub(decoder.AccessTime) > jc.config.TTL {
			jc.decoderCache.Delete(key)
			return true
		}

		decoderCount++

		// Remove least used entries if over limit
		if decoderCount > maxSize {
			jc.decoderCache.Delete(key)
		}

		return true
	})
}

// Close stops the cache cleanup goroutine
func (jc *JITCache) Close() {
	if jc.cleanupTicker != nil {
		close(jc.cleanupStop)
		jc.cleanupTicker.Stop()
	}
}

// GetStats returns cache statistics
func (jc *JITCache) GetStats() CacheStats {
	encoderCount := 0
	decoderCount := 0
	totalCodeSize := 0

	jc.encoderCache.Range(func(key, value interface{}) bool {
		encoder := value.(*CachedEncoder)
		encoderCount++
		totalCodeSize += encoder.CodeSize
		return true
	})

	jc.decoderCache.Range(func(key, value interface{}) bool {
		decoder := value.(*CachedDecoder)
		decoderCount++
		totalCodeSize += decoder.CodeSize
		return true
	})

	hits := atomic.LoadInt64(&jc.cacheHits)
	misses := atomic.LoadInt64(&jc.cacheMisses)
	compilations := atomic.LoadInt64(&jc.compilations)

	return CacheStats{
		EncoderCount:   encoderCount,
		DecoderCount:   decoderCount,
		TypeCount:      int(atomic.LoadInt32(&jc.typeCacheSize)),
		CacheHits:      hits,
		CacheMisses:    misses,
		HitRate:        float64(hits) / float64(hits+misses),
		Compilations:   compilations,
		TotalCodeSize:  totalCodeSize,
		AverageCodeSize: totalCodeSize / (encoderCount + decoderCount + 1),
	}
}

// CacheStats provides cache statistics
type CacheStats struct {
	EncoderCount    int     `json:"encoder_count"`
	DecoderCount    int     `json:"decoder_count"`
	TypeCount       int     `json:"type_count"`
	CacheHits       int64   `json:"cache_hits"`
	CacheMisses     int64   `json:"cache_misses"`
	HitRate         float64 `json:"hit_rate"`
	Compilations    int64   `json:"compilations"`
	TotalCodeSize   int     `json:"total_code_size"`
	AverageCodeSize int     `json:"average_code_size"`
}

// Global cache instance
var globalJITCache *JITCache
var globalCacheOnce sync.Once

// GetGlobalJITCache returns the singleton global JIT cache
func GetGlobalJITCache() *JITCache {
	globalCacheOnce.Do(func() {
		globalJITCache = NewJITCache(DefaultCacheConfig())
	})
	return globalJITCache
}

// ResetGlobalJITCache resets the global JIT cache (mainly for testing)
func ResetGlobalJITCache() {
	if globalJITCache != nil {
		globalJITCache.Close()
	}
	globalCacheOnce = sync.Once{}
	globalJITCache = nil
}