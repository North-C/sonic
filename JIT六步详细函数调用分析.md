# Sonic JIT 六步详细函数调用分析

## 概述

本文档详细分析 Sonic JIT 编码器从 IR 生成到结果返回的六个关键步骤，每个步骤都包含具体的函数调用链和实现细节。

---

## 🔄 步骤 1: IR 生成 (IR Generation)

### 触发入口
```go
// 用户调用 Marshal 时触发
func Encode(val interface{}, opts Options) ([]byte, error) {
    // ... 其他代码 ...
    err := encodeInto(buf, val, opts)  // 进入编码流程
}

func encodeInto(buf *[]byte, val interface{}, opts Options) error {
    efv := rt.UnpackEface(val)  // 解包 interface{}
    return encodeTypedPointer(buf, efv.Type, &efv.Value, stk, uint64(opts))
}
```

### 主要函数调用链

#### 1.1 缓存查找和编译触发
```go
// x86/stbus.go:41-51
func EncodeTypedPointer(buf *[]byte, vt *rt.GoType, vp *unsafe.Pointer, sb *vars.Stack, fv uint64) error {
    if vt == nil {
        return prim.EncodeNil(buf)
    } else if fn, err := vars.FindOrCompile(vt, (fv&(1<<alg.BitPointerValue)) != 0, compiler); err != nil {
        return err
    } else {
        return fn.(vars.Encoder)(buf, *vp, sb, fv)
    }
}
```

#### 1.2 编译器查找或创建
```go
// internal/encoder/vars/cache.go:32-39
func FindOrCompile(vt *rt.GoType, pv bool, compiler func(*rt.GoType, ... interface{}) (interface{}, error)) (interface{}, error) {
    if val := programCache.Get(vt); val != nil {
        return val, nil  // 缓存命中，直接返回
    } else if ret, err := programCache.Compute(vt, compiler, pv); err == nil {
        return ret, nil  // 编译成功并缓存
    } else {
        return nil, err  // 编译失败
    }
}
```

#### 1.3 编译器创建和执行
```go
// internal/encoder/compiler.go:123-127
func (self *Compiler) Compile(vt reflect.Type, pv bool) (ret ir.Program, err error) {
    defer self.rescue(&err)  // 错误恢复
    self.compileOne(&ret, 0, vt, pv)  // 开始编译
    return
}

// internal/encoder/compiler.go:129-135
func (self *Compiler) compileOne(p *ir.Program, sp int, vt reflect.Type, pv bool) {
    if self.tab[vt] {
        p.Vp(ir.OP_recurse, vt, pv)  // 递归检测，避免循环引用
    } else {
        self.compileRec(p, sp, vt, pv)  // 递归编译
    }
}
```

#### 1.4 递归编译核心逻辑
```go
// internal/encoder/compiler.go:167-182
func (self *Compiler) compileRec(p *ir.Program, sp int, vt reflect.Type, pv bool) {
    pr := self.pv

    // 检查是否实现了 Marshaler 接口
    if self.tryCompileMarshaler(p, vt, pv) {
        return
    }

    // 进入递归编译
    self.pv = pv
    self.tab[vt] = true  // 标记已访问
    self.compileOps(p, sp, vt)  // 编译具体操作

    // 退出递归
    self.pv = pr
    delete(self.tab, vt)
}
```

#### 1.5 类型特化编译
```go
// internal/encoder/compiler.go:184-231
func (self *Compiler) compileOps(p *ir.Program, sp int, vt reflect.Type) {
    switch vt.Kind() {
    case reflect.Bool:
        p.Add(ir.OP_bool)  // 布尔类型 -> OP_bool
    case reflect.Int:
        p.Add(ir.OP_int())  // 整数类型 -> OP_int
    case reflect.String:
        self.compileString(p, vt)  // 字符串类型 -> 特殊处理
    case reflect.Struct:
        self.compileStruct(p, sp, vt)  // 结构体 -> 编译字段
    case reflect.Slice:
        self.compileSlice(p, sp, vt.Elem())  // 切片 -> 编译元素类型
    case reflect.Map:
        self.compileMap(p, sp, vt)  // 映射 -> 编译键值对
    case reflect.Ptr:
        self.compilePtr(p, sp, vt.Elem())  // 指针 -> 解引用后编译
    // ... 其他类型处理
    }
}
```

