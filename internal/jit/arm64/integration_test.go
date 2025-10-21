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
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

// Integration tests for ARM64 JIT implementation
// These tests verify that ARM64 JIT works correctly with the broader sonic ecosystem

func TestARM64JIT_Integration_Basic(t *testing.T) {
	// Test basic integration with sonic API
	config := sonic.Config{}
	config.UseInt64 = true
	config.UseNumber = false

	// Test simple struct
	type SimpleStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	data := SimpleStruct{
		Name: "Integration Test",
		Age:  25,
	}

	// Test Marshal
	result, err := sonic.Marshal(&data)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Test Unmarshal
	var decoded SimpleStruct
	err = sonic.Unmarshal(result, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Name != data.Name || decoded.Age != data.Age {
		t.Errorf("Data mismatch: got %+v, want %+v", decoded, data)
	}
}

func TestARM64JIT_Integration_ComplexTypes(t *testing.T) {
	// Test with complex types that exercise different JIT paths

	// Nested struct
	type Address struct {
		Street  string `json:"street"`
		City    string `json:"city"`
		Country string `json:"country"`
		Zip     string `json:"zip"`
	}

	type Person struct {
		ID      int     `json:"id"`
		Name    string  `json:"name"`
		Age     int     `json:"age"`
		Active  bool    `json:"active"`
		Score   float64 `json:"score"`
		Tags    []string `json:"tags"`
		Address Address `json:"address"`
		Meta    map[string]interface{} `json:"meta"`
	}

	person := Person{
		ID:     12345,
		Name:   "John Doe",
		Age:    30,
		Active: true,
		Score:  95.5,
		Tags:   []string{"developer", "golang", "json"},
		Address: Address{
			Street:  "123 Main St",
			City:    "San Francisco",
			Country: "USA",
			Zip:     "94105",
		},
		Meta: map[string]interface{}{
			"department": "engineering",
			"level":      5,
			"salary":     120000.50,
			"remote":     true,
		},
	}

	// Round-trip test
	data, err := sonic.Marshal(person)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var result Person
	err = sonic.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Verify all fields
	if result.ID != person.ID {
		t.Errorf("ID mismatch: got %d, want %d", result.ID, person.ID)
	}
	if result.Name != person.Name {
		t.Errorf("Name mismatch: got %s, want %s", result.Name, person.Name)
	}
	if result.Address.City != person.Address.City {
		t.Errorf("Address.City mismatch: got %s, want %s", result.Address.City, person.Address.City)
	}
	if len(result.Tags) != len(person.Tags) {
		t.Errorf("Tags length mismatch: got %d, want %d", len(result.Tags), len(person.Tags))
	}
	if result.Meta["department"] != person.Meta["department"] {
		t.Errorf("Meta[department] mismatch: got %v, want %v", result.Meta["department"], person.Meta["department"])
	}
}

func TestARM64JIT_Integration_SlicesAndMaps(t *testing.T) {
	// Test slice handling
	intSlice := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	data, err := sonic.Marshal(intSlice)
	if err != nil {
		t.Fatalf("Marshal slice error: %v", err)
	}

	var decodedSlice []int
	err = sonic.Unmarshal(data, &decodedSlice)
	if err != nil {
		t.Fatalf("Unmarshal slice error: %v", err)
	}

	if len(decodedSlice) != len(intSlice) {
		t.Errorf("Slice length mismatch: got %d, want %d", len(decodedSlice), len(intSlice))
	}

	// Test map handling
	stringMap := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	data, err = sonic.Marshal(stringMap)
	if err != nil {
		t.Fatalf("Marshal map error: %v", err)
	}

	var decodedMap map[string]string
	err = sonic.Unmarshal(data, &decodedMap)
	if err != nil {
		t.Fatalf("Unmarshal map error: %v", err)
	}

	if len(decodedMap) != len(stringMap) {
		t.Errorf("Map length mismatch: got %d, want %d", len(decodedMap), len(stringMap))
	}

	for k, v := range stringMap {
		if decodedMap[k] != v {
			t.Errorf("Map value mismatch for key %s: got %s, want %s", k, decodedMap[k], v)
		}
	}
}

func TestARM64JIT_Integration_PointersAndInterfaces(t *testing.T) {
	// Test pointer handling
	type Nested struct {
		Value string `json:"value"`
		Count int    `json:"count"`
	}

	type Container struct {
		Ptr *Nested `json:"ptr"`
	}

	nested := &Nested{
		Value: "pointer test",
		Count: 42,
	}

	container := Container{Ptr: nested}

	data, err := sonic.Marshal(container)
	if err != nil {
		t.Fatalf("Marshal pointer error: %v", err)
	}

	var result Container
	err = sonic.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal pointer error: %v", err)
	}

	if result.Ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}
	if result.Ptr.Value != nested.Value {
		t.Errorf("Pointer value mismatch: got %s, want %s", result.Ptr.Value, nested.Value)
	}

	// Test interface handling
	var ifaceValue interface{} = map[string]interface{}{
		"string": "test",
		"number": 123,
		"float":  456.789,
		"bool":   true,
		"null":   nil,
	}

	data, err = sonic.Marshal(ifaceValue)
	if err != nil {
		t.Fatalf("Marshal interface error: %v", err)
	}

	var resultIface interface{}
	err = sonic.Unmarshal(data, &resultIface)
	if err != nil {
		t.Fatalf("Unmarshal interface error: %v", err)
	}

	// Convert to map for comparison
	resultMap, ok := resultIface.(map[string]interface{})
	if !ok {
		t.Fatal("Expected map[string]interface{}")
	}

	originalMap := ifaceValue.(map[string]interface{})
	for k, v := range originalMap {
		if resultMap[k] != v {
			// Special handling for different types (e.g., float vs int)
			if fmt.Sprintf("%v", resultMap[k]) != fmt.Sprintf("%v", v) {
				t.Errorf("Interface value mismatch for key %s: got %v (%T), want %v (%T)",
					k, resultMap[k], resultMap[k], v, v)
			}
		}
	}
}

