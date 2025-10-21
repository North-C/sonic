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

	"github.com/bytedance/sonic/internal/jit"
	"github.com/twitchyliquid64/golang-asm/obj"
	"github.com/twitchyliquid64/golang-asm/obj/arm64"
)

func TestInstructionTranslator_TranslateMov(t *testing.T) {
	translator := NewInstructionTranslator()

	tests := []struct {
		name      string
		operands  []interface{}
		expectedAs obj.As
	}{
		{
			name:      "register to register",
			operands:  []interface{}{jit.R0, jit.R1},
			expectedAs: arm64.AMOVD,
		},
		{
			name:      "immediate to register",
			operands:  []interface{}{jit.R0, jit.Imm(42)},
			expectedAs: arm64.AMOVD,
		},
		{
			name:      "memory to register",
			operands:  []interface{}{jit.R0, jit.Ptr(jit.R1, 8)},
			expectedAs: arm64.AMOVD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := translator.TranslateInstruction(INSN_MOV, tt.operands...)
			if err != nil {
				t.Fatalf("Translation failed: %v", err)
			}

			if prog.As != tt.expectedAs {
				t.Errorf("Expected instruction %v, got %v", tt.expectedAs, prog.As)
			}
		})
	}
}

func TestInstructionTranslator_TranslateAdd(t *testing.T) {
	translator := NewInstructionTranslator()

	tests := []struct {
		name      string
		operands  []interface{}
		expectedAs obj.As
	}{
		{
			name:      "add immediate",
			operands:  []interface{}{jit.R0, jit.Imm(5)},
			expectedAs: arm64.AADD,
		},
		{
			name:      "add register",
			operands:  []interface{}{jit.R0, jit.R1},
			expectedAs: arm64.AADD,
		},
		{
			name:      "add three operands",
			operands:  []interface{}{jit.R0, jit.R1, jit.R2},
			expectedAs: arm64.AADD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := translator.TranslateInstruction(INSN_ADD, tt.operands...)
			if err != nil {
				t.Fatalf("Translation failed: %v", err)
			}

			if prog.As != tt.expectedAs {
				t.Errorf("Expected instruction %v, got %v", tt.expectedAs, prog.As)
			}
		})
	}
}

func TestInstructionTranslator_TranslateSub(t *testing.T) {
	translator := NewInstructionTranslator()

	tests := []struct {
		name      string
		operands  []interface{}
		expectedAs obj.As
	}{
		{
			name:      "subtract immediate",
			operands:  []interface{}{jit.R0, jit.Imm(5)},
			expectedAs: arm64.ASUB,
		},
		{
			name:      "subtract register",
			operands:  []interface{}{jit.R0, jit.R1},
			expectedAs: arm64.ASUB,
		},
		{
			name:      "subtract three operands",
			operands:  []interface{}{jit.R0, jit.R1, jit.R2},
			expectedAs: arm64.ASUB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := translator.TranslateInstruction(INSN_SUB, tt.operands...)
			if err != nil {
				t.Fatalf("Translation failed: %v", err)
			}

			if prog.As != tt.expectedAs {
				t.Errorf("Expected instruction %v, got %v", tt.expectedAs, prog.As)
			}
		})
	}
}

func TestInstructionTranslator_TranslateMul(t *testing.T) {
	translator := NewInstructionTranslator()

	tests := []struct {
		name      string
		operands  []interface{}
		expectedAs obj.As
	}{
		{
			name:      "multiply register",
			operands:  []interface{}{jit.R0, jit.R1},
			expectedAs: arm64.AMUL,
		},
		{
			name:      "multiply three operands",
			operands:  []interface{}{jit.R0, jit.R1, jit.R2},
			expectedAs: arm64.AMUL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := translator.TranslateInstruction(INSN_MUL, tt.operands...)
			if err != nil {
				t.Fatalf("Translation failed: %v", err)
			}

			if prog.As != tt.expectedAs {
				t.Errorf("Expected instruction %v, got %v", tt.expectedAs, prog.As)
			}
		})
	}
}

func TestInstructionTranslator_TranslateCmp(t *testing.T) {
	translator := NewInstructionTranslator()

	prog, err := translator.TranslateInstruction(INSN_CMP, jit.R0, jit.R1)
	if err != nil {
		t.Fatalf("Translation failed: %v", err)
	}

	if prog.As != arm64.ACMP {
		t.Errorf("Expected instruction %v, got %v", arm64.ACMP, prog.As)
	}

	if prog.From.Reg != jit.R0.Reg || prog.Reg != jit.R1.Reg {
		t.Errorf("Expected operands R0 and R1, got %v and %v", prog.From, prog.Reg)
	}
}