#### 1.6 结构体编译示例
```go
// 以 User 结构体为例，生成的 IR 序列
func generateIRForUser() ir.Program {
    program := ir.Program{
        // 编码 '{'
        {Op: ir.OP_byte, Vi: int64('{')},

        // 编码字段名 "name":"，然后编码字符串值
        {Op: ir.OP_text, Vs: `"name":"`},
        {Op: ir.OP_str},

        // 编码字段分隔符 "","age":，然后编码整数值
        {Op: ir.OP_text, Vs: `","age":`},
        {Op: ir.OP_i64},

        // ... 其他字段

        // 编码 '}'
        {Op: ir.OP_byte, Vi: int64('}')},
    }
    return program
}
```

### IR 生成总结
- **入口**: `EncodeTypedPointer()`
- **核心**: `compiler.Compile()` → `compileOne()` → `compileRec()`
- **策略**: 递归类型分析 + 特化操作码生成
- **输出**: `ir.Program` (中间表示指令序列)

---

## ⚙️ 步骤 2: JIT 编译 (JIT Compilation)

### 触发入口
```go
// internal/encoder/x86/assembler_regabi_amd64.go:199-201
func (self *Assembler) Load() vars.Encoder {
    return ptoenc(self.BaseAssembler.Load("encode_"+self.Name, _FP_size, _FP_args, vars.ArgPtrs, vars.LocalPtrs))
}
```

### 主要函数调用链

#### 2.1 汇编器初始化
```go
// internal/encoder/x86/assembler_regabi_amd64.go:203-207
func (self *Assembler) Init(p ir.Program) *Assembler {
    self.p = p  // 设置 IR 程序
    self.BaseAssembler.Init(self.compile)  // 初始化基类汇编器
    return self
}
```

#### 2.2 编译流程控制
```go
// internal/encoder/x86/assembler_regabi_amd64.go:209-214
func (self *Assembler) compile() {
    self.prologue()   // 生成函数序言
    self.instrs()     // 生成指令序列
    self.epilogue()   // 生成函数尾声
    self.builtins()   // 生成内置函数
}
```

#### 2.3 IR 指令遍历和分发
```go
// internal/encoder/x86/assembler_regabi_amd64.go:252-256
func (self *Assembler) instrs() {
    for i, v := range self.p {
        self.Mark(i)       // 标记指令位置
        self.instr(&v)     // 处理单条指令
        self.debug_instr(i, &v)  // 调试信息
    }
}

