// This file tests ARM64 JIT components without build constraints
// It can run on any platform to validate the implementation structure

//go:build !arm64
// +build !arm64

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

package encoder_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestARM64JITStructure validates the structure of ARM64 JIT implementation
// This test runs on non-ARM64 platforms to validate the code structure
func TestARM64JITStructure(t *testing.T) {
	// Get the directory containing ARM64 code
	dir := "arm64"

	// Read all Go files in the arm64 directory
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)

	var goFiles []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".go") && !strings.HasSuffix(file.Name(), "_test.go") {
			goFiles = append(goFiles, filepath.Join(dir, file.Name()))
		}
	}

	require.Greater(t, len(goFiles), 0, "Should have at least one Go file")

	// Parse and validate each file
	for _, filePath := range goFiles {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			validateARM64File(t, filePath)
		})
	}
}

// validateARM64File validates the structure of an ARM64 JIT implementation file
func validateARM64File(t *testing.T, filePath string) {
	// Read the file
	content, err := ioutil.ReadFile(filePath)
	require.NoError(t, err)

	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	require.NoError(t, err)

	// Check for required build constraints
	assert.True(t, hasBuildConstraints(content),
		"File should have ARM64 build constraints")

	// Validate structure
	validateASTStructure(t, node)
}

// hasBuildConstraints checks if the file has proper ARM64 build constraints
func hasBuildConstraints(content []byte) bool {
	contentStr := string(content)
	return strings.Contains(contentStr, "//go:build arm64") ||
		strings.Contains(contentStr, "// +build arm64")
}

// validateASTStructure validates the AST structure of the ARM64 implementation
func validateASTStructure(t *testing.T, node *ast.File) {
	// Check for required imports
	requiredImports := map[string]bool{
		"github.com/bytedance/sonic/internal/encoder/ir":      false,
		"github.com/bytedance/sonic/internal/encoder/vars":  false,
		"github.com/bytedance/sonic/internal/jit":           false,
		"github.com/twitchyliquid64/golang-asm/obj":        false,
	}

	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if _, exists := requiredImports[importPath]; exists {
			requiredImports[importPath] = true
		}
	}

	// Verify all required imports are present
	for importPath, found := range requiredImports {
		if !found {
			t.Logf("Warning: Missing recommended import: %s", importPath)
		}
	}

	// Check for key functions and types
	var hasAssemblerType, hasNewAssembler, hasLoadMethod bool

	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			// Look for type declarations
			if d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if typeSpec.Name.Name == "Assembler" {
							hasAssemblerType = true
						}
					}
				}
			}
		case *ast.FuncDecl:
			// Look for key functions
			if d.Name.Name == "NewAssembler" {
				hasNewAssembler = true
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				if recvType, ok := d.Recv.List[0].Type.(*ast.StarExpr); ok {
					if ident, ok := recvType.X.(*ast.Ident); ok && ident.Name == "Assembler" {
						if d.Name.Name == "Load" {
							hasLoadMethod = true
						}
					}
				}
			}
		}
	}

	// Validate required components
	assert.True(t, hasAssemblerType, "Should have Assembler type")
	assert.True(t, hasNewAssembler, "Should have NewAssembler function")
	assert.True(t, hasLoadMethod, "Should have Load method on Assembler")
}

// TestARM64InstructionTable validates that the instruction table is properly structured
func TestARM64InstructionTable(t *testing.T) {
	dir := "arm64"
	assemblerFile := filepath.Join(dir, "assembler_regabi_arm64.go")

	// Read the assembler file
	content, err := ioutil.ReadFile(assemblerFile)
	require.NoError(t, err)

	contentStr := string(content)

	// Check for instruction table
	assert.True(t, strings.Contains(contentStr, "_OpFuncTab"),
		"Should have instruction table")

	// Check for basic opcode implementations
	basicOpcodes := []string{
		"_asm_OP_null",
		"_asm_OP_bool",
		"_asm_OP_str",
		"_asm_OP_i8",
		"_asm_OP_u8",
		"_asm_OP_i64",
		"_asm_OP_u64",
		"_asm_OP_f32",
		"_asm_OP_f64",
	}

	for _, opcode := range basicOpcodes {
		assert.True(t, strings.Contains(contentStr, opcode),
			"Should have implementation for %s", opcode)
	}
}

// TestARM64Constants validates that required constants are defined
func TestARM64Constants(t *testing.T) {
	dir := "arm64"
	assemblerFile := filepath.Join(dir, "assembler_regabi_arm64.go")

	// Read the assembler file
	content, err := ioutil.ReadFile(assemblerFile)
	require.NoError(t, err)

	contentStr := string(content)

	// Check for required constants
	requiredConstants := []string{
		"_FP_args",
		"_FP_size",
		"_FP_base",
		"_ARG0",
		"_ARG1",
		"_RET0",
		"_ST",
		"_RP",
		"_RL",
		"_RC",
		"_LB_error",
		"_LB_more_space",
	}

	for _, constant := range requiredConstants {
		assert.True(t, strings.Contains(contentStr, constant),
			"Should have constant %s", constant)
	}
}

// TestARM64JITInterface validates that the JIT interface is complete
func TestARM64JITInterface(t *testing.T) {
	dir := "arm64"

	// List of expected files
	expectedFiles := []string{
		"assembler_regabi_arm64.go",
		"assembler_regabi_arm64_test.go",
		"simple_test.go",
		"syntax_check.go",
	}

	// Check that expected files exist
	for _, filename := range expectedFiles {
		filePath := filepath.Join(dir, filename)
		_, err := ioutil.Stat(filePath)
		assert.NoError(t, err, "Expected file %s should exist", filename)
	}

	// Check that unexpected files don't exist
	// (Files that were removed in refactoring)
	removedFiles := []string{
		"encoder_arm64.go",
		"encoder_arm64_test.go",
	}

	for _, filename := range removedFiles {
		filePath := filepath.Join(dir, filename)
		_, err := ioutil.Stat(filePath)
		assert.True(t, os.IsNotExist(err),
			"Removed file %s should not exist", filename)
	}
}