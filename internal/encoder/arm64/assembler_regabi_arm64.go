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
	"strconv"
	"unsafe"

	"github.com/bytedance/sonic/internal/encoder/alg"
	"github.com/bytedance/sonic/internal/encoder/ir"
	"github.com/bytedance/sonic/internal/encoder/prim"
	"github.com/bytedance/sonic/internal/encoder/vars"
	"github.com/bytedance/sonic/internal/jit"
	"github.com/bytedance/sonic/loader"
	"github.com/twitchyliquid64/golang-asm/obj"

	"github.com/bytedance/sonic/internal/native"
	"github.com/bytedance/sonic/internal/rt"
)

/** ARM64 Register Allocations
 *
 *  State Registers:
 *
 *      X19 : stack base
 *      X20 : result pointer
 *      X21 : result length
 *      X22 : result capacity
 *      X23 : sp->p
 *      X24 : sp->q
 *      X25 : sp->x
 *      X26 : sp->f
 *
 *  Error Registers:
 *
 *      X27 : error type register
 *      X28 : error pointer register
 *
 *  Temporary Registers:
 *
 *      X0-X8 : argument/return registers (caller-saved)
 *      X9-X15 : temporary registers (caller-saved)
 *      X16-X17 : intra-procedure-call temporaries (caller-saved)
 *      X18 : platform register (callee-saved)
 */

/** Function Prototype & Stack Map
 *
 *  func (buf *[]byte, p unsafe.Pointer, sb *_Stack, fv uint64) (err error)
 *
 *  buf    :   (FP)
 *  p      :  8(FP)
 *  sb     : 16(FP)
 *  fv     : 24(FP)
 *  err.vt : 32(FP)
 *  err.vp : 40(FP)
 */

const (
	_S_cond = iota
	_S_init
)

const (
	_FP_args   = 32 // 32 bytes for spill registers of arguments
	_FP_fargs  = 40 // 40 bytes for passing arguments to other Go functions
	_FP_saves  = 80 // 80 bytes for saving the registers before CALL instructions (ARM64 has more callee-saved)
	_FP_locals = 24 // 24 bytes for local variables
)

const (
	_FP_loffs = _FP_fargs + _FP_saves
	FP_offs   = _FP_loffs + _FP_locals
	_FP_size  = FP_offs + 8  // 8 bytes for the parent frame pointer
	_FP_base  = _FP_size + 8 // 8 bytes for the return address
)

const (
	_FM_exp32 = 0x7f800000
	_FM_exp64 = 0x7ff0000000000000
)

const (
	_IM_null   = 0x6c6c756e // 'null'
	_IM_true   = 0x65757274 // 'true'
	_IM_fals   = 0x736c6166 // 'fals' ('false' without the 'e')
	_IM_open   = 0x00225c22 // '"\"∅'
	_IM_array  = 0x5d5b     // '[]'
	_IM_object = 0x7d7b     // '{}'
	_IM_mulv   = -0x5555555555555555
)

const (
	_LB_more_space        = "_more_space"
	_LB_more_space_return = "_more_space_return_"
)

const (
	_LB_error                 = "_error"
	_LB_error_too_deep        = "_error_too_deep"
	_LB_error_invalid_number  = "_error_invalid_number"
	_LB_error_nan_or_infinite = "_error_nan_or_infinite"
	_LB_panic                 = "_panic"
)

// ARM64 Register definitions
var (
	// Argument/Return registers (caller-saved)
	_ARG0 = jit.R0
	_ARG1 = jit.R1
	_ARG2 = jit.R2
	_ARG3 = jit.R3
	_ARG4 = jit.R4
	_ARG5 = jit.R5
	_ARG6 = jit.R6
	_ARG7 = jit.R7

	_RET0 = jit.R0
	_RET1 = jit.R1

	// Temporary registers (caller-saved)
	_TEMP0 = jit.R8
	_TEMP1 = jit.R9
	_TEMP2 = jit.R10
	_TEMP3 = jit.R11
	_TEMP4 = jit.R12
	_TEMP5 = jit.R13
	_TEMP6 = jit.R14
	_TEMP7 = jit.R15

	// Callee-saved registers for state management
	_ST = jit.R19 // stack base
	_RP = jit.R20 // result pointer
	_RL = jit.R21 // result length
	_RC = jit.R22 // result capacity

	// Error registers
	_ET = jit.R27 // error type
	_EP = jit.R28 // error pointer

	// Stack pointer registers
	_SP_p = jit.R23 // sp->p
	_SP_q = jit.R24 // sp->q
	_SP_x = jit.R25 // sp->x
	_SP_f = jit.R26 // sp->f

	// Frame pointer and link register
	_FP_REG = jit.FP // frame pointer
	_LR_REG = jit.LR // link register

	// Zero register
	_ZR = jit.ZR // zero register
)

// Argument locations on stack
var (
	_ARG_rb = jit.Ptr(jit.SP, _FP_base)
	_ARG_vp = jit.Ptr(jit.SP, _FP_base+8)
	_ARG_sb = jit.Ptr(jit.SP, _FP_base+16)
	_ARG_fv = jit.Ptr(jit.SP, _FP_base+24)
)

// Return value locations
var (
	_RET_et = _ET
	_RET_ep = _EP
)

// Local variable locations
var (
	_VAR_sp = jit.Ptr(jit.SP, _FP_fargs+_FP_saves)
	_VAR_dn = jit.Ptr(jit.SP, _FP_fargs+_FP_saves+8)
	_VAR_vp = jit.Ptr(jit.SP, _FP_fargs+_FP_saves+16)
)

// Register sets for different purposes
var (
	_REG_ffi = []obj.Addr{_ARG0, _ARG1, _ARG2, _ARG3, _ARG4, _ARG5, _ARG6, _ARG7}
	_REG_b64 = []obj.Addr{_SP_p, _SP_q}

	_REG_all = []obj.Addr{_ST, _SP_x, _SP_f, _SP_p, _SP_q, _RP, _RL, _RC}
	_REG_ms  = []obj.Addr{_ST, _SP_x, _SP_f, _SP_p, _SP_q, _TEMP0}
	_REG_enc = []obj.Addr{_ST, _SP_x, _SP_f, _SP_p, _SP_q, _RL}
)

// ARM64 Assembler structure
type Assembler struct {
	Name string
	jit.BaseAssembler
	p ir.Program
	x int
}

// NewAssembler creates a new ARM64 assembler
func NewAssembler(p ir.Program) *Assembler {
	return new(Assembler).Init(p)
}

/** Assembler Interface **/

func (self *Assembler) Load() vars.Encoder {
	return ptoenc(self.BaseAssembler.Load("encode_"+self.Name, _FP_size, _FP_args, vars.ArgPtrs, vars.LocalPtrs))
}

