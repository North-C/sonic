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

	"github.com/twitchyliquid64/golang-asm/obj"
	"github.com/twitchyliquid64/golang-asm/obj/arm64"
)

// InstructionTranslator handles translation from AMD64-style instructions to ARM64
// This bridges the gap between the existing JIT instruction set and ARM64 architecture
type InstructionTranslator struct{}

// NewInstructionTranslator creates a new ARM64 instruction translator
func NewInstructionTranslator() *InstructionTranslator {
	return &InstructionTranslator{}
}

// Instruction types for translation
type InstructionType int

const (
	INSN_MOV InstructionType = iota
	INSN_ADD
	INSN_SUB
	INSN_MUL
	INSN_DIV
	INSN_AND
	INSN_OR
	INSN_XOR
	INSN_CMP
	INSN_TEST
	INSN_JMP
	INSN_JCC
	INSN_CALL
	INSN_RET
	INSN_PUSH
	INSN_POP
	INSN_LEA
)

// Condition codes for conditional jumps
type ConditionCode int

const (
	COND_O   ConditionCode = iota // Overflow
	COND_NO                       // No overflow
	COND_B  ConditionCode = 2     // Below/Carry
	COND_NAE                      // Not above or equal
	COND_C                        // Carry
	COND_NC                       // No carry
	COND_NA                       // Not above
	COND_BE                       // Below or equal
	COND_A                        // Above
	COND_E                        // Equal
	COND_Z                        // Zero
	COND_NE                       // Not equal
	COND_NZ                       // Not zero
	COND_NA                       // Not above
	COND_BE                       // Below or equal
	COND_A                        // Above
	COND_S                        // Sign
	COND_NS                       // Not sign
	COND_P                        // Parity
	COND_NP                       // Not parity
	COND_PE                       // Parity even
	COND_PO                       // Parity odd
	COND_L                        // Less
	COND_GE                       // Greater or equal
	COND_NL                       // Not less
	COND_NGE                      // Not greater or equal
	COND_LE                       // Less or equal
	COND_G                        // Greater
	COND_NLE                      // Not less or equal
	COND_NG                       // Not greater
)

// AMD64 to ARM64 condition code mapping
var conditionMap = map[ConditionCode]uint8{
	COND_E:  COND_EQ, // Equal -> Equal
	COND_Z:  COND_EQ, // Zero -> Equal
	COND_NE: COND_NE, // Not equal -> Not equal
	COND_NZ: COND_NE, // Not zero -> Not equal
	COND_L:  COND_LT, // Less (signed) -> Less than
	COND_GE: COND_GE, // Greater or equal -> Greater or equal
	COND_LE: COND_LE, // Less or equal -> Less or equal
	COND_G:  COND_GT, // Greater -> Greater than
	COND_B:  COND_LO, // Below (unsigned) -> Lower
	COND_A:  COND_HI, // Above (unsigned) -> Higher
	COND_BE: COND_LS, // Below or equal -> Lower or same
	COND_AE: COND_HS, // Above or equal -> Higher or same
	COND_S:  COND_MI, // Sign -> Minus
	COND_NS: COND_PL, // Not sign -> Plus
}

// TranslateInstruction translates a generic instruction to ARM64
func (t *InstructionTranslator) TranslateInstruction(insnType InstructionType, operands ...interface{}) (*obj.Prog, error) {
	p := &obj.Prog{}

	switch insnType {
	case INSN_MOV:
		return t.translateMov(operands...)
	case INSN_ADD:
		return t.translateAdd(operands...)
	case INSN_SUB:
		return t.translateSub(operands...)
	case INSN_MUL:
		return t.translateMul(operands...)
	case INSN_DIV:
		return t.translateDiv(operands...)
	case INSN_AND:
		return t.translateAnd(operands...)
	case INSN_OR:
		return t.translateOr(operands...)
	case INSN_XOR:
		return t.translateXor(operands...)
	case INSN_CMP:
		return t.translateCmp(operands...)
	case INSN_TEST:
		return t.translateTest(operands...)
	case INSN_JMP:
		return t.translateJmp(operands...)
	case INSN_JCC:
		return t.translateJcc(operands...)
	case INSN_CALL:
		return t.translateCall(operands...)
	case INSN_RET:
		return t.translateRet(operands...)
	case INSN_PUSH:
		return t.translatePush(operands...)
	case INSN_POP:
		return t.translatePop(operands...)
	case INSN_LEA:
		return t.translateLea(operands...)
	default:
		return nil, fmt.Errorf("unsupported instruction type: %v", insnType)
	}

	return p, nil
}