func (self *Assembler) instr(v *ir.Instr) {
    // 根据操作码查找对应的处理函数
    if fn := _OpFuncTab[v.Op()]; fn != nil {
        fn(self, v)
    } else {
        panic(fmt.Sprintf("invalid opcode: %d", v.Op()))
    }
}
```

#### 2.4 操作码函数表
```go
// internal/encoder/x86/assembler_regabi_amd64.go:218-251
var _OpFuncTab = [256]func(*Assembler, *ir.Instr){
    ir.OP_null:           (*Assembler)._asm_OP_null,
    ir.OP_bool:           (*Assembler)._asm_OP_bool,
    ir.OP_i8:             (*Assembler)._asm_OP_i8,
    ir.OP_i16:            (*Assembler)._asm_OP_i16,
    ir.OP_i32:            (*Assembler)._asm_OP_i32,
    ir.OP_i64:            (*Assembler)._asm_OP_i64,
    ir.OP_str:            (*Assembler)._asm_OP_str,
    ir.OP_text:           (*Assembler)._asm_TEXT,
    // ... 256个操作码对应的处理函数
}
```

#### 2.5 具体指令实现示例
```go
// 布尔类型编码实现
func (self *Assembler) _asm_OP_bool(p *ir.Instr) {
    // 检查缓冲区容量
    self.check_size(5)

    // 比较布尔值
    self.Emit("CMPB", jit.Ptr(_SP_p, 0), jit.Imm(0))
    self.Sjmp("JE", "_false_{n}")  // 如果为false跳转

    // 编码 "true"
    self.Emit("MOVD", jit.Imm(_IM_true), _RT)
    self.Emit("MOVD", _RT, jit.Ptr(_RP, 0))
    self.Emit("ADD", _RL, jit.Imm(4))
    self.Sjmp("JMP", "_end_{n}")

    // 编码 "false"
    self.Link("_false_{n}")
    self.Emit("MOVD", jit.Imm(_IM_fals), _RT)
    self.Emit("MOVD", _RT, jit.Ptr(_RP, 0))
    self.Emit("ADD", _RL, jit.Imm(5))

    self.Link("_end_{n}")
}
```

#### 2.6 函数序言生成
```go
// 保存寄存器并设置栈帧
func (self *Assembler) prologue() {
    // 保存调用者保存的寄存器
    self.Emit("PUSHQ", _RBX)
    self.Emit("PUSHQ", _RBP)

    // 设置新的栈帧
    self.Emit("MOVQ", _RSP, _RBP)
    self.Emit("SUBQ", jit.Imm(_FP_size), _RSP)

    // 加载参数到寄存器
    self.Emit("MOVQ", jit.Ptr(_ARG_rb, 0), _RDI)   // buf.data
    self.Emit("MOVQ", jit.Ptr(_ARG_rb, 8), _RSI)   // buf.len
    self.Emit("MOVQ", jit.Ptr(_ARG_rb, 16), _RDX)  // buf.cap
    self.Emit("MOVQ", _ARG_vp, _R12)               // 数据指针
    self.Emit("MOVQ", _ARG_sb, _RBX)               // 栈基址

    // 初始化状态寄存器
    self.Emit("XORQ", _R13, _R13)  // sp->q = 0
    self.Emit("XORQ", _R14, _R14)  // sp->x = 0
    self.Emit("XORQ", _R15, _R15)  // sp->f = 0
}
```

### JIT 编译总结
- **入口**: `Assembler.Load()` → `BaseAssembler.Load()`
- **核心**: `compile()` → `instrs()` → 操作码分发
- **策略**: IR 指令 -> x86 汇编指令
- **输出**: 汇编指令序列

---

## 🔧 步骤 3: 机器码生成 (Machine Code Generation)

### 触发入口
```go
// internal/jit/assembler_amd64.go:213-216
func (self *BaseAssembler) Load(name string, frameSize int, argSize int, argStackmap []bool, localStackmap []bool) loader.Function {
    self.build()  // 构建机器码
    return jitLoader.LoadOne(self.c, name, frameSize, argSize, argStackmap, localStackmap)
}
```

### 主要函数调用链

#### 3.1 构建流程控制
```go
// internal/jit/assembler_amd64.go:227-236
func (self *BaseAssembler) build() {
    self.o.Do(func() {  // 使用 sync.Once 确保只执行一次
        self.init()      // 初始化汇编器
        self.f()         // 执行编译函数 (即 self.compile())
        self.validate()   // 验证标签解析
        self.assemble()   // 汇编为机器码
        self.resolve()    // 解析符号引用
        self.release()    // 释放资源
    })
}
```

#### 3.2 汇编器初始化
```go
// internal/jit/assembler_amd64.go:220-225
func (self *BaseAssembler) init() {
    self.pb       = newBackend("amd64")  // 创建 AMD64 后端
    self.xrefs    = map[string][]*obj.Prog{}  // 符号引用表
    self.labels   = map[string]*obj.Prog{}    // 标签表
    self.pendings = map[string][]*obj.Prog{}   // 待解析跳转表
}
```

#### 3.3 后端创建
```go
// internal/jit/backend.go:55-61
func newBackend(name string) (ret *Backend) {
    ret      = new(Backend)
    ret.Arch = arch.Set(name)     // 设置架构 (amd64)
    ret.Ctxt = newLinkContext(ret.Arch.LinkArch)  // 创建链接上下文
    ret.Arch.Init(ret.Ctxt)      // 初始化架构
    return
}
```

#### 3.4 指令汇编
```go
// internal/jit/assembler_amd64.go:267-269
func (self *BaseAssembler) assemble() {
    self.c = self.pb.Assemble()  // 将指令序列汇编为机器码
}