// ptoenc converts a loader.Function to vars.Encoder
func ptoenc(p loader.Function) vars.Encoder {
	return *(*vars.Encoder)(unsafe.Pointer(&p))
}

func (self *Assembler) Init(p ir.Program) *Assembler {
	self.p = p
	self.BaseAssembler.Init(self.compile)
	return self
}

func (self *Assembler) compile() {
	self.prologue()
	self.instrs()
	self.epilogue()
	self.builtins()
}

/** Assembler Stages **/

var _OpFuncTab = [256]func(*Assembler, *ir.Instr){
	ir.OP_null:           (*Assembler)._asm_OP_null,
	ir.OP_empty_arr:      (*Assembler)._asm_OP_empty_arr,
	ir.OP_empty_obj:      (*Assembler)._asm_OP_empty_obj,
	ir.OP_bool:           (*Assembler)._asm_OP_bool,
	ir.OP_i8:             (*Assembler)._asm_OP_i8,
	ir.OP_i16:            (*Assembler)._asm_OP_i16,
	ir.OP_i32:            (*Assembler)._asm_OP_i32,
	ir.OP_i64:            (*Assembler)._asm_OP_i64,
	ir.OP_u8:             (*Assembler)._asm_OP_u8,
	ir.OP_u16:            (*Assembler)._asm_OP_u16,
	ir.OP_u32:            (*Assembler)._asm_OP_u32,
	ir.OP_u64:            (*Assembler)._asm_OP_u64,
	ir.OP_f32:            (*Assembler)._asm_OP_f32,
	ir.OP_f64:            (*Assembler)._asm_OP_f64,
	ir.OP_str:            (*Assembler)._asm_OP_str,
	ir.OP_bin:            (*Assembler)._asm_OP_bin,
	ir.OP_quote:          (*Assembler)._asm_OP_quote,
	ir.OP_number:         (*Assembler)._asm_OP_number,
	ir.OP_eface:          (*Assembler)._asm_OP_eface,
	ir.OP_iface:          (*Assembler)._asm_OP_iface,
	ir.OP_byte:           (*Assembler)._asm_OP_byte,
	ir.OP_text:           (*Assembler)._asm_OP_text,
	ir.OP_deref:          (*Assembler)._asm_OP_deref,
	ir.OP_index:          (*Assembler)._asm_OP_index,
	ir.OP_load:           (*Assembler)._asm_OP_load,
	ir.OP_save:           (*Assembler)._asm_OP_save,
	ir.OP_drop:           (*Assembler)._asm_OP_drop,
	ir.OP_drop_2:         (*Assembler)._asm_OP_drop_2,
	ir.OP_recurse:        (*Assembler)._asm_OP_recurse,
	ir.OP_is_nil:         (*Assembler)._asm_OP_is_nil,
	ir.OP_is_nil_p1:      (*Assembler)._asm_OP_is_nil_p1,
	ir.OP_is_zero_1:      (*Assembler)._asm_OP_is_zero_1,
	ir.OP_is_zero_2:      (*Assembler)._asm_OP_is_zero_2,
	ir.OP_is_zero_4:      (*Assembler)._asm_OP_is_zero_4,
	ir.OP_is_zero_8:      (*Assembler)._asm_OP_is_zero_8,
	ir.OP_is_zero_map:    (*Assembler)._asm_OP_is_zero_map,
	ir.OP_goto:           (*Assembler)._asm_OP_goto,
	ir.OP_map_iter:       (*Assembler)._asm_OP_map_iter,
	ir.OP_map_stop:       (*Assembler)._asm_OP_map_stop,
	ir.OP_map_check_key:  (*Assembler)._asm_OP_map_check_key,
	ir.OP_map_write_key:  (*Assembler)._asm_OP_map_write_key,
	ir.OP_map_value_next: (*Assembler)._asm_OP_map_value_next,
	ir.OP_slice_len:      (*Assembler)._asm_OP_slice_len,
	ir.OP_slice_next:     (*Assembler)._asm_OP_slice_next,
	ir.OP_marshal:        (*Assembler)._asm_OP_marshal,
	ir.OP_marshal_p:      (*Assembler)._asm_OP_marshal_p,
	ir.OP_marshal_text:   (*Assembler)._asm_OP_marshal_text,
	ir.OP_marshal_text_p: (*Assembler)._asm_OP_marshal_text_p,
	ir.OP_cond_set:       (*Assembler)._asm_OP_cond_set,
	ir.OP_cond_testc:     (*Assembler)._asm_OP_cond_testc,
	ir.OP_unsupported:    (*Assembler)._asm_OP_unsupported,
	ir.OP_is_zero:        (*Assembler)._asm_OP_is_zero,
}

func (self *Assembler) instr(v *ir.Instr) {
	if fn := _OpFuncTab[v.Op()]; fn != nil {
		fn(self, v)
	} else {
		panic(fmt.Sprintf("invalid opcode: %d", v.Op()))
	}
}

func (self *Assembler) instrs() {
	for i, v := range self.p {
		self.Mark(i)
		self.instr(&v)
		self.debug_instr(i, &v)
	}
}

func (self *Assembler) builtins() {
	self.more_space()
	self.error_too_deep()
	self.error_invalid_number()
	self.error_nan_or_infinite()
	self.go_panic()
}

/** ARM64 Prologue and Epilogue **/

func (self *Assembler) epilogue() {
	self.Mark(len(self.p))

	// Clear error registers
	self.Emit("MOVD", _ET, _ZR) // MOV XZR, X27 (error type)
	self.Emit("MOVD", _EP, _ZR) // MOV XZR, X28 (error pointer)

	self.Link(_LB_error)

	// Save buffer state back to arguments
	self.Emit("MOVD", _ARG_rb, _TEMP0)         // MOV rb, X0
	self.Emit("MOVD", _RL, jit.Ptr(_TEMP0, 8)) // STR RL, [X0, #8]
	self.Emit("MOVD", _ZR, _ARG_rb)            // MOV ZR, rb (clear for GC)
	self.Emit("MOVD", _ZR, _ARG_vp)            // MOV ZR, vp (clear for GC)
	self.Emit("MOVD", _ZR, _ARG_sb)            // MOV ZR, sb (clear for GC)

	// Restore frame pointer and return
	self.Emit("MOVD", jit.Ptr(_SP, FP_offs), _FP_REG) // LDR FP, [SP, #FP_offs]
	self.Emit("ADD", _SP, _SP, jit.Imm(_FP_size))     // ADD SP, SP, #_FP_size
	self.Emit("RET")                                  // RET
}

