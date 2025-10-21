# Sonic ARM64 JIT支持实施方案

## 项目概述

本文档详细描述了为Sonic JSON库增加ARM64平台JIT编译支持的完整实施方案。当前Sonic已在AMD64平台上实现了完整的JIT优化，但在ARM64平台上仅支持SIMD优化，缺乏JIT编译能力。

## 现状分析

### 当前架构支持状况

通过分析代码库发现：

1. **JIT支持现状**：
   - ✅ **AMD64**: 完整的JIT编译支持 (`internal/jit/arch_amd64.go`)
   - ❌ **ARM64**: 缺乏JIT编译架构支持

2. **构建标签分析**：
   ```go
   // sonic.go:1-2
   //go:build (amd64 && go1.17 && !go1.26) || (arm64 && go1.20 && !go1.26)
   // +build amd64,go1.17,!go1.26 arm64,go1.20,!go1.26
   ```
   - 项目已支持ARM64平台的构建条件
   - Go版本要求：arm64需要Go 1.20+，amd64需要Go 1.17+

3. **现有ARM64支持**：
   - ✅ SIMD优化：`internal/native/neon/` 目录包含NEON指令集优化
   - ✅ 基础架构：支持arm64平台的编解码器
   - ❌ JIT编译：缺少ARM64架构的JIT编译器

### 现有JIT架构分析

#### AMD64 JIT架构组件

1. **核心架构文件**：
   ```
   internal/jit/
   ├── arch_amd64.go          # AMD64架构定义
   ├── assembler_amd64.go      # AMD64汇编器
   ├── backend.go             # JIT后端通用框架
   └── runtime.go             # JIT运行时支持
   ```

2. **编码器JIT**：
   ```
   internal/encoder/x86/
   ├── assembler_regabi_amd64.go  # AMD64编码器汇编器
   └── ...
   ```

3. **解码器JIT**：
   ```
   internal/decoder/jitdec/
   ├── assembler_regabi_amd64.go  # AMD64解码器汇编器
   └── ...
   ```

## 技术挑战与解决方案

### 1. 指令集差异

**挑战**：ARM64与AMD64在指令集、寄存器架构、调用约定等方面存在显著差异

**解决方案**：
- **寄存器映射**：建立ARM64寄存器到JIT抽象层的映射
- **指令翻译**：实现AMD64指令到ARM64指令的转换
- **调用约定适配**：适配ARM64的函数调用约定

### 2. 内存管理

**挑战**：ARM64的内存对齐、缓存策略与AMD64不同

**解决方案**：
- **内存对齐处理**：确保ARM64平台的内存对齐要求
- **缓存优化**：针对ARM64缓存特性优化代码生成

### 3. 性能优化

**挑战**：ARM64的性能特性与AMD64不同，需要针对性的优化策略

**解决方案**：
- **分支预测优化**：ARM64分支预测特性的利用
- **指令调度优化**：ARM64流水线的优化
- **SIMD指令集成**：与现有NEON优化集成

## 详细实施方案

### 阶段一：基础架构搭建 (2-3周)

#### 1.1 创建ARM64 JIT架构文件

**目标文件**：`internal/jit/arch_arm64.go`

```go
//go:build arm64 && go1.20 && !go1.26
// +build arm64,go1.20,!go1.26

package jit

import (
    "unsafe"

    "github.com/twitchyliquid64/golang-asm/asm/arch"
    "github.com/twitchyliquid64/golang-asm/obj"
)

var (
    _AC = arch.Set("arm64")
)

// ARM64寄存器定义
var (
    // 通用寄存器
    R0 = jit.Reg("R0")
    R1 = jit.Reg("R1")
    R2 = jit.Reg("R2")
    R3 = jit.Reg("R3")
    R4 = jit.Reg("R4")
    R5 = jit.Reg("R5")
    R6 = jit.Reg("R6")
    R7 = jit.Reg("R7")
    R8 = jit.Reg("R8")
    R9 = jit.Reg("R9")
    R10 = jit.Reg("R10")
    R11 = jit.Reg("R11")
    R12 = jit.Reg("R12")
    R13 = jit.Reg("R13")  // SP
    R14 = jit.Reg("R14")  // LR
    R15 = jit.Reg("R15")  // PC

    // 浮点寄存器
    F0 = jit.Reg("F0")
    F1 = jit.Reg("F1")
    F2 = jit.Reg("F2")
    F3 = jit.Reg("F3")
    // ...
)

// ARM64地址模式函数
func Reg(reg string) obj.Addr {
    if ret, ok := _AC.Register[reg]; ok {
        return obj.Addr{Reg: ret, Type: obj.TYPE_REG}
    } else {
        panic("invalid register name: " + reg)
    }
}

func Imm(imm int64) obj.Addr {
    return obj.Addr{
        Type:   obj.TYPE_CONST,
        Offset: imm,
    }
}

func Ptr(reg obj.Addr, offs int64) obj.Addr {
    return obj.Addr{
        Reg:    reg.Reg,
        Type:   obj.TYPE_MEM,
        Offset: offs,
    }
}

// ARM64特有的地址模式
func OffsetReg(reg obj.Addr, offset obj.Addr) obj.Addr {
    return obj.Addr{
        Reg:     reg.Reg,
        Index:   offset.Reg,
        Type:    obj.TYPE_MEM,
        Offset:  0,
    }
}
```

#### 1.2 ARM64汇编器基础框架

**目标文件**：`internal/jit/assembler_arm64.go`

```go
//go:build arm64 && go1.20 && !go1.26
// +build arm64,go1.20,!go1.26

package jit

import (
    "sync"
    "github.com/twitchyliquid64/golang-asm/obj"
    "github.com/twitchyliquid64/golang-asm/obj/arm64"
)

type ARM64Assembler struct {
    BaseAssembler
}

// ARM64指令生成函数
func (self *ARM64Assembler) MOV(dst, src obj.Addr) {
    p := self.pb.New()
    p.As = arm64.AMOVD
    p.From = src
    p.To = dst
    self.pb.Append(p)
}

func (self *ARM64Assembler) ADD(dst, src1, src2 obj.Addr) {
    p := self.pb.New()
    p.As = arm64.AADD
    p.From = src1
    p.Reg = src2.Reg
    p.To = dst
    self.pb.Append(p)
}

func (self *ARM64Assembler) SUB(dst, src1, src2 obj.Addr) {
    p := self.pb.New()
    p.As = arm64.ASUB
    p.From = src1
    p.Reg = src2.Reg
    p.To = dst
    self.pb.Append(p)
}

func (self *ARM64Assembler) CMP(src1, src2 obj.Addr) {
    p := self.pb.New()
    p.As = arm64.ACMP
    p.From = src1
    p.Reg = src2.Reg
    self.pb.Append(p)
}

// 分支指令
func (self *ARM64Assembler) B(target string) {
    self.Sjmp("B", target)
}

func (self *ARM64Assembler) BEQ(target string) {
    self.Sjmp("BEQ", target)
}

func (self *ARM64Assembler) BNE(target string) {
    self.Sjmp("BNE", target)
}

func (self *ARM64Assembler) BLT(target string) {
    self.Sjmp("BLT", target)
}

func (self *ARM64Assembler) BGT(target string) {
    self.Sjmp("BGT", target)
}

// 函数调用
func (self *ARM64Assembler) BL(target obj.Addr) {
    p := self.pb.New()
    p.As = arm64.ABL
    p.To = target
    self.pb.Append(p)
}

// 加载/存储指令
func (self *ARM64Assembler) LDR(dst, src obj.Addr) {
    p := self.pb.New()
    p.As = arm64.AMOVD
    p.From = src
    p.To = dst
    self.pb.Append(p)
}

func (self *ARM64Assembler) STR(dst, src obj.Addr) {
    p := self.pb.New()
    p.As = arm64.AMOVD
    p.From = src
    p.To = dst
    self.pb.Append(p)
}

// 初始化ARM64后端
func (self *BaseAssembler) init() {
    self.pb = newBackend("arm64")
    self.xrefs = map[string][]*obj.Prog{}
    self.labels = map[string]*obj.Prog{}
    self.pendings = map[string][]*obj.Prog{}
}
```

