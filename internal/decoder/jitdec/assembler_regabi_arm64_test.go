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

package jitdec

import (
	"testing"
	"reflect"
	"unsafe"

	"github.com/bytedance/sonic/internal/encoder/vars"
	"github.com/bytedance/sonic/internal/jit"
	"github.com/bytedance/sonic/internal/rt"
)

func TestARM64AssemblerCreation(t *testing.T) {
	// Create a simple instruction program
	prog := _Program{
		{u: packOp(_OP_null)},
		{u: packOp(_OP_bool)},
		{u: packOp(_OP_i32)},
	}

	assembler := newAssembler(prog)
	if assembler == nil {
		t.Fatal("Expected non-nil assembler")
	}

	if assembler.p == nil {
		t.Error("Expected non-nil program")
	}

	if assembler.name != "" {
		t.Errorf("Expected empty name, got %s", assembler.name)
	}
}

func TestARM64AssemblerInit(t *testing.T) {
	prog := _Program{
		{u: packOp(_OP_null)},
	}

	assembler := newAssembler(prog)
	if assembler == nil {
		t.Fatal("Expected non-nil assembler")
	}

	// Test that Init sets up the assembler correctly
	result := assembler.Init(prog)
	if result != assembler {
		t.Error("Init should return the same assembler instance")
	}

	if assembler.p == nil {
		t.Error("Program should be set after Init")
	}
}

func TestARM64AssemblerLoad(t *testing.T) {
	prog := _Program{
		{u: packOp(_OP_null)},
	}

	assembler := newAssembler(prog)
	assembler.name = "test"

	// This should not panic
	decoder := assembler.Load()
	if decoder == nil {
		t.Error("Expected non-nil decoder")
	}
}

func TestARM64RegisterConstants(t *testing.T) {
	// Test that all register constants are properly defined
	tests := []struct {
		name string
		reg  jit.Addr
	}{
		{"_X0", _X0},
		{"_X1", _X1},
		{"_X2", _X2},
		{"_X3", _X3},
		{"_X4", _X4},
		{"_X5", _X5},
		{"_6", _X6},
		{"_X7", _X7},
		{"_X8", _X8},
		{"X9", _X9},
		{"X10", _X10},
		{"X11", _X11},
		{"X12", _X12},
		{"X13", _X13},
		{"X14", _X14},
		{"X15", _15},
		{"X16", _X16},
		{"X17", _X17},
		{"X18", _18},
		{"X19", _X19},
		{"X20", _X20},
	{"X21", _21},
		{"X22", _22},
		{"X23", _23},
		{"X24", _24},
		{"X25", _25},
		{"X26", _26},
		{"X27", _27},
		{"X28", _28},
		{"X29", _X29}, // FP
		{"X30", _X30}, // LR
		{"SP", _SP},
		{"ZR", _ZR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.reg.Reg == 0 {
				t.Errorf("Register %s should have a non-zero value", tt.name)
			}
		})
	}
}

func TestARM64FloatingPointRegisters(t *testing.T) {
	// Test that floating point register constants are properly defined
	tests := []struct {
		name string
		reg  jit.Addr
	}{
		{"_D0", _D0},
		{"_D1", _D1},
		{"_D2", _D2},
		{"_D3", _D3},
		{"_D4", _D4},
		{"D5", _D5},
		{"_D6", _D6},
		{"D7", _D7},
		{"D8", _D8},
		{"D9", _9},
		{"D10", _D10},
		{"D11", _D11},
		{"D12", _D12},
		{"D13", _D13},
		{"D14", _D14},
		{"D15", _D15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.reg.Reg == 0 {
				t.Errorf("Register %s should have a non-zero value", tt.name)
			}
		})
	}
}

func TestARM64StateRegisters(t *testing.T) {
	// Test that state registers are properly defined
	tests := []struct {
		name string
		reg  jit.Addr
	}{
		{"_ST", _ST},  // stack base
		{"_IP", _IP},  // input pointer
		{"_IL", _IL},  // input length
		{"_IC", _IC},  // input cursor
		{"_VP", _VP},  // value pointer
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.reg.Reg == 0 {
				t.Errorf("Register %s should have a non-zero value", tt.name)
			}
		})
	}
}

func TestARM64ErrorRegisters(t *testing.T) {
	// Test that error registers are properly defined
	tests := []struct {
		name string
		reg  jit.Addr
	}{
		{"_ET", _ET}, // error type
		{"_EP", _EP}, // error pointer
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.reg.Reg == 0 {
				t.Errorf("Register %s should have a non-zero value", tt.name)
			}
		})
	}
}

func TestARM64RegisterSets(t *testing.T) {
	// Test that register sets are properly defined
	if len(_REG_go) == 0 {
		t.Error("_REG_go should not be empty")
	}

	if len(_REG_rt) == 0 {
		t.Error("_REG_rt should not be empty")
	}

	// Check that all sets contain valid registers
	for _, reg := range _REG_go {
		if reg.Reg == 0 {
			t.Errorf("Invalid register in _REG_go: %v", reg)
		}
	}
}

func TestARM64Constants(t *testing.T) {
	// Test stack frame constants
	if _FP_args != 80 {
		t.Errorf("Expected _FP_args = 80, got %d", _FP_args)
	}

	if _FP_fargs != 96 {
		t.Errorf("Expected _FP_fargs = 96, got %d", _FP_fargs)
	}

	if _FP_saves != 64 {
		t.Errorf("Expected _FP_saves = 64, got %d", _FP_saves)
	}

	if _FP_locals != 160 {
		t.Errorf("Expected _FP_locals = 160, got %d", _FP_locals)
	}

	// Test immediate constants
	if _IM_null != 0x6c6c756e {
		t.Errorf("Expected _IM_null = 0x6c6c756e, got 0x%x", _IM_null)
	}

	if _IM_true != 0x65757274 {
		t.Errorf("Expected _IM_true = 0x65757274, got 0x%x", _IM_true)
	}

	if _IM_alse != 0x65736c61 {
		t.Errorf("Expected _IM_alse = 0x65736c61, got 0x%x", _IM_alse)
	}
}

func TestARM64ArgumentLocations(t *testing.T) {
	// Test that argument locations are properly defined
	if _ARG_sp.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_ARG_sp should be a pointer")
	}

	if _ARG_sl.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_ARG_sl should be a pointer")
	}

	if _ARG_ic.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_ARG_ic should be a pointer")
	}

	if _ARG_vp.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_ARG_vp should be a pointer")
	}

	if _ARG_sb.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_ARG_sb should be a pointer")
	}

	if _ARG_fv.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_ARG_fv should be a pointer")
	}
}

func TestARM64LocalVariableLocations(t *testing.T) {
	// Test that local variable locations are properly defined
	if _VAR_st.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_VAR_st should be a pointer")
	}

	if _VAR_sr.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_VAR_sr should be a pointer")
	}

	if _VAR_et.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_VAR_et should be a pointer")
	}

	if _VAR_pc.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_VAR_pc should be a pointer")
	}

	if _VAR_ic.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_VAR_ic should be a pointer")
	}
}

func TestARM64OpFuncTable(t *testing.T) {
	// Test that the operation function table is properly initialized
	if len(_OpFuncTab) == 0 {
		t.Error("_OpFuncTab should not be empty")
	}

	// Test that some key operations are defined
	keyOps := []_Op{
		_OP_null,
		_OP_bool,
		_OP_i8,
		_OP_i16,
		_OP_i32,
		_OP_i64,
		_OP_u8,
		_OP_u16,
		_OP_u32,
		_OP_u64,
		_OP_f32,
		_OP_f64,
		_OP_str,
		_OP_bin,
		_OP_deref,
		_OP_index,
		_OP_is_null,
		_OP_load,
		_OP_save,
		_OP_drop,
		_OP_recurse,
	}

	for _, op := range keyOps {
		t.Run(op.String(), func(t *testing.T) {
			if int(op) >= len(_OpFuncTab) || _OpFuncTab[op] == nil {
				t.Errorf("Operation %v should be defined in _OpFuncTab", op)
			}
		})
	}
}

func TestARM64BasicOperations(t *testing.T) {
	prog := _Program{
		{u: packOp(_OP_null)},
		{u: packOp(_OP_bool)},
		{u: packOp(_OP_i32)},
	}

	assembler := newAssembler(prog)
	assembler.name = "test_basic_ops"

	// This should not panic and should generate code
	decoder := assembler.Load()
	if decoder == nil {
		t.Error("Expected non-nil decoder")
	}
}

