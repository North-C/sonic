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
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/bytedance/sonic/internal/caching"
	"github.com/bytedance/sonic/internal/rt"
	"github.com/bytedance/sonic/internal/native/types"
	"github.com/bytedance/sonic/internal/jit"
)

// Additional helper functions
var (
	_F_memequal         = jit.Func(rt.MemEqual)
	_F_memmove          = jit.Func(rt.Memmove)
	_F_growslice        = jit.Func(rt.GrowSlice)
	_F_makeslice        = jit.Func(rt.MakeSliceStd)
	_F_makemap_small    = jit.Func(rt.MakemapSmall)
	_F_mapassign_fast64 = jit.Func(rt.Mapassign_fast64)
	_F_lspace           = jit.Func(jit.Func(native.S_lspace))
	_F_strhash          = jit.Func(jit.Func(caching.S_strhash))
	_F_b64decode        = jit.Func(jit.Func(rt.SubrB64Decode))
	_F_decodeValue      = jit.Func(jit.Func(_subr_decode_value))
	_F_FieldMap_GetCaseInsensitive = jit.Func((*caching.FieldMap).GetCaseInsensitive)
	_ByteSlice = []byte{}
	_Zero_Base = int64(uintptr(((*rt.GoSlice)(unsafe.Pointer(&_ByteSlice))).Ptr))
)

var (
	_F_convT64 = jit.Func(rt.ConvT64)
	_F_error_wrap = jit.Func(error_wrap)
	_F_error_type = jit.Func(error_type)
	_F_error_field = jit.Func(error_field)
	_F_error_value = jit.Func(error_value)
	_F_error_mismatch = jit.Func(error_mismatch)
)

var (
	_F_println = jit.Func(fmt.Println)
)

// Opcode implementations

func (self *_Assembler) _asm_OP_any(_ *_Instr) {
	self.Emit("MOVD", jit.Ptr(_VP, 8), _X1)            // MOVD    8(VP), X1
	self.Emit("CMP", _X1, _ZR)                         // CMP    X1, ZR
	self.Sjmp("BEQ", "_decode_{n}")                    // BEQ     _decode_{n}
	self.Emit("CMP", _X1, _VP)                         // CMP    X1, VP
	self.Sjmp("BEQ", "_decode_{n}")                    // BEQ     _decode_{n}
	self.Emit("MOVD", jit.Ptr(_VP, 0), _X0)            // MOVD    (VP), X0
	self.Emit("MOVBU", jit.Ptr(_X0, _Gt_KindFlags), _X2) // MOVBU _Gt_KindFlags(X0), X2
	self.Emit("AND", _X2, _X2, jit.Imm(rt.F_kind_mask)) // AND     X2, ${F_kind_mask}, X2
	self.Emit("CMP", _X2, jit.Imm(_Vk_Ptr))            // CMP     X2, ${reflect.Ptr}
	self.Sjmp("BNE", "_decode_{n}")                    // BNE     _decode_{n}
	self.Emit("ADD", _X3, _VP, jit.Imm(8))             // ADD     X3, VP, #8
	self.decode_dynamic(_X0, _X3)                       // DECODE  X0, X3
	self.Sjmp("B", "_decode_end_{n}")                  // B       _decode_end_{n}
	self.Link("_decode_{n}")                           // _decode_{n}:
	self.Emit("MOVD", _ARG_fv, _X4)                    // MOVD    fv, X4
	self.Emit("MOVD", _ST, jit.Ptr(_SP, 0))            // MOVD    _ST, (SP)
	self.call(_F_decodeValue)                          // CALL    decodeValue
	self.Emit("MOVD", _ZR, jit.Ptr(_SP, 0))            // MOVD    _ST, (SP)
	self.Emit("CMP", _EP, _ZR)                         // CMP     EP, ZR
	self.Sjmp("BNE", _LB_parsing_error)               // BNE     _parsing_error
	self.Link("_decode_end_{n}")                       // _decode_end_{n}:
}

func (self *_Assembler) _asm_OP_dyn(p *_Instr) {
	self.Emit("MOVD", jit.Type(p.vt()), _ET)          // MOVD    ${p.vt()}, ET
	self.Emit("CMP", jit.Ptr(_VP, 8), _ZR)            // CMP    8(VP), ZR
	self.Sjmp("BNE", "_decode_dyn_non_nil_{n}")       // BNE     _decode_{n}

	/* if nil iface, call skip one */
	self.Emit("MOVD", _IC, _VAR_ic)
	self.Emit("MOVD", _ET, _VAR_et)
	self.Byte(0x50, 0x00, 0x00, 0x58)
	self.Sref("_decode_end_{n}", 4)
	self.Emit("MOVD", _X16, _VAR_pc)
	self.Sjmp("B", _LB_skip_one)

	self.Link("_decode_dyn_non_nil_{n}")
	self.Emit("MOVD", jit.Ptr(_VP, 0), _X1)            // MOVD    (VP), X1
	self.Emit("MOVD", jit.Ptr(_X1, 8), _X1)            // MOVD    8(X1), X1
	self.Emit("MOVBU", jit.Ptr(_X1, _Gt_KindFlags), _X2) // MOVBU _Gt_KindFlags(X1), X2
	self.Emit("AND", _X2, _X2, jit.Imm(rt.F_kind_mask)) // AND     X2, ${F_kind_mask}, X2
	self.Emit("CMP", _X2, jit.Imm(_Vk_Ptr))            // CMP     X2, ${reflect.Ptr}
	self.Sjmp("BEQ", "_decode_dyn_ptr_{n}")           // BEQ     _decode_dyn_ptr_{n}

	self.Emit("MOVD", _IC, _VAR_ic)
	self.Emit("MOVD", _ET, _VAR_et)
	self.Byte(0x50, 0x00, 0x00, 0x58)
	self.Sref("_decode_end_{n}", 4)
	self.Emit("MOVD", _X16, _VAR_pc)
	self.Sjmp("B", _LB_skip_one)

	self.Link("_decode_dyn_ptr_{n}")
	self.Emit("ADD", _X3, _VP, jit.Imm(8))             // ADD     X3, VP, #8
	self.decode_dynamic(_X1, _X3)                       // DECODE X1, X3
	self.Link("_decode_end_{n}")
}

func (self *_Assembler) _asm_OP_unsupported(p *_Instr) {
	self.Emit("MOVD", jit.Type(p.vt()), _ET)          // MOVD    ${p.vt()}, ET
	self.Sjmp("B", _LB_type_error)                    // B       _LB_type_error
}

func (self *_Assembler) _asm_OP_str(_ *_Instr) {
	self.parse_string()                                 // PARSE   STRING
	self.unquote_once(jit.Ptr(_VP, 0), jit.Ptr(_VP, 8), false, true) // UNQUOTE once, (VP), 8(VP)
}

