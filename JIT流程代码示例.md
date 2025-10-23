# Sonic JIT 流程代码示例

## 1. 用户接口层

### 1.1 基本使用示例
```go
package main

import (
    "fmt"
    "github.com/bytedance/sonic"
)

type User struct {
    Name    string `json:"name"`
    Age     int    `json:"age"`
    Email   string `json:"email"`
    Active  bool   `json:"active"`
    Score   float64 `json:"score"`
}

func main() {
    user := User{
        Name:   "Alice",
        Age:    30,
        Email:  "alice@example.com",
        Active: true,
        Score:  95.5,
    }

    // 编码为JSON
    jsonBytes, err := sonic.Marshal(user)
    if err != nil {
        panic(err)
    }

    fmt.Println(string(jsonBytes))
    // 输出: {"name":"Alice","age":30,"email":"alice@example.com","active":true,"score":95.5}
}
```

## 2. 编码流程追踪

### 2.1 第一层：用户接口到核心编码
```go
// 用户调用
func Encode(val interface{}, opts Options) ([]byte, error) {
    var ret []byte
    buf := vars.NewBytes()  // 从内存池获取缓冲区
    err := encodeIntoCheckRace(buf, val, opts)  // 核心编码函数

    if err != nil {
        vars.FreeBytes(buf)
        return nil, err
    }

    encodeFinishWithPool(buf, opts)  // 后处理（HTML转义等）

    if rt.CanSizeResue(cap(*buf)) {
        ret = dirtmake.Bytes(len(*buf), len(*buf))
        copy(ret, *buf)
        vars.FreeBytes(buf)
    } else {
        ret = *buf
    }

    return ret, nil
}
```

### 2.2 第二层：核心编码到类型特化
```go
func encodeInto(buf *[]byte, val interface{}, opts Options) error {
    stk := vars.NewStack()  // 获取编码栈
    efv := rt.UnpackEface(val)  // 解包interface{}为类型和值
    err := encodeTypedPointer(buf, efv.Type, &efv.Value, stk, uint64(opts))

    // 清理栈
    if err != nil {
        vars.ResetStack(stk)
    }
    vars.FreeStack(stk)
    return err
}
```

### 2.3 第三层：类型特化到JIT编译
```go
// x86版本的实现
func EncodeTypedPointer(buf *[]byte, vt *rt.GoType, vp *unsafe.Pointer, sb *vars.Stack, fv uint64) error {
    if vt == nil {
        return prim.EncodeNil(buf)
    } else if fn, err := vars.FindOrCompile(vt, (fv&(1<<alg.BitPointerValue)) != 0, compiler); err != nil {
        return err
    } else if vt.Indirect() {
        return fn.(vars.Encoder)(buf, *vp, sb, fv)
    } else {
        return fn.(vars.Encoder)(buf, unsafe.Pointer(vp), sb, fv)
    }
}
```

## 3. JIT编译过程详解

### 3.1 编译器初始化
```go
// 编译器设置
func init() {
    // 设置编译器函数
    compiler = makeEncoderCompiler
    // 设置预编译函数
    pretouchType = pretouchTypeJIT
    // 设置编码函数
    encodeTypedPointer = x86.EncodeTypedPointer
}

// 编译器创建
func makeEncoderCompiler(vt *rt.GoType, ex ...interface{}) (interface{}, error) {
    // 创建新的x86汇编器
    assembler := x86.NewAssembler("encoder")
    // 编译IR程序
    encoder := assembler.Load()
    return encoder, nil
}
```

