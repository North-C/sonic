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
	"testing"
	"reflect"
	"unsafe"

	"github.com/bytedance/sonic/internal/encoder/ir"
	"github.com/bytedance/sonic/internal/encoder/vars"
	"github.com/bytedance/sonic/internal/jit"
	"github.com/bytedance/sonic/internal/rt"
)

func TestARM64AssemblerCreation(t *testing.T) {
	// Create a simple instruction program
	prog := ir.Program{
		{Op: ir.OP_null},
		{Op: ir.OP_bool},
		{Op: ir.OP_i32},
	}

	assembler := NewAssembler(prog)
	if assembler == nil {
		t.Fatal("Expected non-nil assembler")
	}

	if assembler.p == nil {
		t.Error("Expected non-nil program")
	}

	if assembler.Name != "" {
		t.Errorf("Expected empty name, got %s", assembler.Name)
	}
}

func TestARM64AssemblerInit(t *testing.T) {
	prog := ir.Program{
		{Op: ir.OP_null},
	}

	assembler := NewAssembler(prog)
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
	prog := ir.Program{
		{Op: ir.OP_null},
	}

	assembler := NewAssembler(prog)
	assembler.Name = "test"

	// This should not panic
	encoder := assembler.Load()
	if encoder == nil {
		t.Error("Expected non-nil encoder")
	}
}

func TestARM64RegisterConstants(t *testing.T) {
	// Test that all register constants are properly defined
	tests := []struct {
		name string
		reg  jit.Addr
	}{
		{"_ARG0", _ARG0},
		{"_ARG1", _ARG1},
		{"_RET0", _RET0},
		{"_RET1", _RET1},
		{"_ST", _ST},
		{"_RP", _RP},
		{"_RL", _RL},
		{"_RC", _RC},
		{"_ET", _ET},
		{"_EP", _EP},
		{"_SP_p", _SP_p},
		{"_SP_q", _SP_q},
		{"_SP_x", _SP_x},
		{"_SP_f", _SP_f},
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
	if len(_REG_ffi) == 0 {
		t.Error("_REG_ffi should not be empty")
	}

	if len(_REG_all) == 0 {
		t.Error("_REG_all should not be empty")
	}

	if len(_REG_ms) == 0 {
		t.Error("_REG_ms should not be empty")
	}

	if len(_REG_enc) == 0 {
		t.Error("_REG_enc should not be empty")
	}

	// Check that all sets contain valid registers
	for _, reg := range _REG_ffi {
		if reg.Reg == 0 {
			t.Errorf("Invalid register in _REG_ffi: %v", reg)
		}
	}
}

func TestARM64Constants(t *testing.T) {
	// Test stack frame constants
	if _FP_args != 32 {
		t.Errorf("Expected _FP_args = 32, got %d", _FP_args)
	}

	if _FP_fargs != 40 {
		t.Errorf("Expected _FP_fargs = 40, got %d", _FP_fargs)
	}

	if _FP_saves != 80 {
		t.Errorf("Expected _FP_saves = 80, got %d", _FP_saves)
	}

	if _FP_locals != 24 {
		t.Errorf("Expected _FP_locals = 24, got %d", _FP_locals)
	}

	// Test immediate constants
	if _IM_null != 0x6c6c756e {
		t.Errorf("Expected _IM_null = 0x6c6c756e, got 0x%x", _IM_null)
	}

	if _IM_true != 0x65757274 {
		t.Errorf("Expected _IM_true = 0x65757274, got 0x%x", _IM_true)
	}

	if _IM_fals != 0x736c6166 {
		t.Errorf("Expected _IM_fals = 0x736c6166, got 0x%x", _IM_fals)
	}
}

func TestARM64ArgumentLocations(t *testing.T) {
	// Test that argument locations are properly defined
	if _ARG_rb.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_ARG_rb should be a pointer")
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
	if _VAR_sp.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_VAR_sp should be a pointer")
	}

	if _VAR_dn.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_VAR_dn should be a pointer")
	}

	if _VAR_vp.Type != jit.Ptr(_SP, 0).Type {
		t.Error("_VAR_vp should be a pointer")
	}
}