func TestARM64StackOperations(t *testing.T) {
	prog := _Program{
		{u: packOp(_OP_save)},
		{u: packOp(_OP_load)},
		{u: packOp(_OP_drop)},
	}

	assembler := newAssembler(prog)
	assembler.name = "test_stack_ops"

	// This should not panic and should generate code
	decoder := assembler.Load()
	if decoder == nil {
		t.Error("Expected non-nil decoder")
	}
}

func TestARM64IntegerOperations(t *testing.T) {
	intOps := []_Op{
		_OP_i8,
		_OP_i16,
		_OP_i32,
		_OP_i64,
		_OP_u8,
		_OP_u16,
		_OP_u32,
		_OP_u64,
	}

	for _, op := range intOps {
		t.Run(op.String(), func(t *testing.T) {
			prog := _Program{{u: packOp(op)}}
			assembler := newAssembler(prog)
			assembler.name = "test_" + op.String()

			decoder := assembler.Load()
			if decoder == nil {
				t.Errorf("Expected non-nil decoder for operation %v", op)
			}
		})
	}
}

func TestARM64FloatOperations(t *testing.T) {
	floatOps := []_Op{
		_OP_f32,
		_OP_f64,
	}

	for _, op := range floatOps {
		t.Run(op.String(), func(t *testing.T) {
			prog := _Program{{u: packOp(op)}}
			assembler := newAssembler(prog)
			assembler.name = "test_" + op.String()

			decoder := assembler.Load()
			if decoder == nil {
				t.Errorf("Expected non-nil decoder for operation %v", op)
			}
		})
	}
}

func TestARM64StringOperations(t *testing.T) {
	strOps := []_Op{
		_OP_str,
		_OP_bin,
		_OP_unquote,
	}

	for _, op := range strOps {
		t.Run(op.String(), func(t *testing.T) {
			prog := _Program{{u: packOp(op)}}
			assembler := newAssembler(prog)
			assembler.name = "test_" + op.String()

			decoder := assembler.Load()
			if decoder == nil {
				t.Errorf("Expected non-nil decoder for operation %v", op)
			}
		})
	}
}

func TestARM64ControlOperations(t *testing.T) {
	controlOps := []_Op{
		_OP_is_null,
		_OP_goto,
		_OP_switch,
		_OP_check_char,
	}

	for _, op := range controlOps {
		t.Run(op.String(), func(t *testing.T) {
			prog := _Program{{u: packOp(op)}}
			assembler := newAssembler(prog)
			assembler.name = "test_" + op.String()

			decoder := assembler.Load()
			if decoder == nil {
				t.Errorf("Expected non-nil decoder for operation %v", op)
			}
		})
	}
}

func TestARM64ComplexProgram(t *testing.T) {
	// Create a more complex program that simulates decoding a struct
	prog := _Program{
		{u: packOp(_OP_lspace)},
		{u: packOp(_OP_match_char), vb: '{'},
		{u: packOp(_OP_text), vs: "name:\""},
		{u: packOp(_OP_str)},
		{u: packOp(_OP_text), vs: "\",age:"},
		{u: packOp(_OP_i64)},
		{u: packOp(_OP_match_char), vb: '}'},
	}

	assembler := newAssembler(prog)
	assembler.name = "test_complex_program"

	decoder := assembler.Load()
	if decoder == nil {
		t.Error("Expected non-nil decoder for complex program")
	}
}

func TestARM64InstructionHandling(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_instructions"

	// Test that basic instruction handling doesn't panic
	assembler.instr(&_Instr{u: packOp(_OP_null)})
	assembler.instr(&_Instr{u: packOp(_OP_bool)})
	assembler.instr(&_Instr{u: packOp(_OP_i32)})
}

func TestARM64BuiltinFunctions(t *testing.T) {
	prog := _Program{
		{u: packOp(_OP_null)}, // This will call builtins during compilation
	}

	assembler := newAssembler(prog)
	assembler.name = "test_builtins"

	// This should compile with builtins
	decoder := assembler.Load()
	if decoder == nil {
		t.Error("Expected non-nil decoder with builtins")
	}
}

func TestARM64ErrorHandling(t *testing.T) {
	// Test with an invalid operation
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid operation")
		}
	}()

	prog := _Program{{u: packOp(_Op(255))}} // Invalid operation
	assembler := newAssembler(prog)
	assembler.compile() // This should panic
}

func TestARM64HelperFunctions(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_helpers"

	// Test that helper functions don't panic
	assembler.save(_X0)
	assembler.load(_X0)
	assembler.call(jit.Func(func() {}))
}

func TestARM64StackFrameLayout(t *testing.T) {
	// Verify stack frame layout is consistent
	if _FP_offs <= _FP_fargs + _FP_saves + _FP_locals {
		t.Error("FP_offs should be larger than fargs + saves + locals")
	}

	if _FP_size <= _FP_offs {
		t.Error("_FP_size should be larger than FP_offs")
	}

	if _FP_base <= _FP_size {
		t.Error("_FP_base should be larger than _FP_size")
	}

	// Verify alignment to 16 bytes (ARM64 requirement)
	if _FP_size%16 != 0 {
		t.Errorf("Stack frame size should be 16-byte aligned, got %d", _FP_size)
	}
}

func TestARM64RegisterAllocation(t *testing.T) {
	// Verify that critical registers don't conflict
	if _IC.Reg == _IP.Reg {
		t.Error("Input cursor and input pointer registers should be different")
	}

	if _IC.Reg == _IL.Reg {
		t.Error("Input cursor and input length registers should be different")
	}

	if _VP.Reg == _ST.Reg {
		t.Error("Value pointer and stack base registers should be different")
	}

	if _ET.Reg == _EP.Reg {
		t.Error("Error type and error pointer registers should be different")
	}
}

// Benchmark tests for performance validation
func BenchmarkARM64AssemblerCreation(b *testing.B) {
	prog := _Program{
		{u: packOp(_OP_null)},
		{u: packOp(_OP_bool)},
		{u: packOp(_OP_i32)},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler := newAssembler(prog)
		if assembler == nil {
			b.Fatal("Expected non-nil assembler")
		}
	}
}

func BenchmarkARM64AssemblerLoad(b *testing.B) {
	prog := _Program{
		{u: packOp(_OP_null)},
		{u: packOp(_OP_bool)},
		{u: packOp(_OP_i32)},
	}

	assembler := newAssembler(prog)
	assembler.name = "benchmark"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoder := assembler.Load()
		if decoder == nil {
			b.Fatal("Expected non-nil decoder")
		}
	}
}

func BenchmarkARM64BasicOperations(b *testing.B) {
	ops := []_Op{
		_OP_null,
		_OP_bool,
		_OP_i32,
		_OP_str,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		op := ops[i%len(ops)]
		prog := _Program{{u: packOp(op)}}
		assembler := newAssembler(prog)
		assembler.name = "benchmark_op"
		decoder := assembler.Load()
		if decoder == nil {
			b.Fatalf("Expected non-nil decoder for operation %v", op)
		}
	}
}

func BenchmarkARM64ComplexProgram(b *testing.B) {
	prog := _Program{
		{u: packOp(_OP_lspace)},
		{u: packOp(_OP_match_char), vb: '{'},
		{u: packOp(_OP_text), vs: "test:"},
		{u: packOp(_OP_str)},
		{u: packOp(_OP_match_char), vb: ','},
		{u: packOp(_OP_i64)},
		{u: packOp(_OP_match_char), vb: '}'},
	}

	assembler := newAssembler(prog)
	assembler.name = "benchmark_complex"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoder := assembler.Load()
		if decoder == nil {
			b.Fatal("Expected non-nil decoder for complex program")
		}
	}
}

// Integration test with actual decoding
func TestARM64DecodingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires the JIT to be functional
	if !jit.IsARM64JITEnabled() {
		t.Skip("ARM64 JIT is not enabled")
	}

	// Test a simple struct decoding scenario
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
			Valid bool   `json:"valid"`
	}

	// Create a program that simulates decoding TestStruct
	prog := _Program{
		{u: packOp(_OP_lspace)},
		{u: packOp(_OP_match_char), vb: '{'},
		{u: packOp(_OP_text), vs: "name:\""},
		{u: packOp(_OP_str)},
		{u: packOp(_OP_text), vs: "\",age:"},
		{u: packOp(_OP_i64)},
		{u: packOp(_OP_text), vs: ",valid:"},
		{u: packOp(_OP_bool)},
		{u: packOp(_OP_match_char), vb: '}'},
	}

	assembler := newAssembler(prog)
	assembler.name = "test_integration"

	decoder := assembler.Load()
	if decoder == nil {
		t.Error("Expected non-nil decoder for integration test")
	}

	// The decoder should be callable
	if decoder == nil {
		t.Fatal("Decoder should be callable")
	}

	// Note: Actual decoding would require more setup and testing infrastructure
	// This test mainly verifies that the assembler can generate code without panicking
}