func (self *Assembler) prologue() {
	// Set up frame
	self.Emit("STP", _FP_REG, _LR_REG, jit.Ptr(_SP, -16)) // STP FP, LR, [SP, #-16]!
	self.Emit("MOVD", _SP, _FP_REG)                       // MOV FP, SP
	self.Emit("SUB", _SP, _SP, jit.Imm(_FP_size))         // SUB SP, SP, #_FP_size

	// Load arguments into registers
	self.Emit("MOVD", _ARG0, _RP)    // MOV X0, X20 (result pointer)
	self.Emit("MOVD", _ARG1, _SP_p)  // MOV X1, X23 (sp->p)
	self.Emit("MOVD", _ARG2, _ST)    // MOV X2, X19 (stack base)
	self.Emit("MOVD", _ARG3, _TEMP0) // MOV X3, X0 (flags)

	// Load buffer fields
	self.Emit("MOVD", jit.Ptr(_RP, 0), _RP)  // LDR X20, [X20] (data pointer)
	self.Emit("MOVD", jit.Ptr(_RP, 8), _RL)  // LDR X21, [X20, #8] (length)
	self.Emit("MOVD", jit.Ptr(_RP, 16), _RC) // LDR X22, [X20, #16] (capacity)

	// Initialize stack pointers
	self.Emit("MOVD", _TEMP0, _SP_x) // MOV X0, X25 (sp->x)
	self.Emit("MOVD", _ZR, _SP_x)    // MOV ZR, X25 (clear sp->x)
	self.Emit("MOVD", _ZR, _SP_f)    // MOV ZR, X26 (clear sp->f)
	self.Emit("MOVD", _ZR, _SP_q)    // MOV ZR, X24 (clear sp->q)
}

/** ARM64 Inline Functions **/

func (self *Assembler) xsave(reg ...obj.Addr) {
	for i, v := range reg {
		if i > _FP_saves/8-1 {
			panic("too many registers to save")
		} else {
			self.Emit("MOVD", v, jit.Ptr(_SP, _FP_fargs+int64(i)*8))
		}
	}
}

func (self *Assembler) xload(reg ...obj.Addr) {
	for i, v := range reg {
		if i > _FP_saves/8-1 {
			panic("too many registers to load")
		} else {
			self.Emit("MOVD", jit.Ptr(_SP, _FP_fargs+int64(i)*8), v)
		}
	}
}

func (self *Assembler) rbuf_rp() {
	// Add result length to result pointer
	self.Emit("ADD", _RP, _RP, _RL) // ADD X20, X20, X21
}

func (self *Assembler) store_int(nd int, fn obj.Addr, ins string) {
	self.check_size(nd)
	self.save_c()                            // SAVE $C_regs
	self.rbuf_rp()                           // ADD RP, RP, RL
	self.Emit(ins, jit.Ptr(_SP_p, 0), _ARG0) // $ins (SP.p), X0
	self.call_c(fn)                          // CALL_C $fn
	self.Emit("ADD", _RL, _RL, _ARG0)        // ADD X21, X21, X0
}

func (self *Assembler) store_str(s string) {
	i := 0
	m := rt.Str2Mem(s)

	/* 8-byte stores */
	for i <= len(m)-8 {
		self.Emit("MOVD", jit.Imm(rt.Get64(m[i:])), _TEMP0) // MOV $s[i:], X0
		self.Emit("MOVD", _TEMP0, jit.Ptr(_RP, int64(i)))   // STR X0, [RP, #i]
		i += 8
	}

	/* 4-byte stores */
	if i <= len(m)-4 {
		self.Emit("MOVW", jit.Imm(int64(rt.Get32(m[i:]))), _TEMP0) // MOVW $s[i:], X0
		self.Emit("MOVWU", _TEMP0, jit.Ptr(_RP, int64(i)))         // STRH X0, [RP, #i]
		i += 4
	}

	/* 2-byte stores */
	if i <= len(m)-2 {
		self.Emit("MOVH", jit.Imm(int64(rt.Get16(m[i:]))), _TEMP0) // MOVH $s[i:], X0
		self.Emit("MOVHU", _TEMP0, jit.Ptr(_RP, int64(i)))         // STRH X0, [RP, #i]
		i += 2
	}

	/* last byte */
	if i < len(m) {
		self.Emit("MOVB", jit.Imm(int64(m[i])), _TEMP0)    // MOVB $s[i:], X0
		self.Emit("MOVBU", _TEMP0, jit.Ptr(_RP, int64(i))) // STRB X0, [RP, #i]
	}
}

func (self *Assembler) check_size(n int) {
	self.check_size_rl(jit.Ptr(_RL, int64(n)))
}

func (self *Assembler) check_size_r(r obj.Addr, d int) {
	// For ARM64, we need to add RP to the register value
	self.Emit("ADD", _TEMP0, _RP, r) // ADD X0, X20, r
	self.Emit("CMP", _TEMP0, _RC)    // CMP X0, X22
	key := "_size_ok_" + strconv.Itoa(self.x)
	self.Sjmp("B.LE", key)  // BLE _size_ok_{n}
	self.slice_grow_x0(key) // GROW $key
	self.Link(key)          // _size_ok_{n}:
}

func (self *Assembler) check_size_rl(v obj.Addr) {
	idx := self.x
	key := _LB_more_space_return + strconv.Itoa(idx)

	// Check for buffer capacity: RP + RL + n <= RC
	self.Emit("ADD", _TEMP0, _RP, v) // ADD X0, X20, v
	self.Emit("CMP", _TEMP0, _RC)    // CMP X0, X22
	self.Sjmp("B.LE", key)           // BLE _more_space_return_{n}
	self.slice_grow_x0(key)          // GROW $key
	self.Link(key)                   // _more_space_return_{n}:
}

func (self *Assembler) slice_grow_x0(ret string) {
	// ARM64 ADR instruction: ADR X30, <label>
	// ADR encoding: 10010000 immlo:2 immhi:19 rd:5
	// For X30 (LR): rd = 11110
	// Base encoding: 10010000 00 xxxxxxx xxxxxxxxxx 11110
	self.Byte(0x10, 0x00, 0x00, 0x90) // ADR X30, ret (placeholder)
	self.Sref(ret, 0)                 // Symbol reference for the address
	self.Sjmp("B", _LB_more_space)    // B _more_space
}

/** State Stack Helpers */