// translateMov translates MOV instructions
func (t *InstructionTranslator) translateMov(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("MOV requires at least 2 operands")
	}

	dst := operands[0].(obj.Addr)
	src := operands[1].(obj.Addr)

	p := &obj.Prog{}

	// Determine the appropriate MOV instruction based on operand types
	if src.Type == obj.TYPE_CONST {
		// Immediate to register
		if dst.Reg != 0 {
			p.As = arm64.AMOVD
			p.From = src
			p.To = dst
		}
	} else if src.Type == obj.TYPE_MEM && dst.Type == obj.TYPE_REG {
		// Memory to register - use load instruction
		p.As = arm64.AMOVD
		p.From = src
		p.To = dst
	} else if src.Type == obj.TYPE_REG && dst.Type == obj.TYPE_MEM {
		// Register to memory - use store instruction
		p.As = arm64.AMOVD
		p.From = src
		p.To = dst
	} else if src.Type == obj.TYPE_REG && dst.Type == obj.TYPE_REG {
		// Register to register
		p.As = arm64.AMOVD
		p.From = src
		p.To = dst
	} else {
		return nil, fmt.Errorf("unsupported MOV operand types: src=%v, dst=%v", src.Type, dst.Type)
	}

	return p, nil
}

// translateAdd translates ADD instructions
func (t *InstructionTranslator) translateAdd(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("ADD requires at least 2 operands")
	}

	p := &obj.Prog{}

	if len(operands) == 2 {
		// ADD dst, src (dst = dst + src)
		dst := operands[0].(obj.Addr)
		src := operands[1].(obj.Addr)

		if src.Type == obj.TYPE_CONST {
			// ADD immediate
			p.As = arm64.AADD
			p.From = src
			p.To = dst
		} else {
			// ADD register
			p.As = arm64.AADD
			p.From = src
			p.To = dst
		}
	} else if len(operands) == 3 {
		// ADD dst, src1, src2 (dst = src1 + src2)
		dst := operands[0].(obj.Addr)
		src1 := operands[1].(obj.Addr)
		src2 := operands[2].(obj.Addr)

		p.As = arm64.AADD
		p.From = src1
		p.Reg = src2.Reg
		p.To = dst
	} else {
		return nil, fmt.Errorf("ADD supports at most 3 operands")
	}

	return p, nil
}

// translateSub translates SUB instructions
func (t *InstructionTranslator) translateSub(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("SUB requires at least 2 operands")
	}

	p := &obj.Prog{}

	if len(operands) == 2 {
		// SUB dst, src (dst = dst - src)
		dst := operands[0].(obj.Addr)
		src := operands[1].(obj.Addr)

		if src.Type == obj.TYPE_CONST {
			// SUB immediate
			p.As = arm64.ASUB
			p.From = src
			p.To = dst
		} else {
			// SUB register
			p.As = arm64.ASUB
			p.From = src
			p.To = dst
		}
	} else if len(operands) == 3 {
		// SUB dst, src1, src2 (dst = src1 - src2)
		dst := operands[0].(obj.Addr)
		src1 := operands[1].(obj.Addr)
		src2 := operands[2].(obj.Addr)

		p.As = arm64.ASUB
		p.From = src1
		p.Reg = src2.Reg
		p.To = dst
	} else {
		return nil, fmt.Errorf("SUB supports at most 3 operands")
	}

	return p, nil
}

// translateMul translates MUL instructions
func (t *InstructionTranslator) translateMul(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("MUL requires at least 2 operands")
	}

	p := &obj.Prog{}

	if len(operands) == 2 {
		// MUL dst, src (dst = dst * src)
		dst := operands[0].(obj.Addr)
		src := operands[1].(obj.Addr)

		p.As = arm64.AMUL
		p.From = dst // ARM64 MUL uses dst as both source and destination
		p.Reg = src.Reg
		p.To = dst
	} else if len(operands) == 3 {
		// MUL dst, src1, src2 (dst = src1 * src2)
		dst := operands[0].(obj.Addr)
		src1 := operands[1].(obj.Addr)
		src2 := operands[2].(obj.Addr)

		p.As = arm64.AMUL
		p.From = src1
		p.Reg = src2.Reg
		p.To = dst
	} else {
		return nil, fmt.Errorf("MUL supports at most 3 operands")
	}

	return p, nil
}