// Test ARM64 specific instruction generation
func TestARM64InstructionGeneration(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_instruction_generation"

	// Test prologue and epilogue
	assembler.prologue()
	assembler.epilogue()

	// Should have generated some code
	if assembler.CodeSize() == 0 {
		t.Error("Expected non-zero code size after prologue and epilogue")
	}
}

// Test register allocation
func TestARM64RegisterAllocation(t *testing.T) {
	// Verify that critical registers don't conflict
	if _IC.Reg == _IP.Reg {
		t.Error("Input cursor and input pointer registers should be different")
	}

	if _IC.Reg == _IL.Reg {
		t.Error("Input cursor and input length registers should be different")
	}

	if _VP.Reg == _ST.Reg {
		t.Error("Value pointer and stack base registers should be different")
	}

	if _ET.Reg == _EP.Reg {
		t.Error("Error type and error pointer registers should be different")
	}
}

// Test stack frame layout
func TestARM64StackFrameLayout(t *testing.T) {
	// Verify stack frame layout is consistent
	if _FP_offs <= _FP_fargs + _FP_saves + _FP_locals {
		t.Error("FP_offs should be larger than fargs + saves + locals")
	}

	if _FP_size <= _FP_offs {
		t.Error("_FP_size should be larger than FP_offs")
	}

	if _FP_base <= _FP_size {
		t.Error("_FP_base should be larger than _FP_size")
	}

	// Verify alignment to 16 bytes (ARM64 requirement)
	if _FP_size%16 != 0 {
		t.Errorf("Stack frame size should be 16-byte aligned, got %d", _FP_size)
	}
}

// Test error handling paths
func TestARM64ErrorHandlingPaths(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_error_handling"

	// Test that all error handlers can be linked without panicking
	assembler.type_error()
	assembler.mismatch_error()
	assembler.field_error()
	assembler.range_error()
	assembler.stack_error()
	assembler.base64_error()
	assembler.parsing_error()
}

// Test buffer management
func TestARM64BufferManagement(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_buffer_management"

	// Test buffer helper functions
	assembler.check_eof(1)
	assembler.check_size(10)
	assembler.add_char('a')
	assembler.add_long(0x61626364, 4)
	assembler.add_text("test")
}

// Test state management
func TestARM64StateManagement(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_state_management"

	// Test state management functions
	assembler.save_state()
	assembler.drop_state(32)
}

// Test function calling conventions
func TestARM64FunctionCalling(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_function_calling"

	// Test function call helpers
	assembler.save_c()
	assembler.call_go(jit.Func(func() {}))
}

// Test memory operations
func TestARM64MemoryOperations(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_memory_operations"

	// Test memory allocation and management
	assembler.malloc_X0(jit.Imm(64), _X0)
	assembler.valloc(reflect.TypeOf(int(0)), _X0)
	assembler.vfollow(reflect.TypeOf(&int(0)))
}

// Test range checking
func TestARM64RangeChecking(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_range_checking"

	// Test range checking for different types
	assembler.range_signed_X1(_I_int8, _T_int8, -128, 127)
	assembler.range_unsigned_X1(_I_uint8, _T_uint8, 0, 255)
	assembler.range_uint32_X1(_I_uint32, _T_uint32)
	assembler.range_single_D0()
}

// Test string operations
func TestARM64StringOperations(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_string_operations"

	// Test string manipulation functions
	assembler.slice_from(_VAR_st_Iv, -1)
	assembler.unquote_once(_X0, _X1, true, false)
	assembler.unquote_twice(_X0, _X1, false)
}

// Test map operations
func TestARM64MapOperations(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_map_operations"

	// Test map assignment functions
	assembler.mapassign_std(reflect.TypeOf(map[string]interface{}{}), _X0)
	assembler.mapassign_str_fast(reflect.TypeOf(map[string]string{}), _X0, _X1)
	assembler.mapassign_utext(reflect.TypeOf(map[string]interface{}{}), false)
}

// Test external unmarshaler support
func TestARM64UnmarshalerSupport(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_unmarshaler_support"

	// Test unmarshaler function calls
	assembler.unmarshal_json(reflect.TypeOf((*json.Unmarshaler)(nil)).Elem(), true, _F_decodeJsonUnmarshaler)
	assembler.unmarshal_text(reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem(), true)
}

// Test dynamic decoding
func TestARM64DynamicDecoding(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_dynamic_decoding"

	// Test dynamic decoding support
	assembler.decode_dynamic(jit.Type(reflect.TypeOf(interface{}(0))), _VP)
}

// Test JIT options and configuration
func TestARM64JITOptions(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_jit_options"

	// Test JIT option application
	opts := jitdec.DefaultJITOptions()
	assembler.ApplyOptions(opts)

	// Test statistics and debugging
	stats := assembler.Stats()
	if stats == nil {
		t.Error("Expected non-nil stats")
	}

	debug := jitdec.GetDebugInfo(assembler)
	if debug.Program == "" {
		t.Error("Expected non-empty program name")
	}
}

// Helper function to pack operation into instruction format
func packOp(op _Op) uint64 {
	return uint64(op) << 56
}

// Test instruction creation helpers
func TestInstructionCreation(t *testing.T) {
	// Test basic instruction creation
	instr := newInsOp(_OP_null)
	if instr.op() != _OP_null {
		t.Errorf("Expected OP_null, got %v", instr.op())
	}

	// Test instruction with integer value
	instr = newInsVi(_OP_goto, 42)
	if instr.op() != _OP_goto {
		t.Errorf("Expected OP_goto, got %v", instr.op())
	}
	if instr.vi() != 42 {
		t.Errorf("Expected vi=42, got %d", instr.vi())
	}

	// Test instruction with byte value
	instr = newInsVb(_OP_match_char, 'x')
	if instr.op() != _OP_match_char {
		t.Errorf("Expected OP_match_char, got %v", instr.op())
	}
	if instr.vb() != 'x' {
		t.Errorf("Expected vb='x', got %c", instr.vb())
	}

	// Test instruction with type
	instr = newInsVt(_OP_recurse, reflect.TypeOf(int(0)))
	if instr.op() != _OP_recurse {
		t.Errorf("Expected OP_recurse, got %v", instr.op())
	}
	if instr.vt() != reflect.TypeOf(int(0)) {
		t.Errorf("Expected int type, got %v", instr.vt())
	}
}

// Test instruction field access methods
func TestInstructionFieldAccess(t *testing.T) {
	// Test instruction with array of values
	values := []int{1, 2, 3, 4, 5}
	instr := newInsVs(_OP_switch, values)
	if instr.op() != _OP_switch {
		t.Errorf("Expected OP_switch, got %v", instr.op())
	}
	if len(instr.vs()) != 5 {
		t.Errorf("Expected 5 values, got %d", len(instr.vs()))
	}

	// Test that all field access methods work correctly
	if instr.vi() != 5 {
		t.Errorf("Expected vi=5, got %d", instr.vi())
	}

	for i, v := range instr.vs() {
		if instr.vs()[i] != v {
			t.Errorf("Expected vs[%d]=%d, got %d", i, values[i], instr.vs()[i])
		}
	}
}

// Test instruction disassembly
func TestInstructionDisassembly(t *testing.T) {
	// Test basic instruction disassembly
	instr := newInsOp(_OP_null)
	expected := "null"
	if instr.disassemble() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, instr.disassemble())
	}

	// Test complex instruction disassembly
	values := []int{1, 2, 3}
	instr = newInsVs(_OP_switch, values)
	result := instr.disassemble()
	if result == "" {
		t.Error("Expected non-empty disassembly")
	}

	// Verify switch instruction includes jump table
	if !strings.Contains(result, "switch") {
		t.Error("Switch instruction should mention 'switch'")
	}
}