// internal/jit/backend.go:106-117
func (self *Backend) Assemble() []byte {
    var sym obj.LSym
    var fnv obj.FuncInfo

    // 构建函数符号
    sym.Func = &fnv
    fnv.Text = self.Head  // 设置指令序列

    // 调用架构特定的汇编器
    self.Arch.Assemble(self.Ctxt, &sym, self.New)
    return sym.P  // 返回机器码
}
```

#### 3.5 符号解析
```go
// internal/jit/assembler_amd64.go:246-259
func (self *BaseAssembler) resolve() {
    for s, v := range self.xrefs {  // 遍历所有符号引用
        for _, prog := range v {
            if prog.As != x86.ALONG {
                panic("invalid RIP relative reference")
            } else if p, ok := self.labels[s]; !ok {
                panic("links are not fully resolved: " + s)
            } else {
                // 计算相对偏移量
                off := prog.From.Offset + p.Pc - prog.Pc
                // 写入机器码 (小端序)
                binary.LittleEndian.PutUint32(self.c[prog.Pc:], uint32(off))
            }
        }
    }
}
```

#### 3.6 指令生成示例
```go
// MOV 指令生成
func (self *BaseAssembler) MOV(dst, src obj.Addr) *obj.Prog {
    p := self.New()        // 创建新指令
    p.As = x86.AMOVQ      // 设置操作码 (MOVQ)
    p.From = src           // 设置源操作数
    p.To = dst             // 设置目标操作数
    self.pb.Append(p)      // 添加到指令序列
    return p
}

// JMP 指令生成
func (self *BaseAssembler) JMP(target string) *obj.Prog {
    p := self.New()
    p.As = obj.AJMP       // 设置操作码
    p.To.Type = obj.TYPE_BRANCH
    // 标记为待解析的跳转
    if self.pendings[target] == nil {
        self.pendings[target] = make([]*obj.Prog, 0, 4)
    }
    self.pendings[target] = append(self.pendings[target], p)
    self.pb.Append(p)
    return p
}
```

#### 3.7 机器码示例
```go
// 汇编指令: MOV QWORD PTR [RDI], RAX
// 生成的机器码: 48 89 07
// 48: REX.W 前缀
// 89: MOV 操作码
// 07: ModR/M 字节 ([RDI], RAX)

// 汇编指令: ADD RSI, 4
// 生成的机器码: 48 83 C6 04
// 48: REX.W 前缀
// 83 C6: ADD r/m32, imm8 操作码
// 04: 立即数 4
```

### 机器码生成总结
- **入口**: `BaseAssembler.build()` → `assemble()`
- **核心**: `pb.Assemble()` → 架构汇编器
- **策略**: 汇编指令 -> 机器码字节序列
- **输出**: `[]byte` (可执行机器码)

---

## 📦 步骤 4: 代码加载 (Code Loading)

### 触发入口
```go
// internal/jit/assembler_amd64.go:213-216
func (self *BaseAssembler) Load(name string, frameSize int, argSize int, argStackmap []bool, localStackmap []bool) loader.Function {
    self.build()  // 生成机器码 (步骤3已完成)
    return jitLoader.LoadOne(self.c, name, frameSize, argSize, argStackmap, localStackmap)
}
```

### 主要函数调用链

#### 4.1 JIT 加载器定义
```go
// internal/jit/assembler_amd64.go:205-211
var jitLoader = loader.Loader{
    Name: "sonic.jit.",                      // 模块名前缀
    File: "github.com/bytedance/sonic/jit.go",  // 源文件名
    Options: loader.Options{
        NoPreempt: true,  // 禁用异步抢占
    },
}
```

#### 4.2 LoadOne 函数实现
```go
// loader/loader_latest.go:67-120
func (self Loader) LoadOne(text []byte, funcName string, frameSize int, argSize int, argPtrs []bool, localPtrs []bool) Function {
    size := uint32(len(text))  // 机器码大小

    // 创建函数信息结构
    fn := Func{
        Name:     funcName,     // 函数名
        TextSize: size,         // 代码大小
        ArgsSize: int32(argSize), // 参数大小
    }

    // 设置栈指针变化数据 (PC -> SP delta)
    fn.Pcsp = &Pcdata{
        {PC: size, Val: int32(frameSize)},  // 函数结束时栈指针变化
    }

    // 设置不安全点数据 (用于GC)
    if self.NoPreempt {
        fn.PcUnsafePoint = &Pcdata{
            {PC: size, Val: PCDATA_UnsafePointUnsafe},
        }
    } else {
        fn.PcUnsafePoint = &Pcdata{
            {PC: size, Val: PCDATA_UnsafePointSafe},
        }
    }

    // 设置参数和局部变量的指针映射
    fn.ArgsPointerMaps = &StackMapBuilder{
        bm: NewBitmap(1 + len(argPtrs)),  // 包含返回值和参数
    }
    fn.LocalsPointerMaps = &StackMapBuilder{
        bm: NewBitmap(len(localPtrs)),     // 局部变量
    }

    // 构建指针位图
    buildStackMap(fn.ArgsPointerMaps.bm, argPtrs, 1)   // 包含返回值
    buildStackMap(fn.LocalsPointerMaps.bm, localPtrs, 0)

    // 加载到 Go 运行时
    mod := moduledata{
        text:           text,           // 机器码
        funcsp:         size,          // 栈大小
        funcname:       []string{funcName},  // 函数名列表
        filetable:      []string{self.File}, // 文件名列表
        pcfile:         &Pcdata{{PC: 0, Val: 0}},
        pcln:           &Pcdata{{PC: 0, Val: 0}},
        funcdata:       []Func{fn},           // 函数数据
        minpc:          0,                  // 最小PC
        maxpc:          size,               // 最大PC
        objects:        []object{},
    }

    // 使用 Go 运行时的模块加载机制
    m := modulink{
        next:    atomic.LoadPointer(&modules),
        module:  unsafe.Pointer(&mod),
    }

    // 原子操作更新模块链表
    for !atomic.CompareAndSwapPointer(&modules, nil, unsafe.Pointer(&m)) {
        m.next = atomic.LoadPointer(&modules)
    }

    // 查找函数地址并返回
    for _, f := range mod.funcdata {
        if f.Name == funcName {
            addr := uintptr(unsafe.Pointer(uintptr(mod.text) + uintptr(f.EntryOff)))
            return Function(&addr)  // 返回函数指针
        }
    }

    return nil  // 未找到函数
}
```

#### 4.3 栈映射构建
```go
// 构建指针位图，用于GC扫描
func buildStackMap(bm *Bitmap, ptrs []bool, base int) {
    if base > 0 {
        bm.Set(0)  // 返回值总是指针
    }

    for i, isPtr := range ptrs {
        if isPtr {
            bm.Set(base + i)  // 标记包含指针的位置
        }
    }
}