// translateDiv translates DIV instructions
func (t *InstructionTranslator) translateDiv(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("DIV requires at least 2 operands")
	}

	dst := operands[0].(obj.Addr)
	src := operands[1].(obj.Addr)

	p := &obj.Prog{}

	// ARM64 has separate instructions for signed and unsigned division
	// Default to signed division
	p.As = arm64.ASDIV
	p.From = dst
	p.Reg = src.Reg
	p.To = dst

	return p, nil
}

// translateAnd translates AND instructions
func (t *InstructionTranslator) translateAnd(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("AND requires at least 2 operands")
	}

	p := &obj.Prog{}

	if len(operands) == 2 {
		dst := operands[0].(obj.Addr)
		src := operands[1].(obj.Addr)

		p.As = arm64.AAND
		p.From = src
		p.To = dst
	} else if len(operands) == 3 {
		dst := operands[0].(obj.Addr)
		src1 := operands[1].(obj.Addr)
		src2 := operands[2].(obj.Addr)

		p.As = arm64.AAND
		p.From = src1
		p.Reg = src2.Reg
		p.To = dst
	}

	return p, nil
}

// translateOr translates OR instructions
func (t *InstructionTranslator) translateOr(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("OR requires at least 2 operands")
	}

	p := &obj.Prog{}

	if len(operands) == 2 {
		dst := operands[0].(obj.Addr)
		src := operands[1].(obj.Addr)

		p.As = arm64.AORR
		p.From = src
		p.To = dst
	} else if len(operands) == 3 {
		dst := operands[0].(obj.Addr)
		src1 := operands[1].(obj.Addr)
		src2 := operands[2].(obj.Addr)

		p.As = arm64.AORR
		p.From = src1
		p.Reg = src2.Reg
		p.To = dst
	}

	return p, nil
}

// translateXor translates XOR instructions
func (t *InstructionTranslator) translateXor(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("XOR requires at least 2 operands")
	}

	p := &obj.Prog{}

	if len(operands) == 2 {
		dst := operands[0].(obj.Addr)
		src := operands[1].(obj.Addr)

		p.As = arm64.AEOR
		p.From = src
		p.To = dst
	} else if len(operands) == 3 {
		dst := operands[0].(obj.Addr)
		src1 := operands[1].(obj.Addr)
		src2 := operands[2].(obj.Addr)

		p.As = arm64.AEOR
		p.From = src1
		p.Reg = src2.Reg
		p.To = dst
	}

	return p, nil
}

// translateCmp translates CMP instructions
func (t *InstructionTranslator) translateCmp(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("CMP requires 2 operands")
	}

	src1 := operands[0].(obj.Addr)
	src2 := operands[1].(obj.Addr)

	p := &obj.Prog{}
	p.As = arm64.ACMP
	p.From = src1
	p.Reg = src2.Reg
	// CMP doesn't have a destination

	return p, nil
}

// translateTest translates TEST instructions
func (t *InstructionTranslator) translateTest(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("TEST requires 2 operands")
	}

	src1 := operands[0].(obj.Addr)
	src2 := operands[1].(obj.Addr)

	p := &obj.Prog{}
	p.As = arm64.ATST
	p.From = src1
	p.Reg = src2.Reg
	// TST doesn't have a destination

	return p, nil
}

// translateJmp translates unconditional JMP instructions
func (t *InstructionTranslator) translateJmp(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 1 {
		return nil, fmt.Errorf("JMP requires 1 operand")
	}

	target := operands[0]

	p := &obj.Prog{}
	p.As = arm64.AB

	switch v := target.(type) {
	case obj.Addr:
		p.To = v
	case string:
		p.To = obj.Addr{
			Type: obj.TYPE_BRANCH,
			Name: v,
		}
	default:
		return nil, fmt.Errorf("unsupported JMP target type: %T", target)
	}

	return p, nil
}