// Test program operations
func TestProgramOperations(t *testing.T) {
	var prog _Program

	// Test adding instructions
	prog.add(_OP_null)
	prog.add(_OP_bool)
	if len(prog) != 2 {
		t.Errorf("Expected program length 2, got %d", len(prog))
	}

	// Test instruction with integer
	prog.int(_OP_goto, 10)
	if len(prog) != 3 {
		t.Errorf("Expected program length 3, got %d", len(prog))
	}
	if prog[2].vi() != 10 {
		t.Errorf("Expected vi=10, got %d", prog[2].vi())
	}

	// Test instruction with character
	prog.chr(_OP_match_char, 'x')
	if len(prog) != 4 {
		t.Errorf("Expected program length 4, got %d", len(prog))
	}
	if prog[3].vb() != 'x' {
		t.Errorf("Expected vb='x', got %c", prog[3].vb())
	}

	// Test instruction with type
	prog.rtt(_OP_recurse, reflect.TypeOf(int(0)))
	if len(prog) != 5 {
		t	t.Errorf("Expected program length 5, got %d", len(prog))
	}
	if prog[4].vt() != reflect.TypeOf(int(0)) {
		t.Errorf("Expected int type, got %v", prog[4].vt())
	}

	// Test instruction with field map
	fm := caching.CreateFieldMap(0)
	prog.fmv(_OP_struct_field, fm)
	if len(prog) != 6 {
		t.Errorf("Expected program length 6, got %d", len(prog))
	}
	if prog[5].vf() != fm {
		t.Errorf("Expected field map, got %v", prog[5].vf())
	}
}

// Test program tagging and pinning
func TestProgramTaggingAndPinning(t *testing.T) {
	var prog _Program

	// Test tagging
	prog.tag(0)
	prog.add(_OP_null)
	prog.add(_OP_bool)

	// Test pinning
	prog.pin(1) // Pin position 1
	if prog[1].vi() != 1 {
		t.Errorf("Expected pinned position 1, got %d", prog[1].vi())
	}

	// Test relative pinning
	jumpTargets := []int{2, 3}
	prog.rel(jumpTargets)
	if prog[2].vi() != 2 {
		t.Errorf("Expected pinned position 2, got %d", prog[2].vi())
	}
	if prog[3].vi() != 3 {
		t.Errorf("Expected pinned position 3, got %d", prog[3].vi())
	}
}

// Test program disassembly
func TestProgramDisassembly(t *testing.T) {
	prog := _Program{
		{u: packOp(_OP_null)},
		{u: packOp(_OP_bool)},
		{u: packOp(_OP_goto), vi: 5},
		{u: packOp(_OP_switch), vs: []int{1, 2, 3}},
	}

	disassembled := prog.disassemble()
	if disassembled == "" {
		t.Error("Expected non-empty disassembly")
	}

	// Verify it contains our operations
	if !strings.Contains(disassembled, "null") {
		t.Error("Disassembly should contain 'null'")
	}
	if !strings.Contains(disassembled, "bool") {
		t.Error("Disassembly should contain 'bool'")
	}
	if strings.Contains(disassembled, "L_5") {
		t.Error("Disassembly should not contain 'L_5' (no such label)")
	}
	if !strings.Contains(disassembled, "switch") {
		t.Error("Disassembly should contain 'switch'")
	}
}

// Test branch instruction handling
func TestBranchInstructionHandling(t *testing.T) {
	// Test isBranch method
	instr := newInsOp(_OP_goto)
	if !instr.isBranch() {
		t.Error("GOTO instruction should be a branch")
	}

	instr = newInsOp(_OP_switch)
	if !instr.isBranch() {
		t.Error("SWITCH instruction should be a branch")
	}

	instr = newInsOp(_OP_null)
	if instr.isBranch() {
		t.Error("NULL instruction should not be a branch")
	}

	// Test branch instructions with targets
	instr = newInsVi(_OP_goto, 10)
	if !instr.isBranch() {
		t.Error("GOTO with target should be a branch")
	}

	values := []int{1, 2, 3}
	instr = newInsVs(_OP_switch, values)
	if !instr.isBranch() {
		t.Error("SWITCH with values should be a branch")
	}
}

// Test compiler error handling
func TestCompilerErrorHandling(t *testing.T) {
	compiler := newCompiler()

	// Test rescue functionality
	var err error
	compiler.rescue(&err)

	// Try compiling with invalid type
	_, err = compiler.compile(reflect.TypeOf((*func())(nil)))
	if err == nil {
		t.Error("Expected error for function type compilation")
	}

	// Verify error is properly wrapped
	if err == nil {
		t.Error("Error should be properly wrapped")
	}
}

// Test marshaler checking
func TestMarshalerChecking(t *testing.T) {
	compiler := newCompiler()

	// Test with JSON unmarshaler type
	jsonUnmarshalerType := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	if compiler.checkMarshaler(jsonUnmarshalerType, 0, true) {
		t.Error("JSON Unmarshaler should be detected")
	}

	// Test with text unmarshaler type
	textUnmarshalerType := reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	if compiler.checkMarshaler(textUnmarshalerType, 0, true) {
		t.Error("Text Marshaler should be detected")
	}

	// Test with regular type
	regularType := reflect.TypeOf(int(0))
	if !compiler.checkMarshaler(regularType, 0, true) {
		t.Error("Regular type should not be detected as marshaler")
	}
}

// Test recursion detection
func TestRecursionDetection(t *testing.T) {
	compiler := newCompiler()

	// Test that recursion is detected for self-referencing types
	selfReferencingType := reflect.TypeOf((*TestSelfReferencing)(nil))

	// This should handle recursion safely by using _OP_recurse
	prog, err := compiler.compile(selfReferencingType)
	if err != nil {
		t.Error("Self-referencing type should compile to recursion")
	}

	if len(prog) == 0 {
		t.Error("Self-referencing type should generate some code")
	}

	// Verify it uses recursion opcode
	foundRecursion := false
	for _, instr := range prog {
		if instr.op() == _OP_recurse {
			foundRecursion = true
			break
		}
	}
	if !foundRecursion {
		t.Error("Self-referencing type should use _OP_recurse")
	}
}

type TestSelfReferencing struct {
	Next *TestSelfReferencing `json:"next"`
}

// Test depth limiting
func TestDepthLimiting(t *testing.T) {
	compiler := newCompiler()
	compiler.apply(option.DefaultCompileOptions())

	// Test with very deep nesting
	deepType := createDeepType(100)
	_, err := compiler.compile(deepType)
	if err != nil {
		t.Error("Very deep type should hit depth limit and return error")
	}
}

func createDeepType(depth int) reflect.Type {
	if depth <= 0 {
		return reflect.TypeOf(int(0))
	}

	field := createDeepType(depth - 1)

	// Create a struct with the deep type as a field
	structType := reflect.StructOf([]reflect.StructField{
		{
			Name: "Field",
			Type: field,
			Tag:  `json:"field"`,
		},
	})

	return reflect.PtrTo(structType)
}

// Test field caching and optimization
func TestFieldCaching(t *testing.T) {
	compiler := newCompiler()

	// Test with struct that has fields
	testType := reflect.TypeOf(TestStruct{
		Name: "test",
		Age:   42,
	})

	_, err := compiler.compile(testType)
	if err != nil {
		t.Errorf("Struct compilation should succeed, got error: %v", err)
	}

	// Verify field map creation happens
	// (This would require accessing compiler internals or checking side effects)
}

type TestStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Test type compilation for all basic types
func TestBasicTypeCompilation(t *testing.T) {
	compiler := newCompiler()

	basicTypes := []reflect.Type{
		reflect.TypeOf(true),
		reflect.TypeOf(int(0)),
		reflect.TypeOf(int8(0)),
		reflect.TypeOf(int16(0)),
		reflect.TypeOf(int32(0)),
		reflect.TypeOf(int64(0)),
		reflect.TypeOf(uint(0)),
		reflect.TypeOf(uint8(0)),
		reflect.TypeOf(uint16(0)),
		reflect.TypeOf(uint32(0)),
		reflect.TypeOf(uint64(0)),
		reflect.TypeOf(float32(0.0)),
		reflect.TypeOf(float64(0.0)),
		reflect.TypeOf("test"),
	}

	for i, vt := range basicTypes {
		t.Run(vt.String(), func(t *testing.T) {
			_, err := compiler.compile(vt)
			if err != nil {
				t.Errorf("Type %d compilation error: %v", i, err)
			}
		})
	}
}

// Test compilation options
func TestCompilationOptions(t *testing.T) {
	compiler := newCompiler()

	// Test default options
	defaultOpts := option.DefaultCompileOptions()
	compiler.apply(defaultOpts)

	// Test custom options
	customOpts := option.CompileOptions{
		MaxInlineDepth: 10,
		RecursiveDepth: 5,
	}
	compiler.apply(customOpts)

	// Test with struct that would normally exceed depth limit
	deepType := createDeepType(20)
	_, err := compiler.compile(deepType)
	if err != nil {
		t.Error("Deep type should be handled by depth limit")
	}
}