// 位图实现
type Bitmap struct {
    n    int       // 位数
    data []uint64  // 位数据
}

func (b *Bitmap) Set(i int) {
    if i >= b.n {
        return
    }
    b.data[i/64] |= 1 << (i % 64)  // 设置对应位
}
```

#### 4.4 内存映射和权限设置
```go
// 使用 mmap 分配可执行内存
func mmapExecutable(size int) ([]byte, error) {
    // 分配内存页
    data, err := unix.Mmap(-1, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANONYMOUS)
    if err != nil {
        return nil, err
    }

    // 设置为可执行
    err = unix.Mprotect(data, unix.PROT_READ|unix.PROT_EXEC)
    if err != nil {
        unix.Munmap(data)
        return nil, err
    }

    return data, nil
}
```

#### 4.5 函数指针转换
```go
// 将机器码地址转换为可调用函数指针
func createFunctionPointer(code []byte, entryOffset int) loader.Function {
    addr := uintptr(unsafe.Pointer(&code[entryOffset]))
    return Function(&addr)  // 转换为 loader.Function (unsafe.Pointer)
}
```

#### 4.6 符号注册
```go
// 向 Go 运行时注册函数符号
func registerFunctionSymbol(name string, code []byte) error {
    // 创建函数符号信息
    funcInfo := &runtime.Func{
        Name:    name,
        Entry:   uintptr(unsafe.Pointer(&code[0])),
        Size:    uintptr(len(code)),
    }

    // 注册到符号表
    runtime.RegisterFunc(funcInfo)

    return nil
}
```

### 代码加载总结
- **入口**: `jitLoader.LoadOne()`
- **核心**: 函数信息构建 + 内存映射 + 符号注册
- **策略**: 机器码字节 -> 可执行内存 -> Go 函数指针
- **输出**: `loader.Function` (可直接调用的函数指针)

---

## ⚡ 步骤 5: 执行编码 (Execution)

### 触发入口
```go
// x86/stbus.go:37-39
func ptoenc(p loader.Function) vars.Encoder {
    return *(*vars.Encoder)(unsafe.Pointer(&p))  // 类型转换
}