#### 1.3 构建系统集成

**修改构建配置**：

```go
// 在相关文件中添加ARM64 JIT支持
//go:build (amd64 && go1.17 && !go1.26) || (arm64 && go1.20 && !go1.26)

// 在编译器选择逻辑中添加ARM64支持
func newBackend(name string) (ret *Backend) {
    ret = new(Backend)
    ret.Arch = arch.Set(name)
    ret.Ctxt = newLinkContext(ret.Arch.LinkArch)
    ret.Arch.Init(ret.Ctxt)
    return ret
}
```

### 阶段二：编码器JIT实现 (3-4周)

#### 2.1 ARM64编码器汇编器

**目标文件**：`internal/encoder/arm64/assembler_regabi_arm64.go`

```go
//go:build arm64 && go1.20 && !go1.26
// +build arm64,go1.20,!go1.26

package arm64

import (
    "reflect"
    "unsafe"

    "github.com/bytedance/sonic/internal/encoder/ir"
    "github.com/bytedance/sonic/internal/encoder/vars"
    "github.com/bytedance/sonic/internal/jit"
    "github.com/twitchyliquid64/golang-asm/obj"
)

/** ARM64寄存器分配
 *
 *  状态寄存器:
 *      X19 : stack base
 *      X20 : result pointer
 *      X21 : result length
 *      X22 : result capacity
 *      X23 : sp->p
 *      X24 : sp->q
 *      X25 : sp->x
 *      X26 : sp->f
 *
 *  错误寄存器:
 *      X27 : error type register
 *      X28 : error pointer register
 */

/** 函数原型 & 栈映射
 *
 *  func (buf *[]byte, p unsafe.Pointer, sb *_Stack, fv uint64) (err error)
 *
 *  buf    :   (FP)
 *  p      :  8(FP)
 *  sb     : 16(FP)
 *  fv     : 24(FP)
 *  err.vt : 32(FP)
 *  err.vp : 40(FP)
 */

const (
    _FP_args   = 32 // 32 bytes for spill registers of arguments
    _FP_fargs  = 40 // 40 bytes for passing arguments to other Go functions
    _FP_saves  = 64 // 64 bytes for saving the registers before CALL instructions
    _FP_locals = 24 // 24 bytes for local variables
)

const (
    _FP_loffs = _FP_fargs + _FP_saves
    FP_offs  = _FP_loffs + _FP_locals
    _FP_size = FP_offs + 8 // 8 bytes for the parent frame pointer
    _FP_base = _FP_size + 8 // 8 bytes for the return address
)

// ARM64寄存器定义
var (
    _R0 = jit.Reg("R0")
    _R1 = jit.Reg("R1")
    _R2 = jit.Reg("R2")
    _R3 = jit.Reg("R3")
    _R4 = jit.Reg("R4")
    _R5 = jit.Reg("R5")
    _R6 = jit.Reg("R6")
    _R7 = jit.Reg("R7")
    _R8 = jit.Reg("R8")
    _R9 = jit.Reg("R9")
    _R10 = jit.Reg("R10")
    _R11 = jit.Reg("R11")
    _R12 = jit.Reg("R12")
    _R13 = jit.Reg("R13")
    _R14 = jit.Reg("R14")
    _R15 = jit.Reg("R15")
    _R16 = jit.Reg("R16")
    _R17 = jit.Reg("R17")
    _R18 = jit.Reg("R18")
    _R19 = jit.Reg("R19")
    _R20 = jit.Reg("R20")
    _R21 = jit.Reg("R21")
    _R22 = jit.Reg("R22")
    _R23 = jit.Reg("R23")
    _R24 = jit.Reg("R24")
    _R25 = jit.Reg("R25")
    _R26 = jit.Reg("R26")
    _R27 = jit.Reg("R27")
    _R28 = jit.Reg("R28")
    _FP = jit.Reg("R29") // Frame Pointer
    _LR = jit.Reg("R30") // Link Register
    _SP = jit.Reg("R31") // Stack Pointer
)

// 浮点寄存器
var (
    _F0 = jit.Reg("F0")
    _F1 = jit.Reg("F1")
    _F2 = jit.Reg("F2")
    _F3 = jit.Reg("F3")
    _F4 = jit.Reg("F4")
    _F5 = jit.Reg("F5")
    _F6 = jit.Reg("F6")
    _F7 = jit.Reg("F7")
)

// 状态寄存器
var (
    _ST = _R19
    _RP = _R20
    _RL = _R21
    _RC = _R22
)

// 指针和数据寄存器
var (
    _SP_p = _R23
    _SP_q = _R24
    _SP_x = _R25
    _SP_f = _R26
)

// 错误寄存器
var (
    _ET = _R27
    _EP = _R28
)

// 参数寄存器 (ARM64调用约定)
var (
    _ARG_rb = _R0
    _ARG_vp = _R1
    _ARG_sb = _R2
    _ARG_fv = _R3
)

// 返回值寄存器
var (
    _RET_et = _R0
    _RET_ep = _R1
)

// 局部变量
var (
    _VAR_sp = jit.Ptr(_SP, _FP_fargs+_FP_saves)
    _VAR_dn = jit.Ptr(_SP, _FP_fargs+_FP_saves+8)
    _VAR_vp = jit.Ptr(_SP, _FP_fargs+_FP_saves+16)
)

// 寄存器集合
var (
    _REG_ffi = []obj.Addr{_R0, _R1, _R2, _R3}
    _REG_b64 = []obj.Addr{_R4, _R5}
    _REG_all = []obj.Addr{_ST, _SP_x, _SP_f, _SP_p, _SP_q, _RP, _RL, _RC}
    _REG_ms  = []obj.Addr{_ST, _SP_x, _SP_f, _SP_p, _SP_q, _LR}
    _REG_enc = []obj.Addr{_ST, _SP_x, _SP_f, _SP_p, _SP_q, _RL}
)

type Assembler struct {
    Name string
    jit.BaseAssembler
    p    ir.Program
    x    int
}

func NewAssembler(p ir.Program) *Assembler {
    return new(Assembler).Init(p)
}

/** 汇编器接口 **/

func (self *Assembler) Load() vars.Encoder {
    return ptoenc(self.BaseAssembler.Load("encode_"+self.Name, _FP_size, _FP_args, vars.ArgPtrs, vars.LocalPtrs))
}

func (self *Assembler) Init(p ir.Program) *Assembler {
    self.p = p
    self.BaseAssembler.Init(self.compile)
    return self
}

func (self *Assembler) compile() {
    self.prologue()
    self.instrs()
    self.epilogue()
    self.builtins()
}

/** 汇编器阶段 **/

var _OpFuncTab = [256]func(*Assembler, *ir.Instr){
    ir.OP_null:           (*Assembler)._asm_OP_null,
    ir.OP_empty_arr:      (*Assembler)._asm_OP_empty_arr,
    ir.OP_empty_obj:      (*Assembler)._asm_OP_empty_obj,
    ir.OP_bool:           (*Assembler)._asm_OP_bool,
    // ... 其他操作码映射
}

func (self *Assembler) instr(v *ir.Instr) {
    if fn := _OpFuncTab[v.Op()]; fn != nil {
        fn(self, v)
    } else {
        panic(fmt.Sprintf("invalid opcode: %d", v.Op()))
    }
}

func (self *Assembler) instrs() {
    for i, v := range self.p {
        self.Mark(i)
        self.instr(&v)
        self.debug_instr(i, &v)
    }
}

func (self *Assembler) builtins() {
    self.more_space()
    self.error_too_deep()
    self.error_invalid_number()
    self.error_nan_or_infinite()
    self.go_panic()
}

/** ARM64特有的指令实现 **/

func (self *Assembler) prologue() {
    // ARM64函数序言
    self.Emit("STP", _FP, _LR, jit.Ptr(_SP, -16))      // STP FP, LR, [SP, #-16]!
    self.Emit("MOV", _FP, _SP)                          // MOV FP, SP
    self.Emit("SUB", _SP, _SP, jit.Imm(_FP_size))      // SUB SP, SP, #_FP_size

    // 保存参数
    self.Emit("MOV", _RP, _ARG_rb)                      // MOV RP, R0 (buf)
    self.Emit("MOV", _RL, jit.Ptr(_RP, 8))              // MOV RL, [RP, #8] (buf.len)
    self.Emit("MOV", _RC, jit.Ptr(_RP, 16))             // MOV RC, [RP, #16] (buf.cap)
    self.Emit("MOV", _SP_p, _ARG_vp)                    // MOV SP.p, R1 (vp)
    self.Emit("MOV", _ST, _ARG_sb)                      // MOV ST, R2 (sb)
    self.Emit("MOV", _SP_x, jit.Imm(0))                 // MOV SP.x, #0
    self.Emit("MOV", _SP_f, jit.Imm(0))                 // MOV SP.f, #0
    self.Emit("MOV", _SP_q, jit.Imm(0))                 // MOV SP.q, #0
}

func (self *Assembler) epilogue() {
    self.Mark(len(self.p))
    self.Emit("MOV", _RET_et, jit.Imm(0))               // MOV X0, #0 (error type)
    self.Emit("MOV", _RET_ep, jit.Imm(0))               // MOV X1, #0 (error pointer)
    self.Link("_error")

    // 恢复缓冲区
    self.Emit("MOV", _R2, _ARG_rb)                      // MOV R2, R0 (buf)
    self.Emit("MOV", jit.Ptr(_R2, 0), _RP)              // MOV [R2, #0], RP (buf.data)
    self.Emit("MOV", jit.Ptr(_R2, 8), _RL)              // MOV [R2, #8], RL (buf.len)

    // ARM64函数尾声
    self.Emit("MOV", _SP, _FP)                          // MOV SP, FP
    self.Emit("LDP", _FP, _LR, jit.Ptr(_SP, 16))       // LDP FP, LR, [SP], #16
    self.Emit("RET")                                    // RET
}

// 操作码实现示例
func (self *Assembler) _asm_OP_null(_ *ir.Instr) {
    self.check_size(4)
    self.Emit("MOV", _R2, jit.Imm(0x6c6c756e))         // MOV R2, #0x6c6c756e ('null')
    self.Emit("STR", _R2, jit.Ptr(_RP, _RL))            // STR R2, [RP, RL]
    self.Emit("ADD", _RL, _RL, jit.Imm(4))              // ADD RL, RL, #4
}

func (self *Assembler) _asm_OP_bool(_ *ir.Instr) {
    self.Emit("CMP", jit.Ptr(_SP_p, 0), jit.Imm(0))     // CMP [SP.p, #0], #0
    self.Emit("B.EQ", "_false_{n}")                     // B.EQ _false_{n}

    self.check_size(4)
    self.Emit("MOV", _R2, jit.Imm(0x65757274))         // MOV R2, #0x65757274 ('true')
    self.Emit("STR", _R2, jit.Ptr(_RP, _RL))            // STR R2, [RP, RL]
    self.Emit("ADD", _RL, _RL, jit.Imm(4))              // ADD RL, RL, #4
    self.Emit("B", "_end_{n}")                          // B _end_{n}

    self.Link("_false_{n}")
    self.check_size(5)
    self.Emit("MOV", _R2, jit.Imm(0x616c7366))         // MOV R2, #0x616c7366 ('fals')
    self.Emit("STR", _R2, jit.Ptr(_RP, _RL))            // STR R2, [RP, RL]
    self.Emit("MOV", _R3, jit.Imm(0x65))                // MOV R3, #0x65 ('e')
    self.Emit("STRB", _R3, jit.Ptr(_RP, _RL, 4))        // STRB R3, [RP, RL, #4]
    self.Emit("ADD", _RL, _RL, jit.Imm(5))              // ADD RL, RL, #5

    self.Link("_end_{n}")
}

// 内建函数实现
func (self *Assembler) more_space() {
    self.Link("_more_space")
    // 调用Go运行时的growslice函数
    self.save_callee_saved()
    self.Emit("MOV", _R0, _RP)                          // MOV R0, RP (buf.data)
    self.Emit("MOV", _R1, _RL)                          // MOV R1, RL (buf.len)
    self.Emit("MOV", _R2, _RC)                          // MOV R2, RC (buf.cap)
    self.Emit("MOV", _R3, jit.Imm(1))                   // MOV R3, #1 (new len)
    self.call_go(_F_growslice)                           // BL growslice
    self.Emit("MOV", _RP, _R0)                          // MOV RP, R0 (new data)
    self.Emit("MOV", _RL, _R1)                          // MOV RL, R1 (new len)
    self.Emit("MOV", _RC, _R2)                          // MOV RC, R2 (new cap)
    self.load_callee_saved()
    self.Emit("B", _LR)                                  // B LR (return)
}

/** ARM64辅助函数 **/

func (self *Assembler) save_callee_saved() {
    // 保存被调用者保存的寄存器
    self.Emit("STP", _R19, _R20, jit.Ptr(_SP, -16))     // STP R19, R20, [SP, #-16]!
    self.Emit("STP", _R21, _R22, jit.Ptr(_SP, -16))     // STP R21, R22, [SP, #-16]!
    self.Emit("STP", _R23, _R24, jit.Ptr(_SP, -16))     // STP R23, R24, [SP, #-16]!
    self.Emit("STP", _R25, _R26, jit.Ptr(_SP, -16))     // STP R25, R26, [SP, #-16]!
}

func (self *Assembler) load_callee_saved() {
    // 恢复被调用者保存的寄存器
    self.Emit("LDP", _R25, _R26, jit.Ptr(_SP, 16))      // LDP R25, R26, [SP], #16
    self.Emit("LDP", _R23, _R24, jit.Ptr(_SP, 16))      // LDP R23, R24, [SP], #16
    self.Emit("LDP", _R21, _R22, jit.Ptr(_SP, 16))      // LDP R21, R22, [SP], #16
    self.Emit("LDP", _R19, _R20, jit.Ptr(_SP, 16))      // LDP R19, R20, [SP], #16
}

func (self *Assembler) check_size(n int) {
    self.check_size_rl(jit.Ptr(_RL, int64(n)))
}

func (self *Assembler) check_size_rl(v obj.Addr) {
    self.Emit("ADD", _R2, _RP, v)                       // ADD R2, RP, v
    self.Emit("CMP", _R2, _RC)                          // CMP R2, RC
    self.Emit("B.LE", "_size_ok_{n}")                    // B.LE _size_ok_{n}
    self.call_more_space("_size_ok_{n}")                // 调用扩容函数
    self.Link("_size_ok_{n}")
}
```

