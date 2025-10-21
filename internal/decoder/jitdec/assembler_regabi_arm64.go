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
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unsafe"

	"github.com/bytedance/sonic/internal/caching"
	"github.com/bytedance/sonic/internal/jit"
	"github.com/bytedance/sonic/internal/native"
	"github.com/bytedance/sonic/internal/native/types"
	"github.com/bytedance/sonic/internal/rt"
	"github.com/twitchyliquid64/golang-asm/obj"
)

/** Register Allocations for ARM64
 *
 *  State Registers (Callee-saved):
 *
 *      x19 : stack base
 *      x20 : input pointer
 *      x21 : input length
 *      x22 : input cursor
 *      x23 : value pointer
 *
 *  Error Registers (Caller-saved):
 *
 *      x0  : error type register
 *      x1  : error pointer register
 */

/** Function Prototype & Stack Map
 *
 *  func (s string, ic int, vp unsafe.Pointer, sb *_Stack, fv uint64, sv string) (rc int, err error)
 *
 *  s.buf  :   0(FP)
 *  s.len  :   8(FP)
 *  ic     :  16(FP)
 *  vp     :  24(FP)
 *  sb     :  32(FP)
 *  fv     :  40(FP)
 *  sv     :  48(FP)
 *  err.vt :  72(FP)
 *  err.vp :  80(FP)
 */

const (
	_FP_args   = 80     // 80 bytes to pass and spill register arguments
	_FP_fargs  = 96     // 96 bytes for passing arguments to other Go functions
	_FP_saves  = 64     // 64 bytes for saving the registers before CALL instructions
	_FP_locals = 160    // 160 bytes for local variables
)

const (
	_FP_offs = _FP_fargs + _FP_saves + _FP_locals
	_FP_size = _FP_offs + 16    // 16 bytes for the parent frame pointer
	_FP_base = _FP_size + 8     // 8 bytes for the return address
)

const (
	_IM_null = 0x6c6c756e   // 'null'
	_IM_true = 0x65757274   // 'true'
	_IM_alse = 0x65736c61   // 'alse' ('false' without the 'f')
)

const (
	_BM_space = (1 << ' ') | (1 << '\t') | (1 << '\r') | (1 << '\n')
)

const (
	_MODE_JSON = 1 << 3 // base64 mode
)

const (
	_LB_error           = "_error"
	_LB_im_error        = "_im_error"
	_LB_eof_error       = "_eof_error"
	_LB_type_error      = "_type_error"
	_LB_field_error     = "_field_error"
	_LB_range_error     = "_range_error"
	_LB_stack_error     = "_stack_error"
	_LB_base64_error    = "_base64_error"
	_LB_unquote_error   = "_unquote_error"
	_LB_parsing_error   = "_parsing_error"
	_LB_parsing_error_v = "_parsing_error_v"
	_LB_mismatch_error   = "_mismatch_error"
)

const (
	_LB_char_0_error  = "_char_0_error"
	_LB_char_1_error  = "_char_1_error"
	_LB_char_2_error  = "_char_2_error"
	_LB_char_3_error  = "_char_3_error"
	_LB_char_4_error  = "_char_4_error"
	_LB_char_m2_error = "_char_m2_error"
	_LB_char_m3_error = "_char_m3_error"
)

const (
	_LB_skip_one = "_skip_one"
	_LB_skip_key_value = "_skip_key_value"
)

// ARM64 Register definitions
var (
	_X0 = jit.Reg("X0")
	_X1 = jit.Reg("X1")
	_X2 = jit.Reg("X2")
	_X3 = jit.Reg("X3")
	_X4 = jit.Reg("X4")
	_X5 = jit.Reg("X5")
	_X6 = jit.Reg("X6")
	_X7 = jit.Reg("X7")
	_X8 = jit.Reg("X8")
	_X9 = jit.Reg("X9")
	_X10 = jit.Reg("X10")
	_X11 = jit.Reg("X11")
	_X12 = jit.Reg("X12")
	_X13 = jit.Reg("X13")
	_X14 = jit.Reg("X14")
	_X15 = jit.Reg("X15")
	_X16 = jit.Reg("X16")
	_X17 = jit.Reg("X17")
	_X18 = jit.Reg("X18")
	_X19 = jit.Reg("X19")
	_X20 = jit.Reg("X20")
	_X21 = jit.Reg("X21")
	_X22 = jit.Reg("X22")
	_X23 = jit.Reg("X23")
	_X24 = jit.Reg("X24")
	_X25 = jit.Reg("X25")
	_X26 = jit.Reg("X26")
	_X27 = jit.Reg("X27")
	_X28 = jit.Reg("X28")
	_X29 = jit.Reg("X29") // FP
	_X30 = jit.Reg("X30") // LR
	_SP = jit.Reg("SP")
	_ZR = jit.Reg("ZR")
)

// ARM64 floating point registers
var (
	_D0 = jit.Reg("D0")
	_D1 = jit.Reg("D1")
	_D2 = jit.Reg("D2")
	_D3 = jit.Reg("D3")
	_D4 = jit.Reg("D4")
	_D5 = jit.Reg("D5")
	_D6 = jit.Reg("D6")
	_D7 = jit.Reg("D7")
	_D8 = jit.Reg("D8")
	_D9 = jit.Reg("D9")
	_D10 = jit.Reg("D10")
	_D11 = jit.Reg("D11")
	_D12 = jit.Reg("D12")
	_D13 = jit.Reg("D13")
	_D14 = jit.Reg("D14")
	_D15 = jit.Reg("D15")
)

// State registers (callee-saved)
var (
	_ST = jit.Reg("X19")  // stack base
	_IP = jit.Reg("X20")  // input pointer
	_IL = jit.Reg("X21")  // input length
	_IC = jit.Reg("X22")  // input cursor
	_VP = jit.Reg("X23")  // value pointer
)

// Error registers (caller-saved)
var (
	_ET = jit.Reg("X0")   // error type
	_EP = jit.Reg("X1")   // error pointer
)

// Argument locations
var (
	_ARG_s  = _ARG_sp
	_ARG_sp = jit.Ptr(_SP, _FP_base + 0)
	_ARG_sl = jit.Ptr(_SP, _FP_base + 8)
	_ARG_ic = jit.Ptr(_SP, _FP_base + 16)
	_ARG_vp = jit.Ptr(_SP, _FP_base + 24)
	_ARG_sb = jit.Ptr(_SP, _FP_base + 32)
	_ARG_fv = jit.Ptr(_SP, _FP_base + 40)
)

var (
	_ARG_sv   = _ARG_sv_p
	_ARG_sv_p = jit.Ptr(_SP, _FP_base + 48)
	_ARG_sv_n = jit.Ptr(_SP, _FP_base + 56)
	_ARG_vk   = jit.Ptr(_SP, _FP_base + 64)
)

// Local variable locations
var (
	_VAR_st = _VAR_st_Vt
	_VAR_sr = jit.Ptr(_SP, _FP_fargs + _FP_saves)
)

var (
	_VAR_st_Vt = jit.Ptr(_SP, _FP_fargs + _FP_saves + 0)
	_VAR_st_Dv = jit.Ptr(_SP, _FP_fargs + _FP_saves + 8)
	_VAR_st_Iv = jit.Ptr(_SP, _FP_fargs + _FP_saves + 16)
	_VAR_st_Ep = jit.Ptr(_SP, _FP_fargs + _FP_saves + 24)
	_VAR_st_Db = jit.Ptr(_SP, _FP_fargs + _FP_saves + 32)
	_VAR_st_Dc = jit.Ptr(_SP, _FP_fargs + _FP_saves + 40)
)

var (
	_VAR_ss_X0 = jit.Ptr(_SP, _FP_fargs + _FP_saves + 48)
	_VAR_ss_X1 = jit.Ptr(_SP, _FP_fargs + _FP_saves + 56)
	_VAR_ss_X2 = jit.Ptr(_SP, _FP_fargs + _FP_saves + 64)
	_VAR_ss_X3 = jit.Ptr(_SP, _FP_fargs + _FP_saves + 72)
)

var (
	_VAR_bs_p = jit.Ptr(_SP, _FP_fargs + _FP_saves + 80)
	_VAR_bs_n = jit.Ptr(_SP, _FP_fargs + _FP_saves + 88)
	_VAR_bs_LR = jit.Ptr(_SP, _FP_fargs + _FP_saves + 96)
)

var _VAR_fl = jit.Ptr(_SP, _FP_fargs + _FP_saves + 104)

