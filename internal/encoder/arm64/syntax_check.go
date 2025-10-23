//go:build arm64 && go1.20 && !go1.26
// +build arm64,go1.20,!go1.26

package arm64

// This file is used for syntax checking only
// It imports all the types we use to ensure they exist and are compatible

import (
	"fmt"
	"reflect"

	"github.com/bytedance/sonic/internal/encoder/ir"
	"github.com/bytedance/sonic/internal/rt"
	"github.com/bytedance/sonic/loader"
)

// Syntax checking function
func checkSyntax() {
	// Test basic type compatibility
	var encoder *Encoder
	var assembler *Assembler
	var program ir.Program

	// Test function signatures
	_ = NewEncoder("test")
	_ = NewAssembler(program)
	_ = encoder.GetProgram()
	_ = encoder.Stats()

	// Test ptoenc function
	var fn loader.Function
	_ = ptoenc(fn)

	// Test encoder compilation
	var vt *rt.GoType
	_, _ = encoder.Compile(vt)

	// Test pretouch
	_ = Pretouch(reflect.TypeOf(""))

	// Test global functions
	_, _ = Encode("test")
	_, _ = EncodeToString("test")
	_, _ = EncodeIndented("test", "", "")

	// Test JIT options
	opts := DefaultJITOptions()
	encoder.ApplyOptions(opts)
	_ = encoder.IsOptimized()
	_ = encoder.CompileTime()
	_ = encoder.CodeSize()
	_ = encoder.InstructionCount()
	_ = encoder.DumpCode()
	_ = encoder.VerifyCode()

	// Test encoder methods
	encoder.SetOptions(0) // Use Options type
	_ = encoder.GetOptions()
	encoder.Reset()

	// Test stream encoder
	var streamEncoder *StreamEncoder
	_ = NewStreamEncoder(nil)
	_ = streamEncoder.Encode("test")
	_ = streamEncoder.Flush()

	// Test global functions
	_ = EncodeTypedPointer(nil, nil, nil, nil, 0)

	// If we get here without compilation errors, syntax is correct
	fmt.Println("ARM64 JIT encoder syntax check passed")
}