### 3.2 IR生成过程
```go
// 以User结构体为例的IR生成
func generateIRForUser() ir.Program {
    program := ir.Program{
        // 编码 '{'
        {Op: ir.OP_byte, Vi: int64('{')},

        // 编码 "name":""
        {Op: ir.OP_text, Vs: `"name":"`},
        {Op: ir.OP_str},  // 编码字符串字段

        // 编码 "","age":
        {Op: ir.OP_text, Vs: `","age":`},
        {Op: ir.OP_i64},  // 编码整数字段

        // 编码 "","email":
        {Op: ir.OP_text, Vs: `","email":`},
        {Op: ir.OP_str},  // 编码字符串字段

        // 编码 "","active":
        {Op: ir.OP_text, Vs: `","active":`},
        {Op: ir.OP_bool}, // 编码布尔字段

        // 编码 "","score":
        {Op: ir.OP_text, Vs: `","score":`},
        {Op: ir.OP_f64},  // 编码浮点数字段

        // 编码 '}'
        {Op: ir.OP_byte, Vi: int64('}')},
    }
    return program
}
```

### 3.3 汇编代码生成
```go
// x86汇编器实现
func (self *Assembler) _asm_OP_bool(p *ir.Instr) {
    // 检查缓冲区容量
    self.check_size(5)

    // 比较布尔值
    self.Emit("CMPB", jit.Ptr(_SP_p, 0), jit.Imm(0))
    self.Sjmp("JE", "_false_{n}")

    // 编码true
    self.Emit("MOVD", jit.Imm(_IM_true), _RT)
    self.Emit("MOVD", _RT, jit.Ptr(_RP, 0))
    self.Emit("ADD", _RL, jit.Imm(4))
    self.Sjmp("JMP", "_end_{n}")

    // 编码false
    self.Link("_false_{n}")
    self.Emit("MOVD", jit.Imm(_IM_fals), _RT)
    self.Emit("MOVD", _RT, jit.Ptr(_RP, 0))
    self.Emit("ADD", _RL, jit.Imm(5))

    self.Link("_end_{n}")
}
```

## 4. JIT机器码生成

### 4.1 函数序言生成
```go
// x86版本函数序言
func (self *Assembler) prologue() {
    // 保存寄存器
    self.Emit("PUSHQ", jit.Ptr(_SP, -8))
    self.Emit("PUSHQ", _RBX)
    self.Emit("PUSHQ", _RBP)
    self.Emit("MOVQ", _RSP, _RBP)
    self.Emit("SUBQ", jit.Imm(_FP_size), _RSP)

    // 加载参数
    self.Emit("MOVQ", jit.Ptr(_ARG_rb, 0), _RDI)  // buf.data
    self.Emit("MOVQ", jit.Ptr(_ARG_rb, 8), _RSI)  // buf.len
    self.Emit("MOVQ", jit.Ptr(_ARG_rb, 16), _RDX) // buf.cap
    self.Emit("MOVQ", _ARG_vp, _R12)              // 数据指针
    self.Emit("MOVQ", _ARG_sb, _RBX)              // 栈基址

    // 初始化状态寄存器
    self.Emit("XORQ", _R13, _R13)  // sp->q = 0
    self.Emit("XORQ", _R14, _R14)  // sp->x = 0
    self.Emit("XORQ", _R15, _R15)  // sp->f = 0
}
```

### 4.2 字符串编码优化
```go
// 高效的字符串编码实现
func (self *Assembler) _asm_OP_str(p *ir.Instr) {
    // 检查字符串长度
    self.Emit("MOVQ", jit.Ptr(_SP_p, 8), _RAX)  // 加载字符串长度
    self.Emit("TESTQ", _RAX, _RAX)
    self.Sjmp("JE", "_str_empty_{n}")

    // 添加开始引号
    self.check_size(2)
    self.Emit("MOVB", jit.Imm('"'), jit.Ptr(_RDI, 0))
    self.Emit("ADDQ", jit.Imm(1), _RSI)

    // 检查缓冲区容量
    self.Emit("ADDQ", _RAX, _RSI)
    self.Emit("CMPQ", _RSI, _RDX)
    self.Sjmp("JBE", "_str_copy_{n}")
    self.call_more_space("_str_copy_{n}")

    // 高效字符串拷贝
    self.Link("_str_copy_{n}")
    self.Emit("MOVQ", jit.Ptr(_SP_p, 0), _RCX)  // 字符串数据指针
    self.Emit("MOVQ", _RAX, _RDX)              // 字符串长度
    self.Emit("ADDQ", _RDI, _RCX)              // 目标地址
    self.call_go(_F_memmove)                   // 调用memmove

    // 添加结束引号
    self.Emit("MOVB", jit.Imm('"'), jit.Ptr(_RDI, _RAX))
    self.Emit("ADDQ", jit.Imm(1), _RSI)

    self.Link("_str_empty_{n}")
    self.check_size(2)
    self.Emit("MOVW", jit.Imm(_IM_empty_str), _RAX)
    self.Emit("MOVW", _RAX, jit.Ptr(_RDI, 0))
    self.Emit("ADDQ", jit.Imm(2), _RSI)
}
```

### 4.3 函数尾声生成
```go
// x86版本函数尾声
func (self *Assembler) epilogue() {
    // 清理错误寄存器
    self.Emit("XORQ", _R10, _R10)
    self.Emit("XORQ", _R11, _R11)

    self.Link("_error")

    // 更新缓冲区长度
    self.Emit("MOVQ", _RSI, jit.Ptr(_ARG_rb, 8))

    // 清理参数寄存器（避免GC问题）
    self.Emit("XORQ", _RDI, _RDI)
    self.Emit("XORQ", _RCX, _RCX)
    self.Emit("XORQ", _RBX, _RBX)

    // 恢复栈帧
    self.Emit("MOVQ", jit.Ptr(_SP, FP_offs), _RBP)
    self.Emit("ADDQ", jit.Imm(_FP_size), _RSP)
    self.Emit("RET")
}
```

## 5. 机器码加载和执行

### 5.1 机器码生成
```go
// golang-asm生成的机器码示例
func generateMachineCode() []byte {
    // 这些是实际的机器码字节（十六进制表示）
    return []byte{
        // 函数序言
        0x55,                   // push rbp
        0x48, 0x89, 0xe5,       // mov rbp, rsp
        0x48, 0x81, 0xec, 0x80, 0x00, 0x00, 0x00, // sub rsp, 0x80

        // 加载参数
        0x48, 0x8b, 0x7f, 0x00, // mov rdi, [rdi]
        0x48, 0x8b, 0x77, 0x08, // mov rsi, [rdi+8]
        0x48, 0x8b, 0x57, 0x10, // mov rdx, [rdi+16]

        // 编码逻辑
        0x48, 0x83, 0x3c, 0x24, 0x00, // cmp [rsp], 0
        0x74, 0x10,                   // je _false

        // 编码"true"
        0x48, 0xc7, 0x07, 0x65, 0x72, 0x75, 0x74, // mov [rdi], 0x74727565
        0x48, 0x83, 0xc6, 0x04,                   // add rsi, 4
        0xeb, 0x08,                               // jmp _end

        // 编码"false"
        0x48, 0xc7, 0x07, 0x65, 0x73, 0x6c, 0x61, // mov [rdi], 0x616c7365
        0xc6, 0x47, 0x04, 0x65,                   // mov [rdi+4], 0x65
        0x48, 0x83, 0xc6, 0x05,                   // add rsi, 5

        // 函数尾声
        0x48, 0x89, 0x77, 0x08, // mov [rdi+8], rsi
        0x48, 0x8b, 0xec,       // mov rbp, rsp
        0x48, 0x81, 0xc4, 0x80, 0x00, 0x00, 0x00, // add rsp, 0x80
        0xc3,                   // ret
    }
}
```

### 5.2 动态加载
```go
// 使用loader加载机器码
func loadJITCode() loader.Function {
    machineCode := generateMachineCode()

    // 创建函数信息
    funcInfo := loader.Func{
        Name:     "encode_User",
        TextSize: uint32(len(machineCode)),
        ArgsSize: int32(32), // 4个参数 * 8字节
    }

    // 设置PC数据
    funcInfo.Pcsp = &loader.Pcdata{
        PC:   0,
        Val:  int32(frameSize),
    }

    // 加载到内存
    funcs := []loader.Func{funcInfo}
    result := loader.Load(machineCode, funcs, "encode_User", []string{"encode_User"})

    return result[0]
}
```

### 5.3 JIT函数调用
```go
// 转换JIT函数为可调用格式
func ptoenc(fn loader.Function) vars.Encoder {
    return *(*vars.Encoder)(unsafe.Pointer(&fn))
}