var (
	_VAR_et = jit.Ptr(_SP, _FP_fargs + _FP_saves + 112) // save mismatched type
	_VAR_pc = jit.Ptr(_SP, _FP_fargs + _FP_saves + 120) // save skip return pc
	_VAR_ic = jit.Ptr(_SP, _FP_fargs + _FP_saves + 128) // save mismatched position
)

type _Assembler struct {
	jit.BaseAssembler
	p _Program
	name string
}

func newAssembler(p _Program) *_Assembler {
	return new(_Assembler).Init(p)
}

/** Assembler Interface **/

func (self *_Assembler) Load() _Decoder {
	return ptodec(self.BaseAssembler.Load("decode_"+self.name, _FP_size, _FP_args, argPtrs, localPtrs))
}

func (self *_Assembler) Init(p _Program) *_Assembler {
	self.p = p
	self.BaseAssembler.Init(self.compile)
	return self
}

func (self *_Assembler) compile() {
	self.prologue()
	self.instrs()
	self.epilogue()
	self.copy_string()
	self.escape_string()
	self.escape_string_twice()
	self.skip_one()
	self.skip_key_value()
	self.type_error()
	self.mismatch_error()
	self.field_error()
	self.range_error()
	self.stack_error()
	self.base64_error()
	self.parsing_error()
}

/** Assembler Stages **/

var _OpFuncTab = [256]func(*_Assembler, *_Instr) {
	_OP_any              : (*_Assembler)._asm_OP_any,
	_OP_dyn              : (*_Assembler)._asm_OP_dyn,
	_OP_str              : (*_Assembler)._asm_OP_str,
	_OP_bin              : (*_Assembler)._asm_OP_bin,
	_OP_bool             : (*_Assembler)._asm_OP_bool,
	_OP_num              : (*_Assembler)._asm_OP_num,
	_OP_i8               : (*_Assembler)._asm_OP_i8,
	_OP_i16              : (*_Assembler)._asm_OP_i16,
	_OP_i32              : (*_Assembler)._asm_OP_i32,
	_OP_i64              : (*_Assembler)._asm_OP_i64,
	_OP_u8               : (*_Assembler)._asm_OP_u8,
	_OP_u16              : (*_Assembler)._asm_OP_u16,
	_OP_u32              : (*_Assembler)._asm_OP_u32,
	_OP_u64              : (*_Assembler)._asm_OP_u64,
	_OP_f32              : (*_Assembler)._asm_OP_f32,
	_OP_f64              : (*_Assembler)._asm_OP_f64,
	_OP_unquote          : (*_Assembler)._asm_OP_unquote,
	_OP_nil_1            : (*_Assembler)._asm_OP_nil_1,
	_OP_nil_2            : (*_Assembler)._asm_OP_nil_2,
	_OP_nil_3            : (*_Assembler)._asm_OP_nil_3,
	_OP_empty_bytes      : (*_Assembler)._asm_OP_empty_bytes,
	_OP_deref            : (*_Assembler)._asm_OP_deref,
	_OP_index            : (*_Assembler)._asm_OP_index,
	_OP_is_null          : (*_Assembler)._asm_OP_is_null,
	_OP_is_null_quote    : (*_Assembler)._asm_OP_is_null_quote,
	_OP_map_init         : (*_Assembler)._asm_OP_map_init,
	_OP_map_key_i8       : (*_Assembler)._asm_OP_map_key_i8,
	_OP_map_key_i16      : (*_Assembler)._asm_OP_map_key_i16,
	_OP_map_key_i32      : (*_Assembler)._asm_OP_map_key_i32,
	_OP_map_key_i64      : (*_Assembler)._asm_OP_map_key_i64,
	_OP_map_key_u8       : (*_Assembler)._asm_OP_map_key_u8,
	_OP_map_key_u16      : (*_Assembler)._asm_OP_map_key_u16,
	_OP_map_key_u32      : (*_Assembler)._asm_OP_map_key_u32,
	_OP_map_key_u64      : (*_Assembler)._asm_OP_map_key_u64,
	_OP_map_key_f32      : (*_Assembler)._asm_OP_map_key_f32,
	_OP_map_key_f64      : (*_Assembler)._asm_OP_map_key_f64,
	_OP_map_key_str      : (*_Assembler)._asm_OP_map_key_str,
	_OP_map_key_utext    : (*_Assembler)._asm_OP_map_key_utext,
	_OP_map_key_utext_p  : (*_Assembler)._asm_OP_map_key_utext_p,
	_OP_array_skip       : (*_Assembler)._asm_OP_array_skip,
	_OP_array_clear      : (*_Assembler)._asm_OP_array_clear,
	_OP_array_clear_p    : (*_Assembler)._asm_OP_array_clear_p,
	_OP_slice_init       : (*_Assembler)._asm_OP_slice_init,
	_OP_slice_append     : (*_Assembler)._asm_OP_slice_append,
	_OP_object_next      : (*_Assembler)._asm_OP_object_next,
	_OP_struct_field     : (*_Assembler)._asm_OP_struct_field,
	_OP_unmarshal        : (*_Assembler)._asm_OP_unmarshal,
	_OP_unmarshal_p      : (*_Assembler)._asm_OP_unmarshal_p,
	_OP_unmarshal_text   : (*_Assembler)._asm_OP_unmarshal_text,
	_OP_unmarshal_text_p : (*_Assembler)._asm_OP_unmarshal_text_p,
	_OP_lspace           : (*_Assembler)._asm_OP_lspace,
	_OP_match_char       : (*_Assembler)._asm_OP_match_char,
	_OP_check_char       : (*_Assembler)._asm_OP_check_char,
	_OP_load             : (*_Assembler)._asm_OP_load,
	_OP_save             : (*_Assembler)._asm_OP_save,
	_OP_drop             : (*_Assembler)._asm_OP_drop,
	_OP_drop_2           : (*_Assembler)._asm_OP_drop_2,
	_OP_recurse          : (*_Assembler)._asm_OP_recurse,
	_OP_goto             : (*_Assembler)._asm_OP_goto,
	_OP_switch           : (*_Assembler)._asm_OP_switch,
	_OP_check_char_0     : (*_Assembler)._asm_OP_check_char_0,
	_OP_dismatch_err     : (*_Assembler)._asm_OP_dismatch_err,
	_OP_go_skip          : (*_Assembler)._asm_OP_go_skip,
	_OP_skip_emtpy       : (*_Assembler)._asm_OP_skip_empty,
	_OP_add              : (*_Assembler)._asm_OP_add,
	_OP_check_empty      : (*_Assembler)._asm_OP_check_empty,
	_OP_unsupported      : (*_Assembler)._asm_OP_unsupported,
	_OP_debug            : (*_Assembler)._asm_OP_debug,
}

func (self *_Assembler) _asm_OP_debug(_ *_Instr) {
	self.Emit("BRK", jit.Imm(0))
}

func (self *_Assembler) instr(v *_Instr) {
	if fn := _OpFuncTab[v.op()]; fn != nil {
		fn(self, v)
	} else {
		panic(fmt.Sprintf("invalid opcode: %d", v.op()))
	}
}

func (self *_Assembler) instrs() {
	for i, v := range self.p {
		self.Mark(i)
		self.instr(&v)
		self.debug_instr(i, &v)
	}
}

func (self *_Assembler) epilogue() {
	self.Mark(len(self.p))
	self.Emit("MOVD", _VAR_et, _ET)                     // MOVD VAR_et, ET
	self.Emit("CMP", _ET, _ZR)                         // CMP ET, ZR
	self.Sjmp("BNE", _LB_mismatch_error)               // BNE _mismatch_error
	self.Link(_LB_error)                               // _error:
	self.Emit("MOVD", _EP, _X2)                        // MOVD EP, X2
	self.Emit("MOVD", _ET, _X1)                        // MOVD ET, X1
	self.Emit("MOVD", _IC, _X0)                        // MOVD IC, X0
	self.Emit("MOVD", _ZR, _ARG_sp)                    // MOVD ZR, sp
	self.Emit("MOVD", _ZR, _ARG_vp)                    // MOVD ZR, vp
	self.Emit("MOVD", _ZR, _ARG_sv_p)                  // MOVD ZR, sv.p
	self.Emit("MOVD", _ZR, _ARG_vk)                    // MOVD ZR, vk
	self.Emit("LDP", _X29, _X30, jit.Ptr(_SP, _FP_offs)) // LDP X29, X30, _FP_offs(SP)
	self.Emit("ADD", _SP, _SP, jit.Imm(_FP_size))       // ADD SP, SP, #_FP_size
	self.Emit("RET")                                   // RET
}

