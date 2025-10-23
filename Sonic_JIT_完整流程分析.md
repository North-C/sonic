# Sonic JIT 完整流程分析

## 1. 完整编码流程图

### 1.1 高层流程图

```
用户调用 JSON 序列化
         │
         ▼
    Encode(val, opts)
         │
         ▼
    encodeInto(buf, val, opts)
         │
         ▼
    encodeTypedPointer(buf, vt, vp, stk, fv)
         │
         ▼
    vars.FindOrCompile(vt, pv, compiler)
         │
         ▼
    [缓存检查] ──→ 已编译程序 ──→ 执行JIT代码
         │                    │
         ▼                    ▼
    [未编译]                直接调用
         │
         ▼
    compiler.Compile(vt, ...)
         │
         ▼
    [类型分析] ──→ [IR生成] ──→ [JIT编译] ──→ [代码加载]
         │              │           │            │
         ▼              ▼           ▼            ▼
    反射类型信息    中间表示指令   汇编指令    可执行机器码
         │              │           │            │
         └──────────────┴───────────┴────────────┘
                              │
                              ▼
                        执行JIT编码
```

### 1.2 详细函数调用流程

```mermaid
graph TD
    A[Encode(val, opts)] --> B[EncodeInto]
    B --> C[encodeIntoCheckRace]
    C --> D[encodeInto]
    D --> E[vars.NewStack]
    E --> F[rt.UnpackEface]
    F --> G[encodeTypedPointer]
    G --> H[vars.FindOrCompile]
    H --> I{缓存中存在?}
    I -->|是| J[直接执行编码器]
    I -->|否| K[调用compiler.Compile]
    K --> L[NewCompiler]
    L --> M[compiler.Compile]
    M --> N[类型分析]
    N --> O[IR程序生成]
    O --> P[Assembler.Load]
    P --> Q[BaseAssembler.Load]
    Q --> R[loader.LoadOne]
    R --> S[JIT机器码生成]
    S --> T[返回loader.Function]
    T --> U[ptoenc函数转换]
    U --> V[vars.Encoder函数]
    V --> W[执行编码]
    W --> X[结果处理]
    X --> Y[encodeFinishWithPool]
    Y --> Z[返回JSON字节切片]
```

## 2. JSON 序列化和反序列化详细步骤

### 2.1 JSON 序列化步骤

#### 2.1.1 用户接口层
```go
// 1. 用户调用入口
func Encode(val interface{}, opts Options) ([]byte, error)

// 2. 核心编码函数
func encodeTypedPointer(buf *[]byte, vt *rt.GoType, vp *unsafe.Pointer, sb *vars.Stack, fv uint64) error
```

#### 2.1.2 类型分析层
```go
// 类型反射和解析
efv := rt.UnpackEface(val)
vt := efv.Type
vp := &efv.Value
```

#### 2.1.3 IR（中间表示）生成层
```go
// IR操作码定义
const (
    OP_null      // 编码null值
    OP_bool      // 编码布尔值
    OP_i8        // 编码8位整数
    OP_i16       // 编码16位整数
    OP_i32       // 编码32位整数
    OP_i64       // 编码64位整数
    OP_u8        // 编码8位无符号整数
    OP_u16       // 编码16位无符号整数
    OP_u32       // 编码32位无符号整数
    OP_u64       // 编码64位无符号整数
    OP_f32       // 编码32位浮点数
    OP_f64       // 编码64位浮点数
    OP_str       // 编码字符串
    OP_empty_arr // 编码空数组
    OP_empty_obj // 编码空对象
    OP_recurse   // 递归编码
    // ... 更多操作码
)
```

#### 2.1.4 JIT编译层
```go
// ARM64 JIT编译流程
func (self *Assembler) Load() vars.Encoder {
    return ptoenc(self.BaseAssembler.Load("encode_"+self.Name, _FP_size, _FP_args, vars.ArgPtrs, vars.LocalPtrs))
}

// JIT机器码生成
func (self *BaseAssembler) Load(name string, framesize int, argsize int, argptrs, localptrs []int64) loader.Function {
    self.Execute()
    argStackmap := make([]bool, len(argptrs))
    localStackmap := make([]bool, len(localptrs))
    return arm64JitLoader.LoadOne(self.c, name, framesize, argsize, argStackmap, localStackmap)
}
```

### 2.2 反序列化步骤

#### 2.2.1 解析器层次结构
```go
// 1. 词法分析器 - 将JSON字节流转换为token
// 2. 语法分析器 - 将token序列转换为AST
// 3. 语义分析器 - 将AST转换为Go对象
```

#### 2.2.2 优化手段
- **零拷贝解析**: 直接操作输入字节切片，避免不必要的数据复制
- **预编译解析器**: 针对特定类型预编译解析逻辑
- **流式解析**: 支持大文件的流式处理

## 3. x86 样本实现架构

### 3.1 x86 JIT 编码器架构

#### 3.1.1 寄存器分配（x86）
```go
/** Register Allocations
 *
 *  State Registers:
 *      %rbx : stack base
 *      %rdi : result pointer
 *      %rsi : result length
 *      %rdx : result capacity
 *      %r12 : sp->p
 *      %r13 : sp->q
 *      %r14 : sp->x
 *      %r15 : sp->f
 *
 *  Error Registers:
 *      %r10 : error type register
 *      %r11 : error pointer register
 */
```