func (self *Assembler) save_state() {
	self.Emit("MOVD", jit.Ptr(_ST, 0), _TEMP0)                // LDR X0, [X19]
	self.Emit("ADD", _TEMP1, _TEMP0, jit.Imm(vars.StateSize)) // ADD X1, X0, #vars.StateSize
	self.Emit("CMP", _TEMP1, jit.Imm(vars.StackLimit))        // CMP X1, #vars.StackLimit
	self.Sjmp("B.HS", _LB_error_too_deep)                     // B.HS _error_too_deep

	// Save current state to stack
	self.Emit("MOVD", _SP_x, jit.Ptr(_ST, 8))  // STR X25, [X19, #8]
	self.Emit("MOVD", _SP_f, jit.Ptr(_ST, 16)) // STR X26, [X19, #16]
	self.Emit("MOVD", _SP_p, jit.Ptr(_ST, 24)) // STR X23, [X19, #24]
	self.Emit("MOVD", _SP_q, jit.Ptr(_ST, 32)) // STR X24, [X19, #32]
	self.Emit("MOVD", _TEMP1, jit.Ptr(_ST, 0)) // STR X1, [X19]
}

func (self *Assembler) drop_state(decr int64) {
	self.Emit("MOVD", jit.Ptr(_ST, 0), _TEMP0)      // LDR X0, [X19]
	self.Emit("SUB", _TEMP0, _TEMP0, jit.Imm(decr)) // SUB X0, X0, #decr
	self.Emit("MOVD", _TEMP0, jit.Ptr(_ST, 0))      // STR X0, [X19]

	// Restore state from stack
	self.Emit("MOVD", jit.Ptr(_ST, 8), _SP_x)  // LDR X25, [X19, #8]
	self.Emit("MOVD", jit.Ptr(_ST, 16), _SP_f) // LDR X26, [X19, #16]
	self.Emit("MOVD", jit.Ptr(_ST, 24), _SP_p) // LDR X23, [X19, #24]
	self.Emit("MOVD", jit.Ptr(_ST, 32), _SP_q) // LDR X24, [X19, #32]

	// Clear remaining state
	self.Emit("MOVD", _ZR, jit.Ptr(_ST, 40)) // STR ZR, [X19, #40]
	self.Emit("MOVD", _ZR, jit.Ptr(_ST, 48)) // STR ZR, [X19, #48]
}

/** Buffer Helpers **/

func (self *Assembler) add_char(ch byte) {
	self.Emit("MOVB", jit.Imm(int64(ch)), jit.Ptr(_RP, 0)) // STRB $ch, [RP]
	self.Emit("ADD", _RL, _RL, jit.Imm(1))                 // ADD X21, X21, #1
}

func (self *Assembler) add_long(ch uint32, n int64) {
	self.Emit("MOVW", jit.Imm(int64(ch)), jit.Ptr(_RP, 0)) // STR $ch, [RP]
	self.Emit("ADD", _RL, _RL, jit.Imm(n))                 // ADD X21, X21, #n
}

func (self *Assembler) add_text(ss string) {
	self.store_str(ss)                                  // TEXT $ss
	self.Emit("ADD", _RL, _RL, jit.Imm(int64(len(ss)))) // ADD X21, X21, #${len(ss)}
}

// get *buf at X0
func (self *Assembler) prep_buffer_X0() {
	self.Emit("MOVD", _ARG_rb, _TEMP0)         // MOV rb, X0
	self.Emit("MOVD", _RL, jit.Ptr(_TEMP0, 8)) // STR X21, [X0, #8]
}

func (self *Assembler) save_buffer() {
	self.Emit("MOVD", _ARG_rb, _TEMP0)          // MOV rb, X0
	self.Emit("MOVD", _RP, jit.Ptr(_TEMP0, 0))  // STR X20, [X0]
	self.Emit("MOVD", _RL, jit.Ptr(_TEMP0, 8))  // STR X21, [X0, #8]
	self.Emit("MOVD", _RC, jit.Ptr(_TEMP0, 16)) // STR X22, [X0, #16]
}

// get *buf at X0
func (self *Assembler) load_buffer_X0() {
	self.Emit("MOVD", _ARG_rb, _TEMP0)          // MOV rb, X0
	self.Emit("MOVD", jit.Ptr(_TEMP0, 0), _RP)  // LDR X20, [X0]
	self.Emit("MOVD", jit.Ptr(_TEMP0, 8), _RL)  // LDR X21, [X0, #8]
	self.Emit("MOVD", jit.Ptr(_TEMP0, 16), _RC) // LDR X22, [X0, #16]
}

/** Function Interface Helpers **/

func (self *Assembler) call(pc obj.Addr) {
	self.Emit("MOVD", pc, _LR_REG) // MOV $pc, LR
	self.Emit("BLR", _LR_REG)      // BLR LR
}

func (self *Assembler) save_c() {
	self.xsave(_REG_ffi...) // SAVE $REG_ffi
}

func (self *Assembler) call_b64(pc obj.Addr) {
	self.xsave(_REG_b64...) // SAVE $REG_all
	self.call(pc)           // CALL $pc
	self.xload(_REG_b64...) // LOAD $REG_ffi
}

func (self *Assembler) call_c(pc obj.Addr) {
	// Swap SP_p with a temp register to preserve it
	self.Emit("MOVD", _SP_p, _TEMP1)
	self.call(pc)           // CALL $pc
	self.xload(_REG_ffi...) // LOAD $REG_ffi
	self.Emit("MOVD", _TEMP1, _SP_p)
}

func (self *Assembler) call_go(pc obj.Addr) {
	self.xsave(_REG_all...) // SAVE $REG_all
	self.call(pc)           // CALL $pc
	self.xload(_REG_all...) // LOAD $REG_all
}

func (self *Assembler) call_more_space(pc obj.Addr) {
	self.xsave(_REG_ms...) // SAVE $REG_all
	self.call(pc)          // CALL $pc
	self.xload(_REG_ms...) // LOAD $REG_all
}

func (self *Assembler) call_encoder(pc obj.Addr) {
	self.xsave(_REG_enc...) // SAVE $REG_all
	self.call(pc)           // CALL $pc
	self.xload(_REG_enc...) // LOAD $REG_all
}

/** OpCode Implementations **/

var (
	_F_f64toa    = jit.Imm(int64(native.S_f64toa))
	_F_f32toa    = jit.Imm(int64(native.S_f32toa))
	_F_i64toa    = jit.Imm(int64(native.S_i64toa))
	_F_u64toa    = jit.Imm(int64(native.S_u64toa))
	_F_b64encode = jit.Imm(int64(rt.SubrB64Encode))
)

var (
	_F_memmove       = jit.Func(rt.Memmove)
	_F_error_number  = jit.Func(vars.Error_number)
	_F_isValidNumber = jit.Func(alg.IsValidNumber)
	_F_is_zero       = jit.Func(rt.IsZero)
)

