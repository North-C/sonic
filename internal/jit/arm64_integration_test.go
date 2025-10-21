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

package jit

import (
	"testing"
	"unsafe"

	"github.com/bytedance/sonic/internal/rt"
)

func TestARM64JITIntegration(t *testing.T) {
	// Test that ARM64 JIT support is available
	if !HasARM64JITSupport {
		t.Skip("ARM64 JIT support not available")
	}

	// Test that ARM64 JIT is enabled
	if !IsARM64JITEnabled() {
		t.Skip("ARM64 JIT is disabled")
	}

	t.Run("Architecture Detection", func(t *testing.T) {
		// Test basic register creation
		r0 := Reg("R0")
		if r0.Reg == 0 {
			t.Error("Failed to create R0 register")
		}

		fp := Reg("FP")
		if fp.Reg == 0 {
			t.Error("Failed to create FP register")
		}

		lr := Reg("LR")
		if lr.Reg == 0 {
			t.Error("Failed to create LR register")
		}

		// Test floating point registers
		f0 := Reg("F0")
		if f0.Reg == 0 {
			t.Error("Failed to create F0 register")
		}
	})

	t.Run("Immediate Operations", func(t *testing.T) {
		imm := Imm(42)
		if imm.Type != 0 { // TYPE_CONST
			t.Errorf("Expected TYPE_CONST, got %v", imm.Type)
		}
		if imm.Offset != 42 {
			t.Errorf("Expected offset 42, got %d", imm.Offset)
		}
	})

	t.Run("Memory Operations", func(t *testing.T) {
		base := Reg("R0")
		ptr := Ptr(base, 8)

		if ptr.Type != 1 { // TYPE_MEM
			t.Errorf("Expected TYPE_MEM, got %v", ptr.Type)
		}
		if ptr.Reg != base.Reg {
			t.Errorf("Expected base register %v, got %v", base.Reg, ptr.Reg)
		}
		if ptr.Offset != 8 {
			t.Errorf("Expected offset 8, got %d", ptr.Offset)
		}
	})

	t.Run("Register Classification", func(t *testing.T) {
		// Test callee-saved registers
		if !IsCalleeSaved(R19) {
			t.Error("R19 should be callee-saved")
		}
		if !IsCalleeSaved(FP) {
			t.Error("FP should be callee-saved")
		}

		// Test caller-saved registers
		if !IsCallerSaved(R0) {
			t.Error("R0 should be caller-saved")
		}
		if !IsCallerSaved(LR) {
			t.Error("LR should be caller-saved")
		}

		// Test argument registers
		if !IsArgumentRegister(R0) {
			t.Error("R0 should be argument register")
		}
		if !IsArgumentRegister(R7) {
			t.Error("R7 should be argument register")
		}
		if IsArgumentRegister(R8) {
			t.Error("R8 should not be argument register")
		}

		// Test floating point registers
		if !IsFloatRegister(F0) {
			t.Error("F0 should be float register")
		}
		if IsFloatRegister(R0) {
			t.Error("R0 should not be float register")
		}
	})

	t.Run("Stack Alignment", func(t *testing.T) {
		// Test stack alignment
		aligned := AlignStack(15)
		if aligned != 16 {
			t.Errorf("Expected 16, got %d", aligned)
		}
		if aligned%16 != 0 {
			t.Errorf("Expected 16-byte alignment, got %d", aligned)
		}

		aligned = AlignStack(16)
		if aligned != 16 {
			t.Errorf("Expected 16, got %d", aligned)
		}

		aligned = AlignStack(23)
		if aligned != 32 {
			t.Errorf("Expected 32, got %d", aligned)
		}
	})

	t.Run("Calling Convention", func(t *testing.T) {
		// Test argument registers
		arg0 := GetArgumentRegister(0)
		if arg0.Reg != R0.Reg {
			t.Errorf("Expected R0, got %v", arg0)
		}

		arg7 := GetArgumentRegister(7)
		if arg7.Reg != R7.Reg {
			t.Errorf("Expected R7, got %v", arg7)
		}

		// Test return registers
		ret0 := GetReturnRegister(0)
		if ret0.Reg != R0.Reg {
			t.Errorf("Expected R0, got %v", ret0)
		}

		ret1 := GetReturnRegister(1)
		if ret1.Reg != R1.Reg {
			t.Errorf("Expected R1, got %v", ret1)
		}
	})
}

