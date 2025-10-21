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
	"reflect"
	"testing"
	"unsafe"

	"github.com/bytedance/sonic/internal/encoder/vars"
	"github.com/bytedance/sonic/internal/jit"
	"github.com/bytedance/sonic/internal/rt"
)

func TestNewEncoder(t *testing.T) {
	encoder := NewEncoder("test_encoder")
	if encoder == nil {
		t.Fatal("Expected non-nil encoder")
	}

	if encoder.name != "test_encoder" {
		t.Errorf("Expected name 'test_encoder', got '%s'", encoder.name)
	}

	if encoder.assembler != nil {
		t.Error("Expected assembler to be nil initially")
	}
}

func TestEncoderCompile(t *testing.T) {
	encoder := NewEncoder("test_compile")

	// Test compiling a basic type
	goType := rt.UnpackType(reflect.TypeOf(42))
	_, err := encoder.Compile(goType)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if encoder.assembler == nil {
		t.Error("Expected assembler to be initialized after compilation")
	}
}

func TestEncoderCompileStruct(t *testing.T) {
	encoder := NewEncoder("test_struct")

	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	goType := rt.UnpackType(reflect.TypeOf(TestStruct{}))
	_, err := encoder.Compile(goType)
	if err != nil {
		t.Errorf("Expected no error for struct compilation, got %v", err)
	}

	if encoder.assembler == nil {
		t.Error("Expected assembler to be initialized after struct compilation")
	}
}

func TestEncoderCompilePointer(t *testing.T) {
	encoder := NewEncoder("test_pointer")

	type TestStruct struct {
		Name string `json:"name"`
	}

	goType := rt.UnpackType(reflect.TypeOf(&TestStruct{}))
	_, err := encoder.Compile(goType)
	if err != nil {
		t.Errorf("Expected no error for pointer compilation, got %v", err)
	}
}

func TestEncoderCompileBasicTypes(t *testing.T) {
	encoder := NewEncoder("test_basic")

	basicTypes := []interface{}{
		true,
		int(42),
		int64(42),
		uint(42),
		uint64(42),
		float32(3.14),
		float64(3.14),
		"test string",
	}

	for i, basic := range basicTypes {
		t.Run(reflect.TypeOf(basic).String(), func(t *testing.T) {
			goType := rt.UnpackType(reflect.TypeOf(basic))
			_, err := encoder.Compile(goType)
			if err != nil {
				t.Errorf("Type %d: Expected no error, got %v", i, err)
			}
		})
	}
}

func TestEncoderCompileMap(t *testing.T) {
	encoder := NewEncoder("test_map")

	goType := rt.UnpackType(reflect.TypeOf(map[string]interface{}{}))
	_, err := encoder.Compile(goType)
	if err != nil {
		t.Errorf("Expected no error for map compilation, got %v", err)
	}
}

func TestEncoderCompileSlice(t *testing.T) {
	encoder := NewEncoder("test_slice")

	goType := rt.UnpackType(reflect.TypeOf([]string{}))
	_, err := encoder.Compile(goType)
	if err != nil {
		t.Errorf("Expected no error for slice compilation, got %v", err)
	}
}

func TestEncoderCompileArray(t *testing.T) {
	encoder := NewEncoder("test_array")

	goType := rt.UnpackType(reflect.TypeOf([5]string{}))
	_, err := encoder.Compile(goType)
	if err != nil {
		t.Errorf("Expected no error for array compilation, got %v", err)
	}
}

func TestEncoderCompileInterface(t *testing.T) {
	encoder := NewEncoder("test_interface")

	goType := rt.UnpackType(reflect.TypeOf((*interface{})(nil)))
	_, err := encoder.Compile(goType)
	if err != nil {
		t.Errorf("Expected no error for interface compilation, got %v", err)
	}
}

func TestPretouch(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	err := Pretouch(reflect.TypeOf(TestStruct{}))
	if err != nil {
		t.Errorf("Expected no error for pretouch, got %v", err)
	}
}