var (
	_F_iteratorStop  = jit.Func(alg.IteratorStop)
	_F_iteratorNext  = jit.Func(alg.IteratorNext)
	_F_iteratorStart = jit.Func(alg.IteratorStart)
)

var (
	_F_encodeTypedPointer  obj.Addr
	_F_encodeJsonMarshaler obj.Addr
	_F_encodeTextMarshaler obj.Addr
)

func init() {
	_F_encodeJsonMarshaler = jit.Func(prim.EncodeJsonMarshaler)
	_F_encodeTextMarshaler = jit.Func(prim.EncodeTextMarshaler)
	_F_encodeTypedPointer = jit.Func(EncodeTypedPointer)
}

// Basic operation implementations
func (self *Assembler) _asm_OP_null(_ *ir.Instr) {
	self.check_size(4)
	self.Emit("MOVW", jit.Imm(_IM_null), _TEMP0) // MOVW $'null', X0
	self.Emit("MOVWU", _TEMP0, jit.Ptr(_RP, 0))  // STRH X0, [RP]
	self.Emit("ADD", _RL, _RL, jit.Imm(4))       // ADD X21, X21, #4
}

func (self *Assembler) _asm_OP_empty_arr(_ *ir.Instr) {
	self.Emit("TST", _ARG_fv, jit.Imm(1<<alg.BitNoNullSliceOrMap)) // TST fv, #(1<<BitNoNullSliceOrMap)
	self.Sjmp("B.NE", "_empty_arr_{n}")                            // B.NE _empty_arr_{n}
	self._asm_OP_null(nil)
	self.Sjmp("B", "_empty_arr_end_{n}") // B _empty_arr_end_{n}
	self.Link("_empty_arr_{n}")
	self.check_size(2)
	self.Emit("MOVH", jit.Imm(_IM_array), _TEMP0) // MOVH $'[]', X0
	self.Emit("MOVHU", _TEMP0, jit.Ptr(_RP, 0))   // STRH X0, [RP]
	self.Emit("ADD", _RL, _RL, jit.Imm(2))        // ADD X21, X21, #2
	self.Link("_empty_arr_end_{n}")
}

func (self *Assembler) _asm_OP_empty_obj(_ *ir.Instr) {
	self.Emit("TST", _ARG_fv, jit.Imm(1<<alg.BitNoNullSliceOrMap)) // TST fv, #(1<<BitNoNullSliceOrMap)
	self.Sjmp("B.NE", "_empty_obj_{n}")                            // B.NE _empty_obj_{n}
	self._asm_OP_null(nil)
	self.Sjmp("B", "_empty_obj_end_{n}") // B _empty_obj_end_{n}
	self.Link("_empty_obj_{n}")
	self.check_size(2)
	self.Emit("MOVH", jit.Imm(_IM_object), _TEMP0) // MOVH $'{}', X0
	self.Emit("MOVHU", _TEMP0, jit.Ptr(_RP, 0))    // STRH X0, [RP]
	self.Emit("ADD", _RL, _RL, jit.Imm(2))         // ADD X21, X21, #2
	self.Link("_empty_obj_end_{n}")
}

func (self *Assembler) _asm_OP_bool(_ *ir.Instr) {
	self.Emit("CMPB", jit.Ptr(_SP_p, 0), jit.Imm(0)) // CMPB (SP.p), #0
	self.Sjmp("B.EQ", "_false_{n}")                  // B.EQ _false_{n}
	self.check_size(4)                               // SIZE $4
	self.Emit("MOVW", jit.Imm(_IM_true), _TEMP0)     // MOVW $'true', X0
	self.Emit("MOVWU", _TEMP0, jit.Ptr(_RP, 0))      // STRH X0, [RP]
	self.Emit("ADD", _RL, _RL, jit.Imm(4))           // ADD X21, X21, #4
	self.Sjmp("B", "_end_{n}")                       // B _end_{n}
	self.Link("_false_{n}")
	self.check_size(5)                               // SIZE $5
	self.Emit("MOVW", jit.Imm(_IM_fals), _TEMP0)     // MOVW $'fals', X0
	self.Emit("MOVWU", _TEMP0, jit.Ptr(_RP, 0))      // STRH X0, [RP]
	self.Emit("MOVB", jit.Imm('e'), jit.Ptr(_RP, 4)) // STRB $'e', [RP, #4]
	self.Emit("ADD", _RL, _RL, jit.Imm(5))           // ADD X21, X21, #5
	self.Link("_end_{n}")
}

// Integer operations
func (self *Assembler) _asm_OP_i8(_ *ir.Instr) {
	self.store_int(4, _F_i64toa, "MOVB")
}

func (self *Assembler) _asm_OP_i16(_ *ir.Instr) {
	self.store_int(6, _F_i64toa, "MOVH")
}

func (self *Assembler) _asm_OP_i32(_ *ir.Instr) {
	self.store_int(17, _F_i64toa, "MOVW")
}

func (self *Assembler) _asm_OP_i64(_ *ir.Instr) {
	self.store_int(21, _F_i64toa, "MOVD")
}

func (self *Assembler) _asm_OP_u8(_ *ir.Instr) {
	self.store_int(3, _F_u64toa, "MOVB")
}

func (self *Assembler) _asm_OP_u16(_ *ir.Instr) {
	self.store_int(5, _F_u64toa, "MOVH")
}

func (self *Assembler) _asm_OP_u32(_ *ir.Instr) {
	self.store_int(16, _F_u64toa, "MOVW")
}

func (self *Assembler) _asm_OP_u64(_ *ir.Instr) {
	self.store_int(20, _F_u64toa, "MOVD")
}

// Float operations
func (self *Assembler) _asm_OP_f32(_ *ir.Instr) {
	self.check_size(32)
	self.Emit("MOVW", jit.Ptr(_SP_p, 0), _TEMP0)         // MOVW (SP.p), X0
	self.Emit("AND", _TEMP0, _TEMP0, jit.Imm(_FM_exp32)) // AND X0, X0, #$_FM_exp32
	self.Emit("EOR", _TEMP0, _TEMP0, jit.Imm(_FM_exp32)) // EOR X0, X0, #$_FM_exp32
	self.Sjmp("B.NE", "_encode_normal_f32_{n}")          // B.NE _encode_normal_f32_{n}

	// Handle NaN/Infinity
	self.Emit("TST", _ARG_fv, jit.Imm(1<<alg.BitEncodeNullForInfOrNan)) // TST fv, #(1<<BitEncodeNullForInfOrNan)
	self.Sjmp("B.EQ", _LB_error_nan_or_infinite)                        // B.EQ _error_nan_or_infinite
	self._asm_OP_null(nil)
	self.Sjmp("B", "_encode_f32_end_{n}") // B _encode_f32_end_{n}

	self.Link("_encode_normal_f32_{n}")
	self.save_c()                                // SAVE $C_regs
	self.rbuf_rp()                               // ADD RP, RP, RL
	self.Emit("MOVS", jit.Ptr(_SP_p, 0), _TEMP1) // MOVS (SP.p), S0
	self.call_c(_F_f32toa)                       // CALL_C f32toa
	self.Emit("ADD", _RL, _RL, _ARG0)            // ADD X21, X21, X0
	self.Link("_encode_f32_end_{n}")
}