#### 2.2 指令集适配层

**创建指令翻译器**：`internal/jit/arm64/instruction_translator.go`

```go
package arm64

import (
    "github.com/twitchyliquid64/golang-asm/obj"
    "github.com/twitchyliquid64/golang-asm/obj/arm64"
)

// AMD64到ARM64指令翻译器
type InstructionTranslator struct{}

func NewInstructionTranslator() *InstructionTranslator {
    return &InstructionTranslator{}
}

// 翻译数据传输指令
func (t *InstructionTranslator) TranslateMov(dst, src obj.Addr) *obj.Prog {
    p := &obj.Prog{}
    p.As = arm64.AMOVD
    p.From = src
    p.To = dst
    return p
}

// 翻译算术运算指令
func (t *InstructionTranslator) TranslateAdd(dst, src1, src2 obj.Addr) *obj.Prog {
    p := &obj.Prog{}
    p.As = arm64.AADD
    p.From = src1
    p.Reg = src2.Reg
    p.To = dst
    return p
}

func (t *InstructionTranslator) TranslateSub(dst, src1, src2 obj.Addr) *obj.Prog {
    p := &obj.Prog{}
    p.As = arm64.ASUB
    p.From = src1
    p.Reg = src2.Reg
    p.To = dst
    return p
}

func (t *InstructionTranslator) TranslateMul(dst, src1, src2 obj.Addr) *obj.Prog {
    p := &obj.Prog{}
    p.As = arm64.AMUL
    p.From = src1
    p.Reg = src2.Reg
    p.To = dst
    return p
}

// 翻译比较指令
func (t *InstructionTranslator) TranslateCmp(src1, src2 obj.Addr) *obj.Prog {
    p := &obj.Prog{}
    p.As = arm64.ACMP
    p.From = src1
    p.Reg = src2.Reg
    return p
}

// 翻译分支指令
func (t *InstructionTranslator) TranslateBranch(condition string, target string) *obj.Prog {
    p := &obj.Prog{}
    switch condition {
    case "EQ":
        p.As = arm64.ABEQ
    case "NE":
        p.As = arm64.ABNE
    case "LT":
        p.As = arm64.ABLT
    case "GT":
        p.As = arm64.ABGT
    case "LE":
        p.As = arm64.ABLE
    case "GE":
        p.As = arm64.ABGE
    default:
        p.As = arm64.AB
    }
    p.To.Type = obj.TYPE_BRANCH
    return p
}

// 翻译函数调用指令
func (t *InstructionTranslator) TranslateCall(target obj.Addr) *obj.Prog {
    p := &obj.Prog{}
    p.As = arm64.ABL
    p.To = target
    return p
}

// 翻译加载/存储指令
func (t *InstructionTranslator) TranslateLoad(dst, src obj.Addr) *obj.Prog {
    p := &obj.Prog{}
    p.As = arm64.AMOVD
    p.From = src
    p.To = dst
    return p
}

func (t *InstructionTranslator) TranslateStore(dst, src obj.Addr) *obj.Prog {
    p := &obj.Prog{}
    p.As = arm64.AMOVD
    p.From = src
    p.To = dst
    return p
}
```