func TestARM64AssemblerIntegration(t *testing.T) {
	if !HasARM64JITSupport {
		t.Skip("ARM64 JIT support not available")
	}

	assembler := NewARM64Assembler()
	if assembler == nil {
		t.Fatal("Failed to create ARM64 assembler")
	}

	t.Run("Basic Instructions", func(t *testing.T) {
		// Test NOP
		assembler.NOP()
		if assembler.Size() == 0 {
			t.Error("NOP should generate code")
		}

		// Test MOV
		assembler.MOV(R0, R1)
		if assembler.Size() == 0 {
			t.Error("MOV should generate code")
		}

		// Test ADD
		assembler.ADD(R0, R1, R2)
		if assembler.Size() == 0 {
			t.Error("ADD should generate code")
		}
	})

	t.Run("Immediate Loading", func(t *testing.T) {
		assembler.LoadImm(0x12345678, R3)
		if assembler.Size() == 0 {
			t.Error("LoadImm should generate code")
		}
	})

	t.Run("Function Prologue/Epilogue", func(t *testing.T) {
		frameSize := int64(32)

		assembler.Prologue(frameSize)
		if assembler.Size() == 0 {
			t.Error("Prologue should generate code")
		}

		assembler.Epilogue(frameSize)
		if assembler.Size() == 0 {
			t.Error("Epilogue should generate code")
		}
	})

	t.Run("Register Save/Restore", func(t *testing.T) {
		assembler.SaveCalleeSaved()
		if assembler.Size() == 0 {
			t.Error("SaveCalleeSaved should generate code")
		}

		assembler.RestoreCalleeSaved()
		if assembler.Size() == 0 {
			t.Error("RestoreCalleeSaved should generate code")
		}
	})
}

func TestARM64InstructionTranslation(t *testing.T) {
	if !HasARM64JITSupport {
		t.Skip("ARM64 JIT support not available")
	}

	// Import the arm64 package for instruction translation
	translator := NewInstructionTranslator()

	t.Run("MOV Translation", func(t *testing.T) {
		prog, err := translator.TranslateInstruction(INSN_MOV, R0, R1)
		if err != nil {
			t.Fatalf("Failed to translate MOV: %v", err)
		}
		if prog == nil {
			t.Error("Expected non-nil program")
		}
	})

	t.Run("ADD Translation", func(t *testing.T) {
		prog, err := translator.TranslateInstruction(INSN_ADD, R0, R1, R2)
		if err != nil {
			t.Fatalf("Failed to translate ADD: %v", err)
		}
		if prog == nil {
			t.Error("Expected non-nil program")
		}
	})

	t.Run("JMP Translation", func(t *testing.T) {
		prog, err := translator.TranslateInstruction(INSN_JMP, "target")
		if err != nil {
			t.Fatalf("Failed to translate JMP: %v", err)
		}
		if prog == nil {
			t.Error("Expected non-nil program")
		}
	})

	t.Run("JCC Translation", func(t *testing.T) {
		prog, err := translator.TranslateInstruction(INSN_JCC, COND_EQ, "target")
		if err != nil {
			t.Fatalf("Failed to translate JCC: %v", err)
		}
		if prog == nil {
			t.Error("Expected non-nil program")
		}
	})
}

func TestARM64ComplexFunctionGeneration(t *testing.T) {
	if !HasARM64JITSupport {
		t.Skip("ARM64 JIT support not available")
	}

	assembler := NewARM64Assembler()

	// Generate a function that adds two 64-bit integers
	frameSize := int64(32)

	// Function prologue
	assembler.Prologue(frameSize)

	// Save arguments from calling convention
	assembler.MOV(R19, ARG0) // Save first argument
	assembler.MOV(R20, ARG1) // Save second argument

	// Add the numbers
	assembler.ADD(R21, R19, R20)

	// Move result to return register
	assembler.MOV(RET0, R21)

	// Function epilogue
	assembler.Epilogue(frameSize)

	// Verify that code was generated
	if assembler.Size() == 0 {
		t.Error("Expected non-zero code size")
	}

	// Should have generated a reasonable amount of code
	if assembler.Size() < 20 {
		t.Errorf("Expected at least 20 bytes, got %d", assembler.Size())
	}

	t.Logf("Generated function size: %d bytes", assembler.Size())
}