// 编码器函数类型
// internal/encoder/vars/cache.go:25-30
type Encoder func(
    rb *[]byte,        // 结果缓冲区
    vp unsafe.Pointer, // 数据指针
    sb *Stack,         // 编码栈
    fv uint64,         // 标志位
) error
```

### 主要函数调用链

#### 5.1 JIT 函数调用准备
```go
// 用户数据编码调用
func encodeTypedPointer(buf *[]byte, vt *rt.GoType, vp *unsafe.Pointer, sb *vars.Stack, fv uint64) error {
    // 从缓存获取编码器
    fn, err := vars.FindOrCompile(vt, (fv&(1<<alg.BitPointerValue)) != 0, compiler)
    if err != nil {
        return err
    }

    // 调用 JIT 编码函数
    if vt.Indirect() {
        return fn.(vars.Encoder)(buf, *vp, sb, fv)  // 解引用指针
    } else {
        return fn.(vars.Encoder)(buf, unsafe.Pointer(vp), sb, fv)
    }
}
```

#### 5.2 JIT 函数执行上下文
```go
// JIT 函数的实际调用 (伪代码，实际是机器码执行)
func jitEncodeFunction(buf *[]byte, data unsafe.Pointer, stack *Stack, flags uint64) error {
    // 这些操作由 JIT 生成的机器码执行：

    // 1. 函数序言 (已编译为机器码)
    // PUSH RBP
    // MOV RSP, RBP
    // SUB RSP, FRAME_SIZE

    // 2. 参数加载 (已编译为机器码)
    // MOV [buf], RDI     // 结果指针
    // MOV [buf+8], RSI  // 结果长度
    // MOV [buf+16], RDX // 结果容量
    // MOV data, R12     // 数据指针
    // MOV stack, RBX    // 栈基址

    // 3. 类型特化编码 (已编译为机器码)
    // 对于 User 结构体:
    // MOV BYTE PTR [RDI], '{'
    // ADD RSI, 1
    // MOV "name":" 到 [RDI+RSI]
    // ADD RSI, 8
    // CALL encodeString  // 编码 name 字段
    // ... 其他字段编码

    // 4. 函数尾声 (已编译为机器码)
    // MOV RSI, [buf+8]  // 更新缓冲区长度
    // ADD RSP, FRAME_SIZE
    // POP RBP
    // RET

    return nil  // 返回成功
}
```

#### 5.3 执行时的寄存器状态
```go
// JIT 函数执行期间的寄存器使用情况
type JITRegisterState struct {
    // 输入参数 (由调用者设置)
    RDI uintptr  // buf.data (结果缓冲区指针)
    RSI uintptr  // buf.len  (结果缓冲区长度)
    RDX uintptr  // buf.cap  (结果缓冲区容量)
    RCX uintptr  // flags    (编码标志)
    R8  uintptr  // data     (数据指针)
    R9  uintptr  // stack    (编码栈基址)

    // 状态寄存器 (由 JIT 函数管理)
    RBX uintptr  // stack.base (栈基址，来自 R9)
    R12 uintptr  // sp->p      (栈指针->数据指针)
    R13 uintptr  // sp->q      (栈指针->数量)
    R14 uintptr  // sp->x      (栈指针->扩展)
    R15 uintptr  // sp->f      (栈指针->浮点)

    // 错误寄存器
    R10 uintptr  // error.type  (错误类型)
    R11 uintptr  // error.ptr  (错误指针)

    // 临时寄存器
    RAX uintptr  // 临时寄存器
    RCX uintptr  // 临时寄存器
    // ...
}
```

#### 5.4 字符串编码执行流程
```go
// JIT 编码字符串的执行流程 (伪代码)
func jitEncodeString() {
    // 1. 检查字符串长度
    // MOV [R12+8], RAX     // 加载字符串长度
    // TEST RAX, RAX
    // JZ empty_string

    // 2. 添加开始引号
    // MOV '"', [RDI+RSI]
    // ADD RSI, 1

    // 3. 检查缓冲区容量
    // ADD RAX, RSI
    // CMP RSI, RDX
    // JBE have_space
    // CALL growBuffer    // 扩展缓冲区
    // JMP have_space

    // 4. 高效字符串拷贝
    // MOV [R12], RCX      // 字符串数据指针
    // MOV RAX, RDX        // 字符串长度
    // ADD RDI, RSI        // 目标地址
    // CALL memmove       // 调用内存拷贝

    // 5. 添加结束引号
    // MOV '"', [RDI+RAX]
    // ADD RSI, 1

    // 6. 空字符串处理
    // empty_string:
    // MOV [RDI+RSI], '""'
    // ADD RSI, 2
}
```

#### 5.5 缓冲区管理执行
```go
// JIT 缓冲区管理执行流程
func jitBufferManagement() {
    // 检查缓冲区容量
    // ADD needed_size, RSI
    // CMP RSI, RDX
    // JBE have_capacity

    // 扩展缓冲区
    // MOV RDI, ARG0       // 缓冲区指针
    // MOV RSI, ARG1       // 当前长度
    // MOV RAX, ARG2       // 需要的容量
    // MOV RDX, ARG3       // 当前容量
    // CALL growslice      // Go 运行时函数

    // 更新缓冲区信息
    // MOV RAX, RDI       // 新的缓冲区指针
    // MOV ARG2, RSI      // 新的长度
    // MOV ARG3, RDX      // 新的容量
    // MOV RAX, [ARG0]    // 更新缓冲区数据指针
    // MOV RSI, [ARG0+8]  // 更新缓冲区长度
    // MOV RDX, [ARG0+16] // 更新缓冲区容量

    // have_capacity:
    // 继续编码逻辑
}
```

#### 5.6 错误处理执行
```go
// JIT 错误处理执行流程
func jitErrorHandling() {
    // 正常执行路径
    // ... 编码逻辑 ...

    // 错误检查点
    // CMP error_condition, 0
    // JNE handle_error

    // 正常返回
    // XOR R10, R10        // 清零错误类型
    // XOR R11, R11        // 清零错误指针
    // JMP epilogue

    // 错误处理路径
    // handle_error:
    // MOV error_type, R10
    // MOV error_ptr, R11

    // epilogue:
    // MOV RSI, [ARG0+8]   // 更新缓冲区长度
    // XOR RDI, RDI        // 清零参数指针
    // XOR RCX, RCX        // 清零参数指针
    // XOR RBX, RBX        // 清零参数指针
    // ADD RSP, FRAME_SIZE
    // POP RBP
    // RET
}
```

### 执行编码总结
- **入口**: `vars.Encoder` 函数调用 (JIT 生成的机器码)
- **核心**: 直接执行机器码，无 Go 函数调用开销
- **策略**: 寄存器优化 + 栈帧管理 + 缓冲区动态扩展
- **输出**: 更新的 JSON 缓冲区

---

## 🏁 步骤 6: 结果返回 (Result Return)

### 触发入口
```go
// JIT 函数返回后的处理流程
func encodeTypedPointer(buf *[]byte, vt *rt.GoType, vp *unsafe.Pointer, sb *vars.Stack, fv uint64) error {
    // 调用 JIT 编码函数
    err := fn.(vars.Encoder)(buf, *vp, sb, fv)

    // 清理栈资源
    if err != nil {
        vars.ResetStack(sb)
    }
    vars.FreeStack(sb)

    return err
}
```

### 主要函数调用链

#### 6.1 编码结果后处理
```go
// internal/encoder/encoder.go:170-196
func Encode(val interface{}, opts Options) ([]byte, error) {
    var ret []byte

    // 从内存池获取缓冲区
    buf := vars.NewBytes()

    // 执行编码
    err := encodeIntoCheckRace(buf, val, opts)

    // 检查编码错误
    if err != nil {
        vars.FreeBytes(buf)  // 释放缓冲区
        return nil, err
    }

    // 后处理 (HTML转义等)
    encodeFinishWithPool(buf, opts)

    // 复制结果或复用缓冲区
    if rt.CanSizeResue(cap(*buf)) {
        // 需要复制结果
        ret = dirtmake.Bytes(len(*buf), len(*buf))
        copy(ret, *buf)
        vars.FreeBytes(buf)  // 释放缓冲区
    } else {
        // 直接复用缓冲区
        ret = *buf
    }

    return ret, nil
}
```

#### 6.2 后处理函数
```go
// 编码完成后的后处理
func encodeFinishWithPool(buf *[]byte, opts Options) {
    // HTML 转义处理
    if opts&EscapeHTML != 0 {
        *buf = htmlEscape(*buf)  // 执行 HTML 转义
    }

    // UTF-8 验证
    if opts&ValidateString != 0 {
        if !utf8.Valid(*buf) {
            // 处理无效 UTF-8
            *buf = sanitizeUTF8(*buf)
        }
    }

    // 添加换行符
    if opts&NoEncoderNewline == 0 && len(*buf) > 0 {
        *buf = append(*buf, '\n')
    }
}
```

#### 6.3 内存池管理
```go
// 从内存池获取字节缓冲区
func NewBytes() *[]byte {
    // 从全局内存池获取
    if buf := bytePool.Get(); buf != nil {
        return buf.(*[]byte)
    }

    // 池为空，创建新缓冲区
    buf := make([]byte, 0, 1024)  // 默认1KB容量
    return &buf
}

