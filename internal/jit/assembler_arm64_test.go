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
	"github.com/twitchyliquid64/golang-asm/obj/arm64"
)

// TestARM64Assembler tests the ARM64 assembler functionality
func TestARM64AssemblerCreation(t *testing.T) {
	assembler := NewARM64Assembler()

	if assembler == nil {
		t.Fatal("Expected non-nil assembler")
	}

	if assembler.pb == nil {
		t.Error("Expected non-nil backend")
	}

	if assembler.xrefs == nil {
		t.Error("Expected non-nil xrefs map")
	}

	if assembler.labels == nil {
		t.Error("Expected non-nil labels map")
	}

	if assembler.pendings == nil {
		t.Error("Expected non-nil pendings map")
	}
}

func TestARM64AssemblerNOP(t *testing.T) {
	assembler := NewARM64Assembler()

	p := assembler.NOP()
	if p.As != obj.ANOP {
		t.Errorf("Expected NOP instruction, got %v", p.As)
	}

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after NOP")
	}
}

func TestARM64AssemblerNOPn(t *testing.T) {
	assembler := NewARM64Assembler()

	assembler.NOPn(5)
	size := assembler.Size()

	if size == 0 {
		t.Error("Expected non-zero size after NOPn")
	}

	// Size should be at least 4 bytes (one NOP instruction)
	if size < 4 {
		t.Errorf("Expected size >= 4 bytes, got %d", size)
	}
}

func TestARM64AssemblerByte(t *testing.T) {
	assembler := NewARM64Assembler()

	bytes := []byte{0x01, 0x02, 0x03, 0x04}
	assembler.Byte(bytes...)

	size := assembler.Size()
	if size == 0 {
		t.Error("Expected non-zero size after Byte")
	}
}

func TestARM64AssemblerLink(t *testing.T) {
	assembler := NewARM64Assembler()

	label := "test_label"
	assembler.Link(label)

	if _, ok := assembler.labels[label]; !ok {
		t.Errorf("Expected label %s to exist", label)
	}
}

func TestARM64AssemblerLinkDuplicate(t *testing.T) {
	assembler := NewARM64Assembler()

	label := "test_label"
	assembler.Link(label)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for duplicate label")
		}
	}()

	assembler.Link(label)
}

func TestARM64AssemblerSjmp(t *testing.T) {
	assembler := NewARM64Assembler()

	target := "test_target"
	assembler.Sjmp("B", target)

	// Should create a pending jump
	if len(assembler.pendings) == 0 {
		t.Error("Expected pending jumps")
	}

	if _, ok := assembler.pendings[target]; !ok {
		t.Errorf("Expected pending jump to target %s", target)
	}
}

func TestARM64AssemblerFrom(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src := Imm(42)

	p := assembler.From("MOVD", src)
	if p.As != As("MOVD") {
		t.Errorf("Expected MOVD instruction, got %v", p.As)
	}

	if p.From.Offset != src.Offset {
		t.Errorf("Expected source offset %d, got %d", src.Offset, p.From.Offset)
	}
}

func TestARM64AssemblerTo(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0

	p := assembler.To("RET", dst)
	if p.As != As("RET") {
		t.Errorf("Expected RET instruction, got %v", p.As)
	}

	if p.To.Reg != dst.Reg {
		t.Errorf("Expected destination register %v, got %v", dst.Reg, p.To.Reg)
	}
}

func TestARM64AssemblerTwo(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src := R1

	p := assembler.Two("MOVD", dst, src)
	if p.As != As("MOVD") {
		t.Errorf("Expected MOVD instruction, got %v", p.As)
	}

	if p.To.Reg != dst.Reg {
		t.Errorf("Expected destination register %v, got %v", dst.Reg, p.To.Reg)
	}

	if p.From.Reg != src.Reg {
		t.Errorf("Expected source register %v, got %v", src.Reg, p.From.Reg)
	}
}

func TestARM64AssemblerThree(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src1 := R1
	src2 := R2

	p := assembler.Three("ADD", dst, src1, src2)
	if p.As != As("ADD") {
		t.Errorf("Expected ADD instruction, got %v", p.As)
	}

	if p.To.Reg != dst.Reg {
		t.Errorf("Expected destination register %v, got %v", dst.Reg, p.To.Reg)
	}

	if p.From.Reg != src1.Reg {
		t.Errorf("Expected source register 1 %v, got %v", src1.Reg, p.From.Reg)
	}

	if p.Reg != src2.Reg {
		t.Errorf("Expected source register 2 %v, got %v", src2.Reg, p.Reg)
	}
}