func (self *Assembler) _asm_OP_f64(_ *ir.Instr) {
	self.check_size(32)
	self.Emit("MOVD", jit.Ptr(_SP_p, 0), _TEMP0)         // MOVD (SP.p), X0
	self.Emit("AND", _TEMP0, _TEMP0, jit.Imm(_FM_exp64)) // AND X0, X0, #$_FM_exp64
	self.Emit("EOR", _TEMP0, _TEMP0, jit.Imm(_FM_exp64)) // EOR X0, X0, #$_FM_exp64
	self.Sjmp("B.NE", "_encode_normal_f64_{n}")          // B.NE _encode_normal_f64_{n}

	// Handle NaN/Infinity
	self.Emit("TST", _ARG_fv, jit.Imm(1<<alg.BitEncodeNullForInfOrNan)) // TST fv, #(1<<BitEncodeNullForInfOrNan)
	self.Sjmp("B.EQ", _LB_error_nan_or_infinite)                        // B.EQ _error_nan_or_infinite
	self._asm_OP_null(nil)
	self.Sjmp("B", "_encode_f64_end_{n}") // B _encode_f64_end_{n}

	self.Link("_encode_normal_f64_{n}")
	self.save_c()                                // SAVE $C_regs
	self.rbuf_rp()                               // ADD RP, RP, RL
	self.Emit("MOVD", jit.Ptr(_SP_p, 0), _TEMP1) // MOVD (SP.p), D0
	self.call_c(_F_f64toa)                       // CALL_C f64toa
	self.Emit("ADD", _RL, _RL, _ARG0)            // ADD X21, X21, X0
	self.Link("_encode_f64_end_{n}")
}

// String operations
func (self *Assembler) _asm_OP_str(_ *ir.Instr) {
	self.encode_string(false)
}

func (self *Assembler) _asm_OP_bin(_ *ir.Instr) {
	self.Emit("MOVD", jit.Ptr(_SP_p, 8), _TEMP0) // LDR X0, [SP_p, #8]
	self.Emit("ADD", _TEMP0, _TEMP0, jit.Imm(2)) // ADD X0, X0, #2
	self.Emit("MOVD", jit.Imm(_IM_mulv), _TEMP1) // MOV $_MF_mulv, X1
	self.Emit("MUL", _TEMP1, _TEMP0, _TEMP1)     // MUL X0, X1, X1
	self.Emit("ADD", _TEMP0, _TEMP1, jit.Imm(1)) // ADD X0, X1, #1
	self.Emit("ADD", _TEMP0, _TEMP0, jit.Imm(2)) // ADD X0, X0, #2
	self.check_size_r(_TEMP0, 0)                 // SIZE X0
	self.add_char('"')                           // CHAR $'"'

	// Prepare for b64encode call
	self.Emit("MOVD", _ARG_rb, _ARG0)        // MOV rb, X0
	self.Emit("STR", _RL, jit.Ptr(_ARG0, 8)) // STR X21, [X0, #8]
	self.Emit("MOVD", _SP_p, _ARG1)          // MOV SP.p, X1

	// Call b64encode
	self.call_b64(_F_b64encode) // CALL b64encode
	self.load_buffer_X0()       // LOAD {buf}
	self.add_char('"')          // CHAR $'"'
}

func (self *Assembler) _asm_OP_quote(_ *ir.Instr) {
	self.encode_string(true)
}

// Number operation
func (self *Assembler) _asm_OP_number(_ *ir.Instr) {
	self.Emit("MOVD", jit.Ptr(_SP_p, 8), _TEMP1) // LDR X1, [SP_p, #8]
	self.Emit("CMP", _TEMP1, _ZR)                // CMP X1, XZR
	self.Sjmp("B.EQ", "_empty_{n}")              // B.EQ _empty_{n}

	self.Emit("MOVD", jit.Ptr(_SP_p, 0), _TEMP0) // LDR X0, [SP_p]
	self.Emit("CMP", _TEMP0, _ZR)                // CMP X0, XZR
	self.Sjmp("B.NE", "_number_next_{n}")        // B.NE _number_next_{n}

	// Handle nil pointer error
	self.Emit("MOVD", jit.Imm(int64(vars.PanicNilPointerOfNonEmptyString)), _TEMP0)
	self.Sjmp("B", _LB_panic)

	self.Link("_number_next_{n}")
	self.call_go(_F_isValidNumber)              // CALL_GO isValidNumber
	self.Emit("CMPW", _ARG0, jit.Imm(0))        // CMPW X0, #0
	self.Sjmp("B.EQ", _LB_error_invalid_number) // B.EQ _error_invalid_number

	self.Emit("MOVD", jit.Ptr(_SP_p, 8), _TEMP1) // LDR X1, [SP_p, #8]
	self.check_size_r(_TEMP1, 0)                 // SIZE X1
	self.Emit("ADD", _TEMP0, _RP, _RL)           // ADD X0, X20, X21
	self.Emit("ADD", _RL, _RL, _TEMP1)           // ADD X21, X21, X1

	// Use memmove to copy the number string
	self.Emit("MOVD", jit.Ptr(_SP_p, 0), _ARG1) // MOV (SP.p), X1
	self.Emit("MOVD", jit.Ptr(_SP_p, 8), _ARG2) // MOV 8(SP.p), X2
	self.Emit("MOVD", _TEMP0, _ARG0)            // MOV X0, X0 (dest)
	self.call_go(_F_memmove)                    // CALL_GO memmove
	self.Emit("MOVD", _ARG_rb, _TEMP0)          // MOV rb, X0
	self.Emit("STR", _RL, jit.Ptr(_TEMP0, 8))   // STR X21, [X0, #8]
	self.Sjmp("B", "_done_{n}")                 // B _done_{n}

	self.Link("_empty_{n}")
	self.check_size(1) // SIZE $1
	self.add_char('0') // CHAR $'0'
	self.Link("_done_{n}")
}

// Helper function to print debug info
func (self *Assembler) debug_instr(i int, p *ir.Instr) {
	// Debug implementation can be added here if needed
}

