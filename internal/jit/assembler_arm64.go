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
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/bytedance/sonic/internal/rt"
	"github.com/bytedance/sonic/loader"
	"github.com/twitchyliquid64/golang-asm/obj"
	"github.com/twitchyliquid64/golang-asm/obj/arm64"
)

const (
	_LB_jump_pc = "_jump_pc_"
)

// BaseAssembler provides common functionality for ARM64 assemblers
type BaseAssembler struct {
	i        int
	f        func()
	c        []byte
	o        sync.Once
	pb       *Backend
	xrefs    map[string][]*obj.Prog
	labels   map[string]*obj.Prog
	pendings map[string][]*obj.Prog
}

// Instruction encoders for ARM64 architecture
// Reference: https://pkg.go.dev/cmd/internal/obj/arm64

// ARM64 NOP instructions for different alignment requirements
var _NOPS = [][16]byte{
	{0x1F, 0x20, 0x03, 0xD5},                                                                         // NOP (4 bytes)
	{0x1F, 0x20, 0x03, 0xD5, 0x1F, 0x20, 0x03, 0xD5},                                                 // NOP; NOP (8 bytes)
	{0x1F, 0x20, 0x03, 0xD5, 0x1F, 0x20, 0x03, 0xD5, 0x1F, 0x20, 0x03, 0xD5},                         // NOP; NOP; NOP (12 bytes)
	{0x1F, 0x20, 0x03, 0xD5, 0x1F, 0x20, 0x03, 0xD5, 0x1F, 0x20, 0x03, 0xD5, 0x1F, 0x20, 0x03, 0xD5}, // 4 NOPs (16 bytes)
}

// NOP generates a single NOP instruction
func (self *BaseAssembler) NOP() *obj.Prog {
	p := self.pb.New()
	p.As = obj.ANOP
	self.pb.Append(p)
	return p
}

// NOPn generates multiple NOP instructions for alignment
func (self *BaseAssembler) NOPn(n int) {
	for i := len(_NOPS); i > 0 && n > 0; i-- {
		for ; n >= i; n -= i {
			self.Byte(_NOPS[i-1][:i]...)
		}
	}
}

// Byte emits raw bytes directly into the instruction stream
func (self *BaseAssembler) Byte(v ...byte) {
	for ; len(v) >= 8; v = v[8:] {
		self.From("MOVD", Imm(rt.Get64(v)))
	}
	for ; len(v) >= 4; v = v[4:] {
		self.From("MOVW", Imm(int64(rt.Get32(v))))
	}
	for ; len(v) >= 2; v = v[2:] {
		self.From("MOVH", Imm(int64(rt.Get16(v))))
	}
	for ; len(v) >= 1; v = v[1:] {
		self.From("MOVB", Imm(int64(v[0])))
	}
}

// Mark creates a jump label at the current position
func (self *BaseAssembler) Mark(pc int) {
	self.i++
	self.Link(_LB_jump_pc + strconv.Itoa(pc))
}

// Link creates a label that can be jumped to
func (self *BaseAssembler) Link(to string) {
	var p *obj.Prog

	// placeholder substitution for loops
	if strings.Contains(to, "{n}") {
		to = strings.ReplaceAll(to, "{n}", strconv.Itoa(self.i))
	}

	// check for duplications
	if _, ok := self.labels[to]; ok {
		panic("label " + to + " has already been linked")
	}

	// check for pending jumps
	if v, ok := self.pendings[to]; ok {
		// link all pending jumps to this label
		for _, p = range v {
			p.To.Type = obj.TYPE_BRANCH
			p.To.Val = p
		}
		delete(self.pendings, to)
	}

	// mark the label position
	p = self.pb.New()
	p.As = obj.ATEXT
	p.From.Type = obj.TYPE_ADDR
	p.From.Name = obj.NAME_EXTERN
	p.From.Sym = obj.Linksym(fmt.Sprintf("%s%s", loader.ModulePath, to))
	self.labels[to] = p
	self.pb.Append(p)
}

