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
	"unsafe"

	"github.com/twitchyliquid64/golang-asm/asm/arch"
	"github.com/twitchyliquid64/golang-asm/obj"
	"github.com/twitchyliquid64/golang-asm/obj/arm64"
)

var (
	// _AC initializes the ARM64 architecture
	_AC = arch.Set("arm64")
)

// ARM64 register definitions based on golang-asm
// References: https://github.com/twitchyliquid64/golang-asm/blob/master/asm/arch/arch.go#L249
// and https://github.com/twitchyliquid64/golang-asm/blob/master/obj/arm64/sysRegEnc.go

// General purpose registers (R0-R30)
var (
	R0  = Reg("R0")
	R1  = Reg("R1")
	R2  = Reg("R2")
	R3  = Reg("R3")
	R4  = Reg("R4")
	R5  = Reg("R5")
	R6  = Reg("R6")
	R7  = Reg("R7")
	R8  = Reg("R8")
	R9  = Reg("R9")
	R10 = Reg("R10")
	R11 = Reg("R11")
	R12 = Reg("R12")
	R13 = Reg("R13")
	R14 = Reg("R14")
	R15 = Reg("R15")
	R16 = Reg("R16")
	R17 = Reg("R17")
	R18 = Reg("R18")
	R19 = Reg("R19")
	R20 = Reg("R20")
	R21 = Reg("R21")
	R22 = Reg("R22")
	R23 = Reg("R23")
	R24 = Reg("R24")
	R25 = Reg("R25")
	R26 = Reg("R26")
	R27 = Reg("R27")
	R28 = Reg("R28")
	FP = Reg("R29") // Frame Pointer
	LR = Reg("R30") // Link Register
)

// Zero register and Stack Pointer
var (
	ZR = Reg("ZR") // Zero Register (always 0)
	SP = Reg("SP") // Stack Pointer
)

// Floating-point and SIMD registers (V0-V31)
var (
	F0  = Reg("F0")
	F1  = Reg("F1")
	F2  = Reg("F2")
	F3  = Reg("F3")
	F4  = Reg("F4")
	F5  = Reg("F5")
	F6  = Reg("F6")
	F7  = Reg("F7")
	F8  = Reg("F8")
	F9  = Reg("F9")
	F10 = Reg("F10")
	F11 = Reg("F11")
	F12 = Reg("F12")
	F13 = Reg("F13")
	F14 = Reg("F14")
	F15 = Reg("F15")
	F16 = Reg("F16")
	F17 = Reg("F17")
	F18 = Reg("F18")
	F19 = Reg("F19")
	F20 = Reg("F20")
	F21 = Reg("F21")
	F22 = Reg("F22")
	F23 = Reg("F23")
	F24 = Reg("F24")
	F25 = Reg("F25")
	F26 = Reg("F26")
	F27 = Reg("F27")
	F28 = Reg("F28")
	F29 = Reg("F29")
	F30 = Reg("F30")
	F31 = Reg("F31")
)

// Platform registers (caller-saved vs callee-saved)
// ARM64 Procedure Call Standard:
// R0-R7: argument/result registers (caller-saved)
// R8-R15: callee-saved registers
// R16-R17: intra-procedure-call registers (caller-saved)
// R18: platform register (callee-saved)
// R19-R28: callee-saved registers
// R29 (FP): frame pointer (callee-saved)
// R30 (LR): link register (caller-saved)

// Argument/Result registers (caller-saved)
var (
	ARG0 = R0
	ARG1 = R1
	ARG2 = R2
	ARG3 = R3
	ARG4 = R4
	ARG5 = R5
	ARG6 = R6
	ARG7 = R7

	RET0 = R0
	RET1 = R1
)

// Callee-saved registers
var (
	CALLEE_SAVED_REGS = []obj.Addr{R19, R20, R21, R22, R23, R24, R25, R26, R27, R28, FP}
)

// Caller-saved registers (excluding argument registers)
var (
	CALLER_SAVED_REGS = []obj.Addr{R8, R9, R10, R11, R12, R13, R14, R15, R16, R17, LR}
)

// Register returns an ARM64 register address
// Reference: https://pkg.go.dev/cmd/internal/obj/arm64
func Reg(reg string) obj.Addr {
	if ret, ok := _AC.Register[reg]; ok {
		return obj.Addr{Reg: ret, Type: obj.TYPE_REG}
	} else {
		panic("invalid ARM64 register name: " + reg)
	}
}

// Imm creates an immediate constant address
func Imm(imm int64) obj.Addr {
	return obj.Addr{
		Type:   obj.TYPE_CONST,
		Offset: imm,
	}
}

// Ptr creates a memory address with base register and offset
func Ptr(reg obj.Addr, offs int64) obj.Addr {
	return obj.Addr{
		Reg:    reg.Reg,
		Type:   obj.TYPE_MEM,
		Offset: offs,
	}
}

// OffsetReg creates a memory address with base register and index register offset
func OffsetReg(base, index obj.Addr) obj.Addr {
	return obj.Addr{
		Reg:     base.Reg,
		Index:   index.Reg,
		Type:    obj.TYPE_MEM,
		Offset:  0,
	}
}

// ImmPtr creates an immediate pointer address from unsafe.Pointer
func ImmPtr(imm unsafe.Pointer) obj.Addr {
	return obj.Addr{
		Type:   obj.TYPE_CONST,
		Offset: int64(uintptr(imm)),
	}
}

// RegShift creates a register with shift amount for ARM64 addressing modes
func RegShift(reg obj.Addr, shift uint8) obj.Addr {
	return obj.Addr{
		Reg:    reg.Reg,
		Type:   obj.TYPE_REG,
		Offset: int64(shift),
	}
}

