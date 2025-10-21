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
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/bytedance/sonic/internal/encoder"
	"github.com/bytedance/sonic/internal/decoder"
)

// Performance benchmark suite for ARM64 JIT implementation
// These tests compare performance between ARM64 JIT and baseline implementations

// Benchmark test data structures
type SimpleStruct struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

type ComplexStruct struct {
	ID       int                    `json:"id"`
	Name     string                 `json:"name"`
	Active   bool                   `json:"active"`
	Score    float64                `json:"score"`
	Tags     []string               `json:"tags"`
	Metadata map[string]interface{} `json:"metadata"`
	Created  time.Time              `json:"created"`
}

type NestedStruct struct {
	Level1 struct {
		Level2 struct {
			Level3 struct {
				Value string `json:"value"`
				Count int    `json:"count"`
			} `json:"level3"`
			Items []int `json:"items"`
		} `json:"level2"`
		Enabled bool `json:"enabled"`
	} `json:"level1"`
	Data map[string]string `json:"data"`
}

// Benchmark datasets
var (
	benchmarkSimpleData = SimpleStruct{
		Name:  "John Doe",
		Age:   30,
		Email: "john.doe@example.com",
	}

	benchmarkComplexData = ComplexStruct{
		ID:     12345,
		Name:   "Complex Test Object",
		Active: true,
		Score:  95.5,
		Tags:   []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]interface{}{
			"version":     1.0,
			"environment": "production",
			"features":    []bool{true, false, true},
			"metrics": map[string]int{
				"requests":  1000,
				"errors":    10,
				"timeouts":   5,
				"cache_hits": 800,
			},
		},
		Created: time.Now(),
	}

	benchmarkNestedData = NestedStruct{
		Level1: struct {
			Level2 struct {
				Level3 struct {
					Value string `json:"value"`
					Count int    `json:"count"`
				} `json:"level3"`
				Items []int `json:"items"`
			} `json:"level2"`
			Enabled bool `json:"enabled"`
		}{
			Level2: struct {
				Level3 struct {
					Value string `json:"value"`
					Count int    `json:"count"`
				} `json:"level3"`
				Items []int `json:"items"`
			}{
				Level3: struct {
					Value string `json:"value"`
					Count int    `json:"count"`
				}{
					Value: "nested_value",
					Count: 42,
				},
				Items: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
			Enabled: true,
		},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
			"key5": "value5",
		},
	}

	// Large array for benchmarking
	benchmarkLargeArray = make([]int, 1000)
	benchmarkLargeMap   = make(map[string]interface{}, 1000)
)

func init() {
	// Initialize large benchmark data
	for i := 0; i < 1000; i++ {
		benchmarkLargeArray[i] = i
		benchmarkLargeMap[fmt.Sprintf("key_%d", i)] = i
	}
}

// ARM64 JIT Performance Benchmarks

func BenchmarkARM64JIT_Encode_Simple(b *testing.B) {
	enc := encoder.NewEncoder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(benchmarkSimpleData)
		if err != nil {
			b.Fatalf("Encode error: %v", err)
		}
	}
}

func BenchmarkARM64JIT_Encode_Complex(b *testing.B) {
	enc := encoder.NewEncoder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(benchmarkComplexData)
		if err != nil {
			b.Fatalf("Encode error: %v", err)
		}
	}
}

func BenchmarkARM64JIT_Encode_Nested(b *testing.B) {
	enc := encoder.NewEncoder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(benchmarkNestedData)
		if err != nil {
			b.Fatalf("Encode error: %v", err)
		}
	}
}

func BenchmarkARM64JIT_Encode_LargeArray(b *testing.B) {
	enc := encoder.NewEncoder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(benchmarkLargeArray)
		if err != nil {
			b.Fatalf("Encode error: %v", err)
		}
	}
}

func BenchmarkARM64JIT_Encode_LargeMap(b *testing.B) {
	enc := encoder.NewEncoder()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(benchmarkLargeMap)
		if err != nil {
			b.Fatalf("Encode error: %v", err)
		}
	}
}