func (self *_Assembler) prologue() {
	self.Emit("SUB", _SP, _SP, jit.Imm(_FP_size))       // SUB SP, SP, #_FP_size
	self.Emit("STP", _X29, _X30, jit.Ptr(_SP, _FP_offs)) // STP X29, X30, [SP,#_FP_offs]
	self.Emit("ADD", _X29, _SP, jit.Imm(_FP_offs))      // ADD X29, SP, #_FP_offs
	self.Emit("MOVD", _X0, _ARG_sp)                    // MOVD X0, s.p
	self.Emit("MOVD", _X0, _IP)                        // MOVD X0, IP
	self.Emit("MOVD", _X1, _ARG_sl)                    // MOVD X1, s.l
	self.Emit("MOVD", _X1, _IL)                        // MOVD X1, IL
	self.Emit("MOVD", _X2, _ARG_ic)                    // MOVD X2, ic
	self.Emit("MOVD", _X2, _IC)                        // MOVD X2, IC
	self.Emit("MOVD", _X3, _ARG_vp)                    // MOVD X3, vp
	self.Emit("MOVD", _X3, _VP)                        // MOVD X3, VP
	self.Emit("MOVD", _X4, _ARG_sb)                    // MOVD X4, sb
	self.Emit("MOVD", _X4, _ST)                        // MOVD X4, ST
	self.Emit("MOVD", _X5, _ARG_fv)                    // MOVD X5, fv
	self.Emit("MOVD", _ZR, _ARG_sv_p)                  // MOVD ZR, sv.p
	self.Emit("MOVD", _ZR, _ARG_sv_n)                  // MOVD ZR, sv.n
	self.Emit("MOVD", _ZR, _ARG_vk)                    // MOVD ZR, vk
	self.Emit("MOVD", _ZR, _VAR_et)                    // MOVD ZR, et
	// initialize digital buffer first
	self.Emit("MOVD", jit.Imm(_MaxDigitNums), _VAR_st_Dc) // MOVD #_MaxDigitNums, st.Dcap
	self.Emit("ADD", _X0, _ST, jit.Imm(_DbufOffset))    // ADD X0, ST, #_DbufOffset
	self.Emit("MOVD", _X0, _VAR_st_Db)                  // MOVD X0, st.Dbuf
}

/** Function Calling Helpers **/

var (
	_REG_go = []obj.Addr { _ST, _VP, _IP, _IL, _IC }
	_REG_rt = []obj.Addr { _ST, _VP, _IP, _IL, _IC }
)

func (self *_Assembler) save(r ...obj.Addr) {
	for i, v := range r {
		if i > _FP_saves / 8 - 1 {
			panic("too many registers to save")
		} else {
			self.Emit("MOVD", v, jit.Ptr(_SP, _FP_fargs + int64(i) * 8))
		}
	}
}

func (self *_Assembler) load(r ...obj.Addr) {
	for i, v := range r {
		if i > _FP_saves / 8 - 1 {
			panic("too many registers to load")
		} else {
			self.Emit("MOVD", jit.Ptr(_SP, _FP_fargs + int64(i) * 8), v)
		}
	}
}

func (self *_Assembler) call(fn obj.Addr) {
	self.Emit("MOVD", fn, _X16)                    // MOVD ${fn}, X16
	self.Rjmp("BLR", _X16)                          // BLR X16
}

func (self *_Assembler) call_go(fn obj.Addr) {
	self.save(_REG_go...)                           // SAVE $REG_go
	self.call(fn)
	self.load(_REG_go...)                           // LOAD $REG_go
}

func (self *_Assembler) call_c(fn obj.Addr) {
	self.save(_IP)
	self.call(fn)
	self.Emit("FMOVD", _ZR, _D15)                    // FMOVD ZR, D15
	self.load(_IP)
}

func (self *_Assembler) call_sf(fn obj.Addr) {
	self.Emit("MOVD", _ARG_sp, _X0)                 // MOVD s, X0
	self.Emit("MOVD", _IC, _ARG_ic)                  // MOVD IC, ic
	self.Emit("MOVD", _ARG_ic, _X1)                 // MOVD ic, X1
	self.Emit("ADD", _X2, _ST, jit.Imm(_FsmOffset)) // ADD X2, ST, #_FsmOffset
	self.Emit("MOVD", _ARG_fv, _X3)                 // MOVD fv, X3
	self.call_c(fn)
	self.Emit("MOVD", _ARG_ic, _IC)                 // MOVD ic, IC
}

func (self *_Assembler) call_vf(fn obj.Addr) {
	self.Emit("MOVD", _ARG_sp, _X0)                 // MOVD s, X0
	self.Emit("MOVD", _IC, _ARG_ic)                  // MOVD IC, ic
	self.Emit("MOVD", _ARG_ic, _X1)                 // MOVD ic, X1
	self.Emit("ADD", _X2, _VAR_st, _ZR)              // ADD X2, st, ZR
	self.call_c(fn)
	self.Emit("MOVD", _ARG_ic, _IC)                 // MOVD ic, IC
}

/** Assembler Error Handlers **/

var (
	_F_convT64        = jit.Func(rt.ConvT64)
	_F_error_wrap     = jit.Func(error_wrap)
	_F_error_type     = jit.Func(error_type)
	_F_error_field    = jit.Func(error_field)
	_F_error_value    = jit.Func(error_value)
	_F_error_mismatch = jit.Func(error_mismatch)
)

var (
	_I_int8    , _T_int8    = rtype(reflect.TypeOf(int8(0)))
	_I_int16   , _T_int16   = rtype(reflect.TypeOf(int16(0)))
	_I_int32   , _T_int32   = rtype(reflect.TypeOf(int32(0)))
	_I_uint8   , _T_uint8   = rtype(reflect.TypeOf(uint8(0)))
	_I_uint16  , _T_uint16  = rtype(reflect.TypeOf(uint16(0)))
	_I_uint32  , _T_uint32  = rtype(reflect.TypeOf(uint32(0)))
	_I_float32 , _T_float32 = rtype(reflect.TypeOf(float32(0)))
)

var (
	_T_error                    = rt.UnpackType(errorType)
	_I_base64_CorruptInputError = jit.Itab(_T_error, base64CorruptInputError)
)

var (
	_V_stackOverflow              = jit.Imm(int64(uintptr(unsafe.Pointer(&stackOverflow))))
	_I_json_UnsupportedValueError = jit.Itab(_T_error, reflect.TypeOf(new(json.UnsupportedValueError)))
	_I_json_MismatchTypeError     = jit.Itab(_T_error, reflect.TypeOf(new(MismatchTypeError)))
	_I_json_MismatchQuotedError   = jit.Itab(_T_error, reflect.TypeOf(new(MismatchQuotedError)))
)

func (self *_Assembler) type_error() {
	self.Link(_LB_type_error)                      // _type_error:
	self.call_go(_F_error_type)                    // CALL_GO error_type
	self.Sjmp("B", _LB_error)                      // B     _error
}

func (self *_Assembler) mismatch_error() {
	self.Link(_LB_mismatch_error)                  // _mismatch_error:
	self.Emit("MOVD", _VAR_et, _ET)                // MOVD _VAR_et, ET
	self.Emit("MOVD", _I_json_MismatchTypeError, _X1) // MOVD _I_json_MismatchType, X1
	self.Emit("CMP", _ET, _X1)                     // CMP ET, X1
	self.Emit("MOVD", jit.Ptr(_ST, _EpOffset), _EP)   // MOVD stack.Ep, EP
	self.Sjmp("BEQ", _LB_error)                    // BEQ _error
	self.Emit("MOVD", _ARG_sp, _X0)                // MOVD sp, X0
	self.Emit("MOVD", _ARG_sl, _X1)                // MOVD sl, X1
	self.Emit("MOVD", _VAR_ic, _X2)                // MOVD VAR_ic, X2
	self.Emit("MOVD", _VAR_et, _X3)                // MOVD VAR_et, X3
	self.call_go(_F_error_mismatch)                // CALL_GO error_mismatch
	self.Sjmp("B", _LB_error)                      // B     _error
}

func (self *_Assembler) field_error() {
	self.Link(_LB_field_error)                      // _field_error:
	self.Emit("MOVD", _ARG_sv_p, _X0)               // MOVD   sv.p, X0
	self.Emit("MOVD", _ARG_sv_n, _X1)               // MOVD   sv.n, X1
	self.call_go(_F_error_field)                    // CALL_GO error_field
	self.Sjmp("B", _LB_error)                      // B     _error
}

