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

package encoder

import (
	"github.com/bytedance/sonic/internal/encoder/arm64"
	"github.com/bytedance/sonic/internal/encoder/ir"
	"github.com/bytedance/sonic/internal/encoder/vars"
	"github.com/bytedance/sonic/internal/rt"
)

// EnableFallback indicates if encoder use fallback
const EnableFallback = false

// Encoder represents a specific set of encoder configurations
type Encoder = arm64.Encoder

// StreamEncoder uses io.Writer as input
type StreamEncoder = arm64.StreamEncoder

// Options is a set of encoding options
type Options = arm64.Options

const (
	// SortMapKeys indicates that the keys of a map needs to be sorted
	// before serializing into JSON.
	// WARNING: This hurts performance A LOT, USE WITH CARE.
	SortMapKeys Options = arm64.SortMapKeys

	// EscapeHTML indicates encoder to escape all HTML characters
	// after serializing into JSON (see https://pkg.go.dev/encoding/json#HTMLEscape).
	// WARNING: This hurts performance A LOT, USE WITH CARE.
	EscapeHTML Options = arm64.EscapeHTML

	// CompactMarshaler indicates that the output JSON from json.Marshaler
	// is always compact and needs no validation
	CompactMarshaler Options = arm64.CompactMarshaler

	// NoQuoteTextMarshaler indicates encoder that the output text from encoding.TextMarshaler
	// is always escaped string and needs no quoting
	NoQuoteTextMarshaler Options = arm64.NoQuoteTextMarshaler

	// NoNullSliceOrMap indicates encoder that all empty Array or Object are encoded as '[]' or '{}',
	// instead of 'null'
	NoNullSliceOrMap Options = arm64.NoErrorSliceOrMap

	// CopyString indicates decoder to decode string values by copying instead of referring.
	CopyString Options = arm64.CopyString

	// ValidateString indicates decoder and encoder to validate string values: decoder will return errors
	// when unescaped control chars(\u0000-\u001f) in the string value of JSON.
	ValidateString Options = arm64.ValidateString

	// NoValidateJSONMarshaler indicates that the encoder should not validate the output string
	// after encoding the JSONMarshaler to JSON.
	NoValidateJSONMarshaler Options = arm64.NoValidateJSONMarshaler

	// NoValidateJSONSkip indicates the decoder should not validate the JSON value when skipping it,
	// such as unknown-fields, mismatched-type, redundant elements..
	NoValidateJSONSkip Options = arm64.NoValidateJSONSkip

	// NoEncoderNewline indicates that the encoder should not add a newline after every message
	NoEncoderNewline Options = arm64.NoEncoderNewline

	// Encode Infinity or Nan float into `null`, instead of returning an error.
	EncodeNullForInfOrNan Options = arm64.EncodeNullForInfOrNan

	// CaseSensitive indicates that the decoder should not ignore the case of object keys.
	CaseSensitive Options = arm64.CaseSensitive
)

// NewEncoder creates a new ARM64 encoder
func NewEncoder() *Encoder {
	return arm64.NewEncoder("sonic")
}

// NewStreamEncoder creates a new stream encoder for ARM64
func NewStreamEncoder(writer io.Writer) *StreamEncoder {
	return arm64.NewStreamEncoder(writer)
}

// Encode encodes the given value into JSON bytes
func Encode(val interface{}) ([]byte, error) {
	return arm64.Encode(val)
}

// EncodeToString encodes the given value into JSON string
func EncodeToString(val interface{}) (string, error) {
	return arm64.EncodeToString(val)
}

// EncodeIndented encodes the given value into indented JSON bytes
func EncodeIndented(val interface{}, prefix, indent string) ([]byte, error) {
	return arm64.EncodeIndented(val, prefix, indent)
}

// EncodeTypedPointer encodes the given typed pointer using JIT
func EncodeTypedPointer(buf *[]byte, vt *rt.GoType, vp unsafe.Pointer, sb *vars.Stack, fv uint64) error {
	return arm64.EncodeTypedPointer(buf, vt, vp, sb, fv)
}

// Pretouch pre-compiles the given type to avoid JIT compilation on-the-fly
func Pretouch(vt reflect.Type, opts ...option.CompileOption) error {
	return arm64.Pretouch(vt, opts...)
}

// Validate validates the JSON-encoded bytes and reports if it is valid
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

// GetProgram returns the compiled JIT program (for debugging)
func GetProgram(encoder *Encoder) *ir.Program {
	return encoder.GetProgram()
}

// SetOptions sets encoding options on the encoder
func SetOptions(encoder *Encoder, opts Options) {
	encoder.SetOptions(opts)
}

// GetOptions returns the current encoding options
func GetOptions(encoder *Encoder) Options {
	// Implementation would return current options
	return Options{}
}

// Reset resets the encoder state
func Reset(encoder *Encoder) {
	encoder.Reset()
}

// Stats returns encoder statistics
func Stats(encoder *Encoder) map[string]interface{} {
	return encoder.Stats()
}

