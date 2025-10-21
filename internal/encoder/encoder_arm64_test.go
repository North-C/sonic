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

package encoder

import (
	"encoding/json"
	"reflect"
	"testing"
	"unsafe"

	"github.com/bytedance/sonic/internal/encoder/vars"
	"github.com/bytedance/sonic/internal/jit"
)

func TestNewEncoder(t *testing.T) {
	encoder := NewEncoder()
	if encoder == nil {
		t.Fatal("Expected non-nil encoder")
	}
}

func TestNewStreamEncoder(t *testing.T) {
	writer := &testWriter{}
	streamEncoder := NewStreamEncoder(writer)
	if streamEncoder == nil {
		t.Fatal("Expected non-nil stream encoder")
	}
}

func TestEncode(t *testing.T) {
	// Test basic encoding (should not panic)
	result, err := Encode(42)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != nil {
		t.Error("Expected nil result from placeholder implementation")
	}
}

func TestEncodeToString(t *testing.T) {
	// Test basic string encoding (should not panic)
	result, err := EncodeToString(42)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != "" {
		t.Error("Expected empty string from placeholder implementation")
	}
}

func TestEncodeIndented(t *testing.T) {
	// Test basic indented encoding (should not panic)
	result, err := EncodeIndented(42, "", "")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != nil {
		t.Error("Expected nil result from placeholder implementation")
	}
}

func TestEncodeTypedPointer(t *testing.T) {
	buf := []byte{}
	var val int64 = 42
	vp := unsafe.Pointer(&val)
	sb := vars.NewStack(10)
	fv := uint64(0)

	// This should not panic but will return error for now
	err := EncodeTypedPointer(&buf, vars.Int64Type, vp, sb, fv)
	if err == nil {
		t.Error("Expected error from placeholder implementation")
	}
}

func TestPretouch(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	// This should not panic
	err := Pretouch(reflect.TypeOf(TestStruct{}))
	if err != nil {
		t.Errorf("Expected no error from pretouch, got %v", err)
	}
}

func TestValid(t *testing.T) {
	// Test valid JSON
	validJSON := []byte(`{"name":"test","age":42}`)
	if !Valid(validJSON) {
		t.Error("Expected valid JSON to be recognized as valid")
	}

	// Test invalid JSON
	invalidJSON := []byte(`{"name":"test","age":42`)
	if Valid(invalidJSON) {
		t.Error("Expected invalid JSON to be recognized as invalid")
	}

	// Test empty JSON
	emptyJSON := []byte{}
	if !Valid(emptyJSON) {
		t.Error("Expected empty JSON to be considered valid")
	}
}

func TestValidString(t *testing.T) {
	// Test valid JSON string
	validJSON := `{"name":"test","age":42}`
	valid, err := ValidString(validJSON)
	if err != nil {
		t.Errorf("Expected no error for valid JSON string, got %v", err)
	}
	if !valid {
		t.Error("Expected valid JSON string to be recognized as valid")
	}

	// Test invalid JSON string
	invalidJSON := `{"name":"test","age":42`
	valid, err = ValidString(invalidJSON)
	if err != nil {
		t.Errorf("Expected no error for invalid JSON string, got %v", err)
	}
	if valid {
		t.Error("Expected invalid JSON string to be recognized as invalid")
	}
}

func TestEncodeTypedPointerWithValidInput(t *testing.T) {
	buf := make([]byte, 0, 1024)
	var val string = "test"
	vp := unsafe.Pointer(&val)
	sb := vars.NewStack(10)
	fv := uint64(0)

	// Even with valid input, this should return error for now
	err := EncodeTypedPointer(&buf, vars.StringType, vp, sb, fv)
	if err == nil {
		t.Error("Expected error from placeholder implementation")
	}
}

func TestOptionsConstants(t *testing.T) {
	// Test that all option constants are properly defined
	options := []Options{
		SortMapKeys,
		EscapeHTML,
		CompactMarshaler,
		NoQuoteTextMarshaler,
		NoNullSliceOrMap,
		CopyString,
		ValidateString,
		NoValidateJSONMarshaler,
		NoValidateJSONSkip,
		NoEncoderNewline,
		EncodeNullForInfOrNan,
		CaseSensitive,
	}

	for _, opt := range options {
		// Just test that the constants exist and can be compared
		_ = opt
	}
}

func TestArchitectureInfo(t *testing.T) {
	encoder := NewEncoder()
	stats := Stats(encoder)

	if stats == nil {
		t.Error("Expected non-nil stats")
	}

	if stats["platform"] != "arm64" {
		t.Errorf("Expected platform 'arm64', got '%v'", stats["platform"])
	}

	if stats["jit"] != "enabled" {
		t.Errorf("Expected jit 'enabled', got '%v'", stats["jit"])
	}
}