func (self *_Assembler) range_error() {
	self.Link(_LB_range_error)                      // _range_error:
	self.Emit("MOVD", _ET, _X1)                     // MOVD    ET, X1
	self.slice_from(_VAR_st_Ep, 0)                  // SLICE   st.Ep, #0
	self.Emit("MOVD", _X3, _X0)                     // MOVD    X3, X0
	self.Emit("MOVD", _EP, _X3)                     // MOVD    EP, X3
	self.Emit("MOVD", _X4, _X1)                     // MOVD    X4, X1
	self.call_go(_F_error_value)                    // CALL_GO error_value
	self.Sjmp("B", _LB_error)                      // B     _error
}

func (self *_Assembler) stack_error() {
	self.Link(_LB_stack_error)                      // _stack_error:
	self.Emit("MOVD", _V_stackOverflow, _EP)        // MOVD ${_V_stackOverflow}, EP
	self.Emit("MOVD", _I_json_UnsupportedValueError, _ET) // MOVD ${_I_json_UnsupportedValueError}, ET
	self.Sjmp("B", _LB_error)                      // B  _error
}

func (self *_Assembler) base64_error() {
	self.Link(_LB_base64_error)
	self.Emit("NEG", _X0, _X0)                      // NEG    X0, X0
	self.Emit("SUB", _X0, _X0, jit.Imm(1))          // SUB    X0, X0, #1
	self.call_go(_F_convT64)                        // CALL_GO convT64
	self.Emit("MOVD", _X0, _EP)                     // MOVD    X0, EP
	self.Emit("MOVD", _I_base64_CorruptInputError, _ET) // MOVD    ${itab(base64.CorruptInputError)}, ET
	self.Sjmp("B", _LB_error)                      // B     _error
}

func (self *_Assembler) parsing_error() {
	self.Link(_LB_eof_error)                        // _eof_error:
	self.Emit("MOVD", _IL, _IC)                     // MOVD    IL, IC
	self.Emit("MOVW", jit.Imm(int64(types.ERR_EOF)), _EP) // MOVW    ${types.ERR_EOF}, EP
	self.Sjmp("B", _LB_parsing_error)               // B     _parsing_error
	self.Link(_LB_unquote_error)                    // _unquote_error:
	self.Emit("SUB", _SI, _SI, _VAR_sr)            // SUB    SI, SI, sr
	self.Emit("SUB", _IC, _IC, _SI)                // SUB    IC, IC, SI
	self.Link(_LB_parsing_error_v)                  // _parsing_error_v:
	self.Emit("MOVD", _X0, _EP)                    // MOVD    X0, EP
	self.Emit("NEG", _EP, _EP)                     // NEG    EP, EP
	self.Sjmp("B", _LB_parsing_error)               // B     _parsing_error
	self.Link(_LB_char_m3_error)                    // _char_m3_error:
	self.Emit("SUB", _IC, _IC, jit.Imm(1))         // SUB    IC, IC, #1
	self.Link(_LB_char_m2_error)                    // _char_m2_error:
	self.Emit("SUB", _IC, _IC, jit.Imm(2))         // SUB    IC, IC, #2
	self.Sjmp("B", _LB_char_0_error)               // B     _char_0_error
	self.Link(_LB_im_error)                         // _im_error:
	self.Emit("CMPB", _X1, jit.Sib(_IP, _IC, 1, 0)) // CMPB    X1, (IP)(IC)
	self.Sjmp("BNE", _LB_char_0_error)             // BNE     _char_0_error
	self.Emit("UBFX", _X1, _X1, jit.Imm(8), jit.Imm(8)) // UBFX X1, X1, #8, #8
	self.Emit("CMPB", _X1, jit.Sib(_IP, _IC, 1, 1)) // CMPB    X1, 1(IP)(IC)
	self.Sjmp("BNE", _LB_char_1_error)             // BNE     _char_1_error
	self.Emit("UBFX", _X1, _X1, jit.Imm(8), jit.Imm(8)) // UBFX X1, X1, #8, #8
	self.Emit("CMPB", _X1, jit.Sib(_IP, _IC, 1, 2)) // CMPB    X1, 2(IP)(IC)
	self.Sjmp("BNE", _LB_char_2_error)             // BNE     _char_2_error
	self.Sjmp("B", _LB_char_3_error)               // B     _char_3_error
	self.Link(_LB_char_4_error)                    // _char_4_error:
	self.Emit("ADD", _IC, _IC, jit.Imm(1))         // ADD    IC, IC, #1
	self.Link(_LB_char_3_error)                    // _char_3_error:
	self.Emit("ADD", _IC, _IC, jit.Imm(1))         // ADD    IC, IC, #1
	self.Link(_LB_char_2_error)                    // _char_2_error:
	self.Emit("ADD", _IC, _IC, jit.Imm(1))         // ADD    IC, IC, #1
	self.Link(_LB_char_1_error)                    // _char_1_error:
	self.Emit("ADD", _IC, _IC, jit.Imm(1))         // ADD    IC, IC, #1
	self.Link(_LB_char_0_error)                    // _char_0_error:
	self.Emit("MOVW", jit.Imm(int64(types.ERR_INVALID_CHAR)), _EP) // MOVW    ${types.ERR_INVALID_CHAR}, EP
	self.Link(_LB_parsing_error)                    // _parsing_error:
	self.Emit("MOVD", _EP, _X3)                    // MOVD    EP, X3
	self.Emit("MOVD", _ARG_sp, _X0)                // MOVD  sp, X0
	self.Emit("MOVD", _ARG_sl, _X1)                // MOVD  sl, X1
	self.Emit("MOVD", _IC, _X2)                    // MOVD    IC, X2
	self.call_go(_F_error_wrap)                     // CALL_GO error_wrap
	self.Sjmp("B", _LB_error)                      // B     _error
}

func (self *_Assembler) _asm_OP_dismatch_err(p *_Instr) {
	self.Emit("MOVD", _IC, _VAR_ic)                 // MOVD IC, VAR_ic
	self.Emit("MOVD", jit.Type(p.vt()), _ET)        // MOVD ${p.vt()}, ET
	self.Emit("MOVD", _ET, _VAR_et)                 // MOVD ET, VAR_et
}

func (self *_Assembler) _asm_OP_go_skip(p *_Instr) {
	self.Byte(0x50, 0x00, 0x00, 0x58)              // ADRP X16, pc+...
	self.Emit("ADD", _X16, _X16, jit.Imm(p.vi())) // ADD X16, X16, #{p.vi()}
	self.Emit("MOVD", _X16, _VAR_pc)                // MOVD X16, VAR_pc
	self.Sjmp("B", _LB_skip_one)                   // B     _skip_one
}

var _F_IndexByte = jit.Func(strings.IndexByte)

func (self *_Assembler) _asm_OP_skip_empty(p *_Instr) {
	self.call_sf(_F_skip_one)                       // CALL_SF skip_one
	self.Emit("CMP", _X0, _ZR)                      // CMP    X0, ZR
	self.Sjmp("BMI", _LB_parsing_error_v)          // BMI      _parse_error_v
	self.Emit("TST", jit.Imm(_F_disable_unknown), _ARG_fv) // TST ${_F_disable_unknown}, fv
	self.Xjmp("BCC", p.vi())
	self.Emit("ADD", _X1, _IC, _X0)                // ADD X1, IC, X0
	self.Emit("MOVD", _X1, _ARG_sv_n)               // MOVD X1, sv.n
	self.Emit("ADD", _X0, _IP, _X0)                // ADD X0, IP, X0
	self.Emit("MOVD", _X0, _ARG_sv_p)               // MOVD X0, sv.p
	self.Emit("MOVD", jit.Imm(':'), _X2)           // MOVD ':', X2
	self.call_go(_F_IndexByte)
	self.Emit("CMP", _X0, _ZR)                      // CMP X0, ZR
	// disallow unknown field
	self.Sjmp("BPL", _LB_field_error)              // BPL _field_error
}

func (self *_Assembler) skip_one() {
	self.Link(_LB_skip_one)                         // _skip:
	self.Emit("MOVD", _VAR_ic, _IC)                 // MOVD    _VAR_ic, IC
	self.call_sf(_F_skip_one)                       // CALL_SF skip_one
	self.Emit("CMP", _X0, _ZR)                      // CMP    X0, ZR
	self.Sjmp("BMI", _LB_parsing_error_v)          // BMI      _parse_error_v
	self.Emit("MOVD", _VAR_pc, _X16)               // MOVD    pc, X16
	self.Rjmp("BR", _X16)                           // BR     (X16)
}