// String encoding routine
func (self *Assembler) encode_string(doubleQuote bool) {
	// Load string length
	self.Emit("MOVD", jit.Ptr(_SP_p, 8), _TEMP0) // LDR X0, [SP_p, #8]
	self.Emit("CMP", _TEMP0, _ZR)                // CMP X0, XZR
	self.Sjmp("B.EQ", "_str_empty_{n}")          // B.EQ _str_empty_{n}

	// For simplicity, implement basic string copying
	// In a full implementation, this would handle escaping properly

	// Add opening quote if needed
	if doubleQuote {
		self.check_size(1)
		self.add_char('"')
	}

	// Load string data pointer and length
	self.Emit("MOVD", jit.Ptr(_SP_p, 0), _TEMP1) // LDR X1, [SP_p] (data)
	self.Emit("MOVD", _TEMP0, _ARG2)             // MOV X0, X2 (len)

	// Check buffer size
	self.check_size_r(_TEMP0, 0)

	// Copy string bytes
	self.Emit("MOVD", _TEMP1, _ARG1)  // MOV src, X1
	self.Emit("ADD", _ARG0, _RP, _RL) // MOV dst, X0
	self.call_go(_F_memmove)          // CALL memmove

	// Update result length
	self.Emit("ADD", _RL, _RL, _TEMP0) // ADD RL, RL, len

	// Add closing quote if needed
	if doubleQuote {
		self.check_size(1)
		self.add_char('"')
	}

	self.Link("_str_empty_{n}")
	// Handle empty string
	if doubleQuote {
		self.check_size(2)
		self.Emit("MOVH", jit.Imm(int64('"'|('"')<<8)), _TEMP0) // MOVW '""', X0
		self.Emit("MOVHU", _TEMP0, jit.Ptr(_RP, 0))             // STRH X0, [RP]
		self.Emit("ADD", _RL, _RL, jit.Imm(2))                  // ADD RL, RL, #2
	}
}