// 执行JIT编码
func executeJITEncoding(buf *[]byte, user *User) error {
    // 获取类型信息
    vt := rt.UnpackType(reflect.TypeOf(User{}))
    vp := unsafe.Pointer(user)

    // 获取编码栈
    sb := vars.NewStack()
    defer vars.FreeStack(sb)

    // 调用JIT编码函数
    encoder := vars.GetProgram(vt).(vars.Encoder)
    return encoder(buf, vp, sb, uint64(sonic.ConfigDefault))
}
```

## 6. 完整流程追踪示例

### 6.1 调用链追踪
```go
// 这个函数展示了完整的调用链
func traceEncodingFlow() {
    user := User{Name: "Alice", Age: 30}

    // 1. 用户接口层
    fmt.Println("1. 用户调用 sonic.Marshal(user)")

    // 2. 编码器层
    fmt.Println("2. Encode() -> encodeInto() -> encodeTypedPointer()")

    // 3. 缓存查找
    fmt.Println("3. vars.FindOrCompile() 查找User类型的编码器")

    // 4. JIT编译（如果缓存未命中）
    fmt.Println("4. compiler.Compile() 开始JIT编译")
    fmt.Println("   4.1 类型分析: User结构体字段分析")
    fmt.Println("   4.2 IR生成: 生成编码指令序列")
    fmt.Println("   4.3 汇编生成: 转换为x86汇编指令")
    fmt.Println("   4.4 机器码生成: 转换为可执行机器码")
    fmt.Println("   4.5 动态加载: 加载到可执行内存")

    // 5. JIT执行
    fmt.Println("5. 执行JIT编码函数")
    fmt.Println("   5.1 函数序言: 保存寄存器，设置栈帧")
    fmt.Println("   5.2 参数加载: 加载编码参数到寄存器")
    fmt.Println("   5.3 字段编码: 依次编码每个字段")
    fmt.Println("   5.4 缓冲区管理: 动态扩展JSON缓冲区")
    fmt.Println("   5.5 函数尾声: 恢复寄存器，返回结果")

    // 6. 后处理
    fmt.Println("6. encodeFinishWithPool() HTML转义等后处理")

    // 实际执行
    json, _ := sonic.Marshal(user)
    fmt.Printf("7. 最终结果: %s\n", string(json))
}
```

### 6.2 性能分析
```go
// 性能关键点分析
func analyzePerformance() {
    fmt.Println("Sonic JIT性能关键点:")
    fmt.Println()
    fmt.Println("1. 编译时优化:")
    fmt.Println("   - 类型特化: 为User类型生成专门的编码函数")
    fmt.Println("   - 常量折叠: JSON结构字符串在编译时确定")
    fmt.Println("   - 内联优化: 简单操作直接内联")
    fmt.Println()
    fmt.Println("2. 运行时优化:")
    fmt.Println("   - 零拷贝: 直接操作用户数据，避免复制")
    fmt.Println("   - 内存池: 复用缓冲区，减少GC压力")
    fmt.Println("   - SIMD指令: 使用向量化指令加速处理")
    fmt.Println()
    fmt.Println("3. 架构优化:")
    fmt.Println("   - 寄存器分配: 充分利用CPU寄存器")
    fmt.Println("   - 分支预测: 优化条件分支，减少pipeline stall")
    fmt.Println("   - 缓存友好: 优化内存访问模式")
}
```

通过这个完整的代码示例，我们可以清楚地看到Sonic JIT是如何从用户的高级Go代码，一步步转换为高效的机器码，最终实现接近硬件极限的JSON编码性能的。整个流程展示了现代JIT技术的强大能力和复杂精妙的设计。