func (self *_Assembler) skip_key_value() {
	self.Link(_LB_skip_key_value)                   // _skip:
	// skip the key
	self.Emit("MOVD", _VAR_ic, _IC)                 // MOVD    _VAR_ic, IC
	self.call_sf(_F_skip_one)                       // CALL_SF skip_one
	self.Emit("CMP", _X0, _ZR)                      // CMP    X0, ZR
	self.Sjmp("BMI", _LB_parsing_error_v)          // BMI      _parse_error_v
	// match char ':'
	self.lspace("_global_1")
	self.Emit("MOVBU", _X3, jit.Sib(_IP, _IC, 1, 0)) // MOVBU (IP)(IC), X3
	self.Emit("CMP", _X3, jit.Imm(':'))             // CMP X3, #':'
	self.Sjmp("BNE", _LB_parsing_error_v)          // BNE     _parse_error_v
	self.Emit("ADD", _IC, _IC, jit.Imm(1))         // ADD    IC, IC, #1
	self.lspace("_global_2")
	// skip the value
	self.call_sf(_F_skip_one)                       // CALL_SF skip_one
	self.Emit("CMP", _X0, _ZR)                      // CMP    X0, ZR
	self.Sjmp("BMI", _LB_parsing_error_v)          // BMI      _parse_error_v
	// jump back to specified address
	self.Emit("MOVD", _VAR_pc, _X16)               // MOVD    pc, X16
	self.Rjmp("BR", _X16)                           // BR     (X16)
}

/** Memory Management Routines **/

var (
	_T_byte     = jit.Type(byteType)
	_F_mallocgc = jit.Func(rt.Mallocgc)
)

func (self *_Assembler) malloc_X0(nb obj.Addr, ret obj.Addr) {
	self.Emit("MOVD", nb, _X0)                      // MOVD    ${nb}, X0
	self.Emit("MOVD", _T_byte, _X1)                 // MOVD    ${type(byte)}, X1
	self.Emit("MOVD", _ZR, _X2)                    // MOVD    ZR, X2
	self.call_go(_F_mallocgc)                       // CALL_GO mallocgc
	self.Emit("MOVD", _X0, ret)                    // MOVD    X0, ${ret}
}

func (self *_Assembler) valloc(vt reflect.Type, ret obj.Addr) {
	self.Emit("MOVD", jit.Imm(int64(vt.Size())), _X0) // MOVD    ${vt.Size()}, X0
	self.Emit("MOVD", jit.Type(vt), _X1)            // MOVD    ${vt}, X1
	self.Emit("MOVD", jit.Imm(1), _X2)              // MOVD    #1, X2
	self.call_go(_F_mallocgc)                       // CALL_GO mallocgc
	self.Emit("MOVD", _X0, ret)                    // MOVD    X0, ${ret}
}

func (self *_Assembler) valloc_X0(vt reflect.Type) {
	self.Emit("MOVD", jit.Imm(int64(vt.Size())), _X0) // MOVD    ${vt.Size()}, X0
	self.Emit("MOVD", jit.Type(vt), _X1)            // MOVD    ${vt}, X1
	self.Emit("MOVD", jit.Imm(1), _X2)              // MOVD    #1, X2
	self.call_go(_F_mallocgc)                       // CALL_GO mallocgc
}

func (self *_Assembler) vfollow(vt reflect.Type) {
	self.Emit("MOVD", jit.Ptr(_VP, 0), _X0)         // MOVD   (VP), X0
	self.Emit("CMP", _X0, _ZR)                      // CMP    X0, ZR
	self.Sjmp("BNE", "_end_{n}")                    // BNE    _end_{n}
	self.valloc_X0(vt)                              // VALLOC ${vt}, X0
	self.WritePtrAX(1, jit.Ptr(_VP, 0), true)       // MOVQ   X0, (VP)
	self.Link("_end_{n}")                           // _end_{n}:
	self.Emit("MOVD", _X0, _VP)                     // MOVD   X0, VP
}

/** Value Parsing Routines **/

var (
	_F_vstring   = jit.Imm(int64(native.S_vstring))
	_F_vnumber   = jit.Imm(int64(native.S_vnumber))
	_F_vsigned   = jit.Imm(int64(native.S_vsigned))
	_F_vunsigned = jit.Imm(int64(native.S_vunsigned))
)

func (self *_Assembler) check_err(vt reflect.Type, pin string, pin2 int) {
	self.Emit("MOVD", _VAR_st_Vt, _X0)              // MOVD st.Vt, X0
	self.Emit("CMP", _X0, _ZR)                      // CMP    X0, ZR
	// try to skip the value
	if vt != nil {
		self.Sjmp("BPL", "_check_err_{n}")           // BPL  _parsing_error_v
		self.Emit("MOVD", jit.Type(vt), _ET)
		self.Emit("MOVD", _ET, _VAR_et)
		if pin2 != -1 {
			self.Emit("SUB", _X1, _X1, jit.Imm(1)) // SUB X1, X1, #1
			self.Emit("MOVD", _X1, _VAR_ic)
			self.Byte(0x50, 0x00, 0x00, 0x58)      // ADRP X16, pc+...
			self.Emit("ADD", _X16, _X16, jit.Imm(pin2)) // ADD X16, X16, #{pin2}
			self.Emit("MOVD", _X16, _VAR_pc)
			self.Sjmp("B", _LB_skip_key_value)
		} else {
			self.Emit("MOVD", _X1, _VAR_ic)
			self.Byte(0x50, 0x00, 0x00, 0x58)      // ADRP X16, pc+...
			self.Sref(pin, 4)
			self.Emit("ADD", _X16, _X16, _X16)       // ADD X16, X16, X16
			self.Emit("MOVD", _X16, _VAR_pc)
			self.Sjmp("B", _LB_skip_one)
		}
		self.Link("_check_err_{n}")
	} else {
		self.Sjmp("BMI", _LB_parsing_error_v)        // BMI    _parsing_error_v
	}
}

func (self *_Assembler) check_eof(d int64) {
	if d == 1 {
		self.Emit("CMP", _IC, _IL)                   // CMP IC, IL
		self.Sjmp("BHS", _LB_eof_error)              // BHS  _eof_error
	} else {
		self.Emit("ADD", _X0, _IC, jit.Imm(d))       // ADD X0, IC, ${d}
		self.Emit("CMP", _X0, _IL)                   // CMP X0, IL
		self.Sjmp("BHI", _LB_eof_error)              // BHI   _eof_error
	}
}

func (self *_Assembler) parse_string() {
	self.Emit("MOVD", _ARG_fv, _X2)                 // MOVD fv, X2
	self.call_vf(_F_vstring)
	self.check_err(nil, "", -1)
}

func (self *_Assembler) parse_number(vt reflect.Type, pin string, pin2 int) {
	self.Emit("MOVD", _IC, _X1)                     // save ic when call native func
	self.call_vf(_F_vnumber)
	self.check_err(vt, pin, pin2)
}

func (self *_Assembler) parse_signed(vt reflect.Type, pin string, pin2 int) {
	self.Emit("MOVD", _IC, _X1)                     // save ic when call native func
	self.call_vf(_F_vsigned)
	self.check_err(vt, pin, pin2)
}

func (self *_Assembler) parse_unsigned(vt reflect.Type, pin string, pin2 int) {
	self.Emit("MOVD", _IC, _X1)                     // save ic when call native func
	self.call_vf(_F_vunsigned)
	self.check_err(vt, pin, pin2)
}

// Pointer: X0, Size: X1, Return: X16
func (self *_Assembler) copy_string() {
	self.Link("_copy_string")
	self.Emit("MOVD", _X0, _VAR_bs_p)
	self.Emit("MOVD", _X1, _VAR_bs_n)
	self.Emit("MOVD", _X16, _VAR_bs_LR)
	self.malloc_X0(_X1, _ARG_sv_p)
	self.Emit("MOVD", _VAR_bs_p, _X1)
	self.Emit("MOVD", _VAR_bs_n, _X2)
	self.call_go(_F_memmove)
	self.Emit("MOVD", _ARG_sv_p, _X0)
	self.Emit("MOVD", _VAR_bs_n, _X1)
	self.Emit("MOVD", _VAR_bs_LR, _X16)
	self.Rjmp("BR", _X16)
}