func TestPretouchBasicTypes(t *testing.T) {
	basicTypes := []reflect.Type{
		reflect.TypeOf(true),
		reflect.TypeOf(int(42)),
		reflect.TypeOf(int64(42)),
		reflect.TypeOf(uint(42)),
		reflect.TypeOf(uint64(42)),
		reflect.TypeOf(float32(3.14)),
		reflect.TypeOf(float64(3.14)),
		reflect.TypeOf("test string"),
	}

	for i, basic := range basicTypes {
		t.Run(basic.String(), func(t *testing.T) {
			err := Pretouch(basic)
			if err != nil {
				t.Errorf("Type %d: Expected no error for pretouch, got %v", i, err)
			}
		})
	}
}

func TestEncoderGetProgram(t *testing.T) {
	encoder := NewEncoder("test_program")

	// Before compilation, should return nil
	program := encoder.GetProgram()
	if program != nil {
		t.Error("Expected nil program before compilation")
	}

	// After compilation, should return program
	goType := rt.UnpackType(reflect.TypeOf(42))
	encoder.Compile(goType)
	program = encoder.GetProgram()
	if program == nil {
		t.Error("Expected non-nil program after compilation")
	}
}

func TestEncoderStats(t *testing.T) {
	encoder := NewEncoder("test_stats")

	stats := encoder.Stats()
	if stats == nil {
		t.Error("Expected non-nil stats")
	}

	if stats["platform"] != "arm64" {
		t.Errorf("Expected platform 'arm64', got '%v'", stats["platform"])
	}

	if stats["name"] != "test_stats" {
		t.Errorf("Expected name 'test_stats', got '%v'", stats["name"])
	}

	if stats["jit"] != "enabled" {
		t.Errorf("Expected jit 'enabled', got '%v'", stats["jit"])
	}
}

func TestEncoderReset(t *testing.T) {
	encoder := NewEncoder("test_reset")

	// Compile something
	goType := rt.UnpackType(reflect.TypeOf(42))
	encoder.Compile(goType)

	// Should have assembler after compilation
	if encoder.assembler == nil {
		t.Error("Expected assembler to be initialized")
	}

	// Reset should clear assembler
	encoder.Reset()
	if encoder.assembler != nil {
		t.Error("Expected assembler to be nil after reset")
	}
}

func TestEncoderIsOptimized(t *testing.T) {
	encoder := NewEncoder("test_optimized")

	// Before compilation, should not be optimized
	if encoder.IsOptimized() {
		t.Error("Expected encoder to not be optimized before compilation")
	}

	// After compilation, should be optimized
	goType := rt.UnpackType(reflect.TypeOf(42))
	encoder.Compile(goType)
	if !encoder.IsOptimized() {
		t.Error("Expected encoder to be optimized after compilation")
	}

	// After reset, should not be optimized
	encoder.Reset()
	if encoder.IsOptimized() {
		t.Error("Expected encoder to not be optimized after reset")
	}
}

func TestEncoderCodeSize(t *testing.T) {
	encoder := NewEncoder("test_code_size")

	// Before compilation, should return 0
	if encoder.CodeSize() != 0 {
		t.Errorf("Expected code size 0 before compilation, got %d", encoder.CodeSize())
	}

	// After compilation, should return non-zero
	goType := rt.UnpackType(reflect.TypeOf(42))
	encoder.Compile(goType)
	if encoder.CodeSize() == 0 {
		t.Error("Expected non-zero code size after compilation")
	}
}

func TestEncoderInstructionCount(t *testing.T) {
	encoder := NewEncoder("test_instruction_count")

	// Before compilation, should return 0
	if encoder.InstructionCount() != 0 {
		t.Errorf("Expected instruction count 0 before compilation, got %d", encoder.InstructionCount())
	}

	// After compilation, should return non-zero
	goType := rt.UnpackType(reflect.TypeOf(42))
	encoder.Compile(goType)
	if encoder.InstructionCount() == 0 {
		t.Error("Expected non-zero instruction count after compilation")
	}
}

func TestDefaultJITOptions(t *testing.T) {
	opts := DefaultJITOptions()

	if opts.OptimizationLevel != DefaultOptLevel {
		t.Errorf("Expected optimization level %d, got %d", DefaultOptLevel, opts.OptimizationLevel)
	}

	if !opts.EnableSIMD {
		t.Error("Expected SIMD to be enabled by default")
	}

	if !opts.EnableInlining {
		t.Error("Expected inlining to be enabled by default")
	}

	if opts.DebugMode {
		t.Error("Expected debug mode to be disabled by default")
	}
}

