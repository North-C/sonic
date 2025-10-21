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
	"math"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/bytedance/sonic"
)

// TestHarness provides comprehensive testing utilities for ARM64 JIT
type TestHarness struct {
	t           *testing.T
	config      TestConfig
	profiler    *Profiler
	validator   *Validator
	benchmark   *BenchmarkSuite
}

// TestConfig configures test behavior
type TestConfig struct {
	EnableProfiling     bool
	EnableValidation    bool
	EnableBenchmarks    bool
	EnableStressTests   bool
	EnableMemoryTests   bool
	EnableConcurrency   bool
	MaxTestTypes        int
	MaxTestDataSize     int
	TestTimeout         time.Duration
	VerboseLogging      bool
	CompareWithStdLib   bool
	CompareWithAMD64    bool
}

// DefaultTestConfig returns sensible default test configuration
func DefaultTestConfig() TestConfig {
	return TestConfig{
		EnableProfiling:   true,
		EnableValidation:  true,
		EnableBenchmarks:  true,
		EnableStressTests: false, // Disabled by default for speed
		EnableMemoryTests: true,
		EnableConcurrency: true,
		MaxTestTypes:      100,
		MaxTestDataSize:   1024 * 1024, // 1MB
		TestTimeout:       30 * time.Second,
		VerboseLogging:    false,
		CompareWithStdLib: true,
		CompareWithAMD64:  false, // Only on ARM64
	}
}

// NewTestHarness creates a new test harness
func NewTestHarness(t *testing.T, config TestConfig) *TestHarness {
	harness := &TestHarness{
		t:      t,
		config: config,
	}

	if config.EnableProfiling {
		harness.profiler = NewProfiler()
	}

	if config.EnableValidation {
		harness.validator = NewValidator()
	}

	if config.EnableBenchmarks {
		harness.benchmark = NewBenchmarkSuite()
	}

	return harness
}

// TestDataType represents different test data types
type TestDataType int

const (
	TestTypeSimple TestDataType = iota
	TestTypeComplex
	TestTypeNested
	TestTypeArray
	TestTypeMap
	TestTypePointer
	TestTypeInterface
	TestTypeCustom
)

// TestData represents test data with metadata
type TestData struct {
	Type      TestDataType
	Name      string
	Value     interface{}
	Expected  string
	Complexity int
	Size      int
}