// Pointer: X0, Size: X1, Return: X16
func (self *_Assembler) escape_string() {
	self.Link("_escape_string")
	self.Emit("MOVD", _X0, _VAR_bs_p)
	self.Emit("MOVD", _X1, _VAR_bs_n)
	self.Emit("MOVD", _X16, _VAR_bs_LR)
	self.malloc_X0(_X1, _X2)                         // MALLOC X1, X2
	self.Emit("MOVD", _X2, _ARG_sv_p)
	self.Emit("MOVD", _VAR_bs_p, _X0)
	self.Emit("MOVD", _VAR_bs_n, _X1)
	self.Emit("ADD", _X2, _SP, jit.Imm(_FP_fargs + _FP_saves + 104)) // ADD X2, SP, #sr_offset
	self.Emit("MOVD", _ZR, _X3)                      // XORL X3, X3
	self.Emit("TST", jit.Imm(_F_disable_urc), _ARG_fv) // TST ${_F_disable_urc}, fv
	self.Emit("CSET", _X3, jit.Imm(1))               // CSET X3, NE
	self.Emit("LSL", _X3, _X3, jit.Imm(types.B_UNICODE_REPLACE)) // LSL X3, X3, ${types.B_UNICODE_REPLACE}
	self.call_c(_F_unquote)                          // CALL   unquote
	self.Emit("MOVD", _VAR_bs_n, _X1)                // MOVD   ${n}, X1
	self.Emit("ADD", _X1, _X1, jit.Imm(1))          // ADD    X1, X1, #1
	self.Emit("CMP", _X0, _ZR)                      // CMP    X0, ZR
	self.Sjmp("BMI", _LB_unquote_error)             // BMI     _unquote_error
	self.Emit("MOVD", _X0, _X1)
	self.Emit("MOVD", _ARG_sv_p, _X0)
	self.Emit("MOVD", _VAR_bs_LR, _X16)
	self.Rjmp("BR", _X16)
}

func (self *_Assembler) escape_string_twice() {
	self.Link("_escape_string_twice")
	self.Emit("MOVD", _X0, _VAR_bs_p)
	self.Emit("MOVD", _X1, _VAR_bs_n)
	self.Emit("MOVD", _X16, _VAR_bs_LR)
	self.malloc_X0(_X1, _X2)                         // MALLOC X1, X2
	self.Emit("MOVD", _X2, _ARG_sv_p)
	self.Emit("MOVD", _VAR_bs_p, _X0)
	self.Emit("MOVD", _VAR_bs_n, _X1)
	self.Emit("ADD", _X2, _SP, jit.Imm(_FP_fargs + _FP_saves + 104)) // ADD X2, SP, #sr_offset
	self.Emit("MOVW", jit.Imm(types.F_DOUBLE_UNQUOTE), _X3) // MOVW ${types.F_DOUBLE_UNQUOTE}, X3
	self.Emit("TST", jit.Imm(_F_disable_urc), _ARG_fv) // TST ${_F_disable_urc}, fv
	self.Emit("CSET", _X4, jit.Imm(1))               // CSET X4, NE
	self.Emit("LSL", _X4, _X4, jit.Imm(types.B_UNICODE_REPLACE)) // LSL X4, X4, ${types.B_UNICODE_REPLACE}
	self.Emit("ORR", _X3, _X3, _X4)                 // ORR X3, X3, X4
	self.call_c(_F_unquote)                          // CALL   unquote
	self.Emit("MOVD", _VAR_bs_n, _X1)                // MOVD   ${n}, X1
	self.Emit("ADD", _X1, _X1, jit.Imm(3))          // ADD    X1, X1, #3
	self.Emit("CMP", _X0, _ZR)                      // CMP    X0, ZR
	self.Sjmp("BMI", _LB_unquote_error)             // BMI     _unquote_error
	self.Emit("MOVD", _X0, _X1)
	self.Emit("MOVD", _ARG_sv_p, _X0)
	self.Emit("MOVD", _VAR_bs_LR, _X16)
	self.Rjmp("BR", _X16)
}

/** Range Checking Routines **/

var (
	_V_max_f32 = jit.Imm(int64(uintptr(unsafe.Pointer(_Vp_max_f32))))
	_V_min_f32 = jit.Imm(int64(uintptr(unsafe.Pointer(_Vp_min_f32))))
)

var (
	_Vp_max_f32 = new(float32)
	_Vp_min_f32 = new(float32)
)

func init() {
	*_Vp_max_f32 = math.MaxFloat32
	*_Vp_min_f32 = -math.MaxFloat32
}

func (self *_Assembler) range_single_D0() {
	self.Emit("FCVT", _S0, _D0, jit.Imm(0))         // FCVT S0, D0
	self.Emit("MOVD", _V_max_f32, _X1)              // MOVD _max_f32, X1
	self.Emit("MOVD", jit.Gitab(_I_float32), _ET)   // MOVD ${itab(float32)}, ET
	self.Emit("MOVD", jit.Gtype(_T_float32), _EP)   // MOVD ${type(float32)}, EP
	self.Emit("FCMP", _S0, jit.Ptr(_X1, 0))         // FCMP S0, (X1)
	self.Sjmp("BGT", _LB_range_error)              // BGT     _range_error
	self.Emit("MOVD", _V_min_f32, _X1)              // MOVD _min_f32, X1
	self.Emit("FCMP", _S0, jit.Ptr(_X1, 0))         // FCMP S0, (X1)
	self.Sjmp("BLT", _LB_range_error)              // BLT     _range_error
}

func (self *_Assembler) range_signed_X1(i *rt.GoItab, t *rt.GoType, a int64, b int64) {
	self.Emit("MOVD", _VAR_st_Iv, _X1)              // MOVD st.Iv, X1
	self.Emit("MOVD", jit.Gitab(i), _ET)            // MOVD ${i}, ET
	self.Emit("MOVD", jit.Gtype(t), _EP)            // MOVD ${t}, EP
	self.Emit("CMP", _X1, jit.Imm(a))               // CMP X1, ${a}
	self.Sjmp("BLT", _LB_range_error)              // BLT   _range_error
	self.Emit("CMP", _X1, jit.Imm(b))               // CMP X1, ${B}
	self.Sjmp("BGT", _LB_range_error)              // BGT   _range_error
}

func (self *_Assembler) range_unsigned_X1(i *rt.GoItab, t *rt.GoType, v uint64) {
	self.Emit("MOVD", _VAR_st_Iv, _X1)              // MOVD  st.Iv, X1
	self.Emit("MOVD", jit.Gitab(i), _ET)            // MOVD  ${i}, ET
	self.Emit("MOVD", jit.Gtype(t), _EP)            // MOVD  ${t}, EP
	self.Emit("CMP", _X1, _ZR)                     // CMP X1, ZR
	self.Sjmp("BMI", _LB_range_error)              // BMI    _range_error
	self.Emit("CMP", _X1, jit.Imm(int64(v)))       // CMP  X1, ${a}
	self.Sjmp("BHI", _LB_range_error)              // BHI   _range_error
}

func (self *_Assembler) range_uint32_X1(i *rt.GoItab, t *rt.GoType) {
	self.Emit("MOVD", _VAR_st_Iv, _X1)              // MOVD  st.Iv, X1
	self.Emit("MOVD", jit.Gitab(i), _ET)            // MOVD  ${i}, ET
	self.Emit("MOVD", jit.Gtype(t), _EP)            // MOVD  ${t}, EP
	self.Emit("CMP", _X1, _ZR)                     // CMP X1, ZR
	self.Sjmp("BMI", _LB_range_error)              // BMI    _range_error
	self.Emit("MOVWU", _X2, _X1)                   // MOVWU X1, X2
	self.Emit("CMP", _X1, _X2)                     // CMP  X1, X2
	self.Sjmp("BNE", _LB_range_error)              // BNE   _range_error
}

/** String Manipulating Routines **/

var (
	_F_unquote = jit.Imm(int64(native.S_unquote))
)

func (self *_Assembler) slice_from(p obj.Addr, d int64) {
	self.Emit("MOVD", p, _X1)                       // MOVD    ${p}, X1
	self.slice_from_r(_X1, d)                       // SLICE_R X1, ${d}
}

func (self *_Assembler) slice_from_r(p obj.Addr, d int64) {
	self.Emit("ADD", _X0, _IP, p)                   // ADD X0, IP, ${p}
	self.Emit("NEG", p, p)                          // NEG ${p}
	self.Emit("ADD", _X1, _IC, p)                   // ADD X1, IC, ${p}
	self.Emit("ADD", _X1, _X1, jit.Imm(d))          // ADD X1, X1, ${d}
}