func TestARM64OpFuncTable(t *testing.T) {
	// Test that the operation function table is properly initialized
	if len(_OpFuncTab) == 0 {
		t.Error("_OpFuncTab should not be empty")
	}

	// Test that some key operations are defined
	keyOps := []ir.OpCode{
		ir.OP_null,
		ir.OP_bool,
		ir.OP_i8,
		ir.OP_i16,
		ir.OP_i32,
		ir.OP_i64,
		ir.OP_u8,
		ir.OP_u16,
		ir.OP_u32,
		ir.OP_u64,
		ir.OP_f32,
		ir.OP_f64,
		ir.OP_str,
		ir.OP_bin,
		ir.OP_deref,
		ir.OP_index,
		ir.OP_load,
		ir.OP_save,
		ir.OP_drop,
		ir.OP_recurse,
	}

	for _, op := range keyOps {
		if int(op) >= len(_OpFuncTab) || _OpFuncTab[op] == nil {
			t.Errorf("Operation %v should be defined in _OpFuncTab", op)
		}
	}
}

func TestARM64BasicOperations(t *testing.T) {
	prog := ir.Program{
		{Op: ir.OP_null},
		{Op: ir.OP_bool},
		{Op: ir.OP_i32},
	}

	assembler := NewAssembler(prog)
	assembler.Name = "test_basic_ops"

	// This should not panic and should generate code
	encoder := assembler.Load()
	if encoder == nil {
		t.Error("Expected non-nil encoder")
	}
}

func TestARM64StackOperations(t *testing.T) {
	prog := ir.Program{
		{Op: ir.OP_save},
		{Op: ir.OP_load},
		{Op: ir.OP_drop},
	}

	assembler := NewAssembler(prog)
	assembler.Name = "test_stack_ops"

	// This should not panic and should generate code
	encoder := assembler.Load()
	if encoder == nil {
		t.Error("Expected non-nil encoder")
	}
}

func TestARM64IntegerOperations(t *testing.T) {
	intOps := []ir.OpCode{
		ir.OP_i8,
		ir.OP_i16,
		ir.OP_i32,
		ir.OP_i64,
		ir.OP_u8,
		ir.OP_u16,
		ir.OP_u32,
		ir.OP_u64,
	}

	for _, op := range intOps {
		t.Run(op.String(), func(t *testing.T) {
			prog := ir.Program{{Op: op}}
			assembler := NewAssembler(prog)
			assembler.Name = "test_" + op.String()

			encoder := assembler.Load()
			if encoder == nil {
				t.Errorf("Expected non-nil encoder for operation %v", op)
			}
		})
	}
}

func TestARM64FloatOperations(t *testing.T) {
	floatOps := []ir.OpCode{
		ir.OP_f32,
		ir.OP_f64,
	}

	for _, op := range floatOps {
		t.Run(op.String(), func(t *testing.T) {
			prog := ir.Program{{Op: op}}
			assembler := NewAssembler(prog)
			assembler.Name = "test_" + op.String()

			encoder := assembler.Load()
			if encoder == nil {
				t.Errorf("Expected non-nil encoder for operation %v", op)
			}
		})
	}
}

func TestARM64StringOperations(t *testing.T) {
	strOps := []ir.OpCode{
		ir.OP_str,
		ir.OP_bin,
		ir.OP_quote,
	}

	for _, op := range strOps {
		t.Run(op.String(), func(t *testing.T) {
			prog := ir.Program{{Op: op}}
			assembler := NewAssembler(prog)
			assembler.Name = "test_" + op.String()

			encoder := assembler.Load()
			if encoder == nil {
				t.Errorf("Expected non-nil encoder for operation %v", op)
			}
		})
	}
}