#### 3.1.2 栈帧布局（x86）
```go
/** Function Prototype & Stack Map
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
```

#### 3.1.3 JIT编译流程（x86）
```go
// 1. IR生成
func (self *Compiler) compile(vt reflect.Type, pv bool) (ir.Program, error)

// 2. 汇编代码生成
func (self *Assembler) Load() vars.Encoder {
    return ptoenc(self.BaseAssembler.Load("encode_"+self.Name, _FP_size, _FP_args, vars.ArgPtrs, vars.LocalPtrs))
}

// 3. 机器码生成和加载
func (self *BaseAssembler) Load(name string, frameSize int, argSize int, argStackmap []bool, localStackmap []bool) loader.Function {
    self.build()
    return jitLoader.LoadOne(self.c, name, frameSize, argSize, argStackmap, localStackmap)
}
```

### 3.2 ARM64 对比实现

#### 3.2.1 寄存器分配（ARM64）
```go
/** ARM64 Register Allocations
 *
 *  State Registers:
 *      X19 : stack base
 *      X20 : result pointer
 *      X21 : result length
 *      X22 : result capacity
 *      X23 : sp->p
 *      X24 : sp->q
 *      X25 : sp->x
 *      X26 : sp->f
 *
 *  Error Registers:
 *      X27 : error type register
 *      X28 : error pointer register
 */
```

#### 3.2.2 架构差异对比

| 特性 | x86 | ARM64 |
|------|-----|-------|
| 调用约定 | System V AMD64 ABI | ARM64 Procedure Call Standard |
| 寄存器数量 | 16个通用寄存器 | 31个通用寄存器 |
| 栈对齐 | 16字节 | 16字节 |
| 参数传递 | 前6个参数通过寄存器(RDI, RSI, RDX, RCX, R8, R9) | 前8个参数通过寄存器(X0-X7) |
| 返回值 | RAX, RDX | X0, X1 |

### 3.3 核心优化技术

#### 3.3.1 编译时优化
1. **类型特化**: 为每种Go类型生成特化的编码函数
2. **内联优化**: 简单函数直接内联，避免函数调用开销
3. **死代码消除**: 移除不会执行的代码路径

#### 3.3.2 运行时优化
1. **预编译(Pretouch)**: 启动时预编译常用类型，避免运行时编译
2. **缓存机制**: 编译后的JIT代码缓存，避免重复编译
3. **内存池**: 复用字节缓冲区和栈对象，减少GC压力

#### 3.3.3 算法优化
1. **SIMD指令**: 使用向量化指令加速数字编码
2. **分支预测优化**: 减少条件分支，提高指令流水线效率
3. **内存访问优化**: 优化内存访问模式，提高缓存命中率

#### 3.3.4 特殊优化
```go
// 数字编码优化 - 使用查表法
var _DIGITS = [100]byte{
    '0', '0', '0', '1', '0', '2', '0', '3', '0', '4', '0', '5', '0', '6', '0', '7',
    '0', '8', '0', '9', '1', '0', '1', '1', '1', '2', '1', '3', '1', '4', '1', '5',
    // ... 更多数字对
}

// 字符串编码优化 - 批量拷贝
func (self *Assembler) add_text(ss string) {
    m := rt.Str2Mem(ss)
    if len(m) >= 16 {
        self.Emit("MOVOU", jit.Imm(rt.Get128(m)), _TEMP0)
        self.Emit("MOVOU", _TEMP0, jit.Ptr(_RP, 0))
        self.Emit("ADD", _RL, jit.Imm(16))
    }
    // 处理剩余字节...
}
```

## 4. 性能关键路径分析

### 4.1 热点函数
1. `encodeTypedPointer` - 主要编码入口
2. JIT生成的编码函数 - 类型特化的编码逻辑
3. 内存分配函数 - 缓冲区管理
4. 字符串处理函数 - 字符串编码和转义

### 4.2 性能瓶颈
1. **反射开销**: 类型信息的获取和处理
2. **内存分配**: 频繁的内存分配和释放
3. **分支预测失败**: 复杂类型判断的分支
4. **缓存未命中**: 大对象的不连续内存访问

### 4.3 优化策略
1. **减少反射**: 使用类型缓存和预编译
2. **内存池化**: 复用缓冲区和对象
3. **分支优化**: 使用查找表替代条件分支
4. **数据局部性**: 优化内存布局和访问模式

## 5. 总结

Sonic 的 JIT 编码器通过以下关键技术实现了高性能 JSON 序列化：

1. **运行时编译**: 将类型特化的编码逻辑编译为本地机器码
2. **内存优化**: 通过内存池和零拷贝技术减少内存开销
3. **算法优化**: 使用SIMD指令和查表法加速编码过程
4. **架构适配**: 针对不同CPU架构进行专门的指令优化

整个系统从用户接口到机器码生成形成了一个完整的优化链，每一层都在前一层的基础上进行进一步的性能优化，最终实现了接近硬件理论极限的 JSON 编码性能。