func (self *_Assembler) _asm_OP_bin(_ *_Instr) {
	self.parse_string()                                 // PARSE  STRING
	self.slice_from(_VAR_st_Iv, -1)                    // SLICE  st.Iv, #-1
	self.Emit("MOVD", _X0, jit.Ptr(_VP, 0))           // MOVD   X0, (VP)
	self.Emit("MOVD", _X1, jit.Ptr(_VP, 8))           // MOVD   X1, 8(VP)
	self.Emit("UBFX", _X1, _X1, jit.Imm(2), jit.Imm(30)) // UBFX X1, X1, #2, #30
	self.Emit("ADD", _X2, _X1, _X1, jit.Imm(1))        // ADD X2, X1, X1, LSL #1
	self.Emit("MOVD", _X2, jit.Ptr(_VP, 16))          // MOVD   X2, 16(VP)
	self.malloc_X0(_X2, _X2)                          // MALLOC X2, X2

	// TODO: due to base64x's bug, only use AVX mode now
	self.Emit("MOVW", jit.Imm(_MODE_JSON), _X2)        // MOVW $_MODE_JSON, X2

	/* call the decoder */
	self.Emit("MOVD", _ZR, _X3)                       // MOVD ZR, X3
	self.Emit("MOVD", _VP, _X0)                       // MOVD  VP, X0

	self.Emit("MOVD", jit.Ptr(_VP, 0), _X4)           // MOVD X4, (VP)
	self.WriteRecNotAX(4, _X2, jit.Ptr(_VP, 0), true, false) // XCHGQ X2, (VP)
	self.Emit("MOVD", _X4, _X2)

	self.Emit("MOVD", _X3, jit.Ptr(_VP, 8))           // XCHGQ X3, 8(VP)
	self.call_c(_F_b64decode)                          // CALL  b64decode
	self.Emit("CMP", _X0, _ZR)                        // CMP X0, ZR
	self.Sjmp("BMI", _LB_base64_error)               // BMI    _base64_error
	self.Emit("MOVD", _X0, jit.Ptr(_VP, 8))          // MOVD  X0, 8(VP)
}

func (self *_Assembler) _asm_OP_bool(_ *_Instr) {
	self.Emit("ADD", _X0, _IC, jit.Imm(4))             // ADD X0, IC, #4
	self.Emit("CMP", _X0, _IL)                         // CMP X0, IL
	self.Sjmp("BHI", _LB_eof_error)                   // BHI   _eof_error
	self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 0), _X1) // MOVBU (IP)(IC), X1
	self.Emit("CMP", _X1, jit.Imm('f'))               // CMP X1, #'f'
	self.Sjmp("BEQ", "_false_{n}")                    // BEQ   _false_{n}
	self.Emit("MOVW", jit.Imm(_IM_true), _X1)         // MOVW $"true", X1
	self.Emit("CMPW", _X1, jit.Sib(_IP, _IC, 1, 0))  // CMPW X1, (IP)(IC)
	self.Sjmp("BEQ", "_bool_true_{n}")
	// try to skip the value
	self.Emit("MOVD", _IC, _VAR_ic)
	self.Emit("MOVD", _T_bool, _ET)
	self.Emit("MOVD", _ET, _VAR_et)
	self.Byte(0x50, 0x00, 0x00, 0x58)
	self.Sref("_end_{n}", 4)
	self.Emit("MOVD", _X16, _VAR_pc)
	self.Sjmp("B", _LB_skip_one)

	self.Link("_bool_true_{n}")
	self.Emit("MOVD", _X0, _IC)                        // MOVD X0, IC
	self.Emit("MOVB", jit.Imm(1), jit.Ptr(_VP, 0))  // MOVB #1, (VP)
	self.Sjmp("B", "_end_{n}")                        // B  _end_{n}
	self.Link("_false_{n}")                            // _false_{n}:
	self.Emit("ADD", _X0, _X0, jit.Imm(1))            // ADD X0, X0, #1
	self.Emit("MOVD", _X0, _IC)                        // MOVD X0, IC
	self.Emit("CMP", _X0, _IL)                         // CMP X0, IL
	self.Sjmp("BHI", _LB_eof_error)                   // BHI   _eof_error
	self.Emit("MOVW", jit.Imm(_IM_alse), _X1)         // MOVW $"alse", X1
	self.Emit("CMPW", _X1, jit.Sib(_IP, _IC, 1, 0))  // CMPW X1, (IP)(IC)
	self.Sjmp("BNE", _LB_im_error)                    // BNE  _im_error
	self.Emit("MOVD", _X0, _IC)                        // MOVD X0, IC
	self.Emit("MOVD", _ZR, _X0)                        // MOVD ZR, X0
	self.Emit("MOVB", _X0, jit.Ptr(_VP, 0))           // MOVB X0, (VP)
	self.Link("_end_{n}")                              // _end_{n}:
}

func (self *_Assembler) _asm_OP_num(_ *_Instr) {
	self.Emit("MOVD", _ZR, _VAR_fl)
	self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 0), _X1) // MOVBU (IP)(IC), X1
	self.Emit("CMP", _X1, jit.Imm('"'))
	self.Emit("MOVD", _IC, _X2)
	self.Sjmp("BNE", "_skip_number_{n}")
	self.Emit("MOVD", jit.Imm(1), _VAR_fl)
	self.Emit("ADD", _IC, _IC, jit.Imm(1))
	self.Link("_skip_number_{n}")

	/* call skip_number */
	self.Emit("MOVD", _ARG_sp, _X0)                    // MOVD  s, X0
	self.Emit("MOVD", _IC, _ARG_ic)                    // MOVD  IC, ic
	self.Emit("MOVD", _ARG_ic, _X1)                    // MOVD  ic, X1
	self.callc(_F_skip_number)                         // CALL  _F_skip_number
	self.Emit("MOVD", _ARG_ic, _IC)                    // MOVD  ic, IC
	self.Emit("CMP", _X0, _ZR)                         // CMP X0, ZR
	self.Sjmp("BPL", "_num_next_{n}")

	/* call skip one */
	self.Emit("MOVD", _X2, _VAR_ic)
	self.Emit("MOVD", _T_number, _ET)
	self.Emit("MOVD", _ET, _VAR_et)
	self.Byte(0x50, 0x00, 0x00, 0x58)
	self.Sref("_num_end_{n}", 4)
	self.Emit("MOVD", _X16, _VAR_pc)
	self.Sjmp("B", _LB_skip_one)

	/* assign string */
	self.Link("_num_next_{n}")
	self.slice_from_r(_X0, 0)
	self.Emit("TST", jit.Imm(_F_copy_string), _ARG_fv)
	self.Sjmp("BCC", "_num_write_{n}")
	self.Byte(0x50, 0x00, 0x00, 0x58)
	self.Sref("_num_write_{n}", 4)
	self.Sjmp("B", "_copy_string")
	self.Link("_num_write_{n}")
	self.Emit("MOVD", _X1, jit.Ptr(_VP, 8))            // MOVD  X1, 8(VP)
	self.WriteRecNotAX(13, _X0, jit.Ptr(_VP, 0), false, false)
	self.Emit("CMP", _VAR_fl, jit.Imm(1))
	self.Sjmp("BNE", "_num_end_{n}")
	self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 0), _X1)  // MOVBU (IP)(IC), X1
	self.Emit("CMP", _X1, jit.Imm('"'))
	self.Sjmp("BNE", _LB_char_0_error)
	self.Emit("ADD", _IC, _IC, jit.Imm(1))
	self.Link("_num_end_{n}")
}

func (self *_Assembler) _asm_OP_i8(_ *_Instr) {
	var pin = "_i8_end_{n}"
	self.parse_signed(int8Type, pin, -1)             // PARSE int8
	self.range_signed_X1(_I_int8, _T_int8, math.MinInt8, math.MaxInt8) // RANGE int8
	self.Emit("MOVB", _X1, jit.Ptr(_VP, 0))           // MOVB  X1, (VP)
	self.Link(pin)
}

func (self *_Assembler) _asm_OP_i16(_ *_Instr) {
	var pin = "_i16_end_{n}"
	self.parse_signed(int16Type, pin, -1)            // PARSE int16
	self.range_signed_X1(_I_int16, _T_int16, math.MinInt16, math.MaxInt16) // RANGE int16
	self.Emit("MOVH", _X1, jit.Ptr(_VP, 0))           // MOVH  X1, (VP)
	self.Link(pin)
}

func (self *_Assembler) _asm_OP_i32(_ *_Instr) {
	var pin = "_i32_end_{n}"
	self.parse_signed(int32Type, pin, -1)            // PARSE int32
	self.range_signed_X1(_I_int32, _T_int32, math.MinInt32, math.MaxInt32) // RANGE int32
	self.Emit("MOVW", _X1, jit.Ptr(_VP, 0))           // MOVW  X1, (VP)
	self.Link(pin)
}