func TestARM64AssemblerEmit(t *testing.T) {
	assembler := NewARM64Assembler()

	// Test with 1 operand
	p1 := assembler.Emit("RET", LR)
	if p1.As != As("RET") {
		t.Errorf("Expected RET instruction, got %v", p1.As)
	}

	// Test with 2 operands
	p2 := assembler.Emit("MOVD", R0, R1)
	if p2.As != As("MOVD") {
		t.Errorf("Expected MOVD instruction, got %v", p2.As)
	}

	// Test with 3 operands
	p3 := assembler.Emit("ADD", R0, R1, R2)
	if p3.As != As("ADD") {
		t.Errorf("Expected ADD instruction, got %v", p3.As)
	}
}

func TestARM64AssemblerLoadImm(t *testing.T) {
	assembler := NewARM64Assembler()

	// Test loading zero
	assembler.LoadImm(0, R0)

	// Test loading small immediate
	assembler.LoadImm(0x1234, R1)

	// Test loading large immediate
	assembler.LoadImm(0x123456789ABCDEF0, R2)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after LoadImm")
	}
}

func TestARM64AssemblerLoadFunction(t *testing.T) {
	assembler := NewARM64Assembler()

	testFunc := func() {}
	assembler.LoadFunction(testFunc, R0)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after LoadFunction")
	}
}

func TestARM64AssemblerBr(t *testing.T) {
	assembler := NewARM64Assembler()

	target := "branch_target"
	assembler.Br(target)

	if len(assembler.pendings) == 0 {
		t.Error("Expected pending branch")
	}
}

func TestARM64AssemblerBrEq(t *testing.T) {
	assembler := NewARM64Assembler()

	target := "eq_target"
	assembler.BrEq(target)

	if len(assembler.pendings) == 0 {
		t.Error("Expected pending conditional branch")
	}
}

func TestARM64AssemblerCmp(t *testing.T) {
	assembler := NewARM64Assembler()

	reg1 := R0
	reg2 := R1

	assembler.Cmp(reg1, reg2)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after Cmp")
	}
}

func TestARM64AssemblerCmpImm(t *testing.T) {
	assembler := NewARM64Assembler()

	reg := R0
	imm := int64(42)

	assembler.CmpImm(reg, imm)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after CmpImm")
	}
}

func TestARM64AssemblerTest(t *testing.T) {
	assembler := NewARM64Assembler()

	reg1 := R0
	reg2 := R1

	assembler.Test(reg1, reg2)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after Test")
	}
}

// ARM64 specific instruction tests
func TestARM64AssemblerMOV(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src := R1

	assembler.MOV(dst, src)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after MOV")
	}
}

func TestARM64AssemblerMOVW(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src := R1

	assembler.MOVW(dst, src)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after MOVW")
	}
}

func TestARM64AssemblerADD(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src1 := R1
	src2 := R2

	assembler.ADD(dst, src1, src2)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after ADD")
	}
}

func TestARM64AssemblerADDImm(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src := R1
	imm := int64(5)

	assembler.ADDImm(dst, src, imm)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after ADDImm")
	}
}

func TestARM64AssemblerSUB(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src1 := R1
	src2 := R2

	assembler.SUB(dst, src1, src2)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after SUB")
	}
}

func TestARM64AssemblerSUBImm(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src := R1
	imm := int64(5)

	assembler.SUBImm(dst, src, imm)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after SUBImm")
	}
}

func TestARM64AssemblerMUL(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src1 := R1
	src2 := R2

	assembler.MUL(dst, src1, src2)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after MUL")
	}
}

func TestARM64AssemblerAND(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src1 := R1
	src2 := R2

	assembler.AND(dst, src1, src2)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after AND")
	}
}

func TestARM64AssemblerORR(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src1 := R1
	src2 := R2

	assembler.ORR(dst, src1, src2)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after ORR")
	}
}

func TestARM64AssemblerEOR(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src1 := R1
	src2 := R2

	assembler.EOR(dst, src1, src2)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after EOR")
	}
}

func TestARM64AssemblerLDR(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src := Ptr(R1, 8)

	assembler.LDR(dst, src)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after LDR")
	}
}

func TestARM64AssemblerSTR(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := Ptr(R0, 8)
	src := R1

	assembler.STR(dst, src)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after STR")
	}
}

func TestARM64AssemblerLDRB(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := R0
	src := Ptr(R1, 8)

	assembler.LDRB(dst, src)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after LDRB")
	}
}

