# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sonic is a blazingly fast JSON serializing & deserializing library for Go, accelerated by JIT (just-in-time compiling) and SIMD (single-instruction-multiple-data). It's developed by ByteDance and is part of the CloudWeGo ecosystem.

## Architecture

The codebase is organized into several key components:

- **Core API** (`api.go`, `compat.go`): Main public API with Marshal/Unmarshal functions
- **Encoder** (`encoder/`): JSON encoding with JIT compilation and SIMD optimizations
- **Decoder** (`decoder/`): JSON decoding with multiple backend strategies (JIT, optimized, native)
- **AST** (`ast/`): Abstract Syntax Tree for JSON manipulation with lazy-loading design
- **Internal** (`internal/`): Low-level optimizations including:
  - `caching/`: Type caching and hashing
  - `decoder/`: JIT compiler and assembler for decoding
  - `encoder/`: VM-based encoding with algorithm optimizations
  - `native/`: SIMD implementations (AVX2, SSE, NEON)
  - `jit/`: JIT compilation backend

## Key Features

- Runtime object binding without code generation
- Complete APIs for JSON value manipulation via `ast.Node`
- Multiple performance configurations (default, std-compatible, fastest)
- JIT-accelerated encoding/decoding using golang-asm
- SIMD optimizations for JSON parsing
- Streaming IO support
- Advanced error reporting with position information

## Development Commands

### Testing
- Run all tests: `go test ./...`
- Run tests with race detection: `go test -race ./...`
- Run specific package tests: `go test ./encoder`, `go test ./decoder`, `go test ./ast`

### Benchmarking
- Full benchmark suite: `./scripts/bench.sh`
- Component-specific benchmarks:
  - Encoder: `cd encoder && go test -bench=BenchmarkEncoder.*`
  - Decoder: `cd decoder && go test -bench=BenchmarkDecoder.*`
  - AST operations: `cd ast && go test -bench=BenchmarkGet.*`

### Build
- Standard Go build: `go build ./...`
- The project uses Go 1.18+ with special handling for Go 1.24+ due to linkname issues

### Code Quality
- Format code: `gofmt ./...`
- Lint: `golangci-lint run`
- The project follows Go Code Review Comments and Uber Go Style Guide

## Important Implementation Notes

### JIT Compilation
- Uses `golang-asm` for runtime code generation
- `Pretouch()` function recommended for huge schemas to avoid runtime compilation overhead
- JIT fallback to `encoding/json` on unsupported environments

### Performance Considerations
- `ast.Node` uses lazy-loading design - not concurrent-safe by default
- Use `Node.LoadAll()` for concurrent access
- String values may reference original JSON buffer for performance
- Configure `CopyString` option to control memory usage vs performance

### Memory Management
- Extensive use of memory pools for performance
- Options available to control pool behavior and memory usage
- Consider `SONIC_NO_ASYNC_GC=1` environment variable for testing

### Configuration Modes
- `ConfigDefault`: Fast with security (EscapeHTML=false, SortKeys=false)
- `ConfigStd`: Std-compatible (EscapeHTML=true, SortKeys=true)
- `ConfigFastest`: Maximum performance (NoQuoteTextMarshaler=true)

## Testing Structure

The repository has extensive test coverage:
- Unit tests in each package (`*_test.go`)
- Integration tests in root directory
- External library compatibility tests in `external_jsonlib_test/`
- Issue regression tests in `issue_test/`
- Fuzzing tests in `fuzz/`
- Performance benchmarks in `generic_test/`

## Development Workflow

The project follows git-flow methodology:
- Main branch: stable code
- Develop branch: development base
- Branch prefixes: `optimize/`, `feature/`, `bugfix/`, `doc/`, `ci/`, `test/`, `refactor/`
- Follows conventional commits specification for PR titles and messages