### 阶段三：解码器JIT实现 (3-4周)

#### 3.1 ARM64解码器汇编器

**目标文件**：`internal/decoder/jitdec/arm64/assembler_regabi_arm64.go`

```go
//go:build arm64 && go1.20 && !go1.26
// +build arm64,go1.20,!go1.26

package arm64

import (
    "encoding/json"
    "fmt"
    "reflect"
    "unsafe"

    "github.com/bytedance/sonic/internal/caching"
    "github.com/bytedance/sonic/internal/jit"
    "github.com/bytedance/sonic/internal/native"
    "github.com/bytedance/sonic/internal/native/types"
    "github.com/bytedance/sonic/internal/rt"
    "github.com/twitchyliquid64/golang-asm/obj"
)

/** ARM64寄存器分配
 *
 *  状态寄存器:
 *      X19 : stack base
 *      X20 : input pointer
 *      X21 : input length
 *      X22 : input cursor
 *      X23 : value pointer
 *
 *  错误寄存器:
 *      X24 : error type register
 *      X25 : error pointer register
 */

/** 函数原型 & 栈映射
 *
 *  func (s string, ic int, vp unsafe.Pointer, sb *_Stack, fv uint64, sv string) (rc int, err error)
 *
 *  s.buf  :   (FP)
 *  s.len  :  8(FP)
 *  ic     : 16(FP)
 *  vp     : 24(FP)
 *  sb     : 32(FP)
 *  fv     : 40(FP)
 *  sv     : 56(FP)
 *  err.vt : 72(FP)
 *  err.vp : 80(FP)
 */

const (
    _FP_args   = 72     // 72 bytes to pass and spill register arguments
    _FP_fargs  = 80     // 80 bytes for passing arguments to other Go functions
    _FP_saves  = 48     // 48 bytes for saving the registers before CALL instructions
    _FP_locals = 144    // 144 bytes for local variables
)

const (
    _FP_offs = _FP_fargs + _FP_saves + _FP_locals
    _FP_size = _FP_offs + 8     // 8 bytes for the parent frame pointer
    _FP_base = _FP_size + 8     // 8 bytes for the return address
)

// ARM64寄存器定义（与编码器共享部分定义）
var (
    // 通用寄存器
    _R0  = jit.Reg("R0")
    _R1  = jit.Reg("R1")
    _R2  = jit.Reg("R2")
    _R3  = jit.Reg("R3")
    _R4  = jit.Reg("R4")
    _R5  = jit.Reg("R5")
    _R6  = jit.Reg("R6")
    _R7  = jit.Reg("R7")
    _R8  = jit.Reg("R8")
    _R9  = jit.Reg("R9")
    _R10 = jit.Reg("R10")
    _R11 = jit.Reg("R11")
    _R12 = jit.Reg("R12")
    _R13 = jit.Reg("R13")
    _R14 = jit.Reg("R14")
    _R15 = jit.Reg("R15")
    _R16 = jit.Reg("R16")
    _R17 = jit.Reg("R17")
    _R18 = jit.Reg("R18")
    _R19 = jit.Reg("R19")
    _R20 = jit.Reg("R20")
    _R21 = jit.Reg("R21")
    _R22 = jit.Reg("R22")
    _R23 = jit.Reg("R23")
    _R24 = jit.Reg("R24")
    _R25 = jit.Reg("R25")
    _R26 = jit.Reg("R26")
    _R27 = jit.Reg("R27")
    _R28 = jit.Reg("R28")
    _FP = jit.Reg("R29") // Frame Pointer
    _LR = jit.Reg("R30") // Link Register
    _SP = jit.Reg("R31") // Stack Pointer
)

// 状态寄存器
var (
    _ST = _R19
    _IP = _R20
    _IL = _R21
    _IC = _R22
    _VP = _R23
)

// 错误寄存器
var (
    _ET = _R24
    _EP = _R25
)

// 参数寄存器（ARM64调用约定）
var (
    _ARG_s  = _R0  // string data pointer
    _ARG_sl = _R1  // string length
    _ARG_ic = _R2  // input cursor
    _ARG_vp = _R3  // value pointer
    _ARG_sb = _R4  // stack base
    _ARG_fv = _R5  // flags
)

// 局部变量
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

type _Assembler struct {
    jit.BaseAssembler
    p    _Program
    name string
}

func newAssembler(p _Program) *_Assembler {
    return new(_Assembler).Init(p)
}

/** 汇编器接口 **/

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
    self.type_error()
    self.mismatch_error()
    self.field_error()
    self.range_error()
    self.stack_error()
    self.parsing_error()
}

/** 汇编器阶段 **/

var _OpFuncTab = [256]func(*_Assembler, *_Instr){
    _OP_any:              (*_Assembler)._asm_OP_any,
    _OP_dyn:              (*_Assembler)._asm_OP_dyn,
    _OP_str:              (*_Assembler)._asm_OP_str,
    _OP_bin:              (*_Assembler)._asm_OP_bin,
    _OP_bool:             (*_Assembler)._asm_OP_bool,
    _OP_num:              (*_Assembler)._asm_OP_num,
    // ... 其他操作码映射
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

/** ARM64序言和尾声 **/

func (self *_Assembler) prologue() {
    // ARM64函数序言
    self.Emit("STP", _FP, _LR, jit.Ptr(_SP, -16))      // STP FP, LR, [SP, #-16]!
    self.Emit("MOV", _FP, _SP)                          // MOV FP, SP
    self.Emit("SUB", _SP, _SP, jit.Imm(_FP_size))      // SUB SP, SP, #_FP_size

    // 保存参数到寄存器
    self.Emit("MOV", _IP, _ARG_s)                       // MOV IP, R0 (string data)
    self.Emit("MOV", _IL, _ARG_sl)                      // MOV IL, R1 (string length)
    self.Emit("MOV", _IC, _ARG_ic)                      // MOV IC, R2 (input cursor)
    self.Emit("MOV", _VP, _ARG_vp)                      // MOV VP, R3 (value pointer)
    self.Emit("MOV", _ST, _ARG_sb)                      // MOV ST, R4 (stack base)

    // 初始化数字缓冲区
    self.Emit("MOV", _R6, jit.Imm(_MaxDigitNums))        // MOV R6, #_MaxDigitNums
    self.Emit("MOV", _VAR_st_Dc, _R6)                   // MOV [VAR_st_Dc], R6
    self.Emit("ADD", _R7, _ST, jit.Imm(_DbufOffset))    // ADD R7, ST, #_DbufOffset
    self.Emit("MOV", _VAR_st_Db, _R7)                   // MOV [VAR_st_Db], R7
}

func (self *_Assembler) epilogue() {
    self.Mark(len(self.p))
    self.Emit("MOV", _RET_et, jit.Imm(0))               // MOV X0, #0 (error type)
    self.Emit("MOV", _RET_ep, jit.Imm(0))               // MOV X1, #0 (error pointer)
    self.Link("_error")

    // ARM64函数尾声
    self.Emit("MOV", _SP, _FP)                          // MOV SP, FP
    self.Emit("LDP", _FP, _LR, jit.Ptr(_SP, 16))       // LDP FP, LR, [SP], #16
    self.Emit("RET")                                    // RET
}

/** 解码器操作码实现示例 **/

func (self *_Assembler) _asm_OP_str(_ *_Instr) {
    self.parse_string()                                     // PARSE STRING
    self.unquote_once(jit.Ptr(_VP, 0), jit.Ptr(_VP, 8), false, true)
}

func (self *_Assembler) _asm_OP_bool(_ *_Instr) {
    self.Emit("ADD", _R6, _IC, jit.Imm(4))               // ADD R6, IC, #4
    self.Emit("CMP", _R6, _IL)                           // CMP R6, IL
    self.Emit("B.GT", "_eof_error")                      // B.GT _eof_error

    self.Emit("LDRB", _R6, jit.Ptr(_IP, _IC))           // LDRB R6, [IP, IC]
    self.Emit("CMP", _R6, jit.Imm('f'))                  // CMP R6, #'f'
    self.Emit("B.EQ", "_false_{n}")                      // B.EQ _false_{n}

    // 检查 "true"
    self.Emit("MOV", _R6, jit.Imm(0x65757274))           // MOV R6, #0x65757274 ("true")
    self.Emit("LDR", _R7, jit.Ptr(_IP, _IC))            // LDR R7, [IP, IC]
    self.Emit("CMP", _R6, _R7)                           // CMP R6, R7
    self.Emit("B.NE", "_skip_{n}")                       // B.NE _skip

    self.Emit("MOV", _IC, _R6)                           // MOV IC, R6 (end of "true")
    self.Emit("MOV", _R6, jit.Imm(1))                    // MOV R6, #1
    self.Emit("STRB", _R6, jit.Ptr(_VP, 0))             // STRB R6, [VP, #0]
    self.Emit("B", "_end_{n}")                           // B _end

    self.Link("_false_{n}")
    self.Emit("ADD", _IC, _IC, jit.Imm(1))               // ADD IC, IC, #1
    self.Emit("CMP", _IC, _IL)                           // CMP IC, IL
    self.Emit("B.GT", "_eof_error")                      // B.GT _eof_error

    // 检查 "alse"
    self.Emit("MOV", _R6, jit.Imm(0x616c7365))           // MOV R6, #0x616c7365 ("alse")
    self.Emit("LDR", _R7, jit.Ptr(_IP, _IC))            // LDR R7, [IP, IC]
    self.Emit("CMP", _R6, _R7)                           // CMP R6, R7
    self.Emit("B.NE", "_im_error")                       // B.NE _im_error

    self.Emit("ADD", _IC, _IC, jit.Imm(4))               // ADD IC, IC, #4
    self.Emit("MOV", _R6, jit.Imm(0))                    // MOV R6, #0
    self.Emit("STRB", _R6, jit.Ptr(_VP, 0))             // STRB R6, [VP, #0]
    self.Emit("B", "_end_{n}")                           // B _end

    self.Link("_skip_{n}")
    self.Emit("MOV", _IC, _VAR_ic)                       // MOV IC, [VAR_ic]
    self.Emit("MOV", _R6, jit.Type(_T_bool))             // MOV R6, type(bool)
    self.Emit("MOV", _VAR_et, _R6)                       // MOV [VAR_et], R6
    self.call_skip_one("_end_{n}")                       // 调用skip_one

    self.Link("_end_{n}")
}

func (self *_Assembler) _asm_OP_num(_ *_Instr) {
    self.Emit("MOV", _VAR_fl, jit.Imm(0))               // MOV [VAR_fl], #0
    self.Emit("LDRB", _R6, jit.Ptr(_IP, _IC))           // LDRB R6, [IP, IC]
    self.Emit("CMP", _R6, jit.Imm('"'))                  // CMP R6, #'"'
    self.Emit("MOV", _VAR_ic, _IC)                       // MOV [VAR_ic], IC
    self.Emit("B.NE", "_skip_number_{n}")                // B.NE _skip_number
    self.Emit("MOV", _VAR_fl, jit.Imm(1))               // MOV [VAR_fl], #1
    self.Emit("ADD", _IC, _IC, jit.Imm(1))               // ADD IC, IC, #1

    self.Link("_skip_number_{n}")

    // 调用native解析函数
    self.call_vf(_F_vnumber)                            // 调用vnumber
    self.check_err(nil, "", -1)
}

/** ARM64辅助函数 **/

func (self *_Assembler) parse_string() {
    self.Emit("MOV", _R0, _ARG_fv)                       // MOV R0, [ARG_fv] (flags)
    self.call_vf(_F_vstring)                             // 调用vstring
    self.check_err(nil, "", -1)
}

func (self *_Assembler) check_err(vt reflect.Type, pin string, pin2 int) {
    self.Emit("LDR", _R6, _VAR_st_Vt)                   // LDR R6, [VAR_st_Vt]
    self.Emit("CMP", _R6, jit.Imm(0))                    // CMP R6, #0

    if vt != nil {
        self.Emit("B.LT", "_check_err_{n}")               // B.LT _check_err
        self.Emit("MOV", _R6, jit.Type(vt))              // MOV R6, type(vt)
        self.Emit("MOV", _VAR_et, _R6)                   // MOV [VAR_et], R6

        if pin2 != -1 {
            self.Emit("SUB", _R6, _VAR_ic, jit.Imm(1))   // SUB R6, [VAR_ic], #1
            self.Emit("MOV", _VAR_ic, _R6)               // MOV [VAR_ic], R6
            self.call_skip_key_value(pin2)               // 调用skip_key_value
        } else {
            self.Emit("MOV", _VAR_ic, _R6)               // MOV [VAR_ic], R6
            self.call_skip_one(pin)                      // 调用skip_one
        }
        self.Link("_check_err_{n}")
    } else {
        self.Emit("B.LT", "_parsing_error_v")            // B.LT _parsing_error_v
    }
}

func (self *_Assembler) check_eof(d int64) {
    if d == 1 {
        self.Emit("CMP", _IC, _IL)                       // CMP IC, IL
        self.Emit("B.GE", "_eof_error")                  // B.GE _eof_error
    } else {
        self.Emit("ADD", _R6, _IC, jit.Imm(d))           // ADD R6, IC, #d
        self.Emit("CMP", _R6, _IL)                       // CMP R6, IL
        self.Emit("B.GT", "_eof_error")                  // B.GT _eof_error
    }
}

/** 内存管理函数 **/

func (self *_Assembler) malloc_AX(nb obj.Addr, ret obj.Addr) {
    self.Emit("MOV", _R0, nb)                            // MOV R0, nb
    self.Emit("MOV", _R1, jit.Type(byteType))           // MOV R1, type(byte)
    self.Emit("MOV", _R2, jit.Imm(0))                    // MOV R2, #0
    self.call_go(_F_mallocgc)                            // BL mallocgc
    self.Emit("MOV", ret, _R0)                           // MOV ret, R0
}

func (self *_Assembler) valloc(vt reflect.Type, ret obj.Addr) {
    self.Emit("MOV", _R0, jit.Imm(int64(vt.Size())))    // MOV R0, #vt.Size()
    self.Emit("MOV", _R1, jit.Type(vt))                 // MOV R1, type(vt)
    self.Emit("MOV", _R2, jit.Imm(1))                    // MOV R2, #1
    self.call_go(_F_mallocgc)                            // BL mallocgc
    self.Emit("MOV", ret, _R0)                           // MOV ret, R0
}

/** 函数调用约定适配 **/

func (self *_Assembler) call(fn obj.Addr) {
    self.Emit("MOV", _LR, fn)                            // MOV LR, fn
    self.Emit("BLR", _LR)                                // BLR LR
}

func (self *_Assembler) call_go(fn obj.Addr) {
    self.save_callee_saved()
    self.call(fn)
    self.load_callee_saved()
}

func (self *_Assembler) call_c(fn obj.Addr) {
    // C函数调用约定处理
    self.Emit("MOV", _R8, _IC)                           // 保存IC到临时寄存器
    self.call(fn)
    self.Emit("MOV", _IC, _R8)                           // 恢复IC
}

func (self *_Assembler) save_callee_saved() {
    // 保存ARM64被调用者保存的寄存器
    self.Emit("STP", _R19, _R20, jit.Ptr(_SP, -16))     // STP R19, R20, [SP, #-16]!
    self.Emit("STP", _R21, _R22, jit.Ptr(_SP, -16))     // STP R21, R22, [SP, #-16]!
    self.Emit("STP", _R23, _R24, jit.Ptr(_SP, -16))     // STP R23, R24, [SP, #-16]!
    self.Emit("STP", _R25, _R26, jit.Ptr(_SP, -16))     // STP R25, R26, [SP, #-16]!
}

func (self *_Assembler) load_callee_saved() {
    // 恢复ARM64被调用者保存的寄存器
    self.Emit("LDP", _R25, _R26, jit.Ptr(_SP, 16))      // LDP R25, R26, [SP], #16
    self.Emit("LDP", _R23, _R24, jit.Ptr(_SP, 16))      // LDP R23, R24, [SP], #16
    self.Emit("LDP", _R21, _R22, jit.Ptr(_SP, 16))      // LDP R21, R22, [SP], #16
    self.Emit("LDP", _R19, _R20, jit.Ptr(_SP, 16))      // LDP R19, R20, [SP], #16
}
```