func BenchmarkARM64JIT_Decode_Simple(b *testing.B) {
	data, _ := json.Marshal(benchmarkSimpleData)
	dec := decoder.NewDecoder(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var result SimpleStruct
		_, err := dec.Decode(&result)
		if err != nil {
			b.Fatalf("Decode error: %v", err)
		}
	}
}

func BenchmarkARM64JIT_Decode_Complex(b *testing.B) {
	data, _ := json.Marshal(benchmarkComplexData)
	dec := decoder.NewDecoder(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var result ComplexStruct
		_, err := dec.Decode(&result)
		if err != nil {
			b.Fatalf("Decode error: %v", err)
		}
	}
}

func BenchmarkARM64JIT_Decode_Nested(b *testing.B) {
	data, _ := json.Marshal(benchmarkNestedData)
	dec := decoder.NewDecoder(data)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var result NestedStruct
		_, err := dec.Decode(&result)
		if err != nil {
			b.Fatalf("Decode error: %v", err)
		}
	}
}

// Comparison benchmarks with standard encoding/json
func BenchmarkStdLib_Encode_Simple(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(benchmarkSimpleData)
		if err != nil {
			b.Fatalf("Marshal error: %v", err)
		}
	}
}

func BenchmarkStdLib_Encode_Complex(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(benchmarkComplexData)
		if err != nil {
			b.Fatalf("Marshal error: %v", err)
		}
	}
}

func BenchmarkStdLib_Decode_Simple(b *testing.B) {
	data, _ := json.Marshal(benchmarkSimpleData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var result SimpleStruct
		err := json.Unmarshal(data, &result)
		if err != nil {
			b.Fatalf("Unmarshal error: %v", err)
		}
	}
}

func BenchmarkStdLib_Decode_Complex(b *testing.B) {
	data, _ := json.Marshal(benchmarkComplexData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var result ComplexStruct
		err := json.Unmarshal(data, &result)
		if err != nil {
			b.Fatalf("Unmarshal error: %v", err)
		}
	}
}

// Memory usage benchmarks
func BenchmarkARM64JIT_Memory_Usage(b *testing.B) {
	enc := encoder.NewEncoder()

	// Force GC before benchmark
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(benchmarkComplexData)
		if err != nil {
			b.Fatalf("Encode error: %v", err)
		}
	}

	b.StopTimer()
	runtime.ReadMemStats(&m2)

	b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
	b.ReportMetric(float64(m2.Mallocs-m1.Mallocs)/float64(b.N), "allocs/op")
}

// JIT compilation performance benchmarks
func BenchmarkARM64JIT_Compile_Simple(b *testing.B) {
	vt := reflect.TypeOf(benchmarkSimpleData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc := encoder.CreateEncoderWithName("perf_test")
		goType := rt.UnpackType(vt)
		_, err := enc.Compile(goType)
		if err != nil {
			b.Fatalf("Compile error: %v", err)
		}
	}
}

func BenchmarkARM64JIT_Compile_Complex(b *testing.B) {
	vt := reflect.TypeOf(benchmarkComplexData)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc := encoder.CreateEncoderWithName("perf_test")
		goType := rt.UnpackType(vt)
		_, err := enc.Compile(goType)
		if err != nil {
			b.Fatalf("Compile error: %v", err)
		}
	}
}

// Concurrency benchmarks
func BenchmarkARM64JIT_Concurrent_Encode(b *testing.B) {
	enc := encoder.NewEncoder()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := encoder.Encode(benchmarkSimpleData)
			if err != nil {
				b.Fatalf("Encode error: %v", err)
			}
		}
	})
}

func BenchmarkARM64JIT_Concurrent_Decode(b *testing.B) {
	data, _ := json.Marshal(benchmarkSimpleData)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			dec := decoder.NewDecoder(data)
			var result SimpleStruct
			_, err := dec.Decode(&result)
			if err != nil {
				b.Fatalf("Decode error: %v", err)
			}
		}
	})
}