func TestARM64ControlOperations(t *testing.T) {
	controlOps := []ir.OpCode{
		ir.OP_is_nil,
		ir.OP_is_zero_1,
		ir.OP_is_zero_2,
		ir.OP_is_zero_4,
		ir.OP_is_zero_8,
		ir.OP_goto,
	}

	for _, op := range controlOps {
		t.Run(op.String(), func(t *testing.T) {
			prog := ir.Program{{Op: op}}
			assembler := NewAssembler(prog)
			assembler.Name = "test_" + op.String()

			encoder := assembler.Load()
			if encoder == nil {
				t.Errorf("Expected non-nil encoder for operation %v", op)
			}
		})
	}
}

func TestARM64ComplexProgram(t *testing.T) {
	// Create a more complex program that simulates encoding a struct
	prog := ir.Program{
		{Op: ir.OP_byte, Vi: int64('{')},
		{Op: ir.OP_text, Vs: `"name":"`},
		{Op: ir.OP_str},
		{Op: ir.OP_text, Vs: `","age":`},
		{Op: ir.OP_i64},
		{Op: ir.OP_byte, Vi: int64('}')},
	}

	assembler := NewAssembler(prog)
	assembler.Name = "test_complex_program"

	encoder := assembler.Load()
	if encoder == nil {
		t.Error("Expected non-nil encoder for complex program")
	}
}

func TestARM64InstructionHandling(t *testing.T) {
	assembler := NewAssembler(ir.Program{})
	assembler.Name = "test_instructions"

	// Test that basic instruction handling doesn't panic
	assembler.instr(&ir.Instr{Op: ir.OP_null})
	assembler.instr(&ir.Instr{Op: ir.OP_bool})
	assembler.instr(&ir.Instr{Op: ir.OP_i32})
}

func TestARM64BuiltinFunctions(t *testing.T) {
	prog := ir.Program{
		{Op: ir.OP_null}, // This will call builtins during compilation
	}

	assembler := NewAssembler(prog)
	assembler.Name = "test_builtins"

	// This should compile with builtins
	encoder := assembler.Load()
	if encoder == nil {
		t.Error("Expected non-nil encoder with builtins")
	}
}

func TestARM64ErrorHandling(t *testing.T) {
	// Test with an invalid operation
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid operation")
		}
	}()

	prog := ir.Program{{Op: ir.OpCode(999)}} // Invalid operation
	assembler := NewAssembler(prog)
	assembler.compile() // This should panic
}

func TestARM64HelperFunctions(t *testing.T) {
	assembler := NewAssembler(ir.Program{})
	assembler.Name = "test_helpers"

	// Test that helper functions don't panic
	assembler.xsave(_ARG0, _ARG1)
	assembler.xload(_ARG0, _ARG1)
	assembler.rbuf_rp()
	assembler.check_size(10)
	assembler.add_char('a')
	assembler.add_long(0x61626364, 4)
	assembler.add_text("test")
}

func TestARM64BufferHelpers(t *testing.T) {
	assembler := NewAssembler(ir.Program{})
	assembler.Name = "test_buffer_helpers"

	// Test buffer helper functions
	assembler.prep_buffer_X0()
	assembler.save_buffer()
	assembler.load_buffer_X0()
}

func TestARM64StateManagement(t *testing.T) {
	assembler := NewAssembler(ir.Program{})
	assembler.Name = "test_state_management"

	// Test state management functions
	assembler.save_state()
	assembler.drop_state(32)
}

func TestARM64FunctionCalls(t *testing.T) {
	assembler := NewAssembler(ir.Program{})
	assembler.Name = "test_function_calls"

	// Test function call helpers
	assembler.save_c()
	assembler.call_go(jit.Func(func() {}))
}

// Benchmark tests for performance validation
func BenchmarkARM64AssemblerCreation(b *testing.B) {
	prog := ir.Program{
		{Op: ir.OP_null},
		{Op: ir.OP_bool},
		{Op: ir.OP_i32},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler := NewAssembler(prog)
		if assembler == nil {
			b.Fatal("Expected non-nil assembler")
		}
	}
}