### 阶段四：测试与优化 (2-3周)

#### 4.1 单元测试

**创建测试文件**：`internal/jit/arm64/arm64_test.go`

```go
//go:build arm64 && go1.20 && !go1.26
// +build arm64,go1.20,!go1.26

package arm64

import (
    "testing"
    "reflect"
    "unsafe"

    "github.com/bytedance/sonic/internal/jit"
    "github.com/bytedance/sonic/internal/rt"
)

func TestARM64RegisterAllocation(t *testing.T) {
    // 测试寄存器分配
    tests := []struct {
        name     string
        register string
        expected bool
    }{
        {"R0", "R0", true},
        {"R30", "R30", true},
        {"Invalid", "R99", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            defer func() {
                if r := recover(); r != nil && tt.expected {
                    t.Errorf("Expected register %s to be valid, but got panic: %v", tt.register, r)
                }
            }()

            reg := jit.Reg(tt.register)
            if tt.expected && reg.Reg == 0 {
                t.Errorf("Expected valid register for %s", tt.register)
            }
        })
    }
}

func TestARM64InstructionGeneration(t *testing.T) {
    // 测试指令生成
    assembler := &ARM64Assembler{}
    assembler.Init()

    // 测试MOV指令
    assembler.MOV(_R0, _R1)

    // 测试ADD指令
    assembler.ADD(_R2, _R0, _R1)

    // 测试CMP指令
    assembler.CMP(_R0, _R1)

    // 验证生成的指令数量
    if assembler.Size() == 0 {
        t.Error("Expected instructions to be generated")
    }
}

func TestARM64AssemblerPrologueEpilogue(t *testing.T) {
    assembler := &ARM64Assembler{}
    assembler.Init()

    assembler.prologue()
    assembler.epilogue()

    // 验证生成了序言和尾声
    size := assembler.Size()
    if size < 20 { // 基本的序言+尾声应该至少有这些指令
        t.Errorf("Expected prologue and epilogue to generate at least 20 bytes, got %d", size)
    }
}

func TestARM64MemoryOperations(t *testing.T) {
    assembler := &ARM64Assembler{}
    assembler.Init()

    // 测试内存操作
    src := jit.Ptr(_R0, 8)
    dst := _R1

    assembler.LDR(dst, src)
    assembler.STR(src, dst)

    size := assembler.Size()
    if size < 8 { // 至少应该有两条指令
        t.Errorf("Expected memory operations to generate at least 8 bytes, got %d", size)
    }
}

func TestARM64BranchInstructions(t *testing.T) {
    assembler := &ARM64Assembler{}
    assembler.Init()

    // 测试分支指令
    assembler.Link("test_label")
    assembler.B("test_label")
    assembler.BEQ("test_label")
    assembler.BNE("test_label")

    size := assembler.Size()
    if size < 12 { // 至少应该有三条分支指令
        t.Errorf("Expected branch instructions to generate at least 12 bytes, got %d", size)
    }
}

func TestARM64FunctionCall(t *testing.T) {
    assembler := &ARM64Assembler{}
    assembler.Init()

    // 测试函数调用
    target := jit.Func(unsafe.Pointer(&testFunction))
    assembler.BL(target)

    size := assembler.Size()
    if size < 4 { // 至少应该有一条BL指令
        t.Errorf("Expected function call to generate at least 4 bytes, got %d", size)
    }
}

func testFunction() {
    // 测试函数
}
```