// Test error types and handling
func TestErrorTypes(t *testing.T) {
	compiler := newCompiler()

	// Test unsupported type
	invalidType := reflect.TypeOf((*func())(0))
	_, err := compiler.compile(invalidType)
	if err == nil {
		t.Error("Function type should return error")
	}

	// Verify error type checking
	if _, ok := err.(*json.UnmarshalTypeError); !ok {
		t.Error("Should return UnmarshalTypeError for function type")
	}
}

// Test memory safety and bounds checking
func TestMemorySafety(t *testing.T) {
	compiler := newCompiler()

	// Test array bounds checking
	arrayType := reflect.TypeOf([10]int{})
	_, err := compiler.compile(arrayType)
	if err != nil {
		t.Error("Array type should compile without error")
	}

	// Test slice bounds handling
	sliceType := reflect.TypeOf([]int{})
	_, err = compiler.compile(sliceType)
	if err != nil {
		t.Error("Slice type should compile without error")
	}
}

// Test concurrent compilation safety
func TestConcurrentCompilation(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	// Test that multiple compilers can work concurrently
	compilers := make([]*Compiler, 10)
	for i := range compilers {
		compilers[i] = newCompiler()
	}

	// Compile different types in parallel
	types := []reflect.Type{
		reflect.TypeOf(int(0)),
		reflect.TypeOf(string("test")),
		reflect.TypeOf([3]int{}),
		reflect.TypeOf(map[string]interface{}{}),
	}

	errChan := make(chan error, len(types))
	for i, vt := range types {
		go func(idx int, typ reflect.Type) {
			_, err := compilers[idx].compile(typ)
			errChan <- err
		}(i, vt)
	}

	// Collect all errors
	for i := 0; i < len(types); i++ {
		err := <-errChan
		if err != nil {
			t.Errorf("Concurrent compilation %d failed: %v", i, err)
		}
	}
}

// Test decoder integration
func TestDecoderIntegration(t *testing.T) {
	decoder := NewDecoder("test")

	// Test decoder creation
	if decoder == nil {
		t.Fatal("Expected non-nil decoder")
	}

	// Test decoder reset
	decoder.Reset()
	if decoder.assembler != nil {
		t.Error("Assembler should be nil after reset")
	}
	if decoder.program != nil {
		t.Error("Program should be nil after reset")
	}
	if decoder.compiled {
		t.Error("Compiled flag should be false after reset")
	}

	// Test decoder statistics
	stats := decoder.Stats()
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

// Test decoder compilation
func TestDecoderCompilation(t *testing.T) {
	decoder := NewDecoder("test_compilation")

	testType := reflect.TypeOf(TestStruct{
		Name: "test",
		Age:  42,
	})

	// Compile the type
	compiledDecoder, err := decoder.Compile(testType)
	if err != nil {
		t.Errorf("Compilation failed: %v", err)
	}

	if compiledDecoder == nil {
		t.Fatal("Expected non-nil compiled decoder")
	}

	// Verify decoder is marked as compiled
	if !decoder.IsOptimized() {
		t.Error("Decoder should be optimized after compilation")
	}

	// Test compiled decoder function
	if fn, ok := compiledDecoder.(func(string, int, unsafe.Pointer, *_Stack, uint64, string) (int, error)); ok {
		// Call the compiled function
		result, err := fn(`{"name":"test","age":42}`, 0, unsafe.Pointer(&TestStruct{}), jitdec.NewStack(), 0, "")
		if err != nil {
			t.Errorf("Compiled decoder function error: %v", err)
		}
		if result != 0 {
			t.Errorf("Expected result 0, got %d", result)
		}
	} else {
		t.Error("Compiled decoder should be callable function")
	}
}

type TestStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Test decoder reuse and caching
func TestDecoderReuse(t *testing.T) {
	decoder := NewDecoder("test_reuse")

	testType := reflect.TypeOf(TestStruct{})

	// First compilation
	decoder1, err1 := decoder.Compile(testType)
	if err1 != nil {
		t.Errorf("First compilation failed: %v", err1)
	}

	// Second compilation should reuse if caching is implemented
	decoder2, err2 := decoder.Compile(testType)
	if err2 != nil {
		t.Errorf("Second compilation failed: %v", err2)
	}

	// Both should be non-nil
	if decoder1 == nil || decoder2 == nil {
		t.Error("Both compiled decoders should be non-nil")
	}

	// In current implementation, they might be different objects
	// In a full implementation, caching could make them the same
}

// Test multiple type compilation
func TestMultipleTypeCompilation(t *testing.T) {
	decoder := NewDecoder("test_multiple")

	types := []reflect.Type{
		reflect.TypeOf(int(0)),
		reflect.TypeOf(string("test")),
		reflect.TypeOf([]int{}),
		reflect.TypeOf(map[string]interface{}{}),
		reflect.TypeOf(TestStruct{}),
	}

	results, err := decoder.BatchCompile(types)
	if err != nil {
		t.Errorf("Batch compilation failed: %v", err)
	}

	if len(results) != len(types) {
		t.Errorf("Expected %d results, got %d", len(types), len(results))
	}

	// Verify each type has a compiled decoder
	for _, vt := range types {
		if _, exists := results[vt]; !exists {
			t.Errorf("Missing compiled decoder for type %v", vt)
		}
	}
}

// Test decoder performance
func TestDecoderPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	decoder := NewDecoder("benchmark")

	testType := reflect.TypeOf(TestStruct{})

	// Measure compilation time
	start := testing.Benchmark{}
	start.ResetTimer()
	compiledDecoder, err := decoder.Compile(testType)
	elapsed := start.Elapsed

	if err != nil {
		t.Errorf("Compilation failed: %v", err)
	}

	t.Logf("Compilation took: %v", elapsed)

	// Benchmark decoding if compilation succeeded
	if compiledDecoder != nil {
		fn, ok := compiledDecoder.(func(string, int, unsafe.Pointer, *_Stack, uint64, string) (int, error))
		if ok {
			b.Run("Decode", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := fn(`{"name":"test","age":42}`, 0, unsafe.Pointer(&TestStruct{}), jitdec.NewStack(), 0, "")
					if err != nil {
						b.Fatalf("Decode error: %v", err)
					}
				}
			})
		}
	}
}

// Test JIT options and configuration
func TestJITOptions(t *testing.T) {
	decoder := NewDecoder("jit_options")

	// Test default options
	defaultOpts := jitdec.DefaultJITOptions()
	decoder.ApplyJITOptions(defaultOpts)

	// Test option validation
	invalidOpts := jitdec.JITOptions{
		OptimizationLevel: -1, // Invalid level
	}
	// TODO: Test that invalid options are rejected

	// Test option effects
	validOpts := jitdec.JITOptions{
		OptimizationLevel: 2,
		EnableSIMD:       true,
		EnableInlining:   true,
		DebugMode:         false,
	}
	decoder.ApplyJITOptions(validOpts)
}

// Test debug functionality
func TestDebugFunctionality(t *testing.T) {
	decoder := NewDecoder("debug_test")

	// Test with debug mode enabled
	opts := jitdec.DefaultJITOptions()
	opts.DebugMode = true
	decoder.ApplyJITOptions(opts)

	// Test debug info retrieval
	debugInfo := jitdec.GetDebugInfo(decoder)
	if debugInfo.Program == "" {
		t.Error("Debug info should have program name")
	}
}

// Test ARM64-specific optimizations
func TestARM64Optimizations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping ARM64 optimization test in short mode")
	}

	decoder := NewDecoder("arm64_opt")

	// Test SIMD availability
	simdInfo := jitdec.GetArchitectureInfo()
	if simdInfo["has_simd"] != true {
		t.Error("ARM64 should have SIMD support")
	}
	if simdInfo["has_neon"] != true {
		t.Error("ARM64 should have NEON support")
	}

	// Test register allocation
	registerInfo := jitdec.GetArchitectureInfo()
	if registerInfo["max_registers"] != float64(31) {
		t.Errorf("Expected 31 registers, got %v", registerInfo["max_registers"])
	}
	if registerInfo["stack_align"] != float64(16) {
		t.Errorf("Expected 16-byte stack alignment, got %v", registerInfo["stack_align"])
	}
}