func (self *_Assembler) _asm_OP_i64(_ *_Instr) {
	var pin = "_i64_end_{n}"
	self.parse_signed(int64Type, pin, -1)            // PARSE int64
	self.Emit("MOVD", _VAR_st_Iv, _X0)              // MOVD  st.Iv, X0
	self.Emit("MOVD", _X0, jit.Ptr(_VP, 0))          // MOVD  X0, (VP)
	self.Link(pin)
}

func (self *_Assembler) _asm_OP_u8(_ *_Instr) {
	var pin = "_u8_end_{n}"
	self.parse_unsigned(uint8Type, pin, -1)           // PARSE uint8
	self.range_unsigned_X1(_I_uint8, _T_uint8, math.MaxUint8) // RANGE uint8
	self.Emit("MOVB", _X1, jit.Ptr(_VP, 0))           // MOVB  X1, (VP)
	self.Link(pin)
}

func (self *_Assembler) _asm_OP_u16(_ *_Instr) {
	var pin = "_u16_end_{n}"
	self.parse_unsigned(uint16Type, pin, -1)          // PARSE uint16
	self.range_unsigned_X1(_I_uint16, _T_uint16, math.MaxUint16) // RANGE uint16
	self.Emit("MOVH", _X1, jit.Ptr(_VP, 0))           // MOVH  X1, (VP)
	self.Link(pin)
}

func (self *_Assembler) _asm_OP_u32(_ *_Instr) {
	var pin = "_u32_end_{n}"
	self.parse_unsigned(uint32Type, pin, -1)          // PARSE uint32
	self.range_uint32_X1(_I_uint32, _T_uint32)       // RANGE uint32
	self.Emit("MOVW", _X1, jit.Ptr(_VP, 0))           // MOVW  X1, (VP)
	self.Link(pin)
}

func (self *_Assembler) _asm_OP_u64(_ *_Instr) {
	var pin = "_u64_end_{n}"
	self.parse_unsigned(uint64Type, pin, -1)          // PARSE uint64
	self.Emit("MOVD", _VAR_st_Iv, _X0)              // MOVD  st.Iv, X0
	self.Emit("MOVD", _X0, jit.Ptr(_VP, 0))          // MOVD  X0, (VP)
	self.Link(pin)
}

func (self *_Assembler) _asm_OP_f32(_ *_Instr) {
	var pin = "_f32_end_{n}"
	self.parse_number(float32Type, pin, -1)          // PARSE NUMBER
	self.range_single_D0()                           // RANGE float32
	self.Emit("FMOVS", _S0, jit.Ptr(_VP, 0))         // FMOVS S0, (VP)
	self.Link(pin)
}

func (self *_Assembler) _asm_OP_f64(_ *_Instr) {
	var pin = "_f64_end_{n}"
	self.parse_number(float64Type, pin, -1)          // PARSE NUMBER
	self.Emit("FMOVD", _VAR_st_Dv, _D0)             // FMOVD st.Dv, D0
	self.Emit("FMOVD", _D0, jit.Ptr(_VP, 0))         // FMOVD D0, (VP)
	self.Link(pin)
}

func (self *_Assembler) _asm_OP_unquote(_ *_Instr) {
	self.check_eof(2)
	self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 0), _X1) // MOVBU (IP)(IC), X1
	self.Emit("CMP", _X1, jit.Imm('\\'))              // CMP X1, #'\\'
	self.Sjmp("BNE", _LB_char_0_error)               // BNE     _char_0_error
	self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 1), _X1) // MOVBU 1(IP)(IC), X1
	self.Emit("CMP", _X1, jit.Imm('"'))               // CMP X1, #'"'
	self.Sjmp("BNE", _LB_char_1_error)               // BNE     _char_1_error
	self.Emit("ADD", _IC, _IC, jit.Imm(2))           // ADD    IC, IC, #2
	self.parse_string()                               // PARSE   STRING
	self.unquote_twice(jit.Ptr(_VP, 0), jit.Ptr(_VP, 8), false) // UNQUOTE twice, (VP), 8(VP)
}

func (self *_Assembler) _asm_OP_nil_1(_ *_Instr) {
	self.Emit("MOVD", _ZR, jit.Ptr(_VP, 0))          // MOVD ZR, (VP)
}

func (self *_Assembler) _asm_OP_nil_2(_ *_Instr) {
	self.Emit("MOVD", _ZR, jit.Ptr(_VP, 0))          // MOVD ZR, (VP)
	self.Emit("MOVD", _ZR, jit.Ptr(_VP, 8))          // MOVD ZR, 8(VP)
}

func (self *_Assembler) _asm_OP_nil_3(_ *_Instr) {
	self.Emit("MOVD", _ZR, jit.Ptr(_VP, 0))          // MOVD ZR, (VP)
	self.Emit("MOVD", _ZR, jit.Ptr(_VP, 8))          // MOVD ZR, 8(VP)
	self.Emit("MOVD", _ZR, jit.Ptr(_VP, 16))         // MOVD ZR, 16(VP)
}

func (self *_Assembler) _asm_OP_empty_bytes(_ *_Instr) {
	self.Emit("MOVD", jit.Imm(_Zero_Base), _X0)
	self.Emit("MOVD", _X0, jit.Ptr(_VP, 0))
	self.Emit("MOVD", _ZR, jit.Ptr(_VP, 8))
	self.Emit("MOVD", _ZR, jit.Ptr(_VP, 16))
}

func (self *_Assembler) _asm_OP_deref(p *_Instr) {
	self.vfollow(p.vt())
}

func (self *_Assembler) _asm_OP_index(p *_Instr) {
	self.Emit("MOVD", jit.Imm(p.i64()), _X0)        // MOVD ${p.vi()}, X0
	self.Emit("ADD", _VP, _VP, _X0)                 // ADD VP, VP, X0
}

func (self *_Assembler) _asm_OP_is_null(p *_Instr) {
	self.Emit("ADD", _X0, _IC, jit.Imm(4))           // ADD X0, IC, #4
	self.Emit("CMP", _X0, _IL)                       // CMP X0, IL
	self.Sjmp("BHI", "_not_null_{n}")                // BHI     _not_null_{n}
	self.Emit("MOVWU", jit.Sib(_IP, _IC, 1, 0), _X1) // MOVWU (IP)(IC), X1
	self.Emit("CMPW", _X1, jit.Imm(_IM_null))        // CMPW X1, $"null"
	self.Emit("CSEL", _IC, _X0, _IC, jit.Imm(4))     // CSEL IC, X0, IC, EQ
	self.Xjmp("BEQ", p.vi())                         // BEQ      {p.vi()}
	self.Link("_not_null_{n}")                       // _not_null_{n}:
}

func (self *_Assembler) _asm_OP_is_null_quote(p *_Instr) {
	self.Emit("ADD", _X0, _IC, jit.Imm(5))           // ADD X0, IC, #5
	self.Emit("CMP", _X0, _IL)                       // CMP X0, IL
	self.Sjmp("BHI", "_not_null_quote_{n}")          // BHI     _not_null_quote_{n}
	self.Emit("MOVWU", jit.Sib(_IP, _IC, 1, 0), _X1) // MOVWU (IP)(IC), X1
	self.Emit("CMPW", _X1, jit.Imm(_IM_null))        // CMPW X1, $"null"
	self.Sjmp("BNE", "_not_null_quote_{n}")          // BNE     _not_null_quote_{n}
	self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 4), _X1) // MOVBU 4(IP)(IC), X1
	self.Emit("CMP", _X1, jit.Imm('"'))             // CMP X1, #'"'
	self.Emit("CSEL", _IC, _X0, _IC, jit.Imm(4))     // CSEL IC, X0, IC, EQ
	self.Xjmp("BEQ", p.vi())                         // BEQ      {p.vi()}
	self.Link("_not_null_quote_{n}")                // _not_null_quote_{n}:
}