func (self *_Assembler) unquote_once(p obj.Addr, n obj.Addr, stack bool, copy bool) {
	self.slice_from(_VAR_st_Iv, -1)                 // SLICE  st.Iv, #-1
	self.Emit("CMP", _VAR_st_Ep, jit.Imm(-1))       // CMP   st.Ep, #-1
	self.Sjmp("BEQ", "_noescape_{n}")               // BEQ     _escape_{n}
	self.Byte(0x50, 0x00, 0x00, 0x58)             // ADRP X16, pc+...
	self.Sref("_unquote_once_write_{n}", 4)
	self.Sjmp("B", "_escape_string")
	self.Link("_noescape_{n}")
	if copy {
		self.Emit("TST", jit.Imm(_F_copy_string), _ARG_fv)
		self.Sjmp("BCC", "_unquote_once_write_{n}")
		self.Byte(0x50, 0x00, 0x00, 0x58)         // ADRP X16, pc+...
		self.Sref("_unquote_once_write_{n}", 4)
		self.Sjmp("B", "_copy_string")
	}
	self.Link("_unquote_once_write_{n}")
	self.Emit("MOVD", _X1, n)                        // MOVD   X1, ${n}
	if stack {
		self.Emit("MOVD", _X0, p)
	} else {
		self.WriteRecNotAX(10, _X0, p, false, false)
	}
}

func (self *_Assembler) unquote_twice(p obj.Addr, n obj.Addr, stack bool) {
	self.Emit("CMP", _VAR_st_Ep, jit.Imm(-1))        // CMP   st.Ep, #-1
	self.Sjmp("BEQ", _LB_eof_error)                  // BEQ     _eof_error
	self.Emit("SUB", _X2, _IC, jit.Imm(3))         // SUB X2, IC, #3
	self.Emit("MOVBU", _X3, jit.Sib(_IP, _X2, 1, 0)) // MOVBU -3(IP)(IC), X3
	self.Emit("CMP", _X3, jit.Imm('\\'))           // CMP X3, #'\\'
	self.Sjmp("BNE", _LB_char_m3_error)             // BNE    _char_m3_error
	self.Emit("ADD", _X2, _X2, jit.Imm(1))         // ADD X2, X2, #1
	self.Emit("MOVBU", _X3, jit.Sib(_IP, _X2, 1, 0)) // MOVBU -2(IP)(IC), X3
	self.Emit("CMP", _X3, jit.Imm('"'))            // CMP X3, #'"'
	self.Sjmp("BNE", _LB_char_m2_error)             // BNE    _char_m2_error
	self.slice_from(_VAR_st_Iv, -3)                 // SLICE  st.Iv, #-3
	self.Emit("MOVD", _X1, _VAR_st_Iv)             // MOVD   st.Iv, X1
	self.Emit("ADD", _X0, _X1, _VAR_st_Iv)         // ADD X0, X1, st.Iv
	self.Emit("CMP", _VAR_st_Ep, _X0)              // CMP st.Ep, X0
	self.Sjmp("BEQ", "_noescape_{n}")               // BEQ     _noescape_{n}
	self.Byte(0x50, 0x00, 0x00, 0x58)             // ADRP X16, pc+...
	self.Sref("_unquote_twice_write_{n}", 4)
	self.Sjmp("B", "_escape_string_twice")
	self.Link("_noescape_{n}")                      // _noescape_{n}:
	self.Emit("TST", jit.Imm(_F_copy_string), _ARG_fv)
	self.Sjmp("BCC", "_unquote_twice_write_{n}")
	self.Byte(0x50, 0x00, 0x00, 0x58)             // ADRP X16, pc+...
	self.Sref("_unquote_twice_write_{n}", 4)
	self.Sjmp("B", "_copy_string")
	self.Link("_unquote_twice_write_{n}")
	self.Emit("MOVD", _X1, n)                        // MOVD   X1, ${n}
	if stack {
		self.Emit("MOVD", _X0, p)
	} else {
		self.WriteRecNotAX(12, _X0, p, false, false)
	}
	self.Link("_unquote_twice_end_{n}")
}

/** Memory Clearing Routines **/

var (
	_F_memclrHasPointers    = jit.Func(rt.MemclrHasPointers)
	_F_memclrNoHeapPointers = jit.Func(rt.MemclrNoHeapPointers)
)

func (self *_Assembler) mem_clear_fn(ptrfree bool) {
	if !ptrfree {
		self.call_go(_F_memclrHasPointers)
	} else {
		self.call_go(_F_memclrNoHeapPointers)
	}
}

func (self *_Assembler) mem_clear_rem(size int64, ptrfree bool) {
	self.Emit("MOVD", jit.Imm(size), _X1)            // MOVD    ${size}, X1
	self.Emit("MOVD", jit.Ptr(_ST, 0), _X0)         // MOVD    (ST), X0
	self.Emit("MOVD", jit.Sib(_ST, _X0, 1, 0), _X0) // MOVD    (ST)(X0), X0
	self.Emit("SUB", _X0, _VP, _X0)                 // SUB    VP, X0
	self.Emit("ADD", _X1, _X1, _X0)                 // ADD    X1, X1, X0
	self.Emit("MOVD", _VP, _X0)                     // MOVD    VP, (SP)
	self.mem_clear_fn(ptrfree)                      // CALL_GO memclr{Has,NoHeap}Pointers
}

/** Map Assigning Routines **/

var (
	_F_mapassign           = jit.Func(rt.Mapassign)
	_F_mapassign_fast32    = jit.Func(rt.Mapassign_fast32)
	_F_mapassign_faststr   = jit.Func(rt.Mapassign_faststr)
	_F_mapassign_fast64ptr = jit.Func(rt.Mapassign_fast64ptr)
)

var (
	_F_decodeJsonUnmarshaler obj.Addr
	_F_decodeJsonUnmarshalerQuoted obj.Addr
	_F_decodeTextUnmarshaler obj.Addr
)

func init() {
	_F_decodeJsonUnmarshaler = jit.Func(decodeJsonUnmarshaler)
	_F_decodeJsonUnmarshalerQuoted = jit.Func(decodeJsonUnmarshalerQuoted)
	_F_decodeTextUnmarshaler = jit.Func(decodeTextUnmarshaler)
}

func (self *_Assembler) mapaccess_ptr(t reflect.Type) {
	if rt.MapType(rt.UnpackType(t)).IndirectElem() {
		self.vfollow(t.Elem())
	}
}

func (self *_Assembler) mapassign_std(t reflect.Type, v obj.Addr) {
	self.Emit("ADD", _X0, v, _ZR)                  // ADD      X0, ${v}, ZR
	self.mapassign_call_from_X0(t, _F_mapassign)    // MAPASSIGN ${t}, mapassign
}

func (self *_Assembler) mapassign_str_fast(t reflect.Type, p obj.Addr, n obj.Addr) {
	self.Emit("MOVD", jit.Type(t), _X0)            // MOVD    ${t}, X0
	self.Emit("MOVD", _VP, _X1)                    // MOVD    VP, X1
	self.Emit("MOVD", p, _X2)                      // MOVD    ${p}, X2
	self.Emit("MOVD", n, _X3)                      // MOVD    ${n}, X3
	self.call_go(_F_mapassign_faststr)              // CALL_GO ${fn}
	self.Emit("MOVD", _X0, _VP)                    // MOVD    X0, VP
	self.mapaccess_ptr(t)
}

func (self *_Assembler) mapassign_call_from_X0(t reflect.Type, fn obj.Addr) {
	self.Emit("MOVD", _X0, _X2)                     // MOVD X0, X2
	self.Emit("MOVD", jit.Type(t), _X0)            // MOVD    ${t}, X0
	self.Emit("MOVD", _VP, _X1)                    // MOVD    VP, X1
	self.call_go(fn)                                // CALL_GO ${fn}
	self.Emit("MOVD", _X0, _VP)                    // MOVD    X0, VP
}

func (self *_Assembler) mapassign_fastx(t reflect.Type, fn obj.Addr) {
	self.mapassign_call_from_X0(t, fn)
	self.mapaccess_ptr(t)
}

