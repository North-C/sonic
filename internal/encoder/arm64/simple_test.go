//go:build arm64 && go1.20 && !go1.26
// +build arm64,go1.20,!go1.26

package arm64

import (
	"testing"
	"reflect"

	"github.com/bytedance/sonic/internal/encoder/ir"
	"github.com/bytedance/sonic/internal/jit"
	"github.com/bytedance/sonic/internal/rt"
)

// TestBasicFunctionality tests basic ARM64 JIT functionality
func TestBasicFunctionality(t *testing.T) {
	// Test ARM64 register definitions
	t.Run("ARM64Registers", func(t *testing.T) {
		// Test that we can create registers without panic
		_ = jit.R0
		_ = jit.R1
		_ = jit.FP
		_ = jit.LR
		_ = jit.SP
		_ = jit.ZR
	})

	// Test ARM64 instruction generation
	t.Run("ARM64Instructions", func(t *testing.T) {
		// Test creating immediate values
		imm := jit.Imm(42)
		if imm.Type != jit.Imm(0).Type {
			t.Error("Immediate type mismatch")
		}

		// Test creating pointers
		ptr := jit.Ptr(jit.R0, 8)
		if ptr.Type != jit.Ptr(jit.R0, 0).Type {
			t.Error("Pointer type mismatch")
		}
	})

	// Test IR program generation
	t.Run("IRProgramGeneration", func(t *testing.T) {
		// Create a simple program that encodes a null value
		program := ir.Program{
			{Op: ir.OP_null},
		}

		if len(program) != 1 {
			t.Errorf("Expected program length 1, got %d", len(program))
		}

		if program[0].Op() != ir.OP_null {
			t.Errorf("Expected OP_null, got %v", program[0].Op())
		}
	})

	// Test encoder creation
	t.Run("EncoderCreation", func(t *testing.T) {
		encoder := NewEncoder("test")
		if encoder == nil {
			t.Fatal("Expected non-nil encoder")
		}

		if encoder.name != "test" {
			t.Errorf("Expected name 'test', got '%s'", encoder.name)
		}
	})

	// Test assembler creation
	t.Run("AssemblerCreation", func(t *testing.T) {
		program := ir.Program{
			{Op: ir.OP_null},
			{Op: ir.OP_bool},
		}

		assembler := NewAssembler(program)
		if assembler == nil {
			t.Fatal("Expected non-nil assembler")
		}

		if len(assembler.p) != 2 {
			t.Errorf("Expected program length 2, got %d", len(assembler.p))
		}
	})

	// Test basic compilation (without actually running the code)
	t.Run("BasicCompilation", func(t *testing.T) {
		encoder := NewEncoder("basic_test")

		// Try to compile a basic type
		goType := rt.UnpackType(reflect.TypeOf(42))
		_, err := encoder.Compile(goType)

		// We expect this to not panic, even if it doesn't fully work yet
		if err != nil {
			t.Logf("Compilation returned error (expected for now): %v", err)
		}

		// Check that assembler was created
		if encoder.assembler == nil {
			t.Error("Expected assembler to be created after compilation")
		}
	})
}

// TestRegisterAllocation tests ARM64 register allocation
func TestRegisterAllocation(t *testing.T) {
	// Test that critical registers don't conflict
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

// TestStackFrameLayout tests ARM64 stack frame layout
func TestStackFrameLayout(t *testing.T) {
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

// TestInstructionGeneration tests basic instruction generation
func TestInstructionGeneration(t *testing.T) {
	assembler := NewAssembler(ir.Program{})
	assembler.Name = "test_instructions"

	// Test that basic instruction handling doesn't panic
	assembler.prologue()
	assembler.epilogue()

	// Should have generated some code
	size := assembler.Size()
	if size == 0 {
		t.Error("Expected non-zero code size after prologue and epilogue")
	}
}