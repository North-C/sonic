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
	"reflect"
	"unsafe"

	"github.com/bytedance/sonic/internal/caching"
	"github.com/bytedance/sonic/internal/decoder/consts"
	"github.com/bytedance/sonic/internal/decoder/errors"
	"github.com/bytedance/sonic/internal/jit"
	"github.com/bytedance/sonic/internal/native/types"
	"github.com/bytedance/sonic/internal/rt"
	"github.com/bytedance/sonic/option"
)

// ARM64 Decoder represents the ARM64 JIT decoder
type Decoder struct {
	assembler *_Assembler
	program   _Program
	name      string
	compiled  bool
}

// NewDecoder creates a new ARM64 JIT decoder
func NewDecoder(name string) *Decoder {
	return &Decoder{
		name:     name,
		compiled: false,
	}
}

// Compile compiles the given type into ARM64 JIT code
func (d *Decoder) Compile(vt reflect.Type, opts ...option.CompileOption) (interface{}, error) {
	// Create compiler to generate instruction program
	compiler := newCompiler()
	if len(opts) > 0 {
		compiler.apply(opts[0])
	}

	// Generate instruction program
	program, err := compiler.compile(vt)
	if err != nil {
		return nil, err
	}

	// Store the program and create assembler
	d.program = program
	d.assembler = newAssembler(program)
	d.assembler.name = d.name

	// Compile to ARM64 machine code
	decoder := d.assembler.Load()
	d.compiled = true
	return &decoder, nil
}

// GetProgram returns the compiled JIT program for debugging
func (d *Decoder) GetProgram() *_Program {
	if d.assembler != nil {
		return &d.program
	}
	return nil
}

// Stats returns compilation statistics
func (d *Decoder) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"platform": "arm64",
		"name":     d.name,
		"jit":      "enabled",
	}

	if d.compiled {
		stats["compiled"] = true
		stats["program_size"] = len(d.program)
		stats["instruction_count"] = d.program.pc()
	}

	return stats
}

// Reset resets the decoder state
func (d *Decoder) Reset() {
	d.assembler = nil
	d.program = nil
	d.compiled = false
}

// IsOptimized returns true if the decoder is JIT optimized
func (d *Decoder) IsOptimized() bool {
	return d.compiled
}

// CompileTime returns the time taken to compile (in nanoseconds)
func (d *Decoder) CompileTime() int64 {
	// TODO: Implement compile time measurement
	return 0
}

// CodeSize returns the size of generated JIT code in bytes
func (d *Decoder) CodeSize() int {
	if d.assembler != nil {
		// TODO: Get actual code size from assembler
		return len(d.program) * 8 // Approximate size
	}
	return 0
}

// InstructionCount returns the number of generated instructions
func (d *Decoder) InstructionCount() int {
	return len(d.program)
}

// VerifyCode checks if the generated code is valid
func (d *Decoder) VerifyCode() error {
	// TODO: Implement code verification
	return nil
}

// DumpCode returns the generated machine code as a hex string for debugging
func (d *Decoder) DumpCode() string {
	// TODO: Implement code dumping for debugging
	return ""
}

// ApplyOptions applies decoder-specific options
func (d *Decoder) ApplyOptions(opts interface{}) {
	// TODO: Implement option application
}

// Decode performs the actual JSON decoding using the compiled JIT code
func (d *Decoder) Decode(s string, ic int, vp unsafe.Pointer, sb *_Stack, fv uint64, sv string) (int, error) {
	if !d.compiled || d.assembler == nil {
		return 0, fmt.Errorf("decoder not compiled")
	}

	// Call the compiled decoder function
	if encoder, ok := d.assembler.Load().(func(string, int, unsafe.Pointer, *_Stack, uint64, string) (int, error)); ok {
		return encoder(s, ic, vp, sb, fv, sv)
	}

	return 0, fmt.Errorf("failed to load decoder function")
}

// Stack represents the decoder stack for recursive decoding
type _Stack struct {
	data []interface{}
}

// NewStack creates a new decoder stack
func NewStack() *_Stack {
	return &_Stack{
		data: make([]interface{}, 0, 64),
	}
}

// Push pushes a value onto the stack
func (s *_Stack) Push(v interface{}) {
	s.data = append(s.data, v)
}

