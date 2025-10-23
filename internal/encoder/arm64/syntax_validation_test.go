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
	"testing"

	"github.com/bytedance/sonic/internal/encoder/ir"
	"github.com/stretchr/testify/assert"
)

// TestSyntaxValidation validates that all ARM64 JIT components can be instantiated
// and basic operations work without actually executing JIT code
func TestSyntaxValidation(t *testing.T) {
	// Test that we can create assembler programs
	programs := []ir.Program{
		{ir.NewInsOp(ir.OP_null)},
		{ir.NewInsOp(ir.OP_bool)},
		{ir.NewInsOp(ir.OP_i8)},
		{ir.NewInsOp(ir.OP_i16)},
		{ir.NewInsOp(ir.OP_i32)},
		{ir.NewInsOp(ir.OP_i64)},
		{ir.NewInsOp(ir.OP_u8)},
		{ir.NewInsOp(ir.OP_u16)},
		{ir.NewInsOp(ir.OP_u32)},
		{ir.NewInsOp(ir.OP_u64)},
		{ir.NewInsOp(ir.OP_str)},
		{ir.NewInsOp(ir.OP_byte)},
		{ir.NewInsOp(ir.OP_text)},
	}

	for i, p := range programs {
		t.Run(fmt.Sprintf("program_%d", i), func(t *testing.T) {
			// Test assembler creation
			a := NewAssembler(p)
			assert.NotNil(t, a)

			// Test that assembler has required methods
			assert.NotNil(t, a.Load)

			// Test that we can access the program
			assert.Equal(t, p, a.p)

			// Test basic assembler properties
			assert.NotEmpty(t, a.Name)
		})
	}
}

// TestInstructionMapping validates that all opcodes have corresponding implementations
func TestInstructionMapping(t *testing.T) {
	// Test that all basic opcodes are mapped in the instruction table
	opcodes := []ir.OpCode{
		ir.OP_null,
		ir.OP_empty_arr,
		ir.OP_empty_obj,
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
		ir.OP_quote,
		ir.OP_byte,
		ir.OP_text,
		ir.OP_deref,
		ir.OP_is_nil,
		ir.OP_is_zero_1,
		ir.OP_is_zero_2,
		ir.OP_is_zero_4,
		ir.OP_is_zero_8,
		ir.OP_goto,
	}

	for _, op := range opcodes {
		t.Run(fmt.Sprintf("opcode_%d", op), func(t *testing.T) {
			// Create a simple program with this opcode
			p := ir.Program{ir.NewInsOp(op)}
			a := NewAssembler(p)

			// Test that the assembler can be created without panicking
			assert.NotNil(t, a)

			// Test that the assembler can be loaded (this should compile the program)
			// We don't execute it here, just ensure it compiles
			assert.NotPanics(t, func() {
				f := a.Load()
				assert.NotNil(t, f)
			})
		})
	}
}

// TestRegisterDefinitions validates that ARM64 registers are properly defined
func TestRegisterDefinitions(t *testing.T) {
	// Test that our register constants are defined
	assert.NotNil(t, _ARG0)
	assert.NotNil(t, _ARG1)
	assert.NotNil(t, _ARG2)
	assert.NotNil(t, _ARG3)
	assert.NotNil(t, _RET0)
	assert.NotNil(t, _RET1)
	assert.NotNil(t, _ST)
	assert.NotNil(t, _RP)
	assert.NotNil(t, _RL)
	assert.NotNil(t, _RC)
	assert.NotNil(t, _ET)
	assert.NotNil(t, _EP)
	assert.NotNil(t, _FP_REG)
	assert.NotNil(t, _LR_REG)
	assert.NotNil(t, _ZR)
}

// TestStackConstants validates that stack-related constants are defined
func TestStackConstants(t *testing.T) {
	// Test frame size constants
	assert.Greater(t, _FP_args, int64(0))
	assert.Greater(t, _FP_size, int64(0))
	assert.Greater(t, _FP_base, int64(0))

	// Test local variable locations
	assert.NotNil(t, _VAR_sp)
	assert.NotNil(t, _VAR_dn)
	assert.NotNil(t, _VAR_vp)

	// Test register sets
	assert.NotNil(t, _REG_ffi)
	assert.NotNil(t, _REG_b64)
	assert.NotNil(t, _REG_all)
	assert.NotNil(t, _REG_ms)
	assert.NotNil(t, _REG_enc)
}

// TestErrorLabels validates that error handling labels are defined
func TestErrorLabels(t *testing.T) {
	// Test that error labels are defined as constants
	assert.NotEmpty(t, _LB_more_space)
	assert.NotEmpty(t, _LB_more_space_return)
	assert.NotEmpty(t, _LB_error)
	assert.NotEmpty(t, _LB_error_too_deep)
	assert.NotEmpty(t, _LB_error_invalid_number)
	assert.NotEmpty(t, _LB_error_nan_or_infinite)
	assert.NotEmpty(t, _LB_panic)
}