func BenchmarkARM64AssemblerLoad(b *testing.B) {
	prog := ir.Program{
		{Op: ir.OP_null},
		{Op: ir.OP_bool},
		{Op: ir.OP_i32},
	}

	assembler := NewAssembler(prog)
	assembler.Name = "benchmark"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder := assembler.Load()
		if encoder == nil {
			b.Fatal("Expected non-nil encoder")
		}
	}
}

func BenchmarkARM64BasicOperations(b *testing.B) {
	ops := []ir.OpCode{
		ir.OP_null,
		ir.OP_bool,
		ir.OP_i32,
		ir.OP_str,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		op := ops[i%len(ops)]
		prog := ir.Program{{Op: op}}
		assembler := NewAssembler(prog)
		assembler.Name = "benchmark_op"
		encoder := assembler.Load()
		if encoder == nil {
			b.Fatal("Expected non-nil encoder")
		}
	}
}

func BenchmarkARM64ComplexProgram(b *testing.B) {
	prog := ir.Program{
		{Op: ir.OP_byte, Vi: int64('{')},
		{Op: ir.OP_text, Vs: `"test":`},
		{Op: ir.OP_str},
		{Op: ir.OP_text, Vs: `,"value":`},
		{Op: ir.OP_i64},
		{Op: ir.OP_byte, Vi: int64('}')},
	}

	assembler := NewAssembler(prog)
	assembler.Name = "benchmark_complex"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder := assembler.Load()
		if encoder == nil {
			b.Fatal("Expected non-nil encoder")
		}
	}
}

// Integration test with actual encoding
func TestARM64EncodingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires the JIT to be functional
	if !jit.IsARM64JITEnabled() {
		t.Skip("ARM64 JIT is not enabled")
	}

	// Test a simple struct encoding scenario
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
		Valid bool   `json:"valid"`
	}

	// Create a program that simulates encoding TestStruct
	prog := ir.Program{
		{Op: ir.OP_byte, Vi: int64('{')},
		{Op: ir.OP_text, Vs: `"name":"`},
		{Op: ir.OP_str},
		{Op: ir.OP_text, Vs: `","age":`},
		{Op: ir.OP_i64},
		{Op: ir.OP_text, Vs: `","valid":`},
		{Op: ir.OP_bool},
		{Op: ir.OP_byte, Vi: int64('}')},
	}

	assembler := NewAssembler(prog)
	assembler.Name = "test_integration"

	encoder := assembler.Load()
	if encoder == nil {
		t.Error("Expected non-nil encoder for integration test")
	}

	// The encoder should be callable
	if encoder == nil {
		t.Fatal("Encoder should be callable")
	}

	// Note: Actual encoding would require more setup and testing infrastructure
	// This test mainly verifies that the assembler can generate code without panicking
}

// Test ARM64 specific instruction generation
func TestARM64InstructionGeneration(t *testing.T) {
	assembler := NewAssembler(ir.Program{})
	assembler.Name = "test_instruction_generation"

	// Test prologue and epilogue
	assembler.prologue()
	assembler.epilogue()

	// Should have generated some code
	if assembler.Size() == 0 {
		t.Error("Expected non-zero code size after prologue and epilogue")
	}
}

// Test register allocation
func TestARM64RegisterAllocation(t *testing.T) {
	// Verify that critical registers don't conflict
	if _RP.Reg == _RL.Reg {
		t.Error("Result pointer and length registers should be different")
	}

	if _RP.Reg == _RC.Reg {
		t.Error("Result pointer and capacity registers should be different")
	}

	if _ST.Reg == _RP.Reg {
		t.Error("Stack base and result pointer registers should be different")
	}

	if _ET.Reg == _EP.Reg {
		t.Error("Error type and error pointer registers should be different")
	}
}

// Test stack frame layout
func TestARM64StackFrameLayout(t *testing.T) {
	// Verify stack frame layout is consistent
	if FP_offs <= _FP_loffs {
		t.Error("FP_offs should be larger than _FP_loffs")
	}

	if _FP_size <= FP_offs {
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