func (self *_Assembler) _asm_OP_map_init(_ *_Instr) {
	self.Emit("MOVD", jit.Ptr(_VP, 0), _X0)         // MOVD    (VP), X0
	self.Emit("CMP", _X0, _ZR)                      // CMP     X0, ZR
	self.Sjmp("BNE", "_end_{n}")                    // BNE     _end_{n}
	self.call_go(_F_makemap_small)                    // CALL_GO makemap_small
	self.WritePtrAX(6, jit.Ptr(_VP, 0), false)       // MOVQ    X0, (VP)
	self.Link("_end_{n}")                            // _end_{n}:
	self.Emit("MOVD", _X0, _VP)                      // MOVD    X0, VP
}

func (self *_Assembler) _asm_OP_map_key_i8(p *_Instr) {
	self.parse_signed(int8Type, "", p.vi())         // PARSE     int8
	self.range_signed_X1(_I_int8, _T_int8, math.MinInt8, math.MaxInt8) // RANGE     int8
	self.match_char('"')
	self.mapassign_std(p.vt(), _VAR_st_Iv)           // MAPASSIGN int8, mapassign, st.Iv
}

func (self *_Assembler) _asm_OP_map_key_i16(p *_Instr) {
	self.parse_signed(int16Type, "", p.vi())        // PARSE     int16
	self.range_signed_X1(_I_int16, _T_int16, math.MinInt16, math.MaxInt16) // RANGE     int16
	self.match_char('"')
	self.mapassign_std(p.vt(), _VAR_st_Iv)           // MAPASSIGN int16, mapassign, st.Iv
}

func (self *_Assembler) _asm_OP_map_key_i32(p *_Instr) {
	self.parse_signed(int32Type, "", p.vi())        // PARSE     int32
	self.range_signed_X1(_I_int32, _T_int32, math.MinInt32, math.MaxInt32) // RANGE     int32
	self.match_char('"')
	if vt := p.vt(); !rt.IsMapfast(vt) {
		self.mapassign_std(vt, _VAR_st_Iv)            // MAPASSIGN int32, mapassign, st.Iv
	} else {
		self.Emit("MOVD", _X1, _X0)                  // MOVD X1, X0
		self.mapassign_fastx(vt, _F_mapassign_fast32) // MAPASSIGN int32, mapassign_fast32
	}
}

func (self *_Assembler) _asm_OP_map_key_i64(p *_Instr) {
	self.parse_signed(int64Type, "", p.vi())        // PARSE     int64
	self.match_char('"')
	if vt := p.vt(); !rt.IsMapfast(vt) {
		self.mapassign_std(vt, _VAR_st_Iv)            // MAPASSIGN int64, mapassign, st.Iv
	} else {
		self.Emit("MOVD", _VAR_st_Iv, _X0)           // MOVD      st.Iv, X0
		self.mapassign_fastx(vt, _F_mapassign_fast64) // MAPASSIGN int64, mapassign_fast64
	}
}

func (self *_Assembler) _asm_OP_map_key_u8(p *_Instr) {
	self.parse_unsigned(uint8Type, "", p.vi())       // PARSE     uint8
	self.range_unsigned_X1(_I_uint8, _T_uint8, math.MaxUint8) // RANGE     uint8
	self.match_char('"')
	self.mapassign_std(p.vt(), _VAR_st_Iv)           // MAPASSIGN uint8, mapassign, st.Iv
}

func (self *_Assembler) _asm_OP_map_key_u16(p *_Instr) {
	self.parse_unsigned(uint16Type, "", p.vi())      // PARSE     uint16
	self.range_unsigned_X1(_I_uint16, _T_uint16, math.MaxUint16) // RANGE     uint16
	self.match_char('"')
	self.mapassign_std(p.vt(), _VAR_st_Iv)           // MAPASSIGN uint16, mapassign, st.Iv
}

func (self *_Assembler) _asm_OP_map_key_u32(p *_Instr) {
	self.parse_unsigned(uint32Type, "", p.vi())      // PARSE     uint32
	self.range_unsigned_X1(_I_uint32, _T_uint32, math.MaxUint32) // RANGE     uint32
	self.match_char('"')
	if vt := p.vt(); !rt.IsMapfast(vt) {
		self.mapassign_std(vt, _VAR_st_Iv)            // MAPASSIGN uint32, mapassign, st.Iv
	} else {
		self.Emit("MOVD", _X1, _X0)                  // MOVD X1, X0
		self.mapassign_fastx(vt, _F_mapassign_fast32) // MAPASSIGN uint32, mapassign_fast32
	}
}

func (self *_Assembler) _asm_OP_map_key_u64(p *_Instr) {
	self.parse_unsigned(uint64Type, "", p.vi())      // PARSE     uint64
	self.match_char('"')
	if vt := p.vt(); !rt.IsMapfast(vt) {
		self.mapassign_std(vt, _VAR_st_Iv)            // MAPASSIGN uint64, mapassign, st.Iv
	} else {
		self.Emit("MOVD", _VAR_st_Iv, _X0)           // MOVD      st.Iv, X0
		self.mapassign_fastx(vt, _F_mapassign_fast64) // MAPASSIGN uint64, mapassign_fast64
	}
}

func (self *_Assembler) _asm_OP_map_key_f32(p *_Instr) {
	self.parse_number(float32Type, "", p.vi())      // PARSE     NUMBER
	self.range_single_D0()                           // RANGE     float32
	self.Emit("FMOVS", _S0, _VAR_st_Dv)            // FMOVS     S0, st.Dv
	self.match_char('"')
	self.mapassign_std(p.vt(), _VAR_st_Dv)           // MAPASSIGN ${p.vt()}, mapassign, st.Dv
}

func (self *_Assembler) _asm_OP_map_key_f64(p *_Instr) {
	self.parse_number(float64Type, "", p.vi())      // PARSE     NUMBER
	self.match_char('"')
	self.mapassign_std(p.vt(), _VAR_st_Dv)           // MAPASSIGN ${p.vt()}, mapassign, st.Dv
}

func (self *_Assembler) _asm_OP_map_key_str(p *_Instr) {
	self.parse_string()                              // PARSE     STRING
	self.unquote_once(_ARG_sv_p, _ARG_sv_n, true, true) // UNQUOTE   once, sv.p, sv.n
	if vt := p.vt(); !rt.IsMapfast(vt) {
		self.valloc(vt.Key(), _X1)
		self.Emit("LDP", _X2, _X3, jit.Ptr(_ARG_sv, 0))
		self.Emit("STP", _X2, _X3, jit.Ptr(_X1, 0))
		self.mapassign_std(vt, jit.Ptr(_X1, 0))     // MAPASSIGN string, X1, X2
	} else {
		self.mapassign_str_fast(vt, _ARG_sv_p, _ARG_sv_n) // MAPASSIGN string, X0, X1
	}
}

func (self *_Assembler) _asm_OP_map_key_utext(p *_Instr) {
	self.parse_string()                               // PARSE     STRING
	self.unquote_once(_ARG_sv_p, _ARG_sv_n, true, true) // UNQUOTE   once, sv.p, sv.n
	self.mapassign_utext(p.vt(), false)              // MAPASSIGN utext, ${p.vt()}, false
}

func (self *_Assembler) _asm_OP_map_key_utext_p(p *_Instr) {
	self.parse_string()                               // PARSE     STRING
	self.unquote_once(_ARG_sv_p, _ARG_sv_n, true, true) // UNQUOTE   once, sv.p, sv.n
	self.mapassign_utext(p.vt(), true)               // MAPASSIGN utext, ${p.vt()}, true
}