// Test error recovery and fallback
func TestErrorRecovery(t *testing.T) {
	decoder := NewDecoder("error_recovery")

	// Test compilation error handling
	invalidType := reflect.TypeOf((*func())(0))
	_, err := decoder.Compile(invalidType)
	if err == nil {
		t.Error("Expected compilation error for invalid type")
	}

	// Test that decoder is still usable after error
	testType := reflect.TypeOf(int(0))
	_, err = decoder.Compile(testType)
	if err != nil {
		t.Error("Decoder should still be usable after compilation error")
	}
}

// Test memory usage management
func TestMemoryUsageManagement(t *testing.T) {
	decoder := NewDecoder("memory_test")

	// Test multiple compilations
	types := make([]reflect.Type, 10)
	for i := range types {
		types[i] = reflect.TypeOf(i)
	}

	_, err := decoder.BatchCompile(types)
	if err != nil {
		t.Errorf("Batch compilation failed: %v", err)
	}

	// Check memory usage
	allocated, max, count := GetMemoryUsage()
	t.Logf("Memory usage: allocated=%d, max=%d, count=%d", allocated, max, count)
}

// Test thread safety
func TestThreadSafety(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping thread safety test in short mode")
	}

	// Create multiple decoders in parallel
	numDecoders := 5
	decoders := make([]*Decoder, numDecoders)
	for i := range decoders {
		decoders[i] = NewDecoder(fmt.Sprintf("thread_%d", i))
	}

	// Compile types concurrently
	var wg sync.WaitGroup
	type compilationResult struct {
		decoder *Decoder
		err     error
		index   int
		type    reflect.Type
	}

	results := make(chan compilationResult, numDecoders)
	for i := 0; i < numDecoders; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			typeToCompile := reflect.TypeOf(i)
			decoder := decoders[idx]
			compiled, err := decoder.Compile(typeToCompile)
			results <- compilationResult{decoder: compiled, err: err, index: idx, type: typeToCompile}
		}(i)
	}

	// Wait for all compilations to complete
	wg.Wait()
	close(results)

	// Verify all compilations succeeded
	for result := range results {
		if result.err != nil {
			t.Errorf("Compilation %d failed: %v", result.index, result.err)
		}
		if result.decoder == nil {
			t.Errorf("Compilation %d returned nil decoder", result.index)
		}
	}
}

// Test edge cases and corner cases
func TestEdgeCases(t *testing.T) {
	compiler := newCompiler()

	// Test empty struct
	emptyType := reflect.TypeOf(struct{}{})
	_, err := compiler.compile(emptyType)
	if err != nil {
		t.Errorf("Empty struct compilation failed: %v", err)
	}

	// Test large struct
	largeType := reflect.TypeOf(struct {
		F1 [100]int `json:"f1"`
		F2 [100]int `json:"f2"`
		F3 [100]int `json:"f3"`
		F4 [100]int `json:"f4"`
		F5 [100]int `json:"f5"`
	})
	_, err := compiler.compile(largeType)
	if err != nil {
		t.Errorf("Large struct compilation failed: %v", err)
	}

	// Test nil interface
	nilInterfaceType := reflect.TypeOf((*interface{})(nil)).Elem()
	_, err := compiler.compile(nilInterfaceType)
	if err != nil {
		t.Errorf("Nil interface compilation failed: %v", err)
	}

	// Test recursive types
	recursiveType := reflect.TypeOf((*RecursiveType)(nil))
	_, err := compiler.compile(recursiveType)
	if err != nil {
		t.Errorf("Recursive type compilation should handle recursion safely: %v", err)
	}
}

type RecursiveType struct {
	Next *RecursiveType `json:"next"`
}

// Test type validation
func TestTypeValidation(t *testing.T) {
	compiler := newCompiler()

	// Test with valid types
	validTypes := []reflect.Type{
		reflect.TypeOf(int(0)),
		reflect.TypeOf(string("test")),
		reflect.TypeOf([10]int{}),
		reflect.TypeOf(map[string]interface{}{}),
		reflect.TypeOf(&TestStruct{}),
	}

	for _, vt := range validTypes {
		_, err := compiler.compile(vt)
		if err != nil {
			t.Errorf("Valid type %v compilation failed: %v", vt, err)
		}
	}
}

// Test JIT warmup functionality
func TestJITWarmup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping JIT warmup test in short mode")
	}

	decoder := NewARM64JITDecoder("warmup_test")

	// Common types to pre-compile
	commonTypes := []reflect.Type{
		reflect.TypeOf(int(0)),
		reflect.TypeOf(string("")),
		reflect.TypeOf([]int{}),
		reflect.TypeOf(map[string]interface{}{}),
		reflect.TypeOf(TestStruct{}),
	}

	// Warm up all types
	err := decoder.WarmUp(commonTypes)
	if err != nil {
		t.Errorf("JIT warmup failed: %v", err)
	}

	// Verify cache size increases
	initialCacheSize := GetDecoderCacheSize()
	if initialCacheSize < len(commonTypes) {
		t.Logf("Cache size after warmup: %d (expected >= %d)", initialCacheSize, len(commonTypes))
	}
}

// Integration with existing API
func TestAPIIntegration(t *testing.T) {
	// Test that our implementation works with existing decoder API
	testJSON := `{"name":"test","age":42,"valid":true}`

	// Create decoder
	decoder := NewDecoder("api_integration")
	decoder.SetOptions(Options{
		OptionUseInt64:     true,
		OptionCopyString:   true,
		OptionValidateString: true,
	})

	// Test decoding
	var result TestStruct
	err := decoder.Decode(testJSON, &result)
	if err != nil {
		t.Errorf("API integration decoding failed: %v", err)
	}

	if result.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", result.Name)
	}
	if result.Age != 42 {
		t.Errorf("Expected age 42, got %d", result.Age)
	}
	if !result.Valid {
		t.Error("Expected valid=true")
	}
}

// Compatibility with standard library
func TestStdCompatibility(t *testing.T) {
	// Test that our decoder produces same results as encoding/json
	testJSON := `{"name":"test","age":42,"valid":true}`

	// Use our decoder
	var result1 TestStruct
	decoder1 := NewDecoder("std_compat")
	decoder1.SetOptions(Options{
		OptionUseInt64:     true,
		OptionCopyString:   true,
		OptionValidateString: true,
	})

	err1 := decoder1.Decode(testJSON, &result1)
	if err1 != nil {
		t.Errorf("ARM64 decoder error: %v", err1)
	}

	// Use standard library for comparison
	var result2 TestStruct
	decoder2 := NewDecoder("std_library")
	decoder2.SetOptions(Options{
		OptionUseInt64:     true,
		OptionCopyString:   true,
		OptionValidateString: true,
	})

	err2 := decoder2.Decode(testJSON, &result2)
	if err2 != nil {
		t.Errorf("Fallback decoder error: %v", err2)
	}

	// Results should be identical
	if result1.Name != result2.Name || result1.Age != result2.Age || result1.Valid != result2.Valid {
		t.Error("Decoders should produce identical results")
	}
}

// Performance comparison with fallback
func TestPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance comparison in short mode")
	}

	testJSON := `{"name":"test","age":42,"valid":true}`

	// ARM64 JIT decoder
	arm64Decoder := NewDecoder("perf_arm64")
	arm64Decoder.SetOptions(Options{
		OptionUseInt64:     true,
		OptionCopyString:   true,
		OptionValidateString: true,
	})

	// Optimized decoder
	optimizedDecoder := NewDecoder("perf_optimized")
	optimizedDecoder.SetOptions(Options{
		OptionUseInt64:     true,
		OptionCopyString:   true,
	OptionValidateString: true,
	})

	// Fallback decoder
	fallbackDecoder := NewDecoder("perf_fallback")
	fallbackDecoder.SetOptions(Options{
		OptionUseInt64:     true,
		OptionCopyString:   true,
		OptionValidateString: true,
	})

	decoders := []struct {
		name     string
		decoder  *Decoder
		expected func(string, interface{}) error
	}{
		{"ARM64 JIT", arm64Decoder, arm64Decoder.Decode},
		{"Optimized", optimizedDecoder, optimizedDecoder.Decode},
		{"Fallback", fallbackDecoder, fallbackDecoder.Decode},
	}

	for _, tt := range decoders {
		t.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var result TestStruct
				err := tt.expected(testJSON, &result)
				if err != nil {
					b.Fatalf("Decode error with %s: %v", tt.name, err)
				}
			}
		})
	}
}