func TestGetProgram(t *testing.T) {
	encoder := NewEncoder()

	// Before compilation, should return nil
	program := GetProgram(encoder)
	if program != nil {
		t.Error("Expected nil program before compilation")
	}

	// After compilation, program should be non-nil
	// Note: This requires actual compilation to work
	// For now, this is a placeholder test
}

func TestIsOptimized(t *testing.T) {
	encoder := NewEncoder()

	// Initially should not be optimized
	if IsOptimized(encoder) {
		t.Error("Expected encoder to not be optimized initially")
	}

	// After compilation, should be optimized
	// Note: This requires actual compilation to work
	// For now, this is a placeholder test
}

func TestCodeSize(t *testing.T) {
	encoder := NewEncoder()

	// Initially should be 0
	if CodeSize(encoder) != 0 {
		t.Errorf("Expected code size 0 initially, got %d", CodeSize(encoder))
	}

	// After compilation, should be non-zero
	// Note: This requires actual compilation to work
	// For now, this is a placeholder test
}

func TestInstructionCount(t *testing.T) {
	encoder := NewEncoder()

	// Initially should be 0
	if InstructionCount(encoder) != 0 {
		t.Errorf("Expected instruction count 0 initially, got %d", InstructionCount(encoder))
	}

	// After compilation, should be non-zero
	// Note: This requires actual compilation to work
	// For now, this is a placeholder test
}

func TestCompileTime(t *testing.T) {
	encoder := NewEncoder()

	// Initially should be 0
	if CompileTime(encoder) != 0 {
		t.Errorf("Expected compile time 0 initially, got %d", CompileTime(encoder))
	}

	// After compilation, should be non-zero
	// Note: This requires actual compilation to work
	// // For now, this is a placeholder test
}

func TestVerifyCode(t *testing.T) {
	encoder := NewEncoder()

	// Should not panic even before compilation
	err := VerifyCode(encoder)
	if err != nil {
		t.Errorf("Expected no error for verification, got %v", err)
	}

	// Should not panic after compilation
	// Note: This requires actual compilation to work
	// For now, this is a placeholder test
}

func TestDumpCode(t *testing.T) {
	encoder := NewEncoder()

	// Should not panic even before compilation
	code := DumpCode(encoder)
	if code == "" {
		t.Error("Expected non-empty code dump")
	}

	// Should not panic after compilation
	// Note: This requires actual compilation to work
	// For now, this is a placeholder test
}

func TestGetJITOptions(t *testing.T) {
	encoder := NewEncoder()
	opts := GetJITOptions()

	if opts.OptimizationLevel < 0 {
		t.Error("Expected non-negative optimization level")
	}

	if !opts.EnableSIMD {
		t.Error("Expected SIMD to be enabled by default")
	}
}

func TestApplyJITOptions(t *testing.T) {
	encoder := NewEncoder()
	opts := GetJITOptions()
	opts.OptimizationLevel = 2

	// Should not panic
	ApplyJITOptions(encoder, opts)
}

func TestIsJITEnabled(t *testing.T) {
	// Test that JIT status can be checked
	isEnabled := IsJITEnabled()
	if isEnabled != jit.IsARM64JITEnabled() {
		t.Error("IsJITEnabled should match jit.IsARM64JITEnabled")
	}
}

func TestEnableJIT(t *testing.T) {
	// Test that JIT can be enabled/disabled
	originalState := IsJITEnabled()

	EnableJIT()
	if !IsJITEnabled() {
		t.Error("Expected JIT to be enabled after EnableJIT")
	}

	DisableJIT()
	if IsJITEnabled() {
		t.Error("Expected JIT to be disabled after DisableJIT")
	}

	// Restore original state
	if originalState {
		EnableJIT()
	}
}

func TestForceUseVM(t *testing.T) {
	// Test that VM can be forced
	originalState := IsJITEnabled()

	ForceUseVM()
	if IsJITEnabled() {
		t.Error("Expected JIT to be disabled after ForceUseVM")
	}

	// Restore original state
	if originalState {
	EnableJIT()
	}
}

func TestCreateEncoderWithName(t *testing.T) {
	encoder := CreateEncoderWithName("custom_encoder")
	if encoder == nil {
		t.Fatal("Expected non-nil encoder")
	}

	// Test that the name is set (though we can't access it directly)
	stats := Stats(encoder)
	if stats["name"] != "custom_encoder" {
		t.Errorf("Expected name 'custom_encoder', got '%v'", stats["name"])
	}
}

func testWriter struct {
	data []byte
}

func (w *testWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func (w *testWriter) String() string {
	return string(w.data)
}

func (w *testWriter) Reset() {
	w.data = w.data[:0]
}

func TestBatchCompile(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(42),
		reflect.TypeOf("test"),
		reflect.TypeOf(true),
		reflect.TypeOf(3.14),
	}

	results, err := BatchCompile(types)
	if err != nil {
		t.Errorf("Expected no error from batch compile, got %v", err)
	}

	if len(results) != len(types) {
		t.Errorf("Expected %d results, got %d", len(types), len(results))
	}

	for _, vt := range types {
		if _, ok := results[vt]; !ok {
			t.Errorf("Expected result for type %v", vt)
		}
	}
}