func (self *_Assembler) _asm_OP_array_skip(_ *_Instr) {
	self.call_sf(_F_skip_array)                       // CALL_SF skip_array
	self.Emit("CMP", _X0, _ZR)                        // CMP    X0, ZR
	self.Sjmp("BMI", _LB_parsing_error_v)             // BMI     _parse_error_v
}

func (self *_Assembler) _asm_OP_array_clear(_ *_Instr) {
	self.mem_clear_rem(p.i64(), true)
}

func (self *_Assembler) _asm_OP_array_clear_p(_ *_Instr) {
	self.mem_clear_rem(p.i64(), false)
}

func (self *_Assembler) _asm_OP_slice_init(p *_Instr) {
	self.Emit("MOVD", _ZR, _X0)                      // MOVD ZR, X0
	self.Emit("MOVD", _X0, jit.Ptr(_VP, 8))           // MOVD    X0, 8(VP)
	self.Emit("MOVD", jit.Ptr(_VP, 16), _X1)         // MOVD    16(VP), X1
	self.Emit("CMP", _X1, _ZR)                      // CMP     X1, ZR
	self.Sjmp("BNE", "_done_{n}")                    // BNE     _done_{n}
	self.Emit("MOVD", jit.Imm(_MinSlice), _X2)      // MOVD    ${_MinSlice}, X2
	self.Emit("MOVD", _X2, jit.Ptr(_VP, 16))         // MOVD    X2, 16(VP)
	self.Emit("MOVD", jit.Type(p.vt()), _X0)        // MOVD    ${p.vt()}, X0
	self.call_go(_F_makeslice)                       // CALL_GO makeslice
	self.WritePtrAX(7, jit.Ptr(_VP, 0), false)      // MOVD    X0, (VP)
	self.Emit("MOVD", _ZR, _X0)                      // MOVD    ZR, X0
	self.Emit("MOVD", _X0, jit.Ptr(_VP, 8))           // MOVD    X0, 8(VP)
	self.Link("_done_{n}")                            // _done_{n}
}

func (self *_Assembler) _asm_OP_check_empty(p *_Instr) {
	rbracket := p.vb()
	if rbracket == ']' {
		self.check_eof(1)
		self.Emit("ADD", _X0, _IC, jit.Imm(1))        // ADD X0, IC, #1
		self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 0), _X1) // MOVBU (IP)(IC), X1
		self.Emit("CMP", _X1, jit.Imm(int64(rbracket))) // CMP X1, ${rbracket}
		self.Sjmp("BNE", "_not_empty_array_{n}")     // BNE     _not_empty_array_{n}
		self.Emit("MOVD", _X0, _IC)                    // MOVD X0, IC
		self.Emit("MOVD", jit.Imm(_Zero_Base), _X0)
		self.WritePtrAX(9, jit.Ptr(_VP, 0), false)
		self.Emit("MOVD", _ZR, jit.Ptr(_VP, 8))         // MOVD ZR, 8(VP)
		self.Emit("MOVD", _ZR, jit.Ptr(_VP, 16))        // MOVD ZR, 16(VP)
		self.Xjmp("B", p.vi())                        // B     {p.vi()}
		self.Link("_not_empty_array_{n}")
	} else {
		panic("only implement check empty array here!")
	}
}

func (self *_Assembler) _asm_OP_slice_append(p *_Instr) {
	self.Emit("MOVD", jit.Ptr(_VP, 8), _X0)          // MOVD    8(VP), X0
	self.Emit("CMP", _X0, jit.Ptr(_VP, 16))          // CMP    X0, 16(VP)
	self.Sjmp("BLO", "_index_{n}")                   // BLO     _index_{n}
	self.Emit("MOVD", _X0, _X1)                      // MOVD    X0, X1
	self.Emit("LSL", _X1, _X1, jit.Imm(1))            // LSL X1, X1, #1
	self.Emit("MOVD", jit.Type(p.vt()), _X0)         // MOVD    ${p.vt()}, X0
	self.Emit("MOVD", jit.Ptr(_VP, 0), _X1)          // MOVD   (VP), X1
	self.Emit("MOVD", jit.Ptr(_VP, 8), _X2)          // MOVD    8(VP), X2
	self.Emit("MOVD", jit.Ptr(_VP, 16), _X3)         // MOVD    16(VP), X3
	self.call_go(_F_growslice)                       // CALL_GO growslice
	self.WritePtrAX(8, jit.Ptr(_VP, 0), false)      // MOVD    X0, (VP)
	self.Emit("MOVD", _X1, jit.Ptr(_VP, 8))          // MOVD    X1, 8(VP)
	self.Emit("MOVD", _X2, jit.Ptr(_VP, 16))         // MOVD    X2, 16(VP)

	// because growslice not zero memory {oldcap, newlen} when append et not has ptrdata.
	// but we should zero it, avoid decode it as random values.
	if rt.UnpackType(p.vt()).PtrData == 0 {
		self.Emit("MOVD", _X2, _X3)                  // MOVD    X2, X3
		self.Emit("SUB", _X3, _X1, _X3)              // SUB     X3, X1, X3

		self.Emit("ADD", jit.Ptr(_VP, 8), jit.Ptr(_VP, 8), jit.Imm(1)) // ADD 8(VP), 8(VP), #1
		self.Emit("MOVD", _X0, _VP)                    // MOVD    X0, VP
		self.Emit("MOVD", jit.Imm(int64(p.vlen())), _X2) // MOVD    ${p.vlen()}, X2
		self.Emit("MUL", _X0, _X1, _X2)                // MUL     X0, X1, X2
		self.Emit("ADD", _VP, _VP, _X0)                // ADD     VP, VP, X0

		self.Emit("MOVD", _X3, _X0)                    // MOVD    X2, X0
		self.Emit("MUL", _X0, _X1, _X2)                // MUL     X0, X1, X2
		self.Emit("MOVD", _X0, _X1)                    // ADD     X0, X1, X2
		self.Emit("MOVD", _VP, _X0)                    // MOVD    VP, X0
		self.mem_clear_fn(true)                        // CALL_GO memclr{Has,NoHeap}
		self.Sjmp("B", "_append_slice_end_{n}")
	}

	self.Emit("MOVD", _X1, _X0)                      // MOVD    X1, X0
	self.Link("_index_{n}")                           // _index_{n}:
	self.Emit("ADD", jit.Ptr(_VP, 8), jit.Ptr(_VP, 8), jit.Imm(1)) // ADD 8(VP), 8(VP), #1
	self.Emit("MOVD", jit.Ptr(_VP, 0), _VP)          // MOVD    (VP), VP
	self.Emit("MOVD", jit.Imm(int64(p.vlen())), _X1) // MOVD    ${p.vlen()}, X1
	self.Emit("MUL", _X0, _VP, _X1)                  // MUL     X0, VP, X1
	self.Emit("ADD", _VP, _VP, _X0)                  // ADD     VP, VP, X0
	self.Link("_append_slice_end_{n}")
}

func (self *_Assembler) _asm_OP_object_next(_ *_Instr) {
	self.call_sf(_F_skip_one)                       // CALL_SF skip_one
	self.Emit("CMP", _X0, _ZR)                       // CMP    X0, ZR
	self.Sjmp("BMI", _LB_parsing_error_v)            // BMI     _parse_error_v
}