// Placeholder implementations for other operations that need full implementation
func (self *Assembler) _asm_OP_eface(_ *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_iface(_ *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_byte(p *ir.Instr) {
	self.check_size(1)
	self.Emit("MOVB", jit.Imm(p.I64()), jit.Ptr(_RP, 0)) // STRB p.Vi(), [RP]
	self.Emit("ADD", _RL, _RL, jit.Imm(1))               // ADD X21, X21, #1
}

func (self *Assembler) _asm_OP_text(p *ir.Instr) {
	self.check_size(len(p.Vs())) // SIZE ${len(p.Vs())}
	self.add_text(p.Vs())        // TEXT ${p.Vs()}
}

func (self *Assembler) _asm_OP_deref(_ *ir.Instr) {
	self.Emit("MOVD", jit.Ptr(_SP_p, 0), _SP_p) // LDR (SP_p), SP_p
}

func (self *Assembler) _asm_OP_index(p *ir.Instr) {
	self.Emit("MOVD", jit.Imm(p.I64()), _TEMP0) // MOV $p.Vi(), X0
	self.Emit("ADD", _SP_p, _SP_p, _TEMP0)      // ADD SP.p, SP.p, X0
}

func (self *Assembler) _asm_OP_load(_ *ir.Instr) {
	self.Emit("MOVD", jit.Ptr(_ST, 0), _TEMP0)           // LDR X0, [ST]
	self.Emit("LDR", _SP_x, jit.Ptr(_ST, _TEMP0, 1, 8))  // LDR X25, [ST, X0, LSL #3, #8]
	self.Emit("LDR", _SP_p, jit.Ptr(_ST, _TEMP0, 1, 24)) // LDR X23, [ST, X0, LSL #3, #24]
	self.Emit("LDR", _SP_q, jit.Ptr(_ST, _TEMP0, 1, 32)) // LDR X24, [ST, X0, LSL #3, #32]
}

func (self *Assembler) _asm_OP_save(_ *ir.Instr) {
	self.save_state()
}

func (self *Assembler) _asm_OP_drop(_ *ir.Instr) {
	self.drop_state(vars.StateSize)
}

func (self *Assembler) _asm_OP_drop_2(_ *ir.Instr) {
	self.drop_state(vars.StateSize * 2)
	// Clear additional state
	self.Emit("MOVD", _ZR, jit.Ptr(_ST, 40)) // STR ZR, [ST, #40]
	self.Emit("MOVD", _ZR, jit.Ptr(_ST, 48)) // STR ZR, [ST, #48]
}

func (self *Assembler) _asm_OP_recurse(p *ir.Instr) {
	self.prep_buffer_X0() // MOVE {buf}, X0
	vt, pv := p.Vp()
	self.Emit("MOVD", jit.Type(vt), _ARG1) // MOV $(type(p.Vt())), X1

	// Check for indirection
	if !rt.UnpackType(vt).Indirect() {
		self.Emit("MOVD", _SP_p, _ARG2) // MOV SP.p, X2
	} else {
		self.Emit("MOVD", _SP_p, _VAR_vp) // MOV SP.p, VAR.vp
		self.Emit("MOVD", _VAR_vp, _ARG2) // MOV VAR.vp, X2
	}

	// Call the encoder
	self.Emit("MOVD", _ST, _ARG3)     // MOV ST, X3
	self.Emit("MOVD", _ARG_fv, _ARG4) // MOV fv, X4
	if pv {
		self.Emit("ORR", _ARG4, _ARG4, jit.Imm(1<<alg.BitPointerValue)) // ORR X4, X4, #(1<<BitPointerValue)
	}

	self.call_encoder(_F_encodeTypedPointer) // CALL encodeTypedPointer
	self.Emit("CMP", _ET, _ZR)               // CMP X27, XZR
	self.Sjmp("B.NE", _LB_error)             // B.NE _error
	self.load_buffer_X0()
}

// Placeholder implementations for remaining operations
func (self *Assembler) _asm_OP_is_nil(p *ir.Instr) {
	self.Emit("CMP", jit.Ptr(_SP_p, 0), _ZR)                // CMP (SP.p), XZR
	self.Sjmp("B.EQ", "_is_nil_"+strconv.Itoa(int(p.Vi()))) // B.EQ p.Vi()
}

func (self *Assembler) _asm_OP_is_nil_p1(p *ir.Instr) {
	self.Emit("CMP", jit.Ptr(_SP_p, 8), _ZR)                   // CMP 8(SP.p), XZR
	self.Sjmp("B.EQ", "_is_nil_p1_"+strconv.Itoa(int(p.Vi()))) // B.EQ p.Vi()
}

func (self *Assembler) _asm_OP_is_zero_1(p *ir.Instr) {
	self.Emit("CMPB", jit.Ptr(_SP_p, 0), _ZR)                  // CMPB (SP.p), #0
	self.Sjmp("B.EQ", "_is_zero_1_"+strconv.Itoa(int(p.Vi()))) // B.EQ p.Vi()
}

func (self *Assembler) _asm_OP_is_zero_2(p *ir.Instr) {
	self.Emit("CMPH", jit.Ptr(_SP_p, 0), _ZR)                  // CMPH (SP.p), #0
	self.Sjmp("B.EQ", "_is_zero_2_"+strconv.Itoa(int(p.Vi()))) // B.EQ p.Vi()
}

func (self *Assembler) _asm_OP_is_zero_4(p *ir.Instr) {
	self.Emit("CMPW", jit.Ptr(_SP_p, 0), _ZR)                  // CMPW (SP.p), #0
	self.Sjmp("B.EQ", "_is_zero_4_"+strconv.Itoa(int(p.Vi()))) // B.EQ p.Vi()
}

func (self *Assembler) _asm_OP_is_zero_8(p *ir.Instr) {
	self.Emit("CMP", jit.Ptr(_SP_p, 0), _ZR)                   // CMP (SP.p), XZR
	self.Sjmp("B.EQ", "_is_zero_8_"+strconv.Itoa(int(p.Vi()))) // B.EQ p.Vi()
}

func (self *Assembler) _asm_OP_is_zero_map(p *ir.Instr) {
	self.Emit("MOVD", jit.Ptr(_SP_p, 0), _TEMP0)                   // LDR X0, [SP_p]
	self.Emit("CMP", _TEMP0, _ZR)                                  // CMP X0, XZR
	self.Sjmp("B.EQ", "_is_zero_map_1_"+strconv.Itoa(int(p.Vi()))) // B.EQ p.Vi()
	self.Emit("CMP", jit.Ptr(_TEMP0, 0), _ZR)                      // CMP [X0], XZR
	self.Sjmp("B.EQ", "_is_zero_map_2_"+strconv.Itoa(int(p.Vi()))) // B.EQ p.Vi()
}

func (self *Assembler) _asm_OP_is_zero(p *ir.Instr) {
	fv := p.VField()
	self.Emit("MOVD", _SP_p, _ARG1)                          // ptr
	self.Emit("MOVD", jit.ImmPtr(unsafe.Pointer(fv)), _ARG2) // fv
	self.call_go(_F_is_zero)                                 // CALL $fn
	self.Emit("CMPB", _ARG0, _ZR)                            // CMPB X0, #0
	self.Sjmp("B.NE", "_is_zero_"+strconv.Itoa(int(p.Vi()))) // B.NE p.Vi()
}

func (self *Assembler) _asm_OP_goto(p *ir.Instr) {
	self.Sjmp("B", "_goto_"+strconv.Itoa(int(p.Vi())))
}

// Placeholder for map operations - these need full implementation
func (self *Assembler) _asm_OP_map_iter(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_map_stop(_ *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_map_check_key(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_map_write_key(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_map_value_next(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_slice_len(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_slice_next(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_marshal(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_marshal_p(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_marshal_text(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_marshal_text_p(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_cond_set(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_cond_testc(p *ir.Instr) {
	// Implementation needed
}

func (self *Assembler) _asm_OP_unsupported(i *ir.Instr) {
	// Implementation needed
}

// Built-in functions
var (
	_T_byte      = jit.Type(vars.ByteType)
	_F_growslice = jit.Func(rt.GrowSlice)
)

func (self *Assembler) more_space() {
	self.Link(_LB_more_space)
	self.Emit("MOVD", _RP, _ARG1)      // MOV X20, X1 (result pointer)
	self.Emit("MOVD", _RL, _ARG2)      // MOV X21, X2 (result length)
	self.Emit("MOVD", _RC, _ARG3)      // MOV X22, X3 (result capacity)
	self.Emit("MOVD", _TEMP0, _ARG4)   // MOV X0, X4 (new length)
	self.Emit("MOVD", _T_byte, _ARG0)  // MOV $_T_byte, X0
	self.call_more_space(_F_growslice) // CALL $pc
	self.Emit("MOVD", _ARG0, _RP)      // MOV X0, X20
	self.Emit("MOVD", _ARG2, _RL)      // MOV X2, X21
	self.Emit("MOVD", _ARG3, _RC)      // MOV X3, X22
	self.save_buffer()                 // SAVE {buf}
	self.Emit("BR", _LR_REG)           // BR LR
}

var (
	_V_ERR_too_deep               = jit.Imm(int64(uintptr(unsafe.Pointer(vars.ERR_too_deep))))
	_V_ERR_nan_or_infinite        = jit.Imm(int64(uintptr(unsafe.Pointer(vars.ERR_nan_or_infinite))))
	_I_json_UnsupportedValueError = jit.Itab(rt.UnpackType(vars.ErrorType), vars.JsonUnsupportedValueType)
)

func (self *Assembler) error_too_deep() {
	self.Link(_LB_error_too_deep)
	self.Emit("MOVD", _V_ERR_too_deep, _EP)               // MOV $_V_ERR_too_deep, X28
	self.Emit("MOVD", _I_json_UnsupportedValueError, _ET) // MOV $_I_json_UnsupportedValuError, X27
	self.Sjmp("B", _LB_error)                             // B _error
}

func (self *Assembler) error_invalid_number() {
	self.Link(_LB_error_invalid_number)
	self.Emit("MOVD", jit.Ptr(_SP_p, 0), _ARG0) // MOV (SP.p), X0
	self.Emit("MOVD", jit.Ptr(_SP_p, 8), _ARG1) // MOV 8(SP_p), X1
	self.call_go(_F_error_number)               // CALL_GO error_number
	self.Sjmp("B", _LB_error)                   // B _error
}

func (self *Assembler) error_nan_or_infinite() {
	self.Link(_LB_error_nan_or_infinite)
	self.Emit("MOVD", _V_ERR_nan_or_infinite, _EP)        // MOV $_V_ERR_nan_or_infinite, X28
	self.Emit("MOVD", _I_json_UnsupportedValueError, _ET) // MOV $_I_json_UnsupportedValuError, X27
	self.Sjmp("B", _LB_error)                             // B _error
}

var (
	_F_quote = jit.Imm(int64(native.S_quote))
	_F_panic = jit.Func(vars.GoPanic)
)

func (self *Assembler) go_panic() {
	self.Link(_LB_panic)
	self.Emit("MOVD", _SP_p, _ARG1) // MOV SP.p, X1
	self.Emit("MOVD", _RP, _ARG2)   // MOV RP, X2
	self.Emit("MOVD", _RL, _ARG3)   // MOV RL, X3
	self.call_go(_F_panic)
}