func TestInstructionTranslator_TranslateJmp(t *testing.T) {
	translator := NewInstructionTranslator()

	tests := []struct {
		name     string
		target   interface{}
		expected obj.Addr
	}{
		{
			name:   "jump to label",
			target: "test_label",
			expected: obj.Addr{
				Type: obj.TYPE_BRANCH,
				Name: "test_label",
			},
		},
		{
			name:   "jump to register",
			target: jit.R0,
			expected: jit.R0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := translator.TranslateInstruction(INSN_JMP, tt.target)
			if err != nil {
				t.Fatalf("Translation failed: %v", err)
			}

			if prog.As != arm64.AB {
				t.Errorf("Expected instruction %v, got %v", arm64.AB, prog.As)
			}

			if prog.To.Type != tt.expected.Type {
				t.Errorf("Expected target type %v, got %v", tt.expected.Type, prog.To.Type)
			}
		})
	}
}

func TestInstructionTranslator_TranslateJcc(t *testing.T) {
	translator := NewInstructionTranslator()

	tests := []struct {
		name     string
		condition ConditionCode
		expectedAs obj.As
	}{
		{
			name:       "jump if equal",
			condition:  COND_E,
			expectedAs: arm64.ABEQ,
		},
		{
			name:       "jump if not equal",
			condition:  COND_NE,
			expectedAs: arm64.ABNE,
		},
		{
			name:       "jump if greater",
			condition:  COND_G,
			expectedAs: arm64.ABGT,
		},
		{
			name:       "jump if less",
			condition:  COND_L,
			expectedAs: arm64.ABLT,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := translator.TranslateInstruction(INSN_JCC, tt.condition, "test_target")
			if err != nil {
				t.Fatalf("Translation failed: %v", err)
			}

			if prog.As != tt.expectedAs {
				t.Errorf("Expected instruction %v, got %v", tt.expectedAs, prog.As)
			}

			if prog.To.Type != obj.TYPE_BRANCH || prog.To.Name != "test_target" {
				t.Errorf("Expected branch target 'test_target', got %v", prog.To)
			}
		})
	}
}

func TestInstructionTranslator_TranslateCall(t *testing.T) {
	translator := NewInstructionTranslator()

	prog, err := translator.TranslateInstruction(INSN_CALL, "test_function")
	if err != nil {
		t.Fatalf("Translation failed: %v", err)
	}

	if prog.As != arm64.ABL {
		t.Errorf("Expected instruction %v, got %v", arm64.ABL, prog.As)
	}

	if prog.To.Type != obj.TYPE_BRANCH || prog.To.Name != "test_function" {
		t.Errorf("Expected branch target 'test_function', got %v", prog.To)
	}
}

func TestInstructionTranslator_TranslateRet(t *testing.T) {
	translator := NewInstructionTranslator()

	prog, err := translator.TranslateInstruction(INSN_RET)
	if err != nil {
		t.Fatalf("Translation failed: %v", err)
	}

	if prog.As != arm64.ARET {
		t.Errorf("Expected instruction %v, got %v", arm64.ARET, prog.As)
	}
}

func TestInstructionTranslator_ErrorCases(t *testing.T) {
	translator := NewInstructionTranslator()

	tests := []struct {
		name     string
		insnType InstructionType
		operands []interface{}
	}{
		{
			name:     "MOV with no operands",
			insnType: INSN_MOV,
			operands: []interface{}{},
		},
		{
			name:     "ADD with one operand",
			insnType: INSN_ADD,
			operands: []interface{}{jit.R0},
		},
		{
			name:     "CMP with one operand",
			insnType: INSN_CMP,
			operands: []interface{}{jit.R0},
		},
		{
			name:     "JMP with no operands",
			insnType: INSN_JMP,
			operands: []interface{}{},
		},
		{
			name:     "JCC with one operand",
			insnType: INSN_JCC,
			operands: []interface{}{COND_E},
		},
		{
			name:     "unsupported instruction type",
			insnType: InstructionType(999),
			operands: []interface{}{jit.R0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := translator.TranslateInstruction(tt.insnType, tt.operands...)
			if err == nil {
				t.Error("Expected error, but got none")
			}
		})
	}
}

func TestInstructionTranslator_TranslateInstructionSequence(t *testing.T) {
	translator := NewInstructionTranslator()

	instructions := []Instruction{
		{Type: INSN_MOV, Operands: []interface{}{jit.R0, jit.Imm(10)}},
		{Type: INSN_ADD, Operands: []interface{}{jit.R0, jit.Imm(5)}},
		{Type: INSN_MOV, Operands: []interface{}{jit.R1, jit.R0}},
	}

	programs, err := translator.TranslateInstructionSequence(instructions)
	if err != nil {
		t.Fatalf("Translation failed: %v", err)
	}

	if len(programs) != len(instructions) {
		t.Errorf("Expected %d programs, got %d", len(instructions), len(programs))
	}

	expectedInstructions := []obj.As{arm64.AMOVD, arm64.AADD, arm64.AMOVD}
	for i, prog := range programs {
		if prog.As != expectedInstructions[i] {
			t.Errorf("Instruction %d: expected %v, got %v", i, expectedInstructions[i], prog.As)
		}
	}
}