func (self *_Assembler) _asm_OP_struct_field(p *_Instr) {
	assert_eq(caching.FieldEntrySize, 32, "invalid field entry size")
	self.Emit("MOVD", jit.Imm(-1), _X0)              // MOVD    $-1, X0
	self.Emit("MOVD", _X0, _VAR_sr)                  // MOVD    X0, sr
	self.parse_string()                               // PARSE   STRING
	self.unquote_once(_ARG_sv_p, _ARG_sv_n, true, false) // UNQUOTE once, sv.p, sv.n
	self.Emit("ADD", _X0, _SP, jit.Imm(_FP_fargs + _FP_saves + 104)) // ADD X0, SP, #sv_offset
	self.Emit("MOVD", _ZR, _X1)                      // XORL    X1, X1
	self.call_go(_F_strhash)                         // CALL_GO strhash
	self.Emit("MOVD", _X0, _X16)                     // MOVD    X0, X16
	self.Emit("MOVD", jit.Imm(freezeFields(p.vf())), _X2) // MOVD    ${p.vf()}, X2
	self.Emit("MOVD", jit.Ptr(_X2, caching.FieldMap_b), _X3) // MOVD    FieldMap.b(X2), X3
	self.Emit("MOVD", jit.Ptr(_X2, caching.FieldMap_N), _X2) // MOVD    FieldMap.N(X2), X2
	self.Emit("CMP", _X2, _ZR)                      // CMP     X2, ZR
	self.Sjmp("BEQ", "_try_lowercase_{n}")          // BEQ     _try_lowercase_{n}
	self.Link("_loop_{n}")                            // _loop_{n}:
	self.Emit("UDIV", _X4, _X1, _X2)                 // UDIV X4, X1, X2
	self.Emit("ADD", _X0, _X4, jit.Imm(1))           // ADD     X0, X4, #1
	self.Emit("LSL", _X4, _X4, jit.Imm(5))            // LSL     X4, X4, #5
	self.Emit("ADD", _X5, _X3, _X4)                  // ADD     X5, X3, X4
	self.Emit("MOVD", jit.Ptr(_X5, _Fe_Hash), _X6)   // MOVD    FieldEntry.Hash(X5), X6
	self.Emit("CMP", _X6, _ZR)                      // CMP     X6, ZR
	self.Sjmp("BEQ", "_try_lowercase_{n}")          // BEQ     _try_lowercase_{n}
	self.Emit("CMP", _X6, _X16)                     // CMP     X6, X16
	self.Sjmp("BNE", "_loop_{n}")                   // BNE     _loop_{n}
	self.Emit("MOVD", jit.Ptr(_X5, _Fe_Name + 8), _X4) // MOVD    FieldEntry.Name+8(X5), X4
	self.Emit("CMP", _X4, _ARG_sv_n)                 // CMP     X4, sv.n
	self.Sjmp("BNE", "_loop_{n}")                   // BNE     _loop_{n}
	self.Emit("MOVD", jit.Ptr(_X5, _Fe_ID), _X6)      // MOVD    FieldEntry.ID(X5), X6
	self.Emit("MOVD", _X0, _VAR_ss_X0)               // MOVD    X0, ss.X0
	self.Emit("MOVD", _X2, _VAR_ss_X1)               // MOVD    X2, ss.X1
	self.Emit("MOVD", _X3, _VAR_ss_X2)               // MOVD    X3, ss.X2
	self.Emit("MOVD", _X6, _VAR_ss_X3)               // MOVD    X6, ss.X3
	self.Emit("MOVD", _X16, _VAR_ss_X4)              // MOVD    X16, ss.X4
	self.Emit("MOVD", _ARG_sv_p, _X0)               // MOVD    _ARG_sv_p, X0
	self.Emit("MOVD", jit.Ptr(_X5, _Fe_Name), _X1)   // MOVD    FieldEntry.Name(X5), X1
	self.Emit("MOVD", _X1, _VAR_ss_X1)               // MOVD    X1, 8(SP)
	self.Emit("MOVD", _X4, _VAR_ss_X1)               // MOVD    X4, 16(SP)
	self.call_go(_F_memequal)                       // CALL_GO memequal
	self.Emit("MOVBU", _X0, _VAR_ss_X0)             // MOVB    24(SP), X0
	self.Emit("MOVD", _VAR_ss_X0, _X0)              // MOVD    ss.X0, X0
	self.Emit("MOVD", _VAR_ss_X1, _X1)              // MOVD    ss.X1, X1
	self.Emit("MOVD", _VAR_ss_X2, _X2)              // MOVD    ss.X2, X2
	self.Emit("MOVD", _VAR_ss_X4, _X16)              // MOVD    ss.X4, X16
	self.Emit("CMP", _X0, _ZR)                      // CMP     X0, ZR
	self.Sjmp("BEQ", "_loop_{n}")                   // BEQ     _loop_{n}
	self.Emit("MOVD", _VAR_ss_X3, _X6)               // MOVD    ss.X3, X6
	self.Emit("MOVD", _X6, _VAR_sr)                  // MOVD    X6, sr
	self.Sjmp("B", "_end_{n}")                      // B       _end_{n}
	self.Link("_try_lowercase_{n}")                 // _try_lowercase_{n}:
	self.Emit("TST", jit.Imm(_F_case_sensitive), _ARG_fv) // check if enable option CaseSensitive
	self.Sjmp("BNE", "_unknown_{n}")
	self.Emit("MOVD", jit.Imm(referenceFields(p.vf())), _X0) // MOVD    ${p.vf()}, X0
	self.Emit("MOVD", _ARG_sv_p, _X1)                // MOVD   sv, X1
	self.Emit("MOVD", _ARG_sv_n, _X2)                // MOVD   sv, X2
	self.call_go(_F_FieldMap_GetCaseInsensitive)     // CALL_GO FieldMap::GetCaseInsensitive
	self.Emit("MOVD", _X0, _VAR_sr)                  // MOVD    X0, _VAR_sr
	self.Emit("CMP", _X0, _ZR)                      // CMP     X0, ZR
	self.Sjmp("BPL", "_end_{n}")                    // BNE     _end_{n}
	self.Link("_unknown_{n}")
	// HACK: because `_VAR_sr` maybe used in `F_vstring`, so we should clear here again for `_OP_switch`.
	self.Emit("MOVD", jit.Imm(-1), _X0)              // MOVD    $-1, X0
	self.Emit("MOVD", _X0, _VAR_sr)                  // MOVD    X0, sr
	self.Emit("TST", jit.Imm(_F_disable_unknown), _ARG_fv) // BTQ     ${_F_disable_unknown}, fv
	self.Sjmp("BNE", _LB_field_error)                // BNE     _field_error
	self.Link("_end_{n}")                             // _end_{n}:
}

func (self *_Assembler) _asm_OP_unmarshal(p *_Instr) {
	if iv := p.i64(); iv != 0 {
		self.unmarshal_json(p.vt(), true, _F_decodeJsonUnmarshalerQuoted)
	} else {
		self.unmarshal_json(p.vt(), true, _F_decodeJsonUnmarshaler)
	}
}

func (self *_Assembler) _asm_OP_unmarshal_p(p *_Instr) {
	if iv := p.i64(); iv != 0 {
		self.unmarshal_json(p.vt(), false, _F_decodeJsonUnmarshalerQuoted)
	} else {
		self.unmarshal_json(p.vt(), false, _F_decodeJsonUnmarshaler)
	}
}

func (self *_Assembler) _asm_OP_unmarshal_text(p *_Instr) {
	self.unmarshal_text(p.vt(), true)
}

func (self *_Assembler) _asm_OP_unmarshal_text_p(p *_Instr) {
	self.unmarshal_text(p.vt(), false)
}

func (self *_Assembler) _asm_OP_lspace(_ *_Instr) {
	self.lspace("_{n}")
}