func TestBatchCompileWithError(t *testing.T) {
	// Create a type that will definitely cause an error
	type ProblematicType struct {
		InvalidField func()  // This type should cause compilation issues
	}

	types := []reflect.Type{
		reflect.TypeOf(42),
		reflect.TypeOf(ProblematicType{}),
		reflect.TypeOf("test"),
	}

	_, err := BatchCompile(types)
	if err == nil {
		t.Error("Expected error from batch compile with problematic type")
	}
}

func TestEnsureCompatibility(t *testing.T) {
	encoder := NewEncoder()

	// Should not panic for a valid encoder
	err := EnsureCompatibility(encoder)
	if err != nil {
		t.Errorf("Expected no compatibility error, got %v", err)
	}
}

func TestEnsureCompatibilityWithNil(t *testing.T) {
	err := EnsureCompatibility(nil)
	if err == nil {
		t.Error("Expected error for nil encoder")
	}
}

func TestConvertOptions(t *testing.T) {
	// Test converting between option types
	options := Options{
		SortMapKeys:     true,
		EscapeHTML:      true,
		CompactMarshaler: true,
	}

	// This should not panic
	converted := convertOptions(options)
	_ = converted
}

func TestValidateOptions(t *testing.T) {
	// Test validating valid options
	options := Options{
		SortMapKeys:     true,
		EscapeHTML:      true,
		CompactMarshaler: true,
	}

	err := ValidateOptions(options)
	if err != nil {
		t.Errorf("Expected no error for valid options, got %v", err)
	}
}

func TestGetPerfStats(t *testing.T) {
	encoder := NewEncoder()
	stats := GetPerfStats(encoder)

	if stats.CompileTime < 0 {
		t.Error("Expected non-negative compile time")
	}

	if stats.CodeSize < 0 {
		t.Error("Expected non-negative code size")
	}

	if stats.InstructionCount < 0 {
		t.Error("Expected non-negative instruction count")
	}

	if len(stats.Optimizations) == 0 {
		t.Error("Expected some optimizations to be listed")
	}
}

func TestGetDebugInfo(t *testing.T) {
	encoder := NewEncoder()
	info := GetDebugInfo(encoder)

	if info.Program == "" {
		t.Error("Expected non-empty program name")
	}

	if info.Stats == nil {
		t.Error("Expected non-empty stats")
	}
}

func TestGetArchitectureInfo(t *testing.T) {
	info := GetArchitectureInfo()

	if info["platform"] != "arm64" {
		t.Errorf("Expected platform 'arm64', got '%v'", info["platform"])
	}

	if info["jit_enabled"] != true {
		t.Errorf("Expected jit_enabled true, got '%v'", info["jit_enabled"])
	}

	if info["stack_align"] != 16 {
		t.Errorf("Expected stack_align 16, got '%v'", info["stack_align"])
	}

	if info["max_registers"] != 31 {
		t.Errorf("Expected max_registers 31, got '%v'", info["max_registers"])
	}
}

// Benchmark tests
func BenchmarkNewEncoder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		encoder := NewEncoder()
		if encoder == nil {
			b.Fatal("Expected non-nil encoder")
		}
	}
}

func BenchmarkEncode(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encode(42)
		if err != nil {
			b.Fatalf("Encode error: %v", err)
		}
	}
}

func BenchmarkEncodeToString(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EncodeToString(42)
		if err != nil {
			b.Fatalf("EncodeToString error: %v", err)
		}
	}
}

func BenchmarkEncodeIndented(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EncodeIndented(42, "", "")
		if err != nil {
			b.Fatalf("EncodeIndented error: %v", err)
		}
	}
}

func BenchmarkValid(b *testing.B) {
	validJSON := []byte(`{"name":"test","age":42}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Valid(validJSON)
	}
}

func BenchmarkValidString(b *testing.B) {
	validJSON := `{"name":"test","age":42}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ValidString(validJSON)
	}
}

func BenchmarkPretouch(b *testing.B) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	vt := reflect.TypeOf(TestStruct{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := Pretouch(vt)
		if err != nil {
			b.Fatalf("Pretouch error: %v", err)
		}
	}
}

func BenchmarkStats(b *testing.B) {
	encoder := NewEncoder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Stats(encoder)
	}
}

func BenchmarkGetPerfStats(b *testing.B) {
	encoder := NewEncoder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetPerfStats(encoder)
	}
}

func BenchmarkGetDebugInfo(b *testing.B) {
	encoder := NewEncoder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetDebugInfo(encoder)
	}
}