func TestEncoderApplyOptions(t *testing.T) {
	encoder := NewEncoder("test_options")

	opts := DefaultJITOptions()
	opts.OptimizationLevel = 2

	// This should not panic
	encoder.ApplyOptions(opts)
}

func TestEncoderVerifyCode(t *testing.T) {
	encoder := NewEncoder("test_verify")

	// Should not panic even before compilation
	err := encoder.VerifyCode()
	if err != nil {
		t.Errorf("Expected no error for verification, got %v", err)
	}

	// Should not panic after compilation
	goType := rt.UnpackType(reflect.TypeOf(42))
	encoder.Compile(goType)
	err = encoder.VerifyCode()
	if err != nil {
		t.Errorf("Expected no error for verification after compilation, got %v", err)
	}
}

func TestEncoderDumpCode(t *testing.T) {
	encoder := NewEncoder("test_dump")

	// Should not panic even before compilation
	code := encoder.DumpCode()
	if code == "" {
		t.Error("Expected non-empty code dump")
	}

	// Should not panic after compilation
	goType := rt.UnpackType(reflect.TypeOf(42))
	encoder.Compile(goType)
	code = encoder.DumpCode()
	if code == "" {
		t.Error("Expected non-empty code dump after compilation")
	}
}

func TestEncoderCompileTime(t *testing.T) {
	encoder := NewEncoder("test_compile_time")

	// Before compilation, should return 0
	if encoder.CompileTime() != 0 {
		t.Errorf("Expected compile time 0 before compilation, got %d", encoder.CompileTime())
	}

	// After compilation, should return some time
	goType := rt.UnpackType(reflect.TypeOf(42))
	encoder.Compile(goType)
	time := encoder.CompileTime()
	if time < 0 {
		t.Errorf("Expected non-negative compile time, got %d", time)
	}
}

func TestGenerateIRProgram(t *testing.T) {
	// Test basic type generation
	program, err := generateIRProgram(reflect.TypeOf(42))
	if err != nil {
		t.Errorf("Expected no error for basic type generation, got %v", err)
	}

	if len(program) == 0 {
		t.Error("Expected non-empty program for basic type")
	}

	// Test struct type generation
	type TestStruct struct {
		Name string `json:"name"`
	}

	program, err = generateIRProgram(reflect.TypeOf(TestStruct{}))
	if err != nil {
		t.Errorf("Expected no error for struct type generation, got %v", err)
	}

	if len(program) == 0 {
		t.Error("Expected non-empty program for struct type")
	}
}

func TestCompileStruct(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	program := compileStruct(reflect.TypeOf(TestStruct{}))
	if len(program) == 0 {
		t.Error("Expected non-empty program for struct compilation")
	}

	// Should start with opening brace
	if program[0].Op != ir.OP_byte || program[0].Vi != '{' {
		t.Error("Expected program to start with opening brace")
	}

	// Should end with closing brace
	if program[len(program)-1].Op != ir.OP_byte || program[len(program)-1].Vi != '}' {
		t.Error("Expected program to end with closing brace")
	}
}

func TestCompileInteger(t *testing.T) {
	tests := []struct {
		vt       reflect.Type
		expected ir.OpCode
	}{
		{reflect.TypeOf(int(0)), ir.OP_i64},
		{reflect.TypeOf(int8(0)), ir.OP_i8},
		{reflect.TypeOf(int16(0)), ir.OP_i16},
		{reflect.TypeOf(int32(0)), ir.OP_i32},
		{reflect.TypeOf(int64(0)), ir.OP_i64},
	}

	for _, tt := range tests {
		t.Run(tt.vt.String(), func(t *testing.T) {
			program := compileInteger(tt.vt)
			if len(program) != 1 {
				t.Errorf("Expected 1 instruction, got %d", len(program))
			}
			if program[0].Op != tt.expected {
				t.Errorf("Expected operation %v, got %v", tt.expected, program[0].Op)
			}
		})
	}
}