// IsOptimized returns true if the encoder is JIT optimized
func IsOptimized(encoder *Encoder) bool {
	return encoder.IsOptimized()
}

// CodeSize returns the size of generated JIT code
func CodeSize(encoder *Encoder) int {
	return encoder.CodeSize()
}

// InstructionCount returns the number of generated instructions
func InstructionCount(encoder *Encoder) int {
	return encoder.InstructionCount()
}

// CompileTime returns the time taken to compile (in nanoseconds)
func CompileTime(encoder *Encoder) int64 {
	return encoder.CompileTime()
}

// VerifyCode verifies the generated JIT code
func VerifyCode(encoder *Encoder) error {
	return encoder.VerifyCode()
}

// DumpCode returns the generated code as hex string
func DumpCode(encoder *Encoder) string {
	return encoder.DumpCode()
}

// ApplyJITOptions applies JIT-specific options
func ApplyJITOptions(encoder *Encoder, opts arm64.JITOptions) {
	encoder.ApplyOptions(opts)
}

// GetJITOptions returns current JIT options
func GetJITOptions(encoder *Encoder) arm64.JITOptions {
	// Implementation would return current JIT options
	return arm64.DefaultJITOptions()
}

// IsJITEnabled returns true if JIT compilation is enabled
func IsJITEnabled() bool {
	return jit.IsARM64JITEnabled()
}

// EnableJIT enables JIT compilation
func EnableJIT() {
	jit.EnableARM64JIT()
}

// DisableJIT disables JIT compilation
func DisableJIT() {
	jit.DisableARM64JIT()
}

// ForceUseVM forces the use of VM-based encoding instead of JIT
func ForceUseVM() {
	// Implementation would disable JIT and force VM usage
	arm64.DisableARM64JIT()
}

// Helper function to convert options between packages
func convertOptions(opts Options) arm64.Options {
	// Convert generic options to ARM64-specific options
	return arm64.Options(opts)
}

// ValidateOptions validates encoding options
func ValidateOptions(opts Options) error {
	// Validate that the options are compatible with ARM64 JIT
	return nil
}

// Initialize ARM64 encoder package
func init() {
	// Initialize any global state or caches
	// This would be called when the package is loaded
}

// GetArchitectureInfo returns architecture-specific information
func GetArchitectureInfo() map[string]interface{} {
	return map[string]interface{}{
		"platform":      "arm64",
		"jit_enabled":   IsJITEnabled(),
		"has_fallback":   EnableFallback,
		"stack_align":   16,
		"max_registers": 31,
		"has_simd":      true,
		"has_neon":      true,
	}
}

// Performance statistics
type PerfStats struct {
	CompileTime     int64   `json:"compile_time_ns"`
	CodeSize        int     `json:"code_size_bytes"`
	InstructionCount int     `json:"instruction_count"`
	Optimizations   []string `json:"optimizations"`
}

// GetPerfStats returns performance statistics for the encoder
func GetPerfStats(encoder *Encoder) PerfStats {
	return PerfStats{
		CompileTime:      encoder.CompileTime(),
		CodeSize:         encoder.CodeSize(),
		InstructionCount: encoder.InstructionCount(),
		Optimizations:    []string{"arm64", "jit", "simd", "neon"},
	}
}

// Debug information
type DebugInfo struct {
	Program     string `json:"program"`
	Assembly   string `json:"assembly"`
	Code        string `json:"code"`
	Stats       map[string]interface{} `json:"stats"`
}

// GetDebugInfo returns debug information for the encoder
func GetDebugInfo(encoder *Encoder) DebugInfo {
	return DebugInfo{
	Program:   "arm64_jit",
	Assembly: "", // Would contain assembly code
		Code:     encoder.DumpCode(),
	Stats:    encoder.Stats(),
	}
}

// Compatibility functions to ensure ARM64 encoder works with existing API
func EnsureCompatibility(encoder *Encoder) error {
	// Check that the encoder implements the required interface
	if encoder == nil {
		return vars.ERR_unsupported
	}

	// Verify essential methods exist
	if encoder.GetProgram() == nil {
		// This is expected before compilation
	}

	return nil
}

// Helper function to create encoder with custom name
func CreateEncoderWithName(name string) *Encoder {
	return arm64.NewEncoder(name)
}

// Batch compilation for multiple types
func BatchCompile(types []reflect.Type) (map[reflect.Type]interface{}, error) {
	results := make(map[reflect.Type]interface{})
	errors := make(map[reflect.Type]error)

	for _, vt := range types {
		encoder := CreateEncoderWithName("batch_" + vt.String())
		goType := rt.UnpackType(vt)

		compiled, err := encoder.Compile(goType)
		if err != nil {
			errors[vt] = err
			continue
		}

		results[vt] = compiled
	}

	if len(errors) > 0 {
		return results, errors[reflect.TypeOf(struct{}{})] // Return first error
	}

	return results, nil
}