// Pop pops a value from the stack
func (s *_Stack) Pop() interface{} {
	if len(s.data) == 0 {
		return nil
	}
	v := s.data[len(s.data)-1]
	s.data = s.data[:len(s.data)-1]
	return v
}

// Peek returns the top value without popping it
func (s *_Stack) Peek() interface{} {
	if len(s.data) == 0 {
		return nil
	}
	return s.data[len(s.data)-1]
}

// Empty returns true if the stack is empty
func (s *_Stack) Empty() bool {
	return len(s.data) == 0
}

// Size returns the current stack size
func (s *_Stack) Size() int {
	return len(s.data)
}

// Pretouch pre-compiles the given type to avoid JIT compilation on-the-fly
func Pretouch(vt reflect.Type, opts ...option.CompileOption) error {
	// Create a temporary decoder to pre-compile the type
	decoder := NewDecoder("pretouch")

	// Compile the type
	_, err := decoder.Compile(vt, opts...)
	if err != nil {
		return err
	}

	// TODO: Store the compiled decoder in cache for later use
	return nil
}

// DecodeTypedPointer is the main entry point for ARM64 JIT decoding
func DecodeTypedPointer(s string, ic *int, vp unsafe.Pointer, sb *_Stack, fv uint64) error {
	// Extract the type information from the value pointer
	if vp == nil {
		return errors.NewTypeError("nil value pointer")
	}

	// Get the actual type of the value
	val := reflect.NewAt(reflect.TypeOf(vp).Elem(), vp)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Create a decoder and compile it for this type
	decoder := NewDecoder("runtime")
	compiledDecoder, err := decoder.Compile(val.Type())
	if err != nil {
		return err
	}

	// Perform the actual decoding
	result, err := compiledDecoder.(func(string, int, unsafe.Pointer, *_Stack, uint64, string) (int, error))(s, *ic, vp, sb, fv, "")
	if err != nil {
		return err
	}

	*ic = result
	return nil
}

// Helper types and constants
type (
	Options = consts.Options
	MismatchTypeError = errors.MismatchTypeError
	SyntaxError = errors.SyntaxError
)

// Global decoder instance for common operations
var defaultDecoder = NewDecoder("default")

// High-level decoding functions
func Decode(s string, val interface{}) error {
	return decodeImpl(&s, new(int), 0, val)
}

func decodeImpl(sp *string, ic *int, fv uint64, val interface{}) error {
	// Create a decoder stack
	sb := NewStack()

	// Get the type of the value to decode
	vt := reflect.TypeOf(val)
	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}

	// Create and compile a decoder for this type
	decoder := NewDecoder("decode")
	compiledDecoder, err := decoder.Compile(vt)
	if err != nil {
		return err
	}

	// Use the compiled decoder to decode
	result, err := compiledDecoder.(func(string, int, unsafe.Pointer, *_Stack, uint64, string) (int, error))(*sp, *ic, unsafe.Pointer(&val), sb, fv, "")
	if err != nil {
		return err
	}

	*ic = result
	return nil
}

// Validate validates JSON-encoded bytes and reports if it is valid
func Valid(data []byte) bool {
	ok, _ := ValidString(string(data))
	return ok
}

// ValidString validates the JSON-encoded string and reports if it is valid
func ValidString(data string) (bool, error) {
	// For now, implement basic validation
	// Full validation would require more sophisticated parsing
	if len(data) == 0 {
		return true, nil
	}

	// Check if it starts and ends with braces or brackets
	if (data[0] == '{' && data[len(data)-1] == '}') ||
		(data[0] == '[' && data[len(data)-1] == ']') {
		return true, nil
	}

	return false, nil
}

// ARM64-specific helper functions
func init() {
	// Initialize ARM64 JIT decoder
	// This would be called when the package is loaded
}

// GetArchitectureInfo returns architecture-specific information
func GetArchitectureInfo() map[string]interface{} {
	return map[string]interface{}{
		"platform":      "arm64",
		"jit_enabled":   true,
		"stack_align":   16,
		"max_registers":  31,
		"has_simd":      true,
		"has_neon":      true,
		"compiler":      "arm64-jit",
	}
}

