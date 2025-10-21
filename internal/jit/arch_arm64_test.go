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

	"github.com/twitchyliquid64/golang-asm/obj"
)

func TestARM64RegisterCreation(t *testing.T) {
	tests := []struct {
		name     string
		register string
		expected bool
	}{
		{"R0", "R0", true},
		{"R1", "R1", true},
		{"R10", "R10", true},
		{"R28", "R28", true},
		{"R29", "R29", true},
		{"R30", "R30", true},
		{"FP", "FP", true},
		{"LR", "LR", true},
		{"ZR", "ZR", true},
		{"SP", "SP", true},
		{"F0", "F0", true},
		{"F15", "F15", true},
		{"F31", "F31", true},
		{"Invalid", "R99", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && tt.expected {
					t.Errorf("Expected register %s to be valid, but got panic: %v", tt.register, r)
				}
			}()

			reg := Reg(tt.register)
			if tt.expected {
				if reg.Reg == 0 {
					t.Errorf("Expected valid register for %s", tt.register)
				}
				if reg.Type != obj.TYPE_REG {
					t.Errorf("Expected register type TYPE_REG, got %v", reg.Type)
				}
			}
		})
	}
}

func TestARM64ImmediateCreation(t *testing.T) {
	tests := []struct {
		name     string
		imm      int64
		expected int64
	}{
		{"zero", 0, 0},
		{"positive", 42, 42},
		{"negative", -1, -1},
		{"large", 0xFFFFFFFF, 0xFFFFFFFF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imm := Imm(tt.imm)

			if imm.Type != obj.TYPE_CONST {
				t.Errorf("Expected immediate type TYPE_CONST, got %v", imm.Type)
			}

			if imm.Offset != tt.expected {
				t.Errorf("Expected offset %d, got %d", tt.expected, imm.Offset)
			}
		})
	}
}

func TestARM64PointerCreation(t *testing.T) {
	base := Reg("R0")
	offset := int64(8)

	ptr := Ptr(base, offset)

	if ptr.Type != obj.TYPE_MEM {
		t.Errorf("Expected memory type TYPE_MEM, got %v", ptr.Type)
	}

	if ptr.Reg != base.Reg {
		t.Errorf("Expected base register %v, got %v", base.Reg, ptr.Reg)
	}

	if ptr.Offset != offset {
		t.Errorf("Expected offset %d, got %d", offset, ptr.Offset)
	}
}

func TestARM64OffsetRegisterCreation(t *testing.T) {
	base := Reg("R0")
	index := Reg("R1")

	offsetReg := OffsetReg(base, index)

	if offsetReg.Type != obj.TYPE_MEM {
		t.Errorf("Expected memory type TYPE_MEM, got %v", offsetReg.Type)
	}

	if offsetReg.Reg != base.Reg {
		t.Errorf("Expected base register %v, got %v", base.Reg, offsetReg.Reg)
	}

	if offsetReg.Index != index.Reg {
		t.Errorf("Expected index register %v, got %v", index.Reg, offsetReg.Index)
	}

	if offsetReg.Offset != 0 {
		t.Errorf("Expected offset 0, got %d", offsetReg.Offset)
	}
}

func TestARM64ImmediatePointer(t *testing.T) {
	testValue := 0x12345678
	ptr := unsafe.Pointer(uintptr(testValue))
	immPtr := ImmPtr(ptr)

	if immPtr.Type != obj.TYPE_CONST {
		t.Errorf("Expected constant type TYPE_CONST, got %v", immPtr.Type)
	}

	if immPtr.Offset != int64(testValue) {
		t.Errorf("Expected offset %d, got %d", testValue, immPtr.Offset)
	}
}

func TestARM64CalleeSavedRegisters(t *testing.T) {
	expected := []obj.Addr{R19, R20, R21, R22, R23, R24, R25, R26, R27, R28, FP}

	if len(CALLEE_SAVED_REGS) != len(expected) {
		t.Errorf("Expected %d callee-saved registers, got %d", len(expected), len(CALLEE_SAVED_REGS))
	}

	for i, reg := range CALLEE_SAVED_REGS {
		if reg.Reg != expected[i].Reg {
			t.Errorf("Callee-saved register %d: expected %v, got %v", i, expected[i].Reg, reg.Reg)
		}
	}
}