// translateJcc translates conditional jump instructions
func (t *InstructionTranslator) translateJcc(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("JCC requires 2 operands")
	}

	condition := operands[0].(ConditionCode)
	target := operands[1]

	p := &obj.Prog{}

	// Map AMD64 condition to ARM64 condition
	arm64Cond, ok := conditionMap[condition]
	if !ok {
		return nil, fmt.Errorf("unsupported condition code: %v", condition)
	}

	// Select appropriate ARM64 conditional branch instruction
	switch arm64Cond {
	case COND_EQ:
		p.As = arm64.ABEQ
	case COND_NE:
		p.As = arm64.ABNE
	case COND_LT:
		p.As = arm64.ABLT
	case COND_GE:
		p.As = arm64.ABGE
	case COND_LE:
		p.As = arm64.ABLE
	case COND_GT:
		p.As = arm64.ABGT
	case COND_HI:
		p.As = arm64.ABHI
	case COND_LS:
		p.As = arm64.ABLS
	case COND_HS:
		p.As = arm64.ABHS
	case COND_LO:
		p.As = arm64.ABLO
	case COND_MI:
		p.As = arm64.ABMI
	case COND_PL:
		p.As = arm64.ABPL
	default:
		return nil, fmt.Errorf("unsupported ARM64 condition: %v", arm64Cond)
	}

	switch v := target.(type) {
	case obj.Addr:
		p.To = v
	case string:
		p.To = obj.Addr{
			Type: obj.TYPE_BRANCH,
			Name: v,
		}
	default:
		return nil, fmt.Errorf("unsupported JCC target type: %T", target)
	}

	return p, nil
}

// translateCall translates CALL instructions
func (t *InstructionTranslator) translateCall(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 1 {
		return nil, fmt.Errorf("CALL requires 1 operand")
	}

	target := operands[0]

	p := &obj.Prog{}
	p.As = arm64.ABL

	switch v := target.(type) {
	case obj.Addr:
		p.To = v
	case string:
		p.To = obj.Addr{
			Type: obj.TYPE_BRANCH,
			Name: v,
		}
	default:
		return nil, fmt.Errorf("unsupported CALL target type: %T", target)
	}

	return p, nil
}

// translateRet translates RET instructions
func (t *InstructionTranslator) translateRet(operands ...interface{}) (*obj.Prog, error) {
	p := &obj.Prog{}
	p.As = arm64.ARET

	// ARM64 RET can optionally specify return register (usually LR)
	if len(operands) > 0 {
		if reg, ok := operands[0].(obj.Addr); ok {
			p.To = reg
		}
	}

	return p, nil
}

// translatePush translates PUSH instructions
func (t *InstructionTranslator) translatePush(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 1 {
		return nil, fmt.Errorf("PUSH requires 1 operand")
	}

	src := operands[0].(obj.Addr)

	p := &obj.Prog{}

	// ARM64 doesn't have PUSH instruction, use STR with pre-decrement
	p.As = arm64.AMOVD
	p.From = src
	p.To = obj.Addr{
		Type:   obj.TYPE_MEM,
		Reg:    SP.Reg,
		 Offset: -16, // Pre-decrement by 16 (stack alignment)
	}

	return p, nil
}

// translatePop translates POP instructions
func (t *InstructionTranslator) translatePop(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 1 {
		return nil, fmt.Errorf("POP requires 1 operand")
	}

	dst := operands[0].(obj.Addr)

	p := &obj.Prog{}

	// ARM64 doesn't have POP instruction, use LDR with post-increment
	p.As = arm64.AMOVD
	p.From = obj.Addr{
		Type:   obj.TYPE_MEM,
		Reg:    SP.Reg,
		Offset: 16, // Post-increment by 16
	}
	p.To = dst

	return p, nil
}

// translateLea translates LEA (Load Effective Address) instructions
func (t *InstructionTranslator) translateLea(operands ...interface{}) (*obj.Prog, error) {
	if len(operands) < 2 {
		return nil, fmt.Errorf("LEA requires 2 operands")
	}

	dst := operands[0].(obj.Addr)
	src := operands[1].(obj.Addr)

	p := &obj.Prog{}

	// ARM64 doesn't have LEA, but we can simulate it with ADD
	if src.Type == obj.TYPE_MEM {
		// LEA dst, [base + offset] -> ADD dst, base, #offset
		p.As = arm64.AADD
		p.From = obj.Addr{
			Type:   obj.TYPE_REG,
			Reg:    src.Reg,
		}
		p.To = dst

		// Add offset if present
		if src.Offset != 0 {
			p.From.Type = obj.TYPE_CONST
			p.From.Offset = src.Offset
		}

		// Handle index register if present
		if src.Index != 0 {
			// Complex addressing mode, need multiple instructions
			// This is a simplified version - real implementation would be more complex
			return nil, fmt.Errorf("complex LEA addressing not fully implemented")
		}
	} else {
		return nil, fmt.Errorf("LEA requires memory operand")
	}

	return p, nil
}