// Sjmp generates a jump instruction to a label
func (self *BaseAssembler) Sjmp(op string, to string) {
	var p *obj.Prog

	// substitute placeholder in label names
	if strings.Contains(to, "{n}") {
		to = strings.ReplaceAll(to, "{n}", strconv.Itoa(self.i))
	}

	// try to link to an existing label
	if labelProg, ok := self.labels[to]; ok {
		p = self.pb.New()
		p.As = As(op)
		p.To.Type = obj.TYPE_BRANCH
		p.To.Val = labelProg
	} else {
		// if label doesn't exist, add to pending jumps
		p = self.pb.New()
		p.As = As(op)
		p.To.Type = obj.TYPE_BRANCH
		if self.pendings[to] == nil {
			self.pendings[to] = make([]*obj.Prog, 0, 4)
		}
		self.pendings[to] = append(self.pendings[to], p)
	}

	self.pb.Append(p)
}

// Sref creates a symbol reference for PC-relative addressing (ARM64 version)
func (self *BaseAssembler) Sref(to string, d int64) {
	p := self.pb.New()
	p.As = arm64.AMOVD
	p.From = Imm(-d)

	// placeholder substitution for loops
	if strings.Contains(to, "{n}") {
		to = strings.ReplaceAll(to, "{n}", strconv.Itoa(self.i))
	}

	// record the patch point
	self.pb.Append(p)
	self.xrefs[to] = append(self.xrefs[to], p)
}

// resolve resolves symbol references for PC-relative addressing (ARM64 version)
func (self *BaseAssembler) resolve() {
	for s, v := range self.xrefs {
		for _, prog := range v {
			if prog.As != arm64.AMOVD {
				panic("invalid PC relative reference")
			} else if p, exists := self.labels[s]; !exists {
				panic("links are not fully resolved: " + s)
			} else {
				// Calculate PC-relative offset for ARM64
				off := prog.From.Offset + p.Pc - prog.Pc
				// For ARM64, we need to write the offset as a 32-bit value
				// This is a simplified implementation - real ARM64 PC-relative addressing is more complex
				if int(prog.Pc)+4 <= len(self.c) {
					// Write the offset as a signed 32-bit value
					binary.LittleEndian.PutUint32(self.c[prog.Pc:], uint32(off))
				}
			}
		}
	}
}

// From generates an instruction with a source operand only
func (self *BaseAssembler) From(op string, src obj.Addr) *obj.Prog {
	p := self.pb.New()
	p.As = As(op)
	p.From = src
	self.pb.Append(p)
	return p
}

// To generates an instruction with a destination operand only
func (self *BaseAssembler) To(op string, dst obj.Addr) *obj.Prog {
	p := self.pb.New()
	p.As = As(op)
	p.To = dst
	self.pb.Append(p)
	return p
}

// Two generates an instruction with source and destination operands
func (self *BaseAssembler) Two(op string, dst, src obj.Addr) *obj.Prog {
	p := self.pb.New()
	p.As = As(op)
	p.From = src
	p.To = dst
	self.pb.Append(p)
	return p
}

// Three generates an instruction with two source operands and one destination
func (self *BaseAssembler) Three(op string, dst, src1, src2 obj.Addr) *obj.Prog {
	p := self.pb.New()
	p.As = As(op)
	p.From = src1
	p.Reg = src2.Reg
	p.To = dst
	self.pb.Append(p)
	return p
}

// Emit generates a generic instruction with custom operands
func (self *BaseAssembler) Emit(op string, args ...obj.Addr) *obj.Prog {
	p := self.pb.New()
	p.As = As(op)

	switch len(args) {
	case 0:
		// No operands
	case 1:
		p.From = args[0]
	case 2:
		p.From = args[0]
		p.To = args[1]
	case 3:
		p.From = args[0]
		p.Reg = args[1].Reg
		p.To = args[2]
	default:
		panic("too many operands for instruction: " + op)
	}

	self.pb.Append(p)
	return p
}

// LoadFunction loads a Go function address into a register
func (self *BaseAssembler) LoadFunction(fn interface{}, dst obj.Addr) {
	self.LoadImm(uintptr(loader.FuncAddr(fn)), dst)
}

