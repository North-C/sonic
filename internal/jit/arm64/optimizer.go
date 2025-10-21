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
	"math"
	"time"

	"github.com/bytedance/sonic/internal/jit"
	"github.com/twitchyliquid64/golang-asm/obj"
	"github.com/twitchyliquid64/golang-asm/obj/arm64"
)

// ARM64Optimizer performs various optimizations on ARM64 JIT code
type ARM64Optimizer struct {
	architecture *Architecture
	options      OptimizationOptions
	stats        OptimizationStats
}

// OptimizationOptions controls which optimizations are applied
type OptimizationOptions struct {
	EnableConstantFolding  bool
	EnableDeadCodeElim     bool
	EnableInstructionSched bool
	EnableRegisterAlloc    bool
	EnableLoopUnroll       bool
	EnableStrengthReduce   bool
	EnablePeephole         bool
	EnableSIMDOptimizations bool
	MaxUnrollFactor       int
	OptimizationLevel      int
}

// OptimizationStats tracks optimization effectiveness
type OptimizationStats struct {
	ConstantFoldsApplied   int
	DeadCodeEliminated     int
	InstructionsScheduled  int
	RegisterReductions     int
	LoopsUnrolled          int
	StrengthReductions     int
	PeepholeOptimizations  int
	SIMDOptimizations      int
	TotalInstructionsIn    int
	TotalInstructionsOut   int
	CompileTimeNs          int64
}

// Default optimization options
func DefaultOptimizationOptions() OptimizationOptions {
	return OptimizationOptions{
		EnableConstantFolding:   true,
		EnableDeadCodeElim:      true,
		EnableInstructionSched:  true,
		EnableRegisterAlloc:     true,
		EnableLoopUnroll:        false, // Conservative default
		EnableStrengthReduce:    true,
		EnablePeephole:          true,
		EnableSIMDOptimizations: true,
		MaxUnrollFactor:         4,
		OptimizationLevel:       2,
	}
}

// NewARM64Optimizer creates a new optimizer instance
func NewARM64Optimizer(arch *Architecture, opts OptimizationOptions) *ARM64Optimizer {
	return &ARM64Optimizer{
		architecture: arch,
		options:      opts,
		stats:        OptimizationStats{},
	}
}

// OptimizeProgram applies all enabled optimizations to a program
func (opt *ARM64Optimizer) OptimizeProgram(prog *obj.Prog) (*obj.Prog, error) {
	startTime := time.Now()

	// Count initial instructions
	opt.stats.TotalInstructionsIn = opt.countInstructions(prog)

	// Apply optimizations in order
	if opt.options.EnableDeadCodeElim {
		prog = opt.eliminateDeadCode(prog)
	}

	if opt.options.EnableConstantFolding {
		prog = opt.foldConstants(prog)
	}

	if opt.options.EnableStrengthReduce {
		prog = opt.reduceStrength(prog)
	}

	if opt.options.EnablePeephole {
		prog = opt.applyPeepholeOptimizations(prog)
	}

	if opt.options.EnableRegisterAlloc {
		prog = opt.optimizeRegisterUsage(prog)
	}

	if opt.options.EnableSIMDOptimizations {
		prog = opt.applySIMDOptimizations(prog)
	}

	if opt.options.EnableInstructionSched {
		prog = opt.scheduleInstructions(prog)
	}

	// Count final instructions
	opt.stats.TotalInstructionsOut = opt.countInstructions(prog)
	opt.stats.CompileTimeNs = time.Since(startTime).Nanoseconds()

	return prog, nil
}

// eliminateDeadCode removes unreachable code and dead instructions
func (opt *ARM64Optimizer) eliminateDeadCode(prog *obj.Prog) *obj.Prog {
	// Build control flow graph
	cfg := opt.buildControlFlowGraph(prog)

	// Mark reachable blocks
	reachable := make(map[*obj.Prog]bool)
	opt.markReachable(prog, reachable, cfg)

	// Remove unreachable instructions
	var newProg *obj.Prog
	var newProgTail *obj.Prog

	for p := prog; p != nil; p = p.Link {
		if reachable[p] && !opt.isDeadInstruction(p) {
			if newProg == nil {
				newProg = p
				newProgTail = p
			} else {
				newProgTail.Link = p
				newProgTail = p
			}
			p.Link = nil // Remove old link
			opt.stats.DeadCodeEliminated++
		}
	}

	if newProg == nil {
		return prog // No changes
	}

	return newProg
}