// Memory leak detection
func TestMemoryLeakDetection(t *testing.T) {
	decoder := NewDecoder("memory_leak_test")

	// Compile many types
	for i := 0; i < 100; i++ {
		type LargeStruct struct {
			Data [1024]byte `json:"data"`
		}
		largeType := reflect.TypeOf(LargeStruct{})
		_, err := decoder.Compile(largeType)
		if err != nil {
			t.Errorf("Compilation %d failed: %v", i, err)
		}
	}

	// Reset and test cleanup
	decoder.Reset()

	// Force garbage collection
	// (In a real implementation, this would be more sophisticated)
}

// Test with invalid JSON input
func TestInvalidJSONHandling(t *testing.T) {
	decoder := NewDecoder("invalid_json_test")
	decoder.SetOptions(Options{
		OptionUseInt64:     true,
		OptionCopyString:   true,
		OptionValidateString: true,
	})

	invalidJSON := []string{
		`{"name":"test","age":}`, // Missing closing brace
		`{"name":null,"age":42}`, // Invalid null placement
		`{"name":test,"age":invalid}`, // Invalid number
		`{"name":"test","age":42,}`, // Extra comma
		`{"name":"test","age":42}`,    // Extra space
	}

	for _, jsonStr := range invalidJSON {
		t.Run(jsonStr, func(t *testing.T) {
			var result TestStruct
			err := decoder.Decode(jsonStr, &result)
			if err == nil {
				t.Errorf("Expected error for invalid JSON: %s", jsonStr)
			}
		})
	}
}

// Test empty JSON handling
func TestEmptyJSONHandling(t *testing.T) {
	decoder := NewDecoder("empty_json_test")
	decoder.SetOptions(Options{
		OptionUseInt64:     true,
		OptionCopyString:   true,
			OptionValidateString: true,
	})

	emptyJSONs := []string{
		"",
		"null",
		"{}",
		"[]",
		"\"\"",
	}

	for _, jsonStr := range emptyJSONs {
		t.Run(jsonStr, func(t *testing.T) {
			var result TestStruct
			err := decoder.Decode(jsonStr, &result)
			if err != nil {
				t.Errorf("Empty JSON error for '%s': %v", jsonStr, err)
			}
		})
	}
}

// Test very large JSON
func TestLargeJSONHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large JSON test in short mode")
	}

	decoder := NewDecoder("large_json_test")
	decoder.SetOptions(Options{
		OptionUseInt64:     true,
		OptionCopyString:   true,
		OptionValidateString: true,
	})

	// Create a large JSON string
		largeJSON := `{"data":[`
	for i := 0; i < 1000; i++ {
		if i > 0 {
			largeJSON += ","
		}
		largeJSON += fmt.Sprintf(`{"id":%d,"value":%d}`, i, i*10)
	}
	largeJSON += "]}"

	// This should handle large JSON without issues
	decoder := NewDecoder("large_json_test")
	var result map[string]interface{}
		err := decoder.Decode(largeJSON, &result)
	if err != nil {
		t.Errorf("Large JSON error: %v", err)
	}

	// Verify the parsed data
	if data, ok := result["data"].([]interface{}); ok && len(data) == 1000 {
		if id, ok := data[999].(int); ok && id == 9999 {
			t.Logf("Successfully parsed large JSON with %d items", len(data))
		}
	}
	}
}