#### 4.2 集成测试

**创建集成测试**：`arm64_jit_integration_test.go`

```go
//go:build arm64 && go1.20 && !go1.26
// +build arm64,go1.20,!go1.26

package sonic

import (
    "testing"
    "encoding/json"
    "reflect"
)

func TestARM64JITEncoding(t *testing.T) {
    // 测试ARM64 JIT编码
    tests := []struct {
        name     string
        input    interface{}
        expected string
    }{
        {"null", nil, "null"},
        {"bool_true", true, "true"},
        {"bool_false", false, "false"},
        {"int", 42, "42"},
        {"string", "hello", `"hello"`},
        {"array", []int{1, 2, 3}, "[1,2,3]"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 使用ARM64 JIT编码
            result, err := ConfigDefault.MarshalToString(tt.input)
            if err != nil {
                t.Fatalf("Marshal failed: %v", err)
            }

            if result != tt.expected {
                t.Errorf("Expected %s, got %s", tt.expected, result)
            }
        })
    }
}

func TestARM64JITDecoding(t *testing.T) {
    // 测试ARM64 JIT解码
    tests := []struct {
        name     string
        input    string
        target   interface{}
        expected interface{}
    }{
        {"null", "null", &struct{}{}, nil},
        {"bool_true", "true", &bool{}, new(bool)},
        {"bool_false", "false", &bool{}, new(bool)},
        {"int", "42", &int{}, new(int)},
        {"string", `"hello"`, &string{}, new(string)},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            target := reflect.New(reflect.TypeOf(tt.target).Elem()).Interface()

            err := ConfigDefault.UnmarshalFromString(tt.input, target)
            if err != nil {
                t.Fatalf("Unmarshal failed: %v", err)
            }

            // 验证解码结果
            // 这里需要根据具体的target类型进行验证
        })
    }
}

func TestARM64JITPerformance(t *testing.T) {
    // 性能测试
    if testing.Short() {
        t.Skip("Skipping performance test in short mode")
    }

    // 测试数据
    data := struct {
        Name  string `json:"name"`
        Age   int    `json:"age"`
        Email string `json:"email"`
    }{
        Name:  "John Doe",
        Age:   30,
        Email: "john@example.com",
    }

    // 预热JIT编译器
    for i := 0; i < 1000; i++ {
        _, err := ConfigDefault.Marshal(&data)
        if err != nil {
            t.Fatalf("Warmup marshal failed: %v", err)
        }
    }

    // 基准测试
    b := testing.B{}
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        _, err := ConfigDefault.Marshal(&data)
        if err != nil {
            t.Fatalf("Marshal failed: %v", err)
        }
    }
}

func TestARM64JITPretouch(t *testing.T) {
    // 测试Pretouch功能
    type TestStruct struct {
        Field1 string `json:"field1"`
        Field2 int    `json:"field2"`
        Field3 bool   `json:"field3"`
    }

    // 预编译类型
    err := Pretouch(reflect.TypeOf(TestStruct{}))
    if err != nil {
        t.Fatalf("Pretouch failed: %v", err)
    }

    // 验证预编译后的性能
    data := TestStruct{
        Field1: "test",
        Field2: 42,
        Field3: true,
    }

    start := testing.AllocsPerRun(1)
    result, err := ConfigDefault.Marshal(&data)
    if err != nil {
        t.Fatalf("Marshal failed: %v", err)
    }

    // 验证结果
    var decoded TestStruct
    err = ConfigDefault.Unmarshal(result, &decoded)
    if err != nil {
        t.Fatalf("Unmarshal failed: %v", err)
    }

    if decoded != data {
        t.Error("Decoded data doesn't match original")
    }
}
```