// TranslateInstructionSequence translates a sequence of instructions
func (t *InstructionTranslator) TranslateInstructionSequence(instructions []Instruction) ([]*obj.Prog, error) {
	var programs []*obj.Prog

	for _, insn := range instructions {
		prog, err := t.TranslateInstruction(insn.Type, insn.Operands...)
		if err != nil {
			return nil, fmt.Errorf("error translating instruction %v: %w", insn, err)
		}
		programs = append(programs, prog)
	}

	return programs, nil
}

// Instruction represents a generic instruction for translation
type Instruction struct {
	Type     InstructionType
	Operands []interface{}
}

// OptimizeForARM64 performs ARM64-specific optimizations on the instruction sequence
func (t *InstructionTranslator) OptimizeForARM64(instructions []Instruction) []Instruction {
	var optimized []Instruction

	for i, insn := range instructions {
		// Skip redundant MOV instructions
		if i > 0 && insn.Type == INSN_MOV {
			prev := instructions[i-1]
			if prev.Type == INSN_MOV && insn.Operands[0] == prev.Operands[1] && insn.Operands[1] == prev.Operands[0] {
				// Redundant MOV, skip it
				continue
			}
		}

		// Optimize common patterns
		optimized = append(optimized, t.optimizeInstruction(insn))
	}

	return optimized
}

// optimizeInstruction performs optimization on a single instruction
func (t *InstructionTranslator) optimizeInstruction(insn Instruction) Instruction {
	switch insn.Type {
	case INSN_ADD:
		// Convert ADD dst, dst, #0 to NOP
		if len(insn.Operands) == 3 {
			if imm, ok := insn.Operands[2].(obj.Addr); ok && imm.Type == obj.TYPE_CONST && imm.Offset == 0 {
				return Instruction{Type: INSN_NOP}
			}
		}
	case INSN_SUB:
		// Convert SUB dst, dst, #0 to NOP
		if len(insn.Operands) == 3 {
			if imm, ok := insn.Operands[2].(obj.Addr); ok && imm.Type == obj.TYPE_CONST && imm.Offset == 0 {
				return Instruction{Type: INSN_NOP}
			}
		}
	}

	return insn
}

// INSN_NOP represents a no-operation instruction
const INSN_NOP InstructionType = 999

// ValidateInstructionSequence validates that an instruction sequence is correct for ARM64
func (t *InstructionTranslator) ValidateInstructionSequence(instructions []Instruction) error {
	for i, insn := range instructions {
		err := t.validateInstruction(insn, i)
		if err != nil {
			return fmt.Errorf("instruction %d validation failed: %w", i, err)
		}
	}
	return nil
}

// validateInstruction validates a single instruction
func (t *InstructionTranslator) validateInstruction(insn Instruction, index int) error {
	switch insn.Type {
	case INSN_MOV, INSN_ADD, INSN_SUB, INSN_MUL, INSN_DIV, INSN_AND, INSN_OR, INSN_XOR:
		if len(insn.Operands) < 2 {
			return fmt.Errorf("instruction requires at least 2 operands")
		}
	case INSN_CMP, INSN_TEST:
		if len(insn.Operands) != 2 {
			return fmt.Errorf("instruction requires exactly 2 operands")
		}
	case INSN_JMP, INSN_CALL:
		if len(insn.Operands) != 1 {
			return fmt.Errorf("instruction requires exactly 1 operand")
		}
	case INSN_JCC:
		if len(insn.Operands) != 2 {
			return fmt.Errorf("instruction requires exactly 2 operands")
		}
		// Validate condition code
		if _, ok := insn.Operands[0].(ConditionCode); !ok {
			return fmt.Errorf("first operand must be a condition code")
		}
	case INSN_RET, INSN_NOP:
		// These instructions don't require operands
	default:
		return fmt.Errorf("unsupported instruction type: %v", insn.Type)
	}

	return nil
}