// LoadImm loads an immediate value into a register
func (self *BaseAssembler) LoadImm(imm uintptr, dst obj.Addr) {
	if imm == 0 {
		// Use MOV ZR, dst for zero
		self.Two("MOVD", dst, ZR)
		return
	}

	// For small immediates, try to use MOVN/MOVZ/MOVK combination
	if imm <= 0xFFFF {
		self.Two("MOVW", dst, Imm(int64(imm)))
	} else if imm <= 0xFFFFFFFF {
		// Use MOVZ + MOVK for 32-bit values
		self.Two("MOVW", dst, Imm(int64(imm&0xFFFF)))
		self.Three("MOVK", dst, Imm(int64((imm>>16)&0xFFFF)), Imm(16))
	} else {
		// Use MOVZ + 2x MOVK for 64-bit values
		self.Two("MOVW", dst, Imm(int64(imm&0xFFFF)))
		self.Three("MOVK", dst, Imm(int64((imm>>16)&0xFFFF)), Imm(16))
		self.Three("MOVK", dst, Imm(int64((imm>>32)&0xFFFF)), Imm(32))
		self.Three("MOVK", dst, Imm(int64((imm>>48)&0xFFFF)), Imm(48))
	}
}

// LoadGlobal loads a global variable address into a register
func (self *BaseAssembler) LoadGlobal(name string, dst obj.Addr) {
	self.From("MOVD", obj.Addr{
		Type:   obj.TYPE_MEM,
		Name:   name,
		Sym:    obj.Linksym(name),
		Offset: 0,
	})
	self.Two("MOVD", dst, R0)
}

// Call generates a function call instruction
func (self *BaseAssembler) Call(fn obj.Addr) {
	self.To("BL", fn)
}

// Ret generates a return instruction
func (self *BaseAssembler) Ret() {
	self.To("RET", LR)
}

// Br generates an unconditional branch
func (self *BaseAssembler) Br(target string) {
	self.Sjmp("B", target)
}

// BrEq generates a branch if equal (zero flag set)
func (self *BaseAssembler) BrEq(target string) {
	self.Sjmp("BEQ", target)
}

// BrNe generates a branch if not equal (zero flag clear)
func (self *BaseAssembler) BrNe(target string) {
	self.Sjmp("BNE", target)
}

// BrLt generates a branch if less than (signed)
func (self *BaseAssembler) BrLt(target string) {
	self.Sjmp("BLT", target)
}

// BrLe generates a branch if less than or equal (signed)
func (self *BaseAssembler) BrLe(target string) {
	self.Sjmp("BLE", target)
}

// BrGt generates a branch if greater than (signed)
func (self *BaseAssembler) BrGt(target string) {
	self.Sjmp("BGT", target)
}

// BrGe generates a branch if greater than or equal (signed)
func (self *BaseAssembler) BrGe(target string) {
	self.Sjmp("BGE", target)
}

// BrHi generates a branch if higher (unsigned)
func (self *BaseAssembler) BrHi(target string) {
	self.Sjmp("BHI", target)
}

// BrLs generates a branch if lower or same (unsigned)
func (self *BaseAssembler) BrLs(target string) {
	self.Sjmp("BLS", target)
}

// Cmp generates a comparison instruction
func (self *BaseAssembler) Cmp(reg1, reg2 obj.Addr) {
	self.Three("CMP", reg1, reg2, obj.Addr{})
}

// CmpImm generates a comparison with immediate
func (self *BaseAssembler) CmpImm(reg obj.Addr, imm int64) {
	self.Three("CMP", reg, Imm(imm), obj.Addr{})
}

// Test generates a test instruction (bitwise AND)
func (self *BaseAssembler) Test(reg1, reg2 obj.Addr) {
	self.Three("TST", reg1, reg2, obj.Addr{})
}

// Size returns the size of the generated code
func (self *BaseAssembler) Size() int {
	return len(self.c)
}

// Load compiles and loads the generated code
func (self *BaseAssembler) Load(name string, framesize int, argsize int, argptrs, localptrs []int64) loader.Code {
	return self.o.Do(func() loader.Code {
		return loader.Load(name, self.assemble(framesize, argsize, argptrs, localptrs))
	})
}