// 释放字节缓冲区到内存池
func FreeBytes(buf *[]byte) {
    // 重置缓冲区
    *buf = (*buf)[:0]

    // 返回到内存池
    if cap(*buf) <= maxBufferSize {
        bytePool.Put(buf)
    }
    // 大缓冲区直接让 GC 回收
}
```

#### 6.4 结果复制优化
```go
// 判断是否可以复用缓冲区
func CanSizeResue(cap int) bool {
    // 如果容量小于阈值，直接复用
    if cap <= reuseThreshold {
        return true
    }

    // 如果容量过大，需要复制以避免内存浪费
    return false
}

// 高效的字节复制
func fastCopy(dst, src []byte) {
    copy(dst, src)  // Go 的 copy 函数已经高度优化
}
```

#### 6.5 错误处理和清理
```go
// 错误情况下的资源清理
func handleEncodingError(buf *[]byte, sb *vars.Stack, err error) {
    // 释放缓冲区
    vars.FreeBytes(buf)

    // 重置并释放栈
    vars.ResetStack(sb)
    vars.FreeStack(sb)

    // 记录错误统计
    atomic.AddUint64(&errorCount, 1)

    // 可选的错误日志
    if debugMode {
        log.Printf("Encoding error: %v", err)
    }
}
```

#### 6.6 性能统计
```go
// 编码性能统计
type EncodingStats struct {
    TotalCalls    uint64  // 总调用次数
    TotalBytes    uint64  // 总编码字节数
    TotalTime     uint64  // 总耗时 (纳秒)
    CacheHits     uint64  // 缓存命中次数
    CacheMisses   uint64  // 缓存未命中次数
    BufferReused  uint64  // 缓冲区复用次数
    BufferAlloced uint64  // 缓冲区分配次数
}