func (self *_Assembler) mapassign_utext(t reflect.Type, addressable bool) {
	pv := false
	vk := t.Key()
	tk := t.Key()

	/* deref pointer if needed */
	if vk.Kind() == reflect.Ptr {
		pv = true
		vk = vk.Elem()
	}

	/* addressable value with pointer receiver */
	if addressable {
		pv = false
		tk = reflect.PtrTo(tk)
	}

	/* allocate the key, and call the unmarshaler */
	self.valloc(vk, _X1)                            // VALLOC  ${vk}, X1
	// must spill vk pointer since next call_go may invoke GC
	self.Emit("MOVD", _X1, _ARG_vk)
	self.Emit("MOVD", jit.Type(tk), _X0)            // MOVD    ${tk}, X0
	self.Emit("MOVD", _ARG_sv_p, _X2)               // MOVD    sv.p, X2
	self.Emit("MOVD", _ARG_sv_n, _X3)               // MOVD    sv.n, X3
	self.call_go(_F_decodeTextUnmarshaler)          // CALL_GO decodeTextUnmarshaler
	self.Emit("CMP", _ET, _ZR)                      // CMP   ET, ZR
	self.Sjmp("BNE", _LB_error)                     // BNE     _error
	self.Emit("MOVD", _ARG_vk, _X0)                 // MOVD    VAR.vk, X0
	self.Emit("MOVD", _ZR, _ARG_vk)

	/* select the correct assignment function */
	if !pv {
		self.mapassign_call_from_X0(t, _F_mapassign)
	} else {
		self.mapassign_fastx(t, _F_mapassign_fast64ptr)
	}
}

/** External Unmarshaler Routines **/

var (
	_F_skip_one = jit.Imm(int64(native.S_skip_one))
	_F_skip_array  = jit.Imm(int64(native.S_skip_array))
	_F_skip_number = jit.Imm(int64(native.S_skip_number))
)

func (self *_Assembler) unmarshal_json(t reflect.Type, deref bool, f obj.Addr) {
	self.call_sf(_F_skip_one)                       // CALL_SF   skip_one
	self.Emit("CMP", _X0, _ZR)                      // CMP     X0, ZR
	self.Sjmp("BMI", _LB_parsing_error_v)           // BMI        _parse_error_v
	self.Emit("MOVD", _IC, _VAR_ic)                 // store for mismatche error skip
	self.slice_from_r(_X0, 0)                       // SLICE_R   X0, #0
	self.Emit("MOVD", _X0, _ARG_sv_p)               // MOVD      X0, sv.p
	self.Emit("MOVD", _X1, _ARG_sv_n)               // MOVD      X1, sv.n
	self.unmarshal_func(t, f, deref)                // UNMARSHAL json, ${t}, ${deref}
}

func (self *_Assembler) unmarshal_text(t reflect.Type, deref bool) {
	self.parse_string()                             // PARSE     STRING
	self.unquote_once(_ARG_sv_p, _ARG_sv_n, true, true) // UNQUOTE   once, sv.p, sv.n
	self.unmarshal_func(t, _F_decodeTextUnmarshaler, deref) // UNMARSHAL text, ${t}, ${deref}
}

func (self *_Assembler) unmarshal_func(t reflect.Type, fn obj.Addr, deref bool) {
	pt := t
	vk := t.Kind()

	/* allocate the field if needed */
	if deref && vk == reflect.Ptr {
		self.Emit("MOVD", _VP, _X1)                 // MOVD   VP, X1
		self.Emit("MOVD", jit.Ptr(_X1, 0), _X1)     // MOVD   (X1), X1
		self.Emit("CMP", _X1, _ZR)                 // CMP  X1, ZR
		self.Sjmp("BNE", "_deref_{n}")             // BNE    _deref_{n}
		self.valloc(t.Elem(), _X1)                  // VALLOC ${t.Elem()}, X1
		self.WriteRecNotAX(3, _X1, jit.Ptr(_VP, 0), false, false) // MOVD   X1, (VP)
		self.Link("_deref_{n}")                     // _deref_{n}:
	} else {
		/* set value pointer */
		self.Emit("MOVD", _VP, _X1)                 // MOVD   (VP), X1
	}

	/* set value type */
	self.Emit("MOVD", jit.Type(pt), _X0)           // MOVD ${pt}, X0

	/* set the source string and call the unmarshaler */
	self.Emit("MOVD", _ARG_sv_p, _X2)              // MOVD    sv.p, X2
	self.Emit("MOVD", _ARG_sv_n, _X3)              // MOVD    sv.n, X3
	self.call_go(fn)                                // CALL_GO ${fn}
	self.Emit("CMP", _ET, _ZR)                      // CMP   ET, ZR
	if fn == _F_decodeJsonUnmarshalerQuoted {
		self.Sjmp("BEQ", "_unmarshal_func_end_{n}")        // BEQ   _unmarshal_func_end_{n}
		self.Emit("MOVD", _I_json_MismatchQuotedError, _X1) // MOVD _I_json_MismatchQuotedError, X1
		self.Emit("CMP", _ET, _X1)                   // CMP ET, X1
		self.Sjmp("BNE", _LB_error)                  // BNE     _error
		self.Emit("MOVD", jit.Type(t), _X1)          // store current type
		self.Emit("MOVD", _X1, _VAR_et)              // store current type as mismatched type
		self.Emit("MOVD", _VAR_ic, _IC)              // recover the pos at mismatched, continue to parse
		self.Emit("MOVD", _ZR, _ET)                  // clear ET
		self.Link("_unmarshal_func_end_{n}")
	} else {
		self.Sjmp("BNE", _LB_error)                  // BNE     _error
	}
}

/** Dynamic Decoding Routine **/

var (
	_F_decodeTypedPointer obj.Addr
)

func init() {
	_F_decodeTypedPointer = jit.Func(decodeTypedPointer)
}

func (self *_Assembler) decode_dynamic(vt obj.Addr, vp obj.Addr) {
	self.Emit("MOVD", vp, _X1)                       // MOVD    ${vp}, X1
	self.Emit("MOVD", vt, _X2)                       // MOVD    ${vt}, X2
	self.Emit("MOVD", _ARG_sp, _X0)                  // MOVD    sp, X0
	self.Emit("MOVD", _ARG_sl, _X1)                  // MOVD    sp, X1
	self.Emit("MOVD", _IC, _X2)                      // MOVD    IC, X2
	self.Emit("MOVD", _ST, _X3)                      // MOVD    ST, X3
	self.Emit("MOVD", _ARG_fv, _X4)                  // MOVD    fv, X4
	self.save(_REG_rt...)
	self.Emit("MOVD", _F_decodeTypedPointer, _X5)    // MOVD ${fn}, X5
	self.Rjmp("BLR", _X5)                           // BLR X5
	self.load(_REG_rt...)
	self.Emit("MOVD", _X0, _IC)                      // MOVD    X0, IC
	self.Emit("MOVD", _X1, _ET)                      // MOVD    X1, ET
	self.Emit("MOVD", _X2, _EP)                      // MOVD    X2, EP
	self.Emit("CMP", _ET, _ZR)                      // CMP    ET, ZR
	self.Sjmp("BEQ", "_decode_dynamic_end_{n}")      // BEQ, _decode_dynamic_end_{n}
	self.Emit("MOVD", _I_json_MismatchTypeError, _X1) // MOVD _I_json_MismatchTypeError, X1
	self.Emit("CMP", _ET, _X1)                      // CMP ET, X1
	self.Sjmp("BNE", _LB_error)                     // BNE  LB_error
	self.Emit("MOVD", _ET, _VAR_et)                  // MOVD ET, VAR_et
	self.WriteRecNotAX(14, _EP, jit.Ptr(_ST, _EpOffset), false, false) // MOVD EP, stack.Ep
	self.Link("_decode_dynamic_end_{n}")
}

// Continue implementing the remaining operation code handlers...
// This file is getting large, so I'll continue with the remaining
// opcode implementations in the next part.

// Placeholder for the remaining opcode implementations
func (self *_Assembler) _asm_OP_any(_ *_Instr) {
	// Implementation for OP_any
}

func (self *_Assembler) _asm_OP_dyn(_ *_Instr) {
	// Implementation for OP_dyn
}

// ... More opcode implementations will follow

// Helper functions for ARM64 instruction generation
var (
	argPtrs = []obj.Addr{_ARG_sp, _ARG_sl, _ARG_ic, _ARG_vp, _ARG_sb, _ARG_fv}
	localPtrs = []obj.Addr{_VAR_st, _VAR_st_Vt, _VAR_st_Dv, _VAR_st_Iv}
)

func ptodec(code jit.Code) _Decoder {
	// Convert JIT code to decoder interface
	return _Decoder{
		encode: func(s string, ic int, vp unsafe.Pointer, sb *_Stack, fv uint64, sv string) (int, error) {
			// Implementation would call the generated code
			return 0, nil
		},
	}
}

type _Decoder struct {
	encode func(s string, ic int, vp unsafe.Pointer, sb *_Stack, fv uint64, sv string) (int, error)
}

// Placeholder for debug instruction
func (self *_Assembler) debug_instr(i int, v *_Instr) {
	// Implementation for debugging instructions
}