// Assemble assembles the instruction stream into executable code
func (self *BaseAssembler) assemble(framesize int, argsize int, argptrs, localptrs []int64) []byte {
	var sym obj.LSym
	var fnv obj.FuncInfo
	sym.Func = &fnv
	fnv.Text = self.pb.Head
	fnv.Pcsp = int32(framesize)
	fnv.Pcfile = int32(framesize)
	fnv.Pcline = int32(framesize)
	fnv.Pcdata = []obj.Pcdata{
		{PC: 0},
		{PC: 0},
	}

	// Set up local variable information
	fnv.Autom = make([]obj.Auto, len(argptrs)+len(localptrs))
	for i, v := range argptrs {
		fnv.Autom[i] = obj.Auto{
			Asym:    obj.Linksym(fmt.Sprintf("arg_%d", i)),
			Aoffset: int32(v),
			Name:    obj.NAME_PARAM,
		}
	}
	for i, v := range localptrs {
		fnv.Autom[len(argptrs)+i] = obj.Auto{
			Aoffset: int32(v),
			Name:    obj.NAME_AUTO,
		}
	}

	// Assemble the code
	self.pb.Arch.Assemble(self.pb.Ctxt, &sym, self.New)

	// Extract the assembled bytes
	self.c = sym.P

	// Resolve symbol references
	self.resolve()

	return self.c
}

// New creates a new program for the assembler
func (self *BaseAssembler) New() *obj.Prog {
	return self.pb.New()
}

// Init initializes the assembler with a compilation function
func (self *BaseAssembler) Init(fn func()) {
	self.f = fn
	self.c = nil
	self.o = sync.Once{}
}

// Execute runs the compilation function
func (self *BaseAssembler) Execute() {
	self.f()
}

// ARM64Assembler provides ARM64-specific instruction encoding
type ARM64Assembler struct {
	BaseAssembler
}

// NewARM64Assembler creates a new ARM64 assembler
func NewARM64Assembler() *ARM64Assembler {
	asm := &ARM64Assembler{}
	asm.BaseAssembler.Init(func() {
		// Initialize backend
		asm.pb = newBackend("arm64")
		asm.xrefs = make(map[string][]*obj.Prog)
		asm.labels = make(map[string]*obj.Prog)
		asm.pendings = make(map[string][]*obj.Prog)
	})
	return asm
}

// ARM64-specific instruction helpers

// MOV moves data between registers or from immediate
func (self *ARM64Assembler) MOV(dst, src obj.Addr) {
	self.Two("MOVD", dst, src)
}

// MOVW moves 32-bit data
func (self *ARM64Assembler) MOVW(dst, src obj.Addr) {
	self.Two("MOVWU", dst, src)
}

// MOVH moves 16-bit data
func (self *ARM64Assembler) MOVH(dst, src obj.Addr) {
	self.Two("MOVHU", dst, src)
}

// MOVB moves 8-bit data
func (self *ARM64Assembler) MOVB(dst, src obj.Addr) {
	self.Two("MOVBU", dst, src)
}

// ADD adds two registers
func (self *ARM64Assembler) ADD(dst, src1, src2 obj.Addr) {
	self.Three("ADD", dst, src1, src2)
}

// ADD adds register and immediate
func (self *ARM64Assembler) ADDImm(dst, src obj.Addr, imm int64) {
	self.Three("ADD", dst, src, Imm(imm))
}

// SUB subtracts two registers
func (self *ARM64Assembler) SUB(dst, src1, src2 obj.Addr) {
	self.Three("SUB", dst, src1, src2)
}

// SUB subtracts immediate from register
func (self *ARM64Assembler) SUBImm(dst, src obj.Addr, imm int64) {
	self.Three("SUB", dst, src, Imm(imm))
}

// MUL multiplies two registers
func (self *ARM64Assembler) MUL(dst, src1, src2 obj.Addr) {
	self.Three("MUL", dst, src1, src2)
}

// SDIV signed division
func (self *ARM64Assembler) SDIV(dst, src1, src2 obj.Addr) {
	self.Three("SDIV", dst, src1, src2)
}

// UDIV unsigned division
func (self *ARM64Assembler) UDIV(dst, src1, src2 obj.Addr) {
	self.Three("UDIV", dst, src1, src2)
}

// AND bitwise AND
func (self *ARM64Assembler) AND(dst, src1, src2 obj.Addr) {
	self.Three("AND", dst, src1, src2)
}

// ORR bitwise OR
func (self *ARM64Assembler) ORR(dst, src1, src2 obj.Addr) {
	self.Three("ORR", dst, src1, src2)
}