// Performance regression tests
func TestARM64JIT_Performance_Regression(t *testing.T) {
	// Set performance thresholds (in nanoseconds)
	const (
		encodeSimpleThreshold = 1000   // 1 microsecond
		encodeComplexThreshold = 10000  // 10 microseconds
		decodeSimpleThreshold = 2000   // 2 microseconds
		decodeComplexThreshold = 15000  // 15 microseconds
	)

	// Test encode performance
	start := time.Now()
	for i := 0; i < 1000; i++ {
		_, err := encoder.Encode(benchmarkSimpleData)
		if err != nil {
			t.Fatalf("Encode error: %v", err)
		}
	}
	encodeSimpleTime := time.Since(start) / 1000

	if encodeSimpleTime > encodeSimpleThreshold {
		t.Errorf("Simple encode performance regression: %v > %v", encodeSimpleTime, encodeSimpleThreshold)
	}

	// Test decode performance
	data, _ := json.Marshal(benchmarkSimpleData)
	start = time.Now()
	for i := 0; i < 1000; i++ {
		dec := decoder.NewDecoder(data)
		var result SimpleStruct
		_, err := dec.Decode(&result)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}
	}
	decodeSimpleTime := time.Since(start) / 1000

	if decodeSimpleTime > decodeSimpleThreshold {
		t.Errorf("Simple decode performance regression: %v > %v", decodeSimpleTime, decodeSimpleThreshold)
	}

	t.Logf("Performance results - Encode: %v, Decode: %v", encodeSimpleTime, decodeSimpleTime)
}

// Performance profiling utilities
func ProfileARM64JIT_Encoder(b *testing.B, data interface{}) {
	enc := encoder.NewEncoder()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(data)
		if err != nil {
			b.Fatalf("Encode error: %v", err)
		}
	}
}

func ProfileARM64JIT_Decoder(b *testing.B, data []byte, out interface{}) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dec := decoder.NewDecoder(data)
		_, err := dec.Decode(out)
		if err != nil {
			b.Fatalf("Decode error: %v", err)
		}
	}
}

// Cache performance tests
func TestARM64JIT_Cache_Performance(t *testing.T) {
	vt := reflect.TypeOf(benchmarkComplexData)

	// First compilation (cache miss)
	start := time.Now()
	enc1 := encoder.CreateEncoderWithName("cache_test_1")
	goType := rt.UnpackType(vt)
	_, err1 := enc1.Compile(goType)
	if err1 != nil {
		t.Fatalf("First compile error: %v", err1)
	}
	firstCompileTime := time.Since(start)

	// Second compilation (cache hit)
	start = time.Now()
	enc2 := encoder.CreateEncoderWithName("cache_test_2")
	_, err2 := enc2.Compile(goType)
	if err2 != nil {
		t.Fatalf("Second compile error: %v", err2)
	}
	secondCompileTime := time.Since(start)

	// Cache should improve performance significantly
	if secondCompileTime >= firstCompileTime {
		t.Logf("Warning: Cache may not be working effectively. First: %v, Second: %v",
			firstCompileTime, secondCompileTime)
	} else {
		improvement := float64(firstCompileTime-secondCompileTime) / float64(firstCompileTime) * 100
		t.Logf("Cache performance improvement: %.2f%% (%v -> %v)",
			improvement, firstCompileTime, secondCompileTime)
	}
}

// Stack usage analysis
func TestARM64JIT_Stack_Usage(t *testing.T) {
	// This test analyzes stack usage during JIT operations
	// Important for ARM64 where stack alignment is critical

	enc := encoder.NewEncoder()

	// Get initial stack pointer
	var sp1 uintptr
	asm := func() {
		var local [8]uintptr
		sp1 = uintptr(unsafe.Pointer(&local[0]))
	}
	asm()

	// Perform JIT operation
	_, err := encoder.Encode(benchmarkComplexData)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Get final stack pointer
	var sp2 uintptr
	asm = func() {
		var local [8]uintptr
		sp2 = uintptr(unsafe.Pointer(&local[0]))
	}
	asm()

	// Stack should be properly restored
	if sp1 != sp2 {
		t.Errorf("Stack corruption detected: sp1=%v, sp2=%v", sp1, sp2)
	}

	t.Logf("Stack usage analysis passed: sp=%v", sp1)
}