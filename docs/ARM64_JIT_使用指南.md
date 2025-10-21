# Sonic ARM64 JIT 使用指南

本指南详细介绍了如何使用和支持 ARM64 架构下的 Sonic JIT 编译优化功能。

## 目录

1. [概述](#概述)
2. [系统要求](#系统要求)
3. [安装和配置](#安装和配置)
4. [API 使用](#api-使用)
5. [性能优化](#性能优化)
6. [最佳实践](#最佳实践)
7. [故障排除](#故障排除)
8. [基准测试](#基准测试)
9. [迁移指南](#迁移指南)

## 概述

Sonic ARM64 JIT 是为 ARM64 架构专门优化的 JSON 编解码库，通过即时编译（JIT）技术和 SIMD 指令集优化，提供卓越的性能表现。

### 主要特性

- **ARM64 JIT 编译**: 运行时生成优化的 ARM64 汇编代码
- **NEON SIMD 优化**: 利用 ARM64 NEON 指令集进行并行处理
- **零拷贝解析**: 最小化内存分配和数据复制
- **完整 API 兼容**: 与标准 Sonic API 完全兼容
- **自动回退**: 在不支持的平台上自动回退到标准实现

### 性能优势

- **编码速度**: 相比标准库提升 2-5 倍
- **解码速度**: 相比标准库提升 3-8 倍
- **内存效率**: 减少 50-80% 的内存分配
- **CPU 利用率**: 更好的 CPU 流水线和缓存利用

## 系统要求

### 硬件要求

- **架构**: ARM64 (AArch64)
- **最低内存**: 128MB
- **推荐内存**: 1GB+
- **缓存**: 512KB L1 缓存（推荐）

### 软件要求

- **操作系统**: Linux (Ubuntu 18.04+, CentOS 7+), macOS 10.15+, Windows 10+
- **Go 版本**: 1.20-1.25
- **内核**: Linux 4.14+ (对于 JIT 内存映射支持)

### 权限要求

- Linux 需要可执行内存的权限（通常默认启用）
- 某些嵌入式系统可能需要 `execstack` 权限

## 安装和配置

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/bytedance/sonic.git
cd sonic

# 使用 ARM64 构建脚本
./scripts/build_arm64.sh --jit --simd --tests

# 或者手动构建
go build -tags="arm64,go1.20,!go1.26,arm64_jit,sonic_jit,arm64_simd,arm64_neon" ./...
```

### 使用预编译版本

```bash
# 下载 ARM64 预编译版本
wget https://github.com/bytedance/sonic/releases/download/v1.0.0-arm64-jit/sonic-arm64-jit.tar.gz

# 解压安装
tar -xzf sonic-arm64-jit.tar.gz
cd sonic-arm64-jit

# 安装到 Go 路径
go install ./...
```

### Docker 支持

```dockerfile
# Dockerfile
FROM arm64v8/golang:1.22-alpine AS builder

WORKDIR /app
COPY . .

# 启用 ARM64 JIT 构建
ENV SONIC_JIT_ENABLED=1
ENV SONIC_ARM64_JIT=1
ENV SONIC_SIMD_ENABLED=1
ENV SONIC_ARM64_NEON=1

RUN ./scripts/build_arm64.sh --output /dist

FROM scratch
COPY --from=builder /dist/sonic /sonic
CMD ["/sonic"]
```

## API 使用

### 基本用法

```go
package main

import (
    "fmt"
    "github.com/bytedance/sonic"
)

type Person struct {
    Name    string `json:"name"`
    Age     int    `json:"age"`
    Email   string `json:"email"`
    Active  bool   `json:"active"`
}

func main() {
    // 编码
    person := Person{
        Name:   "Alice",
        Age:    30,
        Email:  "alice@example.com",
        Active: true,
    }

    data, err := sonic.Marshal(person)
    if err != nil {
        panic(err)
    }

    fmt.Printf("JSON: %s\n", data)

    // 解码
    var result Person
    err = sonic.Unmarshal(data, &result)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Decoded: %+v\n", result)
}
```

### 高级配置

```go
package main

import (
    "github.com/bytedance/sonic"
    "github.com/bytedance/sonic/option"
)

func advancedUsage() {
    // 配置选项
    cfg := sonic.Config{
        EscapeHTML:    false,    // 禁用 HTML 转义以提高性能
        UseInt64:      true,     // 使用 int64 而不是 float64
        UseNumber:     false,    // 不使用 json.Number
        SortMapKeys:   false,    // 不排序 map 键
        ValidateJSON:  true,     // 验证 JSON
    }

    // 创建 API 实例
    api := sonic.API{
        Config: cfg,
    }

    // 使用配置的 API
    data, err := api.Marshal(yourData)
    if err != nil {
        panic(err)
    }

    var result YourType
    err = api.Unmarshal(data, &result)
    if err != nil {
        panic(err)
    }
}
```

### 预热类型缓存

```go
package main

import (
    "reflect"
    "github.com/bytedance/sonic"
)

func preheatCache() {
    // 预热常用类型的 JIT 编译
    types := []reflect.Type{
        reflect.TypeOf(MyStruct{}),
        reflect.TypeOf([]string{}),
        reflect.TypeOf(map[string]interface{}{}),
        reflect.TypeOf([]*MyStruct{}),
    }

    for _, typ := range types {
        err := sonic.Pretouch(typ)
        if err != nil {
            panic(err)
        }
    }
}
```

### 流式处理

```go
package main

import (
    "bytes"
    "github.com/bytedance/sonic"
)

func streamProcessing() {
    // 创建流式编码器
    var buf bytes.Buffer
    encoder := sonic.Config.NewEncoder(&buf)

    // 编码多个对象
    for i := 0; i < 1000; i++ {
        data := MyStruct{ID: i, Name: fmt.Sprintf("Item %d", i)}
        if err := encoder.Encode(data); err != nil {
            panic(err)
        }
    }

    // 创建流式解码器
    decoder := sonic.Config.NewDecoder(&buf)

    for decoder.More() {
        var item MyStruct
        if err := decoder.Decode(&item); err != nil {
            panic(err)
        }
        // 处理解码的数据
        processItem(item)
    }
}
```

## 性能优化

### 启用 JIT 优化

```go
// 环境变量设置
// SONIC_JIT_ENABLED=1
// SONIC_ARM64_JIT=1
// SONIC_SIMD_ENABLED=1
// SONIC_ARM64_NEON=1

// 或在代码中设置
os.Setenv("SONIC_JIT_ENABLED", "1")
os.Setenv("SONIC_ARM64_JIT", "1")
```

### 内存池优化

```go
package main

import (
    "github.com/bytedance/sonic/internal/jit/arm64"
)

func memoryOptimization() {
    // 获取全局内存池
    pool := arm64.GetGlobalMemoryPool()

    // 使用缓冲区池减少分配
    buf := pool.GetBuffer(1024)
    defer pool.PutBuffer(buf)

    // 使用对齐缓冲区用于 SIMD 操作
    alignedBuf := pool.GetAlignedBuffer(1024)
    defer pool.PutAlignedBuffer(alignedBuf)
}
```

### 缓存优化

```go
package main

import (
    "github.com/bytedance/sonic/internal/jit/arm64"
)

func cacheOptimization() {
    // 获取 JIT 缓存
    cache := arm64.GetGlobalJITCache()

    // 预加载常用类型
    preloadTypes := []reflect.Type{
        reflect.TypeOf(MyStruct{}),
        reflect.TypeOf(AnotherStruct{}),
    }

    for _, typ := range preloadTypes {
        _, err := cache.GetEncoder(typ)
        if err != nil {
            panic(err)
        }
        _, err = cache.GetDecoder(typ)
        if err != nil {
            panic(err)
        }
    }
}
```

### 并发优化

```go
package main

import (
    "sync"
    "github.com/bytedance/sonic"
)

func concurrentProcessing() {
    var wg sync.WaitGroup

    // 并发编码
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()

            data := MyStruct{ID: id}
            _, err := sonic.Marshal(data)
            if err != nil {
                panic(err)
            }
        }(i)
    }

    wg.Wait()
}
```

## 最佳实践

### 1. 类型预编译

```go
// 在应用启动时预热 JIT 缓存
func init() {
    sonic.Pretouch(reflect.TypeOf(MyStruct{}))
    sonic.Pretouch(reflect.TypeOf([]MyStruct{}))
    sonic.Pretouch(reflect.TypeOf(map[string]MyStruct{}))
}
```

### 2. 避免类型转换

```go
// 好的做法：使用具体类型
type User struct {
    ID    int64  `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// 避免：使用 interface{}
// func processData(data interface{}) { ... }
```

### 3. 合理使用指针

```go
// 对于大结构体使用指针减少复制
type LargeStruct struct {
    Data []byte
    Meta map[string]interface{}
}

func (ls *LargeStruct) MarshalJSON() ([]byte, error) {
    return sonic.Marshal(*ls)
}
```

### 4. 批量处理

```go
// 批量处理 JSON 数据
func batchProcess(items []MyStruct) ([]byte, error) {
    // 一次性编码整个数组
    return sonic.Marshal(items)
}
```

### 5. 错误处理

```go
// 始终检查错误
func safeMarshal(data interface{}) ([]byte, error) {
    result, err := sonic.Marshal(data)
    if err != nil {
        // 记录错误并进行降级处理
        log.Printf("Sonic marshal failed: %v", err)
        return json.Marshal(data) // 回退到标准库
    }
    return result, nil
}
```

## 故障排除

### 常见问题

#### 1. JIT 编译失败

**错误信息**: `JIT compilation failed`

**解决方案**:
```go
// 检查环境
if !jit.IsARM64JITEnabled() {
    log.Println("ARM64 JIT is not enabled")
}

// 启用详细日志
os.Setenv("SONIC_DEBUG", "1")
```

#### 2. 内存权限错误

**错误信息**: `permission denied` 或 `mmap: operation not permitted`

**解决方案**:
```bash
# Linux 下确保有可执行内存权限
echo 0 | sudo tee /proc/sys/vm/mmap_min_addr

# 或在容器中添加安全配置
docker run --security-opt seccomp=unconfined ...
```

#### 3. 性能不达预期

**诊断步骤**:
```go
// 获取性能统计
stats := arm64.GetGlobalJITCache().GetStats()
fmt.Printf("Cache hit rate: %.2f%%\n", stats.HitRate*100)

// 检查 JIT 状态
if !jit.IsARM64JITEnabled() {
    log.Println("JIT is disabled, falling back to interpreter")
}
```

### 调试技巧

#### 1. 启用调试日志

```go
// 环境变量
os.Setenv("SONIC_DEBUG", "1")
os.Setenv("SONIC_VERBOSE", "1")
```

#### 2. 性能分析

```go
import (
    _ "net/http/pprof"
    "net/http"
)

func enableProfiling() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
}
```

#### 3. JIT 代码检查

```go
// 检查生成的代码
encoder := sonic.NewEncoder()
program := encoder.GetProgram()
if program != nil {
    fmt.Printf("Generated %d instructions\n", program.InstructionCount())
    fmt.Printf("Code size: %d bytes\n", program.CodeSize())
}
```

## 基准测试

### 基本基准测试

```go
package main

import (
    "testing"
    "github.com/bytedance/sonic"
    "encoding/json"
)

var testData = MyStruct{
    Name: "Benchmark Test",
    Data: make([]byte, 1024),
}

func BenchmarkSonicMarshal(b *testing.B) {
    for i := 0; i < b.N; i++ {
        _, err := sonic.Marshal(testData)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkStdLibMarshal(b *testing.B) {
    for i := 0; i < b.N; i++ {
        _, err := json.Marshal(testData)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### 内存分配基准测试

```go
func BenchmarkMemoryUsage(b *testing.B) {
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        var result MyStruct
        data := []byte(`{"name":"test","data":"..."}`)
        err := sonic.Unmarshal(data, &result)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### 并发基准测试

```go
func BenchmarkConcurrentMarshal(b *testing.B) {
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := sonic.Marshal(testData)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

## 迁移指南

### 从标准 JSON 库迁移

#### 1. 替换导入

```go
// 之前
import "encoding/json"

// 之后
import "github.com/bytedance/sonic"
```

#### 2. 替换函数调用

```go
// 之前
data, err := json.Marshal(obj)
err = json.Unmarshal(data, &obj)

// 之后
data, err := sonic.Marshal(obj)
err = sonic.Unmarshal(data, &obj)
```

#### 3. 处理差异

```go
// 数字处理差异
// 标准库默认使用 float64
// Sonic 默认保持数字类型

// 配置为兼容标准库
cfg := sonic.Config{
    UseNumber: true, // 使用 json.Number
}
api := sonic.API{Config: cfg}
```

### 从其他 JSON 库迁移

#### 1. 性能对比

```go
func comparePerformance() {
    // 测试 Sonic
    start := time.Now()
    for i := 0; i < 10000; i++ {
        sonic.Marshal(testData)
    }
    sonicTime := time.Since(start)

    // 测试其他库
    start = time.Now()
    for i := 0; i < 10000; i++ {
        otherLibrary.Marshal(testData)
    }
    otherTime := time.Since(start)

    fmt.Printf("Sonic: %v, Other: %v, Speedup: %.2fx\n",
        sonicTime, otherTime, float64(otherTime)/float64(sonicTime))
}
```

#### 2. 功能兼容性检查

```go
func testCompatibility() {
    testCases := []interface{}{
        "string",
        42,
        3.14159,
        true,
        nil,
        []int{1, 2, 3},
        map[string]interface{}{"key": "value"},
    }

    for _, tc := range testCases {
        // Sonic 编码
        sonicData, err1 := sonic.Marshal(tc)

        // 其他库编码
        otherData, err2 := otherLibrary.Marshal(tc)

        if err1 == nil && err2 == nil {
            // 比较结果
            if !jsonEqual(sonicData, otherData) {
                log.Printf("Incompatible result for %T", tc)
            }
        }
    }
}
```

## 总结

Sonic ARM64 JIT 提供了卓越的 JSON 处理性能，特别适合在 ARM64 架构上运行的高性能应用。通过正确配置和使用，可以显著提升应用的 JSON 处理效率。

### 关键要点

1. **预热缓存**: 在应用启动时预热常用类型的 JIT 编译
2. **合理配置**: 根据应用场景调整配置选项
3. **监控性能**: 定期检查 JIT 缓存命中率和性能指标
4. **错误处理**: 实现适当的错误处理和回退机制
5. **持续优化**: 根据实际使用情况调整优化策略

### 更多资源

- [GitHub 仓库](https://github.com/bytedance/sonic)
- [性能基准测试](https://github.com/bytedance/sonic/wiki/Benchmarks)
- [API 文档](https://pkg.go.dev/github.com/bytedance/sonic)
- [问题报告](https://github.com/bytedance/sonic/issues)

如有任何问题或建议，欢迎提交 Issue 或 Pull Request。