func TestCompileUnsigned(t *testing.T) {
	tests := []struct {
		vt       reflect.Type
		expected ir.OpCode
	}{
		{reflect.TypeOf(uint(0)), ir.OP_u64},
		{reflect.TypeOf(uint8(0)), ir.OP_u8},
		{reflect.TypeOf(uint16(0)), ir.OP_u16},
		{reflect.TypeOf(uint32(0)), ir.OP_u32},
		{reflect.TypeOf(uint64(0)), ir.OP_u64},
	}

	for _, tt := range tests {
		t.Run(tt.vt.String(), func(t *testing.T) {
			program := compileUnsigned(tt.vt)
			if len(program) != 1 {
				t.Errorf("Expected 1 instruction, got %d", len(program))
			}
			if program[0].Op != tt.expected {
				t.Errorf("Expected operation %v, got %v", tt.expected, program[0].Op)
			}
		})
	}
}

func TestCompileFloat(t *testing.T) {
	tests := []struct {
		vt       reflect.Type
		expected ir.OpCode
	}{
		{reflect.TypeOf(float32(0)), ir.OP_f32},
		{reflect.TypeOf(float64(0)), ir.OP_f64},
	}

	for _, tt := range tests {
		t.Run(tt.vt.String(), func(t *testing.T) {
			program := compileFloat(tt.vt)
			if len(program) != 1 {
				t.Errorf("Expected 1 instruction, got %d", len(program))
			}
			if program[0].Op != tt.expected {
				t.Errorf("Expected operation %v, got %v", tt.expected, program[0].Op)
			}
		})
	}
}

func TestPtoenc(t *testing.T) {
	// Test that ptoenc doesn't panic
	code := jit.Code{}
	encoder := ptoenc(code)

	if encoder.Encode == nil {
		t.Error("Expected non-nil Encode function")
	}

	if encoder.EncodeToString == nil {
		t.Error("Expected non-nil EncodeToString function")
	}

	if encoder.EncodeIndented == nil {
		t.Error("Expected non-nil EncodeIndented function")
	}
}

func TestGlobalEncodeFunctions(t *testing.T) {
	// Test that global encode functions don't panic
	result, err := Encode(42)
	if err != nil {
		t.Errorf("Expected no error from global Encode, got %v", err)
	}
	if result != nil {
		t.Error("Expected nil result from placeholder implementation")
	}

	str, err := EncodeToString(42)
	if err != nil {
		t.Errorf("Expected no error from global EncodeToString, got %v", err)
	}
	if str != "" {
		t.Error("Expected empty string from placeholder implementation")
	}

	result, err = EncodeIndented(42, "", "")
	if err != nil {
		t.Errorf("Expected no error from global EncodeIndented, got %v", err)
	}
	if result != nil {
		t.Error("Expected nil result from placeholder implementation")
	}
}

func TestConstants(t *testing.T) {
	if StackAlignment != 16 {
		t.Errorf("Expected StackAlignment = 16, got %d", StackAlignment)
	}

	if MaxRegisters != 31 {
		t.Errorf("Expected MaxRegisters = 31, got %d", MaxRegisters)
	}

	if StackHeaderSize != 16 {
		t.Errorf("Expected StackHeaderSize = 16, got %d", StackHeaderSize)
	}

	if DefaultOptLevel != 1 {
		t.Errorf("Expected DefaultOptLevel = 1, got %d", DefaultOptLevel)
	}
}

// Benchmark tests
func BenchmarkNewEncoder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		encoder := NewEncoder("benchmark")
		if encoder == nil {
			b.Fatal("Expected non-nil encoder")
		}
	}
}

func BenchmarkEncoderCompile(b *testing.B) {
	encoder := NewEncoder("benchmark_compile")
	goType := rt.UnpackType(reflect.TypeOf(42))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encoder.Compile(goType)
		if err != nil {
			b.Fatalf("Compilation error: %v", err)
		}
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

func BenchmarkGenerateIRProgram(b *testing.B) {
	vt := reflect.TypeOf(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generateIRProgram(vt)
		if err != nil {
			b.Fatalf("IR generation error: %v", err)
		}
	}
}

func BenchmarkCompileStruct(b *testing.B) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
		Valid bool   `json:"valid"`
	}
	vt := reflect.TypeOf(TestStruct{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		program := compileStruct(vt)
		if len(program) == 0 {
			b.Fatal("Expected non-empty program")
		}
	}
}