func (self *_Assembler) lspace(subfix string) {
	var label = "_lspace" + subfix
	self.Emit("CMP", _IC, _IL)                      // CMP    IC, IL
	self.Sjmp("BHS", _LB_eof_error)                 // BHS    _eof_error
	self.Emit("MOVD", jit.Imm(_BM_space), _X1)      // MOVD   _BM_space, X1
	self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 0), _X0) // MOVBU (IP)(IC), X0
	self.Emit("CMP", _X0, jit.Imm(' '))              // CMP    X0, #' '
	self.Sjmp("BHI", label)                         // BHI     _nospace_{n}
	self.Emit("TST", _X0, _X1)                      // TST     X0, X1
	self.Sjmp("BCC", label)                          // BCC     _nospace_{n}

	/* test up to 4 characters */
	for i := 0; i < 3; i++ {
		self.Emit("ADD", _IC, _IC, jit.Imm(1))         // ADD     IC, IC, #1
		self.Emit("CMP", _IC, _IL)                    // CMP     IC, IL
		self.Sjmp("BHS", _LB_eof_error)               // BHS     _eof_error
		self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 0), _X0) // MOVBU (IP)(IC), X0
		self.Emit("CMP", _X0, jit.Imm(' '))            // CMP     X0, #' '
		self.Sjmp("BHI", label)                       // BHI     _nospace_{n}
		self.Emit("TST", _X0, _X1)                     // TST     X0, X1
		self.Sjmp("BCC", label)                       // BCC     _nospace_{n}
	}

	/* handle over to the native function */
	self.Emit("MOVD", _IP, _X0)                      // MOVD    IP, X0
	self.Emit("MOVD", _IL, _X1)                      // MOVD    IL, X1
	self.Emit("MOVD", _IC, _X2)                      // MOVD    IC, X2
	self.callc(_F_lspace)                            // CALL    lspace
	self.Emit("CMP", _X0, _ZR)                      // CMP     X0, ZR
	self.Sjmp("BMI", _LB_parsing_error_v)           // BMI     _parsing_error_v
	self.Emit("CMP", _X0, _IL)                      // CMP     X0, IL
	self.Sjmp("BHS", _LB_eof_error)                 // BHS     _eof_error
	self.Emit("MOVD", _X0, _IC)                      // MOVD    X0, IC
	self.Link(label)                                 // _nospace_{n}:
}

func (self *_Assembler) _asm_OP_match_char(p *_Instr) {
	self.match_char(p.vb())
}

func (self *_Assembler) match_char(char byte) {
	self.check_eof(1)
	self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 0), _X0) // MOVBU (IP)(IC), X0
	self.Emit("CMP", _X0, jit.Imm(int64(char)))     // CMP X0, ${p.vb()}
	self.Sjmp("BNE", _LB_char_0_error)               // BNE  _char_0_error
	self.Emit("ADD", _IC, _IC, jit.Imm(1))          // ADD IC, IC, #1
}

func (self *_Assembler) _asm_OP_check_char(p *_Instr) {
	self.check_eof(1)
	self.Emit("ADD", _X0, _IC, jit.Imm(1))           // ADD X0, IC, #1
	self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 0), _X1) // MOVBU (IP)(IC), X1)
	self.Emit("CMP", _X1, jit.Imm(int64(p.vb())))  // CMP    X1, ${p.vb()}
	self.Emit("CSEL", _IC, _X0, _IC, jit.Imm(4))     // CSEL IC, X0, IC, EQ
	self.Xjmp("BEQ", p.vi())                        // BEQ      {p.vi()}
}

func (self *_Assembler) _asm_OP_check_char_0(p *_Instr) {
	self.check_eof(1)
	self.Emit("MOVBU", jit.Sib(_IP, _IC, 1, 0), _X0) // MOVBU (IP)(IC), X0)
	self.Emit("CMP", _X0, jit.Imm(int64(p.vb())))   // CMP    X0, ${p.vb()}
	self.Xjmp("BEQ", p.vi())                        // BEQ      {p.vi()}
}

func (self *_Assembler) _asm_OP_add(p *_Instr) {
	self.Emit("ADD", _IC, _IC, jit.Imm(int64(p.vi()))) // ADD ${p.vi()}, IC
}

func (self *_Assembler) _asm_OP_load(_ *_Instr) {
	self.Emit("MOVD", jit.Ptr(_ST, 0), _X0)          // MOVD (ST), X0
	self.Emit("MOVD", jit.Sib(_ST, _X0, 1, 0), _VP)  // MOVD (ST)(X0), VP
}

func (self *_Assembler) _asm_OP_save(_ *_Instr) {
	self.Emit("MOVD", jit.Ptr(_ST, 0), _X1)          // MOVD (ST), X1
	self.Emit("CMP", _X1, jit.Imm(_MaxStackBytes))   // CMP X1, ${_MaxStackBytes}
	self.Sjmp("BHS", _LB_stack_error)               // BHS   _stack_error
	self.WriteRecNotAX(0, _VP, jit.Sib(_ST, _X1, 1, 8), false, false) // MOVD VP, 8(ST)(X1)
	self.Emit("ADD", _X1, _X1, jit.Imm(8))           // ADD X1, X1, #8
	self.Emit("MOVD", _X1, jit.Ptr(_ST, 0))          // MOVD X1, (ST)
}

func (self *_Assembler) _asm_OP_drop(_ *_Instr) {
	self.Emit("MOVD", jit.Ptr(_ST, 0), _X0)          // MOVD (ST), X0
	self.Emit("SUB", _X0, _X0, jit.Imm(8))           // SUB X0, X0, #8
	self.Emit("MOVD", jit.Sib(_ST, _X0, 1, 8), _VP)  // MOVD 8(ST)(X0), VP
	self.Emit("MOVD", _X0, jit.Ptr(_ST, 0))          // MOVD X0, (ST)
	self.Emit("MOVD", _ZR, jit.Sib(_ST, _X0, 1, 8))  // MOVD ZR, 8(ST)(X0)
}

func (self *_Assembler) _asm_OP_drop_2(_ *_Instr) {
	self.Emit("MOVD", jit.Ptr(_ST, 0), _X0)          // MOVD  (ST), X0
	self.Emit("SUB", _X0, _X0, jit.Imm(16))          // SUB  X0, X0, #16
	self.Emit("MOVD", jit.Sib(_ST, _X0, 1, 8), _VP)  // MOVD  8(ST)(X0), VP
	self.Emit("MOVD", _X0, jit.Ptr(_ST, 0))          // MOVD  X0, (ST)
	self.Emit("MOVD", _ZR, jit.Sib(_ST, _X0, 1, 8))  // MOVD ZR, 8(ST)(X0))
	self.Emit("MOVD", _ZR, jit.Sib(_ST, _X0, 1, 16)) // MOVD ZR, 16(ST)(X0))
}

func (self *_Assembler) _asm_OP_recurse(p *_Instr) {
	self.Emit("MOVD", jit.Type(p.vt()), _X0)         // MOVD   ${p.vt()}, X0
	self.decode_dynamic(_X0, _VP)                    // DECODE X0, VP
}

func (self *_Assembler) _asm_OP_goto(p *_Instr) {
	self.Xjmp("B", p.vi())
}

func (self *_Assembler) _asm_OP_switch(p *_Instr) {
	self.Emit("MOVD", _VAR_sr, _X0)                 // MOVD sr, X0
	self.Emit("CMP", _X0, jit.Imm(p.i64()))          // CMP X0, ${len(p.vs())}
	self.Sjmp("BHS", "_default_{n}")                 // BHS  _default_{n}

	/* jump table selector */
	self.Emit("ADR", _X1, "_switch_table_{n}")     // ADR    X1, ?(PC)
	self.Emit("MOVWU", jit.Sib(_X1, _X0, 2), _X0)  // MOVWU (X1)(X0*2), X0
	self.Emit("ADD", _X0, _X0, _X1)                 // ADD     X0, X0, X1
	self.Rjmp("BR", _X0)                           // BR      X0
	self.Link("_switch_table_{n}")                  // _switch_table_{n}:

	/* generate the jump table */
	for i, v := range p.vs() {
		self.Sref(v, int64(-i) * 4)
	}

	/* default case */
	self.Link("_default_{n}")
	self.Emit("NOP")
}