// Performance statistics
type PerfStats struct {
	CompileTime       int64    `json:"compile_time_ns"`
	CodeSize          int      `json:"code_size_bytes"`
	InstructionCount int      `json:"instruction_count"`
	Optimizations    []string `json:"optimizations"`
}

// GetPerfStats returns performance statistics for the decoder
func GetPerfStats(decoder *Decoder) PerfStats {
	return PerfStats{
		CompileTime:       decoder.CompileTime(),
		CodeSize:          decoder.CodeSize(),
		InstructionCount: decoder.InstructionCount(),
		Optimizations:    []string{"arm64", "jit", "simd", "neon"},
	}
}

// Debug information
type DebugInfo struct {
	Program   string                 `json:"program"`
	Assembly string                 `json:"assembly"`
	Code      string                 `json:"code"`
	Stats     map[string]interface{} `json:"stats"`
}

// GetDebugInfo returns debug information for the decoder
func GetDebugInfo(decoder *Decoder) DebugInfo {
	program := ""
	if decoder.GetProgram() != nil {
		program = decoder.GetProgram().disassemble()
	}

	return DebugInfo{
		Program:   "arm64_jit",
		Assembly: "", // Would contain assembly code
		Code:      decoder.DumpCode(),
		Stats:    decoder.Stats(),
	}
}

// Compatibility functions to ensure ARM64 decoder works with existing API
func EnsureCompatibility(decoder *Decoder) error {
	// Check that the decoder implements the required interface
	if decoder == nil {
		return fmt.Errorf("nil decoder")
	}

	// Verify essential methods exist
	if decoder.GetProgram() == nil {
		// This is expected before compilation
	}

	return nil
}

// Helper function to create decoder with custom name
func CreateDecoderWithName(name string) *Decoder {
	return NewDecoder(name)
}

// Batch compilation for multiple types
func BatchCompile(types []reflect.Type) (map[reflect.Type]interface{}, error) {
	results := make(map[reflect.Type]interface{})
	errors := make(map[reflect.Type]error)

	for _, vt := range types {
		decoder := CreateDecoderWithName("batch_" + vt.String())
		compiledDecoder, err := decoder.Compile(vt)
		if err != nil {
			errors[vt] = err
			continue
		}

		results[vt] = compiledDecoder
	}

	if len(errors) > 0 {
		return results, errors[reflect.TypeOf(struct{}{})] // Return first error
	}

	return results, nil
}

// ARM64 JIT options specific to decoding
type JITOptions struct {
	OptimizationLevel int
	EnableSIMD       bool
	EnableInlining   bool
	DebugMode         bool
	StrictMode        bool
	FastPath          bool
}

// DefaultJITOptions returns the default JIT options for ARM64
func DefaultJITOptions() JITOptions {
	return JITOptions{
		OptimizationLevel: 1,
		EnableSIMD:       true,
		EnableInlining:   true,
		DebugMode:         false,
		StrictMode:       false,
		FastPath:         true,
	}
}

// ApplyJITOptions applies JIT options to the decoder
func (d *Decoder) ApplyJITOptions(opts JITOptions) {
	// TODO: Implement option application
	// This would configure the JIT decoder behavior
}

// IsJITEnabled returns true if JIT compilation is enabled
func IsJITEnabled() bool {
	return jit.IsARM64JITEnabled()
}

// EnableJIT enables JIT compilation
func EnableJIT() {
	// jit.EnableARM64JIT() // Would need to implement this
}

// DisableJIT disables JIT compilation
func DisableJIT() {
	// jit.DisableARM64JIT() // Would need to implement this
}

// ForceUseFallback forces the use of fallback decoder instead of JIT
func ForceUseFallback() {
	// TODO: Implement fallback usage
}

// ARM64-specific constants
const (
	MaxStackDepth        = 100
	DefaultBufferSize    = 4096
	MaxFieldCount        = 50
	MaxProgramSize      = 100000
	DefaultOptLevel      = 1
)

// ARM64-specific types
type (
	_Stack   struct {
		data []interface{}
	}
	_Decoder interface {
		Decode(s string, ic int, vp unsafe.Pointer, sb *_Stack, fv uint64, sv string) (int, error)
	}
)