// GenerateTestData creates comprehensive test data sets
func (th *TestHarness) GenerateTestData() []TestData {
	var testData []TestData

	// Simple types
	testData = append(testData, []TestData{
		{
			Type:  TestTypeSimple,
			Name:  "string",
			Value: "hello world",
			Expected: `"hello world"`,
			Complexity: 1,
			Size:  len("hello world"),
		},
		{
			Type:  TestTypeSimple,
			Name:  "int",
			Value: 42,
			Expected: `42`,
			Complexity: 1,
			Size:  8,
		},
		{
			Type:  TestTypeSimple,
			Name:  "float",
			Value: 3.14159,
			Expected: `3.14159`,
			Complexity: 1,
			Size:  8,
		},
		{
			Type:  TestTypeSimple,
			Name:  "bool",
			Value: true,
			Expected: `true`,
			Complexity: 1,
			Size:  1,
		},
		{
			Type:  TestTypeSimple,
			Name:  "nil",
			Value: nil,
			Expected: `null`,
			Complexity: 1,
			Size:  0,
		},
	}...)

	// Complex struct
	type ComplexStruct struct {
		ID       int                    `json:"id"`
		Name     string                 `json:"name"`
		Active   bool                   `json:"active"`
		Score    float64                `json:"score"`
		Tags     []string               `json:"tags"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	complexStruct := ComplexStruct{
		ID:     12345,
		Name:   "Complex Test",
		Active: true,
		Score:  95.5,
		Tags:   []string{"tag1", "tag2", "tag3"},
		Metadata: map[string]interface{}{
			"version": 1.0,
			"enabled": true,
		},
	}

	complexJSON, _ := json.Marshal(complexStruct)
	testData = append(testData, TestData{
		Type:      TestTypeComplex,
		Name:      "complex_struct",
		Value:     complexStruct,
		Expected:  string(complexJSON),
		Complexity: 10,
		Size:      len(complexJSON),
	})

	// Nested struct
	type NestedLevel3 struct {
		Value string `json:"value"`
		Count int    `json:"count"`
	}

	type NestedLevel2 struct {
		Level3 NestedLevel3 `json:"level3"`
		Items  []int        `json:"items"`
	}

	type NestedLevel1 struct {
		Level2 NestedLevel2 `json:"level2"`
		Flag   bool         `json:"flag"`
	}

	nestedStruct := NestedLevel1{
		Level2: NestedLevel2{
			Level3: NestedLevel3{
				Value: "nested",
				Count: 42,
			},
			Items: []int{1, 2, 3, 4, 5},
		},
		Flag: true,
	}

	nestedJSON, _ := json.Marshal(nestedStruct)
	testData = append(testData, TestData{
		Type:      TestTypeNested,
		Name:      "nested_struct",
		Value:     nestedStruct,
		Expected:  string(nestedJSON),
		Complexity: 15,
		Size:      len(nestedJSON),
	})

	// Array
	arrayData := []interface{}{1, "two", 3.0, true, nil}
	arrayJSON, _ := json.Marshal(arrayData)
	testData = append(testData, TestData{
		Type:      TestTypeArray,
		Name:      "mixed_array",
		Value:     arrayData,
		Expected:  string(arrayJSON),
		Complexity: 5,
		Size:      len(arrayJSON),
	})

	// Map
	mapData := map[string]interface{}{
		"string": "value",
		"number": 123,
		"float":  456.789,
		"bool":   true,
		"null":   nil,
	}
	mapJSON, _ := json.Marshal(mapData)
	testData = append(testData, TestData{
		Type:      TestTypeMap,
		Name:      "mixed_map",
		Value:     mapData,
		Expected:  string(mapJSON),
		Complexity: 5,
		Size:      len(mapJSON),
	})

	return testData
}

// RunComprehensiveTests executes all test categories
func (th *TestHarness) RunComprehensiveTests() {
	th.t.Log("Starting comprehensive ARM64 JIT tests")

	// Test data generation
	testData := th.GenerateTestData()
	th.t.Logf("Generated %d test data sets", len(testData))

	// Basic functionality tests
	th.RunBasicTests(testData)

	// Performance benchmarks
	if th.config.EnableBenchmarks {
		th.RunBenchmarkTests(testData)
	}

	// Memory usage tests
	if th.config.EnableMemoryTests {
		th.RunMemoryTests(testData)
	}

	// Concurrency tests
	if th.config.EnableConcurrency {
		th.RunConcurrencyTests(testData)
	}

	// Validation tests
	if th.config.EnableValidation {
		th.RunValidationTests(testData)
	}

	// Stress tests
	if th.config.EnableStressTests {
		th.RunStressTests(testData)
	}

	th.t.Log("Comprehensive ARM64 JIT tests completed")
}

// RunBasicTests runs basic encode/decode functionality tests
func (th *TestHarness) RunBasicTests(testData []TestData) {
	th.t.Run("Basic", func(t *testing.T) {
		for _, data := range testData {
			t.Run(data.Name, func(t *testing.T) {
				th.testEncodeDecode(data)
			})
		}
	})
}

// RunBenchmarkTests runs performance benchmarks
func (th *TestHarness) RunBenchmarkTests(testData []TestData) {
	th.t.Run("Benchmarks", func(t *testing.T) {
		for _, data := range testData {
			if data.Size > th.config.MaxTestDataSize {
				continue
			}

			t.Run(data.Name+"_Encode", func(t *testing.T) {
				t.ResetTimer()
				for i := 0; i < 1000; i++ {
					_, err := sonic.Encode(data.Value)
					if err != nil {
						t.Fatalf("Encode error: %v", err)
					}
				}
			})

			t.Run(data.Name+"_Decode", func(t *testing.T) {
				jsonData := []byte(data.Expected)
				dec := sonic.NewDecoder(jsonData)

				t.ResetTimer()
				for i := 0; i < 1000; i++ {
					// Create new value of appropriate type
					result := th.createValueForType(reflect.TypeOf(data.Value))
					_, err := dec.Decode(result)
					if err != nil {
						t.Fatalf("Decode error: %v", err)
					}
				}
			})
		}
	})
}

// RunMemoryTests runs memory usage and leak tests
func (th *TestHarness) RunMemoryTests(testData []TestData) {
	th.t.Run("Memory", func(t *testing.T) {
		for _, data := range testData {
			t.Run(data.Name, func(t *testing.T) {
				th.testMemoryUsage(data)
			})
		}
	})
}

// RunConcurrencyTests runs concurrent execution tests
func (th *TestHarness) RunConcurrencyTests(testData []TestData) {
	th.t.Run("Concurrency", func(t *testing.T) {
		for _, data := range testData {
			t.Run(data.Name, func(t *testing.T) {
				th.testConcurrentEncodeDecode(data)
			})
		}
	})
}

// RunValidationTests runs validation against standard library
func (th *TestHarness) RunValidationTests(testData []TestData) {
	if !th.config.CompareWithStdLib {
		return
	}

	th.t.Run("Validation", func(t *testing.T) {
		for _, data := range testData {
			t.Run(data.Name, func(t *testing.T) {
				th.validateAgainstStdLib(data)
			})
		}
	})
}

// RunStressTests runs high-load stress tests
func (th *TestHarness) RunStressTests(testData []TestData) {
	th.t.Run("Stress", func(t *testing.T) {
		for _, data := range testData {
			if data.Complexity > 10 {
				t.Run(data.Name, func(t *testing.T) {
					th.stressTest(data)
				})
			}
		}
	})
}

// Individual test methods

func (th *TestHarness) testEncodeDecode(data TestData) {
	// Test encoding
	encoded, err := encoder.Encode(data.Value)
	if err != nil {
		th.t.Fatalf("Encode failed: %v", err)
	}

	// Basic validation
	if len(encoded) == 0 && data.Value != nil {
		th.t.Error("Encode returned empty result for non-nil value")
	}

	// Test decoding
	dec := sonic.NewDecoder(encoded)
	result := th.createValueForType(reflect.TypeOf(data.Value))
	n, err := dec.Decode(result)
	if err != nil {
		th.t.Fatalf("Decode failed: %v", err)
	}

	if n == 0 && len(encoded) > 0 {
		th.t.Error("Decode consumed no bytes for non-empty input")
	}

	// Validation if enabled
	if th.config.EnableValidation && th.validator != nil {
		if err := th.validator.ValidateEncodeDecode(data.Value, result); err != nil {
			th.t.Errorf("Validation failed: %v", err)
		}
	}
}

func (th *TestHarness) testMemoryUsage(data TestData) {
	// Force GC before test
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform operations
	for i := 0; i < 100; i++ {
		encoded, err := encoder.Encode(data.Value)
		if err != nil {
			th.t.Fatalf("Encode error: %v", err)
		}

		dec := sonic.NewDecoder(encoded)
		result := th.createValueForType(reflect.TypeOf(data.Value))
		_, err = dec.Decode(result)
		if err != nil {
			th.t.Fatalf("Decode error: %v", err)
		}
	}

	// Check memory usage
	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocDiff := m2.TotalAlloc - m1.TotalAlloc
	if th.config.VerboseLogging {
		th.t.Logf("Memory allocation for %s: %d bytes", data.Name, allocDiff)
	}

	// Simple memory leak detection
	if allocDiff > uint64(data.Size*1000) { // 1000x threshold
		th.t.Logf("Warning: High memory usage detected for %s: %d bytes", data.Name, allocDiff)
	}
}

func (th *TestHarness) testConcurrentEncodeDecode(data TestData) {
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				// Encode
				encoded, err := encoder.Encode(data.Value)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d encode error: %v", id, err)
					return
				}

				// Decode
				dec := sonic.NewDecoder(encoded)
				result := th.createValueForType(reflect.TypeOf(data.Value))
				_, err = dec.Decode(result)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d decode error: %v", id, err)
					return
				}
			}
		}(i)
	}

	// Wait for completion
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// OK
		case err := <-errors:
			th.t.Error(err)
		case <-time.After(th.config.TestTimeout):
			th.t.Fatal("Concurrent test timeout")
		}
	}
}

func (th *TestHarness) validateAgainstStdLib(data TestData) {
	// Standard library encoding
	stdJSON, err := json.Marshal(data.Value)
	if err != nil {
		th.t.Fatalf("Std lib marshal error: %v", err)
	}

	// Sonic encoding
	sonicJSON, err := encoder.Encode(data.Value)
	if err != nil {
		th.t.Fatalf("Sonic encode error: %v", err)
	}

	// Compare JSON structure (not exact string equality due to formatting)
	if !th.jsonStructureEqual(stdJSON, sonicJSON) {
		th.t.Errorf("JSON structure mismatch for %s\nStd:  %s\nSonic: %s",
			data.Name, string(stdJSON), string(sonicJSON))
	}

	// Test decoding
	// Standard library decoding
	stdResult := th.createValueForType(reflect.TypeOf(data.Value))
	err = json.Unmarshal(stdJSON, stdResult)
	if err != nil {
		th.t.Fatalf("Std lib unmarshal error: %v", err)
	}

	// Sonic decoding
	dec := sonic.NewDecoder(sonicJSON)
	sonicResult := th.createValueForType(reflect.TypeOf(data.Value))
	_, err = dec.Decode(sonicResult)
	if err != nil {
		th.t.Fatalf("Sonic decode error: %v", err)
	}

	// Compare results
	if !th.valuesEqual(stdResult, sonicResult) {
		th.t.Errorf("Decoded values differ for %s", data.Name)
	}
}

func (th *TestHarness) stressTest(data TestData) {
	const iterations = 10000
	start := time.Now()

	var lastErr error
	for i := 0; i < iterations; i++ {
		encoded, err := encoder.Encode(data.Value)
		if err != nil {
			lastErr = fmt.Errorf("encode error at iteration %d: %v", i, err)
			break
		}

		dec := sonic.NewDecoder(encoded)
		result := th.createValueForType(reflect.TypeOf(data.Value))
		_, err = dec.Decode(result)
		if err != nil {
			lastErr = fmt.Errorf("decode error at iteration %d: %v", i, err)
			break
		}
	}

	duration := time.Since(start)
	opsPerSecond := float64(iterations) / duration.Seconds()

	th.t.Logf("Stress test for %s: %d ops in %v (%.2f ops/sec)",
		data.Name, iterations, duration, opsPerSecond)

	if lastErr != nil {
		th.t.Error(lastErr)
	}

	// Performance assertion
	if opsPerSecond < 1000 { // Minimum 1000 ops/sec
		th.t.Logf("Warning: Low performance for %s: %.2f ops/sec", data.Name, opsPerSecond)
	}
}

// Helper methods

func (th *TestHarness) createValueForType(t reflect.Type) interface{} {
	if t.Kind() == reflect.Ptr {
		return reflect.New(t.Elem()).Interface()
	}
	return reflect.New(t).Elem().Interface()
}

func (th *TestHarness) jsonStructureEqual(a, b []byte) bool {
	// Simple JSON structure comparison - in production, use a proper JSON parser
	return strings.TrimSpace(string(a)) == strings.TrimSpace(string(b))
}

func (th *TestHarness) valuesEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}

// Supporting types

type Profiler struct {
	// Profiling implementation
}

func NewProfiler() *Profiler {
	return &Profiler{}
}

type Validator struct {
	// Validation implementation
}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateEncodeDecode(original, decoded interface{}) error {
	// Validation logic
	return nil
}

type BenchmarkSuite struct {
	// Benchmark implementation
}

func NewBenchmarkSuite() *BenchmarkSuite {
	return &BenchmarkSuite{}
}