// foldConstants evaluates constant expressions at compile time
func (opt *ARM64Optimizer) foldConstants(prog *obj.Prog) *obj.Prog {
	constantMap := make(map[int64]int64)

	for p := prog; p != nil; p = p.Link {
		switch p.As {
		case arm64.AMOVD, arm64.AMOVW, arm64.AMOVH, arm64.AMOVB:
			// Track constant loads
			if p.From.Type == obj.TYPE_CONST {
				constantMap[p.To.Reg] = p.From.Offset
			}

		case arm64.AADD, arm64.ASUB, arm64.AMUL, arm64.AAND, arm64.AORR, arm64.AEOR:
			// Try to fold binary operations with constants
			if p.From.Type == obj.TYPE_REG {
				if val1, ok1 := constantMap[p.From.Reg]; ok1 {
					if val2, ok2 := constantMap[p.To.Reg]; ok2 {
						result := opt.evaluateBinaryOp(p.As, val1, val2)
						p.As = arm64.AMOVD
						p.From = obj.Addr{Type: obj.TYPE_CONST, Offset: result}
						constantMap[p.To.Reg] = result
						opt.stats.ConstantFoldsApplied++
					}
				}
			}
		}
	}

	return prog
}

// reduceStrength replaces expensive operations with cheaper equivalents
func (opt *ARM64Optimizer) reduceStrength(prog *obj.Prog) *obj.Prog {
	for p := prog; p != nil; p = p.Link {
		switch p.As {
		case arm64.AMUL:
			// Replace multiplication by power of 2 with shift
			if p.From.Type == obj.TYPE_CONST {
				if p.From.Offset > 0 && (p.From.Offset&(p.From.Offset-1)) == 0 {
					// It's a power of 2
					shift := int64(math.Log2(float64(p.From.Offset)))
					p.As = arm64.ALSL
					p.From = obj.Addr{Type: obj.TYPE_CONST, Offset: shift}
					opt.stats.StrengthReductions++
				}
			}

		case arm64.ADIV:
			// Replace division by power of 2 with shift
			if p.From.Type == obj.TYPE_CONST {
				if p.From.Offset > 0 && (p.From.Offset&(p.From.Offset-1)) == 0 {
					// It's a power of 2
					shift := int64(math.Log2(float64(p.From.Offset)))
					p.As = arm64.AASR
					p.From = obj.Addr{Type: obj.TYPE_CONST, Offset: shift}
					opt.stats.StrengthReductions++
				}
			}

		case arm64.AADD:
			// Replace addition with OR when safe (bitwise operation)
			if p.From.Type == obj.TYPE_CONST {
				if p.From.Offset == 0 {
					// Remove addition of zero
					p.As = arm64.ANOP
					opt.stats.StrengthReductions++
				}
			}
		}
	}

	return prog
}

// applyPeepholeOptimizations performs local instruction-level optimizations
func (opt *ARM64Optimizer) applyPeepholeOptimizations(prog *obj.Prog) *obj.Prog {
	for p := prog; p != nil; p = p.Link {
		if p.Link != nil {
			// Look for MOV X, Y; MOV Y, Z patterns
			if p.As == arm64.AMOVD && p.Link.As == arm64.AMOVD {
				if p.To.Type == obj.TYPE_REG && p.Link.From.Type == obj.TYPE_REG {
					if p.To.Reg == p.Link.From.Reg {
						// Replace with MOV X, Z
						p.Link.From = p.From
						opt.stats.PeepholeOptimizations++
					}
				}
			}

			// Look for ADD X, Y; SUB Y, X patterns
			if p.As == arm64.AADD && p.Link.As == arm64.ASUB {
				if p.To.Reg == p.Link.From.Reg && p.To.Type == obj.TYPE_REG && p.Link.From.Type == obj.TYPE_REG {
					if p.To.Reg == p.Link.To.Reg {
						// These cancel each other out, remove both
						p.As = arm64.ANOP
						p.Link.As = arm64.ANOP
						opt.stats.PeepholeOptimizations++
					}
				}
			}
		}
	}

	return prog
}

// optimizeRegisterUsage reduces register pressure through better allocation
func (opt *ARM64Optimizer) optimizeRegisterUsage(prog *obj.Prog) *obj.Prog {
	// Build live variable analysis
	liveRanges := opt.analyzeLiveRanges(prog)

	// Apply register allocation algorithm
	regMap := make(map[int16]int16)

	for p := prog; p != nil; p = p.Link {
		if p.To.Type == obj.TYPE_REG {
			if newReg, ok := regMap[p.To.Reg]; ok {
				p.To.Reg = newReg
				opt.stats.RegisterReductions++
			} else {
				// Find best register for this variable
				bestReg := opt.findBestRegister(p.To.Reg, liveRanges, regMap)
				if bestReg != p.To.Reg {
					regMap[p.To.Reg] = bestReg
					p.To.Reg = bestReg
					opt.stats.RegisterReductions++
				}
			}
		}

		if p.From.Type == obj.TYPE_REG {
			if newReg, ok := regMap[p.From.Reg]; ok {
				p.From.Reg = newReg
			}
		}
	}

	return prog
}