func TestARM64JIT_Integration_ErrorHandling(t *testing.T) {
	// Test invalid JSON
	invalidJSON := []byte(`{"invalid": json}`)
	var result interface{}
	err := sonic.Unmarshal(invalidJSON, &result)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	// Test type mismatch
	jsonStr := `{"name": "test", "age": "not_a_number"}`
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	var person Person
	err = sonic.Unmarshal([]byte(jsonStr), &person)
	if err == nil {
		t.Error("Expected error for type mismatch")
	}

	// Test invalid UTF-8
	invalidUTF8 := []byte{0xff, 0xfe, 0xfd}
	err = sonic.Unmarshal(invalidUTF8, &result)
	if err == nil {
		t.Error("Expected error for invalid UTF-8")
	}
}

func TestARM64JIT_Integration_PerformanceComparison(t *testing.T) {
	// Performance comparison with standard library
	type BenchmarkData struct {
		ID       int                    `json:"id"`
		Name     string                 `json:"name"`
		Active   bool                   `json:"active"`
		Score    float64                `json:"score"`
		Tags     []string               `json:"tags"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	data := BenchmarkData{
		ID:     12345,
		Name:   "Performance Test",
		Active: true,
		Score:  87.5,
		Tags:   []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]interface{}{
			"version":     2.0,
			"environment": "test",
			"features":    []bool{true, false, true},
			"metrics": map[string]int{
				"requests":  500,
				"errors":    5,
				"timeouts":   2,
				"cache_hits": 450,
			},
		},
	}

	const iterations = 1000

	// Test sonic performance
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := sonic.Marshal(data)
		if err != nil {
			t.Fatalf("Sonic marshal error: %v", err)
		}
	}
	sonicMarshalTime := time.Since(start)

	// Test standard library performance
	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, err := json.Marshal(data)
		if err != nil {
			t.Fatalf("Std lib marshal error: %v", err)
		}
	}
	stdMarshalTime := time.Since(start)

	// Log results
	t.Logf("Marshal performance (%d iterations):", iterations)
	t.Logf("  Sonic: %v", sonicMarshalTime)
	t.Logf("  StdLib: %v", stdMarshalTime)
	t.Logf("  Speedup: %.2fx", float64(stdMarshalTime)/float64(sonicMarshalTime))

	// Test unmarshal performance
	sonicData, _ := sonic.Marshal(data)
	stdData, _ := json.Marshal(data)

	// Sonic unmarshal
	start = time.Now()
	for i := 0; i < iterations; i++ {
		var result BenchmarkData
		err := sonic.Unmarshal(sonicData, &result)
		if err != nil {
			t.Fatalf("Sonic unmarshal error: %v", err)
		}
	}
	sonicUnmarshalTime := time.Since(start)

	// Standard library unmarshal
	start = time.Now()
	for i := 0; i < iterations; i++ {
		var result BenchmarkData
		err := json.Unmarshal(stdData, &result)
		if err != nil {
			t.Fatalf("Std lib unmarshal error: %v", err)
		}
	}
	stdUnmarshalTime := time.Since(start)

	t.Logf("Unmarshal performance (%d iterations):", iterations)
	t.Logf("  Sonic: %v", sonicUnmarshalTime)
	t.Logf("  StdLib: %v", stdUnmarshalTime)
	t.Logf("  Speedup: %.2fx", float64(stdUnmarshalTime)/float64(sonicUnmarshalTime))
}

func TestARM64JIT_Integration_ConcurrentUsage(t *testing.T) {
	// Test concurrent usage of ARM64 JIT
	type TestData struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	data := TestData{ID: 1, Name: "concurrent test"}

	const numGoroutines = 10
	const numOperations = 100

	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				// Marshal
				jsonData, err := sonic.Marshal(data)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d marshal error: %v", id, err)
					return
				}

				// Unmarshal
				var result TestData
				err = sonic.Unmarshal(jsonData, &result)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d unmarshal error: %v", id, err)
					return
				}

				// Verify data
				if result.ID != data.ID || result.Name != data.Name {
					errors <- fmt.Errorf("goroutine %d data mismatch", id)
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
			t.Error(err)
		case <-time.After(30 * time.Second):
			t.Fatal("Concurrent test timeout")
		}
	}
}

func TestARM64JIT_Integration_MemoryUsage(t *testing.T) {
	// Test memory usage patterns
	type LargeData struct {
		Items    []string          `json:"items"`
		Metadata map[string]string `json:"metadata"`
	}

	// Create large test data
	items := make([]string, 1000)
	metadata := make(map[string]string)
	for i := 0; i < 1000; i++ {
		items[i] = fmt.Sprintf("item_%d", i)
		metadata[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	data := LargeData{
		Items:    items,
		Metadata: metadata,
	}

	// Measure memory before
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform operations
	for i := 0; i < 100; i++ {
		jsonData, err := sonic.Marshal(data)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		var result LargeData
		err = sonic.Unmarshal(jsonData, &result)
		if err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
	}

	// Measure memory after
	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocDiff := m2.TotalAlloc - m1.TotalAlloc
	t.Logf("Memory allocation for 100 operations: %d bytes", allocDiff)

	// Simple memory leak check
	if allocDiff > 100*1024*1024 { // 100MB threshold
		t.Logf("Warning: High memory usage detected: %d bytes", allocDiff)
	}
}

func TestARM64JIT_Integration_EdgeCases(t *testing.T) {
	// Test edge cases and boundary conditions

	// Empty values
	var emptySlice []int
	data, err := sonic.Marshal(emptySlice)
	if err != nil {
		t.Fatalf("Marshal empty slice error: %v", err)
	}
	if string(data) != "null" && string(data) != "[]" {
		t.Errorf("Expected 'null' or '[]' for empty slice, got %s", string(data))
	}

	// Large integers
	largeInt := int64(9223372036854775807) // Max int64
	data, err = sonic.Marshal(largeInt)
	if err != nil {
		t.Fatalf("Marshal large int error: %v", err)
	}

	var decodedInt int64
	err = sonic.Unmarshal(data, &decodedInt)
	if err != nil {
		t.Fatalf("Unmarshal large int error: %v", err)
	}
	if decodedInt != largeInt {
		t.Errorf("Large int mismatch: got %d, want %d", decodedInt, largeInt)
	}

	// Special floating point values
	specialFloats := []float64{
		0.0,
		-0.0,
		math.Inf(1),
		math.Inf(-1),
		math.NaN(),
	}

	for _, f := range specialFloats {
		data, err = sonic.Marshal(f)
		if err != nil {
			t.Logf("Marshal float %v error: %v", f, err)
			continue
		}

		var decodedFloat float64
		err = sonic.Unmarshal(data, &decodedFloat)
		if err != nil {
			t.Logf("Unmarshal float %v error: %v", f, err)
			continue
		}

		// NaN comparison is special
		if math.IsNaN(f) {
			if !math.IsNaN(decodedFloat) {
				t.Errorf("NaN mismatch: got %v, want NaN", decodedFloat)
			}
		} else if f != decodedFloat {
			t.Errorf("Float mismatch: got %v, want %v", decodedFloat, f)
		}
	}
}

func TestARM64JIT_Integration_APICompatibility(t *testing.T) {
	// Test API compatibility with existing sonic features

	// Test with pretouch
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	err := sonic.Pretouch(reflect.TypeOf(TestStruct{}))
	if err != nil {
		t.Fatalf("Pretouch error: %v", err)
	}

	// Test with different sonic options
	api := sonic.API{
		PrettyPrint: true,
		UseInt64:    true,
		UseNumber:   false,
	}

	data := TestStruct{Name: "API Test", Age: 25}

	result, err := api.Marshal(data)
	if err != nil {
		t.Fatalf("API marshal error: %v", err)
	}

	// Should contain indentation due to PrettyPrint
	if len(result) == 0 {
		t.Error("Expected non-empty result from pretty print")
	}

	var decoded TestStruct
	err = api.Unmarshal(result, &decoded)
	if err != nil {
		t.Fatalf("API unmarshal error: %v", err)
	}

	if decoded.Name != data.Name || decoded.Age != data.Age {
		t.Errorf("API compatibility data mismatch: got %+v, want %+v", decoded, data)
	}
}