func TestARM64JITConfiguration(t *testing.T) {
	t.Run("Default State", func(t *testing.T) {
		if !HasARM64JITSupport {
			t.Error("ARM64 JIT support should be available")
		}

		if !IsARM64JITEnabled() {
			t.Error("ARM64 JIT should be enabled by default")
		}
	})

	t.Run("Enable/Disable", func(t *testing.T) {
		// Disable JIT
		DisableARM64JIT()
		if IsARM64JITEnabled() {
			t.Error("ARM64 JIT should be disabled")
		}

		// Re-enable JIT
		EnableARM64JIT()
		if !IsARM64JITEnabled() {
			t.Error("ARM64 JIT should be enabled")
		}
	})

	t.Run("Version Check", func(t *testing.T) {
		if ARM64JITVersion == "" {
			t.Error("ARM64 JIT version should not be empty")
		}

		t.Logf("ARM64 JIT Version: %s", ARM64JITVersion)
	})
}

// Performance tests
func TestARM64JITPerformance(t *testing.T) {
	if !HasARM64JITSupport {
		t.Skip("ARM64 JIT support not available")
	}

	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	t.Run("Instruction Generation Performance", func(t *testing.T) {
		assembler := NewARM64Assembler()

		// Generate a large number of instructions
		for i := 0; i < 1000; i++ {
			assembler.MOV(Reg("R"+string(rune('0'+i%8))), Imm(int64(i)))
			assembler.ADD(Reg("R"+string(rune('0'+i%8))), Reg("R"+string(rune('0'+i%8))), Imm(1))
		}

		size := assembler.Size()
		if size == 0 {
			t.Error("Expected non-zero code size")
		}

		t.Logf("Generated %d bytes for 2000 instructions", size)
	})
}

// Error handling tests
func TestARM64JITErrorHandling(t *testing.T) {
	t.Run("Invalid Register", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid register")
			}
		}()

		Reg("INVALID")
	})

	t.Run("Invalid Argument Register", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid argument register")
			}
		}()

		GetArgumentRegister(8) // Only 0-7 are valid
	})

	t.Run("Invalid Return Register", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid return register")
			}
		}()

		GetReturnRegister(2) // Only 0-1 are valid
	})
}

// Memory safety tests
func TestARM64JITMemorySafety(t *testing.T) {
	if !HasARM64JITSupport {
		t.Skip("ARM64 JIT support not available")
	}

	t.Run("Pointer Operations", func(t *testing.T) {
		// Test pointer creation with various values
		base := Reg("R0")
		ptr1 := Ptr(base, 0)
		ptr2 := Ptr(base, 8)
		ptr3 := Ptr(base, -8)

		if ptr1.Reg != base.Reg || ptr1.Offset != 0 {
			t.Error("Invalid pointer 1")
		}
		if ptr2.Reg != base.Reg || ptr2.Offset != 8 {
			t.Error("Invalid pointer 2")
		}
		if ptr3.Reg != base.Reg || ptr3.Offset != -8 {
			t.Error("Invalid pointer 3")
		}
	})

	t.Run("Immediate Pointer", func(t *testing.T) {
		testValue := uintptr(0x12345678)
		ptr := unsafe.Pointer(testValue)
		immPtr := ImmPtr(ptr)

		if immPtr.Type != 0 { // TYPE_CONST
			t.Errorf("Expected TYPE_CONST, got %v", immPtr.Type)
		}
		if immPtr.Offset != int64(testValue) {
			t.Errorf("Expected offset %d, got %d", testValue, immPtr.Offset)
		}
	})
}