// applySIMDOptimizations leverages ARM64 NEON instructions for vector operations
func (opt *ARM64Optimizer) applySIMDOptimizations(prog *obj.Prog) *obj.Prog {
	// Look for opportunities to use SIMD instructions
	for p := prog; p != nil; p = p.Link {
		switch p.As {
		case arm64.AMOVD:
			// Check if this could be a vector load/store
			if p.To.Type == obj.TYPE_REG && p.From.Type == obj.TYPE_MEM {
				if opt.isVectorAddress(p.From) {
					// Replace with vector load
					p.As = arm64.ALD1
					opt.stats.SIMDOptimizations++
				}
			} else if p.From.Type == obj.TYPE_REG && p.To.Type == obj.TYPE_MEM {
				if opt.isVectorAddress(p.To) {
					// Replace with vector store
					p.As = arm64.AST1
					opt.stats.SIMDOptimizations++
				}
			}

		case arm64.AADD:
			// Check if this could be a vector add
			if p.From.Type == obj.TYPE_REG && p.To.Type == obj.TYPE_REG {
				if opt.BothRegistersVector(p.From.Reg, p.To.Reg) {
					p.As = arm64.AADD
					opt.stats.SIMDOptimizations++
				}
			}
		}
	}

	return prog
}

// scheduleInstructions reorders instructions for better pipeline utilization
func (opt *ARM64Optimizer) scheduleInstructions(prog *obj.Prog) *obj.Prog {
	// Build dependency graph
	deps := opt.buildDependencyGraph(prog)

	// Simple list scheduling
	scheduled := make([]*obj.Prog, 0)
	ready := make([]*obj.Prog, 0)

	// Find initial ready instructions (no dependencies)
	for p := prog; p != nil; p = p.Link {
		if len(deps[p]) == 0 {
			ready = append(ready, p)
		}
	}

	// Schedule instructions
	for len(ready) > 0 {
		// Pick instruction with highest priority
		selected := opt.selectHighestPriority(ready)

		// Add to scheduled list
		scheduled = append(scheduled, selected)

		// Remove from ready list
		ready = opt.removeFromList(ready, selected)

		// Update dependencies
		for dep := range deps {
			if opt.hasDependency(dep, selected) {
				delete(deps[dep], selected)
				if len(deps[dep]) == 0 {
					ready = append(ready, dep)
				}
			}
		}

		opt.stats.InstructionsScheduled++
	}

	// Rebuild program with scheduled order
	return opt.rebuildProgram(scheduled)
}

// Helper methods

func (opt *ARM64Optimizer) countInstructions(prog *obj.Prog) int {
	count := 0
	for p := prog; p != nil; p = p.Link {
		if p.As != obj.ANOP && p.As != obj.ATEXT {
			count++
		}
	}
	return count
}

func (opt *ARM64Optimizer) isDeadInstruction(p *obj.Prog) bool {
	// Check if instruction has no effect
	switch p.As {
	case arm64.AMOVD:
		// MOV X, X is dead
		return p.From.Type == obj.TYPE_REG && p.To.Type == obj.TYPE_REG && p.From.Reg == p.To.Reg
	case arm64.AADD, arm64.ASUB, arm64.AMUL:
		// ADD X, 0 is dead
		return p.From.Type == obj.TYPE_CONST && p.From.Offset == 0
	}
	return false
}

func (opt *ARM64Optimizer) evaluateBinaryOp(as obj.As, val1, val2 int64) int64 {
	switch as {
	case arm64.AADD:
		return val1 + val2
	case arm64.ASUB:
		return val1 - val2
	case arm64.AMUL:
		return val1 * val2
	case arm64.AAND:
		return val1 & val2
	case arm64.AORR:
		return val1 | val2
	case arm64.AEOR:
		return val1 ^ val2
	default:
		return val1
	}
}

func (opt *ARM64Optimizer) isVectorAddress(addr obj.Addr) bool {
	// Check if address is suitable for vector operations
	return addr.Type == obj.TYPE_MEM && addr.Offset%16 == 0
}

func (opt *ARM64Optimizer) BothRegistersVector(reg1, reg2 int16) bool {
	// Check if both registers are suitable for vector operations
	// This is a simplified check
	return reg1 >= arm64.REG_V0 && reg1 <= arm64.REG_V31 &&
		   reg2 >= arm64.REG_V0 && reg2 <= arm64.REG_V31
}

// GetOptimizationStats returns the optimization statistics
func (opt *ARM64Optimizer) GetOptimizationStats() OptimizationStats {
	return opt.stats
}

// String returns a string representation of optimization results
func (stats OptimizationStats) String() string {
	reduction := float64(stats.TotalInstructionsIn-stats.TotalInstructionsOut) /
				  float64(stats.TotalInstructionsIn) * 100

	return fmt.Sprintf(
		"Optimization Results:\n"+
		"  Instructions: %d -> %d (%.1f%% reduction)\n"+
		"  Constant folds: %d\n"+
		"  Dead code eliminated: %d\n"+
		"  Strength reductions: %d\n"+
		"  Peephole optimizations: %d\n"+
		"  SIMD optimizations: %d\n"+
		"  Register reductions: %d\n"+
		"  Compile time: %v",
		stats.TotalInstructionsIn, stats.TotalInstructionsOut, reduction,
		stats.ConstantFoldsApplied,
		stats.DeadCodeEliminated,
		stats.StrengthReductions,
		stats.PeepholeOptimizations,
		stats.SIMDOptimizations,
		stats.RegisterReductions,
		time.Duration(stats.CompileTimeNs),
	)
}