func TestInstructionTranslator_OptimizeForARM64(t *testing.T) {
	translator := NewInstructionTranslator()

	instructions := []Instruction{
		{Type: INSN_MOV, Operands: []interface{}{jit.R0, jit.Imm(10)}},
		{Type: INSN_ADD, Operands: []interface{}{jit.R0, jit.Imm(0)}}, // Should be optimized to NOP
		{Type: INSN_SUB, Operands: []interface{}{jit.R1, jit.R1, jit.Imm(0)}}, // Should be optimized to NOP
		{Type: INSN_MOV, Operands: []interface{}{jit.R2, jit.R0}},
	}

	optimized := translator.OptimizeForARM64(instructions)

	// Should have 2 instructions (ADD and SUB optimized to NOP)
	if len(optimized) != 3 {
		t.Errorf("Expected 3 instructions after optimization, got %d", len(optimized))
	}

	// Check that ADD with zero was optimized
	if optimized[1].Type != INSN_NOP {
		t.Errorf("Expected NOP at position 1, got %v", optimized[1].Type)
	}
}

func TestInstructionTranslator_ValidateInstructionSequence(t *testing.T) {
	translator := NewInstructionTranslator()

	tests := []struct {
		name         string
		instructions []Instruction
		shouldError  bool
	}{
		{
			name: "valid sequence",
			instructions: []Instruction{
				{Type: INSN_MOV, Operands: []interface{}{jit.R0, jit.R1}},
				{Type: INSN_ADD, Operands: []interface{}{jit.R0, jit.R2}},
				{Type: INSN_RET},
			},
			shouldError: false,
		},
		{
			name: "invalid MOV - no operands",
			instructions: []Instruction{
				{Type: INSN_MOV, Operands: []interface{}{}},
			},
			shouldError: true,
		},
		{
			name: "invalid CMP - wrong number of operands",
			instructions: []Instruction{
				{Type: INSN_CMP, Operands: []interface{}{jit.R0}},
			},
			shouldError: true,
		},
		{
			name: "invalid JCC - wrong condition type",
			instructions: []Instruction{
				{Type: INSN_JCC, Operands: []interface{}{"invalid", "target"}},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := translator.ValidateInstructionSequence(tt.instructions)
			if tt.shouldError && err == nil {
				t.Error("Expected validation error, but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestConditionMapping(t *testing.T) {
	translator := NewInstructionTranslator()

	tests := []struct {
		amd64Cond   ConditionCode
		expectedARM64 uint8
	}{
		{COND_E, COND_EQ},
		{COND_Z, COND_EQ},
		{COND_NE, COND_NE},
		{COND_NZ, COND_NE},
		{COND_L, COND_LT},
		{COND_GE, COND_GE},
		{COND_LE, COND_LE},
		{COND_G, COND_GT},
		{COND_B, COND_LO},
		{COND_A, COND_HI},
		{COND_BE, COND_LS},
		{COND_AE, COND_HS},
		{COND_S, COND_MI},
		{COND_NS, COND_PL},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.amd64Cond)), func(t *testing.T) {
			arm64Cond, ok := conditionMap[tt.amd64Cond]
			if !ok {
				t.Errorf("Condition %v not found in mapping", tt.amd64Cond)
				return
			}

			if arm64Cond != tt.expectedARM64 {
				t.Errorf("Expected ARM64 condition %v, got %v", tt.expectedARM64, arm64Cond)
			}

			// Test that the translation works
			prog, err := translator.TranslateInstruction(INSN_JCC, tt.amd64Cond, "target")
			if err != nil {
				t.Fatalf("Translation failed: %v", err)
			}

			// Verify the correct conditional branch instruction was generated
			var expectedAs obj.As
			switch arm64Cond {
			case COND_EQ:
				expectedAs = arm64.ABEQ
			case COND_NE:
				expectedAs = arm64.ABNE
			case COND_LT:
				expectedAs = arm64.ABLT
			case COND_GE:
				expectedAs = arm64.ABGE
			case COND_LE:
				expectedAs = arm64.ABLE
			case COND_GT:
				expectedAs = arm64.ABGT
			case COND_HI:
				expectedAs = arm64.ABHI
			case COND_LS:
				expectedAs = arm64.ABLS
			case COND_HS:
				expectedAs = arm64.ABHS
			case COND_LO:
				expectedAs = arm64.ABLO
			case COND_MI:
				expectedAs = arm64.ABMI
			case COND_PL:
				expectedAs = arm64.ABPL
			}

			if prog.As != expectedAs {
				t.Errorf("Expected instruction %v, got %v", expectedAs, prog.As)
			}
		})
	}
}

// Benchmark tests for performance validation
func BenchmarkInstructionTranslator_TranslateMov(b *testing.B) {
	translator := NewInstructionTranslator()
	dst := jit.R0
	src := jit.R1

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := translator.TranslateInstruction(INSN_MOV, dst, src)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkInstructionTranslator_TranslateAdd(b *testing.B) {
	translator := NewInstructionTranslator()
	dst := jit.R0
	src := jit.Imm(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := translator.TranslateInstruction(INSN_ADD, dst, src)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkInstructionTranslator_TranslateJcc(b *testing.B) {
	translator := NewInstructionTranslator()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := translator.TranslateInstruction(INSN_JCC, COND_EQ, "target")
		if err != nil {
			b.Fatal(err)
		}
	}
}