func updateStats(bytes int, duration time.Duration, cacheHit bool) {
    atomic.AddUint64(&stats.TotalCalls, 1)
    atomic.AddUint64(&stats.TotalBytes, uint64(bytes))
    atomic.AddUint64(&stats.TotalTime, uint64(duration.Nanoseconds()))

    if cacheHit {
        atomic.AddUint64(&stats.CacheHits, 1)
    } else {
        atomic.AddUint64(&stats.CacheMisses, 1)
    }
}
```

### 结果返回总结
- **入口**: JIT 函数返回后的后处理流程
- **核心**: 错误检查 + 后处理 + 内存管理 + 结果复制
- **策略**: 内存池复用 + 零拷贝优化 + 性能统计
- **输出**: 最终的 JSON 字节切片

---

## 🎯 完整流程图总结

### 六步流程时序图
```
用户调用 → [1]IR生成 → [2]JIT编译 → [3]机器码生成 → [4]代码加载 → [5]执行编码 → [6]结果返回
    │           │           │             │            │            │
    │           │           │             │            │            │
    ▼           ▼           ▼             ▼            ▼            ▼
sonic.Marshal → compiler.Compile → Assembler.Load → pb.Assemble → LoadOne → ptoenc → encodeFinishWithPool
    │           │           │             │            │            │
    │           │           │             │            │            │
    ▼           ▼           ▼             ▼            ▼            ▼
类型分析 → IR指令序列 → 汇编指令 → 机器码字节 → 可执行函数 → 直接执行 → JSON字节
```

### 性能关键点
1. **编译优化**: 类型特化、常量折叠、死代码消除
2. **JIT优势**: 直接机器码执行、零函数调用开销
3. **内存优化**: 内存池复用、零拷贝、动态扩展
4. **架构优化**: 寄存器分配、指令选择、分支预测

这种分层优化的设计使得 Sonic 能够实现接近硬件理论极限的 JSON 编码性能，相比标准库有显著的性能提升。