func (self *_Assembler) _asm_OP_skip_empty(p *_Instr) {
	self.call_sf(_F_skip_one)                       // CALL_SF skip_one
	self.Emit("CMP", _X0, _ZR)                      // CMP    X0, ZR
	self.Sjmp("BMI", _LB_parsing_error_v)           // BMI     _parse_error_v
	self.Emit("TST", jit.Imm(_F_disable_unknown), _ARG_fv)
	self.Xjmp("BCC", p.vi())
	self.Emit("ADD", _X1, _IC, _X0)                 // ADD X1, IC, X0
	self.Emit("MOVD", _X1, _ARG_sv_n)               // MOVD X1, sv.n
	self.Emit("ADD", _X0, _IP, _X0)                 // ADD X0, IP, X0
	self.Emit("MOVD", _X0, _ARG_sv_p)               // MOVD X0, sv.p
	self.Emit("MOVD", jit.Imm(':'), _X2)           // MOVD ':', X2
	self.call_go(_F_IndexByte)
	self.Emit("CMP", _X0, _ZR)                      // CMP X0, ZR
	// disallow unknown field
	self.Sjmp("BPL", _LB_field_error)              // BPL _field_error
}

func (self *_Assembler) _asm_OP_debug(_ *_Instr) {
	self.Emit("BRK", jit.Imm(0))
}

// Additional helper functions
func assert_eq(a, b interface{}, msg string) {
	// Implementation for assertion
}

func freezeFields(vf *caching.FieldMap) int64 {
	// Implementation for freezing fields
	return 0
}

func referenceFields(vf *caching.FieldMap) int64 {
	// Implementation for reference fields
	return 0
}

// Constants needed for ARM64 implementation
const (
	_F_convT64 = 0
	_F_error_wrap = 1
	_F_error_type = 2
	_F_error_field = 3
	_F_error_value = 4
	_F_error_mismatch = 5
	_F_memequal = 6
	_F_memmove = 7
	_F_growslice = 8
	_F_makeslice = 9
	_F_makemap_small = 10
	_F_mapassign_fast64 = 11
	_F_lspace = 12
	_F_strhash = 13
	_F_b64decode = 14
	_F_decodeValue = 15
	_F_FieldMap_GetCaseInsensitive = 16
	_F_println = 17
	_F_unquote = 18
	_F_skip_one = 19
	_F_skip_array = 20
	_F_skip_number = 21
	_F_vstring = 22
	_F_vnumber = 23
	_F_vsigned = 24
	_F_vunsigned = 25
	_F_IndexByte = 26
	_F_mallocgc = 27
	_F_memclrHasPointers = 28
	_F_memclrNoHeapPointers = 29
	_F_mapassign = 30
	_F_mapassign_fast32 = 31
	_F_mapassign_faststr = 32
	_F_mapassign_fast64ptr = 33
	_F_decodeJsonUnmarshaler = 34
	_F_decodeJsonUnmarshalerQuoted = 35
	_F_decodeTextUnmarshaler = 36
	_F_decodeTypedPointer = 37
)

const (
	_Fe_ID = int64(unsafe.Offsetof(caching.FieldEntry{}.ID))
	_Fe_Name = int64(unsafe.Offsetof(caching.FieldEntry{}.Name))
	_Fe_Hash = int64(unsafe.Offsetof(caching.FieldEntry{}.Hash))
)

const (
	_Vk_Ptr = int64(reflect.Ptr)
	_Gt_KindFlags = int64(unsafe.Offsetof(rt.GoType{}.KindFlags))
)

const (
	_MaxStackBytes = 1 << 20 // 1MB
	_MinSlice = 32
	_MaxDigitNums = 32
	_DbufOffset = 64
	_FsmOffset = 128
	_EpOffset = 136
)

// Type references
var (
	int8Type    = reflect.TypeOf(int8(0))
	int16Type   = reflect.TypeOf(int16(0))
	int32Type   = reflect.TypeOf(int32(0))
	uint8Type   = reflect.TypeOf(uint8(0))
	uint16Type  = reflect.TypeOf(uint16(0))
	uint32Type  = reflect.TypeOf(uint32(0))
	float32Type = reflect.TypeOf(float32(0))
	byteType    = reflect.TypeOf(byte(0))
	jsonNumberType = reflect.TypeOf(json.Number(""))
	stringType   = reflect.TypeOf("")
)

	_I_int8, _T_int8    = rtype(int8Type)
	_I_int16, _T_int16   = rtype(int16Type)
	_I_int32, _T_int32   = rtype(int32Type)
	_I_uint8, _T_uint8   = rtype(uint8Type)
	_I_uint16, _T_uint16 = rtype(uint16Type)
	_I_uint32, _T_uint32 = rtype(uint32Type)
	_I_float32, _T_float32 = rtype(float32Type)

	_T_error                = rt.UnpackType(reflect.TypeOf((*error)(nil)).Elem())
	_I_base64_CorruptInputError = jit.Itab(_T_error, reflect.TypeOf(base64.CorruptInputError(0)))

	_I_json_UnsupportedValueError = jit.Itab(_T_error, reflect.TypeOf(new(json.UnsupportedValueError)))
	_I_json_MismatchTypeError     = jit.Itab(_T_error, reflect.TypeOf(new(MismatchTypeError)))
	_I_json_MismatchQuotedError   = jit.Itab(_T_error, reflect.TypeOf(new(MismatchQuotedError)))

	_T_bool   = rt.UnpackType(reflect.TypeOf(false))
	_T_number = rt.UnpackType(reflect.TypeOf(json.Number("")))
)

	jsonUnmarshalerType         = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	encodingTextUnmarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

// Helper functions
func rtype(t reflect.Type) (*rt.GoItab, *rt.GoType) {
	// Implementation for getting runtime type information
	return nil, rt.UnpackType(t)
}

func error_wrap(s string, ic int, et interface{}, ep interface{}) error {
	// Implementation for error wrapping
	return nil
}

func error_type(vt interface{}) error {
	// Implementation for type error
	return nil
}

func error_field(field string) error {
	// Implementation for field error
	return nil
}

func error_value(value interface{}, vt interface{}, ep interface{}) error {
	// Implementation for value error
	return nil
}

func error_mismatch(s string, ic int, vt interface{}, et interface{}) error {
	// Implementation for mismatch error
	return nil
}

var (
	stackOverflow = new(stackOverflowType)
)

type stackOverflowType struct{}

func decodeTypedPointer(s string, ic int, vp unsafe.Pointer, sb *_Stack, fv uint64, sv string) (int, error) {
	// Implementation for typed pointer decoding
	return 0, nil
}

func _subr_decode_value(s string, ic int, vp unsafe.Pointer, sb *_Stack, fv uint64) (int, error) {
	// Implementation for value decoding
	return 0, nil
}

func (self *_Assembler) WritePtrAX(reg int, addr obj.Addr, spill bool) {
	// Implementation for writing pointer from AX register
}

func (self *_Assembler) WriteRecNotAX(reg int, src obj.Addr, dst obj.Addr, spill bool, zero bool) {
	// Implementation for writing register to memory
}

func (self *_Assembler) Sref(label string, offset int) {
	// Implementation for symbol reference
}

func (self *_Assembler) Xref(label int, offset int) {
	// Implementation for cross reference
}

func (self *_Assembler) Xjmp(op string, target int) {
	// Implementation for extended jump
}

func (self *_Assembler) Byte(b ...byte) {
	// Implementation for writing bytes directly
}