func TestARM64AssemblerSTRB(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := Ptr(R0, 8)
	src := R1

	assembler.STRB(dst, src)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after STRB")
	}
}

func TestARM64AssemblerLDP(t *testing.T) {
	assembler := NewARM64Assembler()

	dst1 := R0
	dst2 := R1
	src := Ptr(R2, 8)

	assembler.LDP(dst1, dst2, src)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after LDP")
	}
}

func TestARM64AssemblerSTP(t *testing.T) {
	assembler := NewARM64Assembler()

	dst := Ptr(R0, 8)
	src1 := R1
	src2 := R2

	assembler.STP(dst, src1, src2)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after STP")
	}
}

func TestARM64AssemblerSUBSP(t *testing.T) {
	assembler := NewARM64Assembler()

	size := int64(32)
	assembler.SUBSP(size)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after SUBSP")
	}
}

func TestARM64AssemblerADDSP(t *testing.T) {
	assembler := NewARM64Assembler()

	size := int64(32)
	assembler.ADDSP(size)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after ADDSP")
	}
}

func TestARM64AssemblerPrologue(t *testing.T) {
	assembler := NewARM64Assembler()

	frameSize := int64(32)
	assembler.Prologue(frameSize)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after Prologue")
	}
}

func TestARM64AssemblerEpilogue(t *testing.T) {
	assembler := NewARM64Assembler()

	frameSize := int64(32)
	assembler.Epilogue(frameSize)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after Epilogue")
	}
}

func TestARM64AssemblerSaveCalleeSaved(t *testing.T) {
	assembler := NewARM64Assembler()

	assembler.SaveCalleeSaved()

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after SaveCalleeSaved")
	}
}

func TestARM64AssemblerRestoreCalleeSaved(t *testing.T) {
	assembler := NewARM64Assembler()

	assembler.RestoreCalleeSaved()

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after RestoreCalleeSaved")
	}
}

func TestARM64AssemblerCallGo(t *testing.T) {
	assembler := NewARM64Assembler()

	testFunc := func() {}
	assembler.CallGo(testFunc)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size after CallGo")
	}
}

func TestARM64AssemblerComplexFunction(t *testing.T) {
	assembler := NewARM64Assembler()

	// Generate a simple function that adds two numbers
	frameSize := int64(32)

	// Prologue
	assembler.Prologue(frameSize)

	// Save arguments
	assembler.MOV(R19, ARG0) // Save first argument
	assembler.MOV(R20, ARG1) // Save second argument

	// Perform addition
	assembler.ADD(R21, R19, R20)

	// Move result to return register
	assembler.MOV(RET0, R21)

	// Epilogue
	assembler.Epilogue(frameSize)

	if assembler.Size() == 0 {
		t.Error("Expected non-zero size for complex function")
	}

	// Should have generated multiple instructions
	if assembler.Size() < 20 {
		t.Errorf("Expected at least 20 bytes for complex function, got %d", assembler.Size())
	}
}

// Error handling tests
func TestARM64AssemblerEmitTooManyOperands(t *testing.T) {
	assembler := NewARM64Assembler()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for too many operands")
		}
	}()

	assembler.Emit("MOVD", R0, R1, R2, R3, R4) // Too many operands
}

// Benchmark tests
func BenchmarkARM64AssemblerNOP(b *testing.B) {
	assembler := NewARM64Assembler()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.NOP()
	}
}

func BenchmarkARM64AssemblerMOV(b *testing.B) {
	assembler := NewARM64Assembler()
	dst := R0
	src := R1

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.MOV(dst, src)
	}
}

func BenchmarkARM64AssemblerADD(b *testing.B) {
	assembler := NewARM64Assembler()
	dst := R0
	src1 := R1
	src2 := R2

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.ADD(dst, src1, src2)
	}
}

func BenchmarkARM64AssemblerLoadImm(b *testing.B) {
	assembler := NewARM64Assembler()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.LoadImm(uintptr(i), R0)
	}
}

func BenchmarkARM64AssemblerLink(b *testing.B) {
	assembler := NewARM64Assembler()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.Link("label_" + string(rune('a'+i%26)))
	}
}

func BenchmarkARM64AssemblerPrologue(b *testing.B) {
	assembler := NewARM64Assembler()
	frameSize := int64(32)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.Prologue(frameSize)
	}
}

func BenchmarkARM64AssemblerEpilogue(b *testing.B) {
	assembler := NewARM64Assembler()
	frameSize := int64(32)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assembler.Epilogue(frameSize)
	}
}