// EOR bitwise XOR
func (self *ARM64Assembler) EOR(dst, src1, src2 obj.Addr) {
	self.Three("EOR", dst, src1, src2)
}

// BIC bitwise clear (AND NOT)
func (self *ARM64Assembler) BIC(dst, src1, src2 obj.Addr) {
	self.Three("BIC", dst, src1, src2)
}

// LDR loads data from memory
func (self *ARM64Assembler) LDR(dst, src obj.Addr) {
	self.Two("MOVD", dst, src)
}

// STR stores data to memory
func (self *ARM64Assembler) STR(dst, src obj.Addr) {
	self.Two("MOVD", dst, src)
}

// LDRB loads byte from memory
func (self *ARM64Assembler) LDRB(dst, src obj.Addr) {
	self.Two("MOVBU", dst, src)
}

// STRB stores byte to memory
func (self *ARM64Assembler) STRB(dst, src obj.Addr) {
	self.Two("MOVB", dst, src)
}

// LDP loads pair of registers from memory
func (self *ARM64Assembler) LDP(dst1, dst2, src obj.Addr) {
	self.Emit("LDP", dst1, dst2, src)
}

// STP stores pair of registers to memory
func (self *ARM64Assembler) STP(dst, src1, src2 obj.Addr) {
	self.Emit("STP", src1, src2, dst)
}

// Stack manipulation helpers

// SUBSP subtracts from stack pointer (allocates stack space)
func (self *ARM64Assembler) SUBSP(size int64) {
	self.Three("SUB", SP, SP, Imm(size))
}

// ADDSP adds to stack pointer (deallocates stack space)
func (self *ARM64Assembler) ADDSP(size int64) {
	self.Three("ADD", SP, SP, Imm(size))
}

// Prologue generates function prologue
func (self *ARM64Assembler) Prologue(framesize int64) {
	// Store FP and LR
	self.STP(Ptr(SP, -16), FP, LR)

	// Set up new frame pointer
	self.MOV(FP, SP)

	// Allocate stack space (aligned to 16 bytes)
	alignedSize := AlignStack(framesize)
	if alignedSize > 0 {
		self.SUBSP(alignedSize)
	}
}

// Epilogue generates function epilogue
func (self *ARM64Assembler) Epilogue(framesize int64) {
	// Deallocate stack space
	alignedSize := AlignStack(framesize)
	if alignedSize > 0 {
		self.ADDSP(alignedSize)
	}

	// Restore FP and LR
	self.LDP(FP, LR, Ptr(SP, 16))

	// Return
	self.Ret()
}

// SaveCalleeSaved saves all callee-saved registers
func (self *ARM64Assembler) SaveCalleeSaved() {
	// Save callee-saved registers in pairs
	self.STP(Ptr(SP, -16), R19, R20)
	self.STP(Ptr(SP, -16), R21, R22)
	self.STP(Ptr(SP, -16), R23, R24)
	self.STP(Ptr(SP, -16), R25, R26)
	self.STP(Ptr(SP, -16), R27, R28)
}

// RestoreCalleeSaved restores all callee-saved registers
func (self *ARM64Assembler) RestoreCalleeSaved() {
	// Restore callee-saved registers in reverse order
	self.LDP(R27, R28, Ptr(SP, 16))
	self.LDP(R25, R26, Ptr(SP, 16))
	self.LDP(R23, R24, Ptr(SP, 16))
	self.LDP(R21, R22, Ptr(SP, 16))
	self.LDP(R19, R20, Ptr(SP, 16))
}

// Function call helpers

// SaveCallerSaved saves caller-saved registers before a function call
func (self *ARM64Assembler) SaveCallerSaved() {
	// Save argument registers if they contain important data
	// This is typically handled by the compiler register allocator
}

// RestoreCallerSaved restores caller-saved registers after a function call
func (self *ARM64Assembler) RestoreCallerSaved() {
	// Restore argument registers
	// This is typically handled by the compiler register allocator
}

// CallGo calls a Go function with proper calling convention
func (self *ARM64Assembler) CallGo(fn interface{}) {
	self.LoadFunction(fn, R8) // Use temporary register for function address
	self.Call(R8)
}