// ExtReg creates an extended register for ARM64 addressing modes
// extType: 0=UXTB, 1=UXTH, 2=UXTW, 3=UXTX, 4=SXTB, 5=SXTH, 6=SXTW, 7=SXTX
func ExtReg(reg obj.Addr, extType uint8) obj.Addr {
	return obj.Addr{
		Reg:    reg.Reg,
		Type:   obj.TYPE_REG,
		Offset: int64(extType),
	}
}

// ARM64 condition codes
const (
	COND_EQ = 0 // Equal
	COND_NE = 1 // Not equal
	COND_CS = 2 // Carry set (unsigned higher or same)
	COND_HS = COND_CS // Alias for CS
	COND_CC = 3 // Carry clear (unsigned lower)
	COND_LO = COND_CC // Alias for CC
	COND_MI = 4 // Minus (negative)
	COND_PL = 5 // Plus (positive or zero)
	COND_VS = 6 // Overflow set
	COND_VC = 7 // Overflow clear
	COND_HI = 8 // Unsigned higher
	COND_LS = 9 // Unsigned lower or same
	COND_GE = 10 // Signed greater than or equal
	COND_LT = 11 // Signed less than
	COND_GT = 12 // Signed greater than
	COND_LE = 13 // Signed less than or equal
	COND_AL = 14 // Always (unconditional)
	COND_NV = 15 // Never (unconditional)
)

// ARM64 memory access sizes
const (
	SIZE_B = 0 // Byte
	SIZE_H = 1 // Halfword (2 bytes)
	SIZE_W = 2 // Word (4 bytes)
	SIZE_X = 3 // Doubleword (8 bytes)
)

// ARM64 barrier types
const (
	BARRIER_SY = 0xF // Full system barrier
	BARRIER_ST = 0xE // Store barrier
	BARRIER_LD = 0xD // Load barrier
	BARRIER_ISH = 0xB // Inner shareable barrier
	BARRIER_ISHST = 0xA // Inner shareable store barrier
	BARRIER_ISHLD = 0x9 // Inner shareable load barrier
	BARRIER_NSH = 0x7 // Non-shareable barrier
	BARRIER_NSHST = 0x6 // Non-shareable store barrier
	BARRIER_NSHLD = 0x5 // Non-shareable load barrier
	BARRIER_OSH = 0x3 // Outer shareable barrier
	BARRIER_OSHST = 0x2 // Outer shareable store barrier
	BARRIER_OSHLD = 0x1 // Outer shareable load barrier
)

// ARM64 system registers
const (
	SYSREG_DC_CIVAC = 0x390 // Data or unified cache line invalidate by VA to PoC
	SYSREG_IC_IALLU = 0x250 // Invalidate all instruction caches Inner Shareable
	SYSREG_TLBI_VMALLE1IS = 0x600 // Invalidate all stage-1 translations
)

// ARM64 processor state (PSTATE) fields
const (
	PSTATE_SP = 0x3  // Stack pointer selection
	PSTATE_DAIF = 0x7 // Disable exceptions
	PSTATE_NZCV = 0xF // Condition flags
)

// IsCalleeSaved checks if a register is callee-saved according to ARM64 PCS
func IsCalleeSaved(reg obj.Addr) bool {
	for _, r := range CALLEE_SAVED_REGS {
		if r.Reg == reg.Reg {
			return true
		}
	}
	return false
}

// IsCallerSaved checks if a register is caller-saved according to ARM64 PCS
func IsCallerSaved(reg obj.Addr) bool {
	for _, r := range CALLER_SAVED_REGS {
		if r.Reg == reg.Reg {
			return true
		}
	}
	// Argument registers are also caller-saved
	for i := 0; i < 8; i++ {
		if reg.Reg == Reg("R"+string(rune('0'+i))).Reg {
			return true
		}
	}
	return false
}

// IsArgumentRegister checks if a register is used for arguments
func IsArgumentRegister(reg obj.Addr) bool {
	for i := 0; i < 8; i++ {
		if reg.Reg == Reg("R"+string(rune('0'+i))).Reg {
			return true
		}
	}
	return false
}

// IsFloatRegister checks if a register is a floating-point/SIMD register
func IsFloatRegister(reg obj.Addr) bool {
	return reg.Reg >= F0.Reg && reg.Reg <= F31.Reg
}

// GetRegisterSize returns the size of a register in bytes
func GetRegisterSize(reg obj.Addr) int {
	if IsFloatRegister(reg) {
		return 8 // All FP registers are 64-bit in our usage
	}
	return 8 // All general purpose registers are 64-bit
}

// AlignStack aligns the stack pointer to 16-byte boundary as required by ARM64 ABI
func AlignStack(size int64) int64 {
	if size%16 != 0 {
		return size + (16 - size%16)
	}
	return size
}

// ARM64 calling convention constants
const (
	// Stack alignment
	STACK_ALIGNMENT = 16

	// Maximum argument count passed in registers
	MAX_REG_ARGS = 8

	// Stack frame header size (FP + LR)
	FRAME_HEADER_SIZE = 16
)

// GetArgumentRegister returns the register used for the n-th argument
func GetArgumentRegister(n int) obj.Addr {
	if n < 0 || n >= MAX_REG_ARGS {
		panic("argument index out of range")
	}
	return Reg("R" + string(rune('0'+n)))
}

// GetReturnRegister returns the register used for the n-th return value
func GetReturnRegister(n int) obj.Addr {
	if n < 0 || n >= 2 {
		panic("return index out of range")
	}
	return Reg("R" + string(rune('0'+n)))
}