#### 4.3 性能基准测试

**创建基准测试**：`arm64_bench_test.go`

```go
//go:build arm64 && go1.20 && !go1.26
// +build arm64,go1.20,!go1.26

package sonic

import (
    "testing"
    "encoding/json"
)

func BenchmarkARM64JITMarshal(b *testing.B) {
    data := generateLargeTestData()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := ConfigDefault.Marshal(data)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkARM64JITUnmarshal(b *testing.B) {
    jsonStr := `{"name":"John Doe","age":30,"email":"john@example.com","active":true,"scores":[95,87,92]}`

    var result struct {
        Name   string  `json:"name"`
        Age    int     `json:"age"`
        Email  string  `json:"email"`
        Active bool    `json:"active"`
        Scores []int   `json:"scores"`
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        err := ConfigDefault.UnmarshalFromString(jsonStr, &result)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkARM64JITVsStdLib(b *testing.B) {
    data := generateLargeTestData()

    b.Run("Sonic_ARM64_JIT", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            _, err := ConfigDefault.Marshal(data)
            if err != nil {
                b.Fatal(err)
            }
        }
    })

    b.Run("StdLib", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            _, err := json.Marshal(data)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}

func generateLargeTestData() interface{} {
    // 生成大型测试数据
    type LargeStruct struct {
        ID       int                    `json:"id"`
        Name     string                 `json:"name"`
        Active   bool                   `json:"active"`
        Metadata map[string]interface{} `json:"metadata"`
        Tags     []string               `json:"tags"`
        Nested   struct {
            Field1 string `json:"field1"`
            Field2 int    `json:"field2"`
        } `json:"nested"`
    }

    return LargeStruct{
        ID:     12345,
        Name:   "Large Test Data",
        Active: true,
        Metadata: map[string]interface{}{
            "created": "2023-01-01T00:00:00Z",
            "updated": "2023-12-31T23:59:59Z",
            "version": 1.0,
        },
        Tags: []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
        Nested: struct {
            Field1 string `json:"field1"`
            Field2 int    `json:"field2"`
        }{
            Field1: "nested_value",
            Field2: 999,
        },
    }
}
```