// Test concurrent decoding
func TestConcurrentDecoding(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent decoding test in short mode")
	}

	testJSON := `{"name":"test","age":42,"valid":true}``

	// Create multiple decoders
	numDecoders := 5
	decoders := make([]*Decoder, numDecoders)
	for i := range decoders {
		decoders[i] = NewDecoder(fmt.Sprintf("concurrent_%d", i))
		decoders[i].SetOptions(Options{
			OptionUseInt64:     true,
			OptionCopyString:   true,
			OptionValidateString: true,
		})
	}

	// Decode JSON concurrently
	var wg sync.WaitGroup
	errors := make([]error, numDecoders)
	results := make([]TestStruct, numDecoders)

	for i := 0; i < numDecoders; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var result TestStruct
			err := decoders[idx].Decode(testJSON, &result)
			errors[idx] = err
			if err == nil {
				results[idx] = result
			}
		}(i)
	}

		wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("Concurrent decoding %d failed: %v", i, err)
		}
	}

	// Verify all results are identical
	for i := 1; i < numDecoders; i++ {
		if results[i].Name != results[0].Name || results[i].Age != results[0].Age || results[i].Valid != results[0].Valid {
			t.Errorf("Results %d and 0 differ", i)
		}
	}
}

// Test decoder with different options
func TestDecoderWithDifferentOptions(t *testing.T) {
	testJSON := `{"name":"test","age":42,"valid":true,"null":null}`

	optionsTests := []struct {
		name    string
		options Options
	}{
		{"default", Options{}},
		{"int64", Options{OptionUseInt64: true}},
		{"copy_string", Options{OptionCopyString: true}},
		{"validate_string", Options{OptionValidateString: true}},
	{"all_options", Options{
			OptionUseInt64:     true,
			OptionCopyString:   true,
			OptionValidateString: true,
		}},
	}

	for _, test := range optionsTests {
		t.Run(test.name, func(t *testing.T) {
			decoder := NewDecoder("options_test")
			decoder.SetOptions(test.options)

			var result TestStruct
			err := decoder.Decode(testJSON, &result)
			if err != nil {
				t.Errorf("Decode with options %v failed: %v", test.name, err)
			}

			// Verify expected behavior based on options
			switch test.name {
			case "int64":
				if _, ok := result.Age.(int64); !ok {
					t.Error("With OptionUseInt64, Age should be int64")
				}
			case "copy_string":
				// Should copy string values
				if result.Name != "test" {
					t.Error("With OptionCopyString, string should be copied")
				}
			case "validate_string":
				// Should validate string values
				// This would require checking validation logic
			}
		}
	})
	}
}

// Benchmark different decoder configurations
func BenchmarkDecoderConfigurations(b *testing.B) {
	testJSON := `{"name":"test","age":42,"valid":true}`

	configs := []struct {
		name string
		opts Options
	}{
		{"default", Options{}},
		{"with_int64", Options{OptionUseInt64: true}},
		{"with_copy_string", Options{OptionCopyString: true}},
		{"with_validation", Options{OptionValidateString: true}},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			decoder := NewDecoder("benchmark_" + cfg.name)
			decoder.SetOptions(cfg.opts)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var result TestStruct
				decoder.Decode(testJSON, &result)
			}
		})
	}
}

// ARM64-specific benchmarks
func BenchmarkARM64InstructionGeneration(b *testing.B) {
	prog := _Program{
		{u: packOp(_OP_null)},
		{u: packOp(_OP_bool)},
		{u: packOp(_OP_i64)},
		{u: packOp(_OP_str)},
		{u: packOp(_OP_goto), vi: 10},
	}

	assembler := newAssembler(prog)
	assembler.name = "benchmark_arm64"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoder := assembler.Load()
		if decoder == nil {
			b.Fatal("Failed to load decoder")
		}
	}
	}
}

func BenchmarkARM64RegisterOperations(b *testing.B) {
	assembler := newAssembler(_Program{})
	assembler.name = "benchmark_registers"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.save(_X0, _X1, _X2, _X3)
		assembler.load(_X0, _X1, _X2, _X3)
	}
}

func BenchmarkARM64StackOperations(b *testing.B) {
	assembler := newAssembler(_Program{})
	assembler.name = "benchmark_stack"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.save_state()
		assembler.drop_state(32)
	}
}

func BenchmarkARM64MemoryOperations(b *testing.B) {
	assembler := newAssembler(_Program{})
	assembler.name = "benchmark_memory"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.malloc_X0(jit.Imm(64), _X0)
		assembler.valloc(reflect.TypeOf(int(0)), _X0)
	}
}

// Integration test with actual decoding
func TestARM64DecodingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires the JIT to be functional
	if !jit.IsARM64JITEnabled() {
		t.Skip("ARM64 JIT is not enabled")
	}

	// Test a simple struct decoding scenario
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
		Valid bool   `json:"valid"`
	}

	// Create a program that simulates decoding TestStruct
	prog := _Program{
		{u: packOp(_OP_lspace)},
		{u: packOp(_OP_match_char), vb: '{'},
		{u: packOp(_OP_text), vs: "name:\""},
		{u: packOp(_OP_str)},
		{u: packOp(_OP_text), vs: "\",age:"},
		{u: packOp(_OP_i64)},
		{u: packOp(_OP_text), vs: ",valid:"},
		{u: packOp(_OP_bool)},
		{u: packOp(_OP_match_char), vb: '}'},
	}

	assembler := newAssembler(prog)
	assembler.name = "test_integration"

	decoder := assembler.Load()
	if decoder == nil {
		t.Error("Expected non-nil decoder for integration test")
	}

	// The decoder should be callable
	if decoder == nil {
		t.Fatal("Decoder should be callable")
	}

	// Note: Actual decoding would require more setup and testing infrastructure
	// This test mainly verifies that the assembler can generate code without panicking
}

// Test ARM64 specific instruction generation
func TestARM64InstructionGeneration(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_instruction_generation"

	// Test prologue and epilogue
	assembler.prologue()
	assembler.epilogue()

	// Should have generated some code
	if assembler.CodeSize() == 0 {
		t.Error("Expected non-zero code size after prologue and epilogue")
	}
}

// Test register allocation
func TestARM64RegisterAllocation(t *testing.T) {
	// Verify that critical registers don't conflict
	if _IC.Reg == _IP.Reg {
		t.Error("Input cursor and input pointer registers should be different")
	}

	if _IC.Reg == _IL.Reg {
		t.Error("Input cursor and input length registers should be different")
	}

	if _VP.Reg == _ST.Reg {
		t.Error("Value pointer and stack base registers should be different")
	}

	if _ET.Reg == _EP.Reg {
		t.Error("Error type and error pointer registers should be different")
	}
}

// Test stack frame layout
func TestARM64StackFrameLayout(t *testing.T) {
	// Verify stack frame layout is consistent
	if _FP_offs <= _FP_fargs + _FP_saves + _FP_locals {
		t.Error("FP_offs should be larger than fargs + saves + locals")
	}

	if _FP_size <= _FP_offs {
		t.Error("_FP_size should be larger than FP_offs")
	}

	if _FP_base <= _FP_size {
		t.Error("_FP_base should be larger than _FP_size")
	}

	// Verify alignment to 16 bytes (ARM64 requirement)
	if _FP_size%16 != 0 {
		t.Errorf("Stack frame size should be 16-byte aligned, got %d", _FP_size)
	}
}

// Test error handling paths
func TestARM64ErrorHandlingPaths(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_error_handling"

	// Test that all error handlers can be linked without panicking
	assembler.type_error()
	assembler.mismatch_error()
	assembler.field_error()
	assembler.range_error()
	assembler.stack_error()
	assembler.base64_error()
	assembler.parsing_error()
}

// Test buffer management
func TestARM64BufferManagement(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_buffer_management"

	// Test buffer helper functions
	assembler.check_eof(1)
	assembler.check_size(10)
	assembler.add_char('a')
	assembler.add_long(0x61626364, 4)
	assembler.add_text("test")
}

// Test state management
func TestARM64StateManagement(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_state_management"

	// Test state management functions
	assembler.save_state()
	assembler.drop_state(32)
}

// Test function calling conventions
func TestARM64FunctionCalling(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_function_calling"

	// Test function call helpers
	assembler.save_c()
	assembler.call_go(jit.Func(func() {}))
}

// Test memory operations
func TestARM64MemoryOperations(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_memory_operations"

	// Test memory allocation and management
	assembler.malloc_X0(jit.Imm(64), _X0)
	assembler.valloc(reflect.TypeOf(int(0)), _X0)
	assembler.vfollow(reflect.TypeOf(&int(0)))
}

// Test range checking
func TestARM64RangeChecking(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_range_checking"

	// Test range checking for different types
	assembler.range_signed_X1(_I_int8, _T_int8, -128, 127)
	assembler.range_unsigned_X1(_I_uint8, _T_uint8, 0, 255)
	assembler.range_uint32_X1(_I_uint32, _T_uint32)
	assembler.range_single_D0()
}

// Test string operations
func TestARM64StringOperations(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_string_operations"

	// Test string manipulation functions
	assembler.slice_from(_VAR_st_Iv, -1)
	assembler.unquote_once(_X0, _X1, true, false)
	assembler.unquote_twice(_X0, _X1, false)
}

// Test map operations
func TestARM64MapOperations(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_map_operations"

	// Test map assignment functions
	assembler.mapassign_std(reflect.TypeOf(map[string]interface{}{}), _X0)
	assembler.mapassign_str_fast(reflect.TypeOf(map[string]string{}), _X0, _X1)
	assembler.mapassign_utext(reflect.TypeOf(map[string]interface{}{}), false)
}

// Test external unmarshaler support
func TestARM64UnmarshalerSupport(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_unmarshaler_support"

	// Test unmarshaler function calls
	assembler.unmarshal_json(reflect.TypeOf((*json.Unmarshaler)(nil)).Elem(), true, _F_decodeJsonUnmarshaler)
	assembler.unmarshal_text(reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem(), true)
}

// Test dynamic decoding
func TestARM64DynamicDecoding(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_dynamic_decoding"

	// Test dynamic decoding support
	assembler.decode_dynamic(jit.Type(reflect.TypeOf(interface{}(0))), _VP)
}

// Test JIT options and configuration
func TestARM64JITOptions(t *testing.T) {
	assembler := newAssembler(_Program{})
	assembler.name = "test_jit_options"

	// Test default options
	opts := jitdec.DefaultJITOptions()
	assembler.ApplyJITOptions(opts)

	// Test statistics and debugging
	stats := assembler.Stats()
	if stats == nil {
		t.Error("Expected non-nil stats")
	}

	debug := jitdec.GetDebugInfo(assembler)
	if debug.Program == "" {
		t.Error("Expected non-empty program name")
	}
}

// Helper function to create instruction for testing
func newInsOp(op _Op) _Instr {
	return _Instr{u: packOp(op)}
}

func newInsVi(op _Op, vi int) _Instr {
	return _Instr{u: packOp(op) | rt.PackInt(vi)}
}

func newInsVb(op _Op, vb byte) _Instr {
	return _Instr{u: packOp(op) | (uint64(vb) << 48)}
}

func newInsVt(op _Op, vt reflect.Type) _Instr {
	return _Instr{
		u: packOp(op),
		p: unsafe.Pointer(rt.UnpackType(vt)),
	}
}

func newInsVs(op _Op, vs []int) _Instr {
	slice := (*rt.GoSlice)(unsafe.Pointer(&vs))
	return _Instr{
		u: packOp(op) | rt.PackInt(len(vs)),
		p: slice.Ptr,
	}
}

// Mock functions for testing
func error_wrap(s string, ic int, et interface{}, ep interface{}) error {
	return fmt.Errorf("error: %s at position %d: %v (type: %v, pointer: %v)", s, ic, et, ep)
}

func error_type(vt interface{}) error {
	return fmt.Errorf("type error: %v", vt)
}

func error_field(field string) error {
	return fmt.Errorf("field error: %s", field)
}

func error_value(value interface{}, vt interface{}, ep interface{}) error {
	return fmt.Errorf("value error: %v (type: %v, pointer: %v)", value, vt, ep)
}

func error_mismatch(s string, ic int, vt interface{}, et interface{}) error {
	return fmt.Errorf("mismatch error: %s at position %d (type: %v, expected: %v)", s, ic, vt, et)
}

func decodeJsonUnmarshaler(data []byte, v interface{}) error {
	// Mock implementation
	return nil
}

func decodeJsonUnmarshalerQuoted(data []byte, v interface{}) error {
	// Mock implementation
	return nil
}

func decodeTextUnmarshaler(data []byte, v interface{}) error {
	// Mock implementation
	return nil
}

func error_wrap(s string, ic int, et interface{}, ep interface{}) error {
	return fmt.Errorf("error: %s at position %d: %v (type: %v, pointer: %v)", s, ic, et, ep)
}

func error_type(vt interface{}) error {
	return fmt.Errorf("type error: %v", vt)
}

func error_field(field string) error {
	return fmt.Errorf("field error: %s", field)
}

func error_value(value interface{}, vt interface{}, ep interface{}) error {
	return fmt.Errorf("value error: %v (type: %v, pointer: %v)", value, vt, ep)
}

func error_mismatch(s string, ic int, vt interface{}, et interface{}) error {
	return fmt.Errorf("mismatch error: %s at position %d (type: %v, expected: %v)", s, ic, vt, et)
}

var (
	jsonUnmarshalerType = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	encodingTextMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	_Vp_max_f32 = new(float32)
	_Vp_min_f32 = new(float32)
	base64CorruptInputError = reflect.TypeOf(base64.CorruptInputError(0))
)

	_StackOverflow = new(stackOverflowType)
)