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
	"testing"
	"unsafe"

	"github.com/bytedance/sonic/internal/encoder"
	"github.com/bytedance/sonic/internal/encoder/ir"
	"github.com/bytedance/sonic/internal/encoder/vars"
	"github.com/bytedance/sonic/internal/rt"
	"github.com/stretchr/testify/assert"
)

func TestBasicAssembler(t *testing.T) {
	// Test basic assembler creation and loading
	p := ir.Program{ir.NewInsOp(ir.OP_null)}
	a := NewAssembler(p)
	assert.NotNil(t, a)

	// Test loading
	f := a.Load()
	assert.NotNil(t, f)
}

func TestBasicNullEncoding(t *testing.T) {
	// Test null encoding
	p := ir.Program{ir.NewInsOp(ir.OP_null)}
	a := NewAssembler(p)
	f := a.Load()

	var buf []byte
	var stack vars.Stack
	vt := rt.UnpackType((*interface{})(nil))
	var vp unsafe.Pointer

	err := f(&buf, vt, vp, &stack, 0)
	assert.Nil(t, err)
	assert.Equal(t, "null", string(buf))
}

func TestBasicBoolEncoding(t *testing.T) {
	tests := []struct {
		name     string
		value    bool
		expected string
	}{
		{"true", true, "true"},
		{"false", false, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ir.Program{ir.NewInsOp(ir.OP_bool)}
			a := NewAssembler(p)
			f := a.Load()

			var buf []byte
			var stack vars.Stack

			err := f(&buf, rt.UnpackEface(tt.value).Value, &stack, 0)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, string(buf))
		})
	}
}

func TestBasicIntegerEncoding(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
		opcode   ir.OpCode
	}{
		{"int8", int8(42), "42", ir.OP_i8},
		{"int16", int16(42), "42", ir.OP_i16},
		{"int32", int32(42), "42", ir.OP_i32},
		{"int64", int64(42), "42", ir.OP_i64},
		{"uint8", uint8(42), "42", ir.OP_u8},
		{"uint16", uint16(42), "42", ir.OP_u16},
		{"uint32", uint32(42), "42", ir.OP_u32},
		{"uint64", uint64(42), "42", ir.OP_u64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ir.Program{ir.NewInsOp(tt.opcode)}
			a := NewAssembler(p)
			f := a.Load()

			var buf []byte
			var stack vars.Stack

			err := f(&buf, rt.UnpackEface(tt.value).Value, &stack, 0)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, string(buf))
		})
	}
}

func TestBasicStringEncoding(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{"empty", "", `""`},
		{"simple", "hello", `"hello"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ir.Program{ir.NewInsOp(ir.OP_str)}
			a := NewAssembler(p)
			f := a.Load()

			var buf []byte
			var stack vars.Stack

			err := f(&buf, rt.UnpackEface(tt.value).Value, &stack, 0)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, string(buf))
		})
	}
}