### 阶段五：集成与部署 (1-2周)

#### 5.1 构建系统更新

**修改构建配置**：

```go
// 更新构建标签以支持ARM64 JIT
//go:build (amd64 && go1.17 && !go1.26) || (arm64 && go1.20 && !go1.26)
// +build amd64,go1.17,!go1.26 arm64,go1.20,!go1.26

// 在sonic.go中添加ARM64 JIT支持检测
const (
    hasJITSupport = true // ARM64从Go 1.20开始支持JIT
)
```

#### 5.2 CI/CD流水线更新

**GitHub Actions配置**：

```yaml
# .github/workflows/arm64-jit.yml
name: ARM64 JIT Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test-arm64-jit:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.20

    - name: Install QEMU
      run: |
        sudo apt-get update
        sudo apt-get install -y qemu-user-static

    - name: Run ARM64 tests
      run: |
        docker run --rm -v $(pwd):/workspace -w /workspace \
          arm64v8/golang:1.20 \
          go test -v ./...
```

## 实施时间线

| 阶段 | 时间 | 主要任务 | 交付物 |
|------|------|----------|--------|
| 阶段一 | 2-3周 | 基础架构搭建 | ARM64 JIT框架、架构定义、汇编器基础 |
| 阶段二 | 3-4周 | 编码器JIT实现 | ARM64编码器汇编器、指令翻译器 |
| 阶段三 | 3-4周 | 解码器JIT实现 | ARM64解码器汇编器、操作码实现 |
| 阶段四 | 2-3周 | 测试与优化 | 单元测试、集成测试、性能优化 |
| 阶段五 | 1-2周 | 集成与部署 | 构建系统更新、CI/CD集成 |
| **总计** | **11-16周** | **完整ARM64 JIT支持** | **生产就绪的ARM64 JIT实现** |

## 技术风险与缓解策略

### 1. 性能风险

**风险**：ARM64 JIT性能可能不如AMD64版本

**缓解策略**：
- 在开发过程中持续进行性能基准测试
- 针对ARM64特性进行专门优化
- 与现有的NEON SIMD优化协同工作

### 2. 兼容性风险

**风险**：ARM64 JIT可能与现有代码不兼容

**缓解策略**：
- 保持API完全兼容
- 提供降级到非JIT实现的选项
- 全面的回归测试

### 3. 维护风险

**风险**：增加ARM64 JIT会提高代码维护复杂度

**缓解策略**：
- 保持良好的代码结构和文档
- 共享尽可能多的代码逻辑
- 自动化测试和验证流程

## 成功标准

### 功能标准
- ✅ ARM64平台支持JIT编译
- ✅ 所有现有API保持兼容
- ✅ 支持完整的JSON编解码功能

### 性能标准
- 🎯 ARM64 JIT性能 >= 当前ARM64 SIMD性能的150%
- 🎯 ARM64 JIT性能达到AMD64 JIT性能的80%以上
- 🎯 内存使用与现有实现相当

### 质量标准
- ✅ 95%以上的测试覆盖率
- ✅ 通过所有现有测试用例
- ✅ 无内存泄漏和安全问题

## 后续优化方向

1. **性能优化**：
   - ARM64特有的指令级优化
   - 缓存友好的数据布局
   - 分支预测优化

2. **功能扩展**：
   - 支持更多ARM64特性（如SVE）
   - 与其他优化技术集成
   - 支持更多JSON特性

3. **工具支持**：
   - JIT调试工具
   - 性能分析工具
   - 自动化优化建议

## 结论

本实施方案为Sonic JSON库增加ARM64平台JIT编译支持提供了详细的技术路线图。通过分阶段实施，我们可以在保证代码质量的前提下，为ARM64平台带来显著的性能提升。实施完成后，Sonic将在ARM64平台上具备与AMD64平台相当的JIT优化能力，为用户提供跨平台的高性能JSON处理能力。