func TestARM64CallerSavedRegisters(t *testing.T) {
	expected := []obj.Addr{R8, R9, R10, R11, R12, R13, R14, R15, R16, R17, LR}

	if len(CALLER_SAVED_REGS) != len(expected) {
		t.Errorf("Expected %d caller-saved registers, got %d", len(expected), len(CALLER_SAVED_REGS))
	}

	for i, reg := range CALLER_SAVED_REGS {
		if reg.Reg != expected[i].Reg {
			t.Errorf("Caller-saved register %d: expected %v, got %v", i, expected[i].Reg, reg.Reg)
		}
	}
}

func TestARM64RegisterClassification(t *testing.T) {
	tests := []struct {
		name         string
		register     obj.Addr
		isCalleeSaved bool
		isCallerSaved bool
		isArgument    bool
		isFloat       bool
	}{
		{"R19", R19, true, false, false, false},
		{"R20", R20, true, false, false, false},
		{"R28", R28, true, false, false, false},
		{"FP", FP, true, false, false, false},
		{"R0", R0, false, true, true, false},
		{"R1", R1, false, true, true, false},
		{"R5", R5, false, true, true, false},
		{"R7", R7, false, true, true, false},
		{"R8", R8, false, true, false, false},
		{"LR", LR, false, true, false, false},
		{"F0", F0, false, false, false, true},
		{"F15", F15, false, false, false, true},
		{"F31", F31, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsCalleeSaved(tt.register) != tt.isCalleeSaved {
				t.Errorf("IsCalleeSaved(%s) = %v, expected %v", tt.name, IsCalleeSaved(tt.register), tt.isCalleeSaved)
			}

			if IsCallerSaved(tt.register) != tt.isCallerSaved {
				t.Errorf("IsCallerSaved(%s) = %v, expected %v", tt.name, IsCallerSaved(tt.register), tt.isCallerSaved)
			}

			if IsArgumentRegister(tt.register) != tt.isArgument {
				t.Errorf("IsArgumentRegister(%s) = %v, expected %v", tt.name, IsArgumentRegister(tt.register), tt.isArgument)
			}

			if IsFloatRegister(tt.register) != tt.isFloat {
				t.Errorf("IsFloatRegister(%s) = %v, expected %v", tt.name, IsFloatRegister(tt.register), tt.isFloat)
			}
		})
	}
}

func TestARM64RegisterSize(t *testing.T) {
	tests := []struct {
		name     string
		register obj.Addr
		expected int
	}{
		{"R0", R0, 8},
		{"R15", R15, 8},
		{"FP", FP, 8},
		{"LR", LR, 8},
		{"F0", F0, 8},
		{"F31", F31, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := GetRegisterSize(tt.register)
			if size != tt.expected {
				t.Errorf("GetRegisterSize(%s) = %d, expected %d", tt.name, size, tt.expected)
			}
		})
	}
}

func TestARM64StackAlignment(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		expected int64
	}{
		{"zero", 0, 0},
		{"already aligned", 16, 16},
		{"already aligned 32", 32, 32},
		{"not aligned", 8, 16},
		{"not aligned 23", 23, 32},
		{"not aligned 1", 1, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aligned := AlignStack(tt.size)
			if aligned != tt.expected {
				t.Errorf("AlignStack(%d) = %d, expected %d", tt.size, aligned, tt.expected)
			}
			if aligned%16 != 0 {
				t.Errorf("AlignStack(%d) = %d is not 16-byte aligned", tt.size, aligned)
			}
		})
	}
}

func TestARM64ArgumentRegister(t *testing.T) {
	tests := []struct {
		name     string
		index    int
		expected obj.Addr
	}{
		{"ARG0", 0, R0},
		{"ARG1", 1, R1},
		{"ARG2", 2, R2},
		{"ARG3", 3, R3},
		{"ARG4", 4, R4},
		{"ARG5", 5, R5},
		{"ARG6", 6, R6},
		{"ARG7", 7, R7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := GetArgumentRegister(tt.index)
			if reg.Reg != tt.expected.Reg {
				t.Errorf("GetArgumentRegister(%d) = %v, expected %v", tt.index, reg.Reg, tt.expected.Reg)
			}
		})
	}
}

func TestARM64ArgumentRegisterPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid argument register index")
		}
	}()

	GetArgumentRegister(-1)
}

func TestARM64ArgumentRegisterPanicHigh(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid argument register index")
		}
	}()

	GetArgumentRegister(8)
}

func TestARM64ReturnRegister(t *testing.T) {
	tests := []struct {
		name     string
		index    int
		expected obj.Addr
	}{
		{"RET0", 0, R0},
		{"RET1", 1, R1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := GetReturnRegister(tt.index)
			if reg.Reg != tt.expected.Reg {
				t.Errorf("GetReturnRegister(%d) = %v, expected %v", tt.index, reg.Reg, tt.expected.Reg)
			}
		})
	}
}

func TestARM64ReturnRegisterPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid return register index")
		}
	}()

	GetReturnRegister(-1)
}

func TestARM64ReturnRegisterPanicHigh(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid return register index")
		}
	}()

	GetReturnRegister(2)
}

func TestARM64ConditionCodes(t *testing.T) {
	tests := []struct {
		name     string
		value    uint8
		expected string
	}{
		{"EQ", COND_EQ, "Equal"},
		{"NE", COND_NE, "Not equal"},
		{"HS", COND_HS, "Higher or same"},
		{"LO", COND_LO, "Lower"},
		{"MI", COND_MI, "Minus"},
		{"PL", COND_PL, "Plus"},
		{"VS", COND_VS, "Overflow set"},
		{"VC", COND_VC, "Overflow clear"},
		{"HI", COND_HI, "Higher"},
		{"LS", COND_LS, "Lower or same"},
		{"GE", COND_GE, "Greater or equal"},
		{"LT", COND_LT, "Less than"},
		{"GT", COND_GT, "Greater than"},
		{"LE", COND_LE, "Less or equal"},
		{"AL", COND_AL, "Always"},
		{"NV", COND_NV, "Never"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value < 16 { // Valid condition codes are 0-15
				// Just test that the constants are defined correctly
				_ = tt.value
			}
		})
	}
}

func TestARM64MemoryAccessSizes(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		expected string
	}{
		{"Byte", SIZE_B, "Byte"},
		{"Halfword", SIZE_H, "Halfword"},
		{"Word", SIZE_W, "Word"},
		{"Doubleword", SIZE_X, "Doubleword"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just test that the constants are defined correctly
			if tt.size < 0 || tt.size > 3 {
				t.Errorf("Invalid size constant: %d", tt.size)
			}
		})
	}
}

func TestARM64ConstantValues(t *testing.T) {
	// Test architecture constants
	if MAX_REG_ARGS != 8 {
		t.Errorf("Expected MAX_REG_ARGS = 8, got %d", MAX_REG_ARGS)
	}

	if STACK_ALIGNMENT != 16 {
		t.Errorf("Expected STACK_ALIGNMENT = 16, got %d", STACK_ALIGNMENT)
	}

	if FRAME_HEADER_SIZE != 16 {
		t.Errorf("Expected FRAME_HEADER_SIZE = 16, got %d", FRAME_HEADER_SIZE)
	}
}

// Benchmark tests
func BenchmarkARM64Reg(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Reg("R0")
	}
}

func BenchmarkARM64Imm(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Imm(42)
	}
}

func BenchmarkARM64Ptr(b *testing.B) {
	base := Reg("R0")
	for i := 0; i < b.N; i++ {
		_ = Ptr(base, int64(i))
	}
}

func BenchmarkARM64IsCalleeSaved(b *testing.B) {
	reg := R19
	for i := 0; i < b.N; i++ {
		_ = IsCalleeSaved(reg)
	}
}

func BenchmarkARM64IsArgumentRegister(b *testing.B) {
	reg := R0
	for i := 0; i < b.N; i++ {
		_ = IsArgumentRegister(reg)
	}
}

func BenchmarkARM64GetArgumentRegister(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GetArgumentRegister(i % 8)
	}
}