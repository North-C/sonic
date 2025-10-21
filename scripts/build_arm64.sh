#!/bin/bash

# Build script for ARM64 JIT enabled sonic
# This script provides a convenient wrapper for building sonic with ARM64 JIT support

set -e

# Default values
OUTPUT_DIR="${OUTPUT_DIR:-build/arm64}"
ENABLE_JIT="${ENABLE_JIT:-true}"
ENABLE_SIMD="${ENABLE_SIMD:-true}"
ENABLE_TESTS="${ENABLE_TESTS:-true}"
ENABLE_BENCH="${ENABLE_BENCH:-false}"
VERBOSE="${VERBOSE:-false}"
CROSS_COMPILE="${CROSS_COMPILE:-false}"
TARGET_OS="${TARGET_OS:-linux}"
TARGET_ARCH="${TARGET_ARCH:-arm64}"
BUILD_TAGS="${BUILD_TAGS:-}"
LD_FLAGS="${LD_FLAGS:-}"
GC_FLAGS="${GC_FLAGS:-}"
PARALLELISM="${PARALLELISM:-$(nproc)}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Print usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Build ARM64 JIT enabled sonic library.

OPTIONS:
    -o, --output DIR          Output directory (default: build/arm64)
    -j, --jit                 Enable ARM64 JIT (default: true)
    -s, --simd                Enable ARM64 SIMD (default: true)
    -t, --tests               Enable tests (default: true)
    -b, --bench               Enable benchmarks (default: false)
    -v, --verbose             Verbose output (default: false)
    -c, --cross               Cross-compile (default: false)
    --target-os OS            Target OS (default: linux)
    --target-arch ARCH        Target architecture (default: arm64)
    --tags TAGS               Additional build tags
    --ldflags FLAGS           Additional linker flags
    --gcflags FLAGS           Additional compiler flags
    -p, --parallelism N       Build parallelism (default: $(nproc))
    -h, --help                Show this help message

ENVIRONMENT VARIABLES:
    OUTPUT_DIR                Output directory
    ENABLE_JIT                Enable ARM64 JIT
    ENABLE_SIMD               Enable ARM64 SIMD
    ENABLE_TESTS              Enable tests
    ENABLE_BENCH              Enable benchmarks
    VERBOSE                   Verbose output
    CROSS_COMPILE             Cross-compile
    TARGET_OS                 Target OS
    TARGET_ARCH               Target architecture
    BUILD_TAGS                Additional build tags
    LD_FLAGS                  Additional linker flags
    GC_FLAGS                  Additional compiler flags
    PARALLELISM               Build parallelism

EXAMPLES:
    # Basic build
    $0

    # Build with benchmarks
    $0 --bench

    # Cross-compile for ARM64
    $0 --cross --target-os linux --target-arch arm64

    # Build with custom tags
    $0 --tags "custom1,custom2"

    # Verbose build with custom flags
    $0 --verbose --ldflags "-s -w" --gcflags "-N -l"

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -o|--output)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            -j|--jit)
                ENABLE_JIT="$2"
                shift 2
                ;;
            -s|--simd)
                ENABLE_SIMD="$2"
                shift 2
                ;;
            -t|--tests)
                ENABLE_TESTS="$2"
                shift 2
                ;;
            -b|--bench)
                ENABLE_BENCH="$2"
                shift 2
                ;;
            -v|--verbose)
                VERBOSE="$2"
                shift 2
                ;;
            -c|--cross)
                CROSS_COMPILE="$2"
                shift 2
                ;;
            --target-os)
                TARGET_OS="$2"
                shift 2
                ;;
            --target-arch)
                TARGET_ARCH="$2"
                shift 2
                ;;
            --tags)
                BUILD_TAGS="$2"
                shift 2
                ;;
            --ldflags)
                LD_FLAGS="$2"
                shift 2
                ;;
            --gcflags)
                GC_FLAGS="$2"
                shift 2
                ;;
            -p|--parallelism)
                PARALLELISM="$2"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check Go version
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi

    GO_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | sed 's/go//')
    if [[ $(echo "$GO_VERSION < 1.20" | bc -l) -eq 1 ]] || [[ $(echo "$GO_VERSION >= 1.26" | bc -l) -eq 1 ]]; then
        log_error "Go version $GO_VERSION is not supported. Require Go 1.20-1.25"
        exit 1
    fi

    # Check Git
    if ! command -v git &> /dev/null; then
        log_error "Git is not installed or not in PATH"
        exit 1
    fi

    # Check if we're on ARM64 or cross-compiling
    CURRENT_ARCH=$(go env GOARCH)
    if [[ "$CURRENT_ARCH" != "arm64" && "$CROSS_COMPILE" != "true" ]]; then
        log_error "This script must be run on ARM64 or with --cross flag"
        exit 1
    fi

    log_success "Prerequisites check passed"
}

# Prepare build environment
prepare_environment() {
    log_info "Preparing build environment..."

    # Create output directory
    mkdir -p "$OUTPUT_DIR"

    # Set environment variables
    export GOOS="$TARGET_OS"
    export GOARCH="$TARGET_ARCH"
    export GOMAXPROCS="$PARALLELISM"

    if [[ "$ENABLE_JIT" == "true" ]]; then
        export SONIC_JIT_ENABLED=1
        export SONIC_ARM64_JIT=1
    fi

    if [[ "$ENABLE_SIMD" == "true" ]]; then
        export SONIC_SIMD_ENABLED=1
        export SONIC_ARM64_NEON=1
    fi

    log_success "Build environment prepared"
}

# Generate build tags
generate_build_tags() {
    local tags="arm64,go1.20,!go1.26"

    if [[ "$ENABLE_JIT" == "true" ]]; then
        tags="$tags,arm64_jit,sonic_jit"
    fi

    if [[ "$ENABLE_SIMD" == "true" ]]; then
        tags="$tags,arm64_simd,arm64_neon"
    fi

    if [[ -n "$BUILD_TAGS" ]]; then
        tags="$tags,$BUILD_TAGS"
    fi

    echo "$tags"
}

# Build library
build_library() {
    log_info "Building sonic library..."

    local tags=$(generate_build_tags)
    local build_args=()

    build_args+=("build")
    build_args+=("-v")
    build_args+=("-tags" "$tags")
    build_args+=("-p" "$PARALLELISM")

    if [[ -n "$GC_FLAGS" ]]; then
        build_args+=("-gcflags" "$GC_FLAGS")
    fi

    if [[ -n "$LD_FLAGS" ]]; then
        build_args+=("-ldflags" "$LD_FLAGS")
    fi

    if [[ -n "$OUTPUT_DIR" ]]; then
        build_args+=("-o" "$OUTPUT_DIR/sonic")
    fi

    build_args+=("./...")

    if [[ "$VERBOSE" == "true" ]]; then
        log_info "Build command: go ${build_args[*]}"
    fi

    if go "${build_args[@]}"; then
        log_success "Library build completed"
    else
        log_error "Library build failed"
        exit 1
    fi
}

# Run tests
run_tests() {
    if [[ "$ENABLE_TESTS" != "true" ]]; then
        log_info "Tests disabled, skipping"
        return
    fi

    log_info "Running tests..."

    local tags=$(generate_build_tags)
    local test_args=()

    test_args+=("test")
    test_args+=("-v")
    test_args+=("-tags" "$tags")
    test_args+=("-timeout" "30m")
    test_args+=("-p" "$PARALLELISM")

    if [[ "$CROSS_COMPILE" != "true" ]]; then
        test_args+=("-race")
    fi

    test_args+=("-coverprofile=$OUTPUT_DIR/coverage.out")
    test_args+=("-covermode=atomic")

    local test_packages=(
        "./..."
        "./internal/..."
        "./internal/jit/arm64/..."
        "./internal/encoder/..."
        "./internal/decoder/..."
    )

    for pkg in "${test_packages[@]}"; do
        log_info "Testing package: $pkg"
        if go "${test_args[@]}" "$pkg"; then
            log_success "Tests passed for $pkg"
        else
            log_error "Tests failed for $pkg"
            exit 1
        fi
    done

    log_success "All tests passed"
}

# Run benchmarks
run_benchmarks() {
    if [[ "$ENABLE_BENCH" != "true" ]]; then
        log_info "Benchmarks disabled, skipping"
        return
    fi

    log_info "Running benchmarks..."

    local tags=$(generate_build_tags)
    local bench_args=()

    bench_args+=("test")
    bench_args+=("-bench=.")
    bench_args+=("-benchmem")
    bench_args+=("-tags" "$tags")
    bench_args+=("-timeout" "1h")
    bench_args+=("-count" "5")
    bench_args+=("-benchtime" "10s")
    bench_args+=("-run=^$")

    local bench_packages=(
        "./internal/jit/arm64/..."
        "./internal/encoder/..."
        "./internal/decoder/..."
    )

    for pkg in "${bench_packages[@]}"; do
        log_info "Benchmarking package: $pkg"
        if go "${bench_args[@]}" "$pkg" | tee "$OUTPUT_DIR/benchmark_$(basename $pkg).txt"; then
            log_success "Benchmarks completed for $pkg"
        else
            log_error "Benchmarks failed for $pkg"
            exit 1
        fi
    done

    # Combine benchmark results
    cat "$OUTPUT_DIR"/benchmark_*.txt > "$OUTPUT_DIR/benchmark.txt"
    log_success "All benchmarks completed"
}

# Generate build artifacts
generate_artifacts() {
    log_info "Generating build artifacts..."

    # Generate version info
    local version_info=$(cat << EOF
{
  "version": "v1.0.0-arm64-jit",
  "go_version": "$(go version)",
  "goos": "$TARGET_OS",
  "goarch": "$TARGET_ARCH",
  "build_time": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "git_commit": "$(git rev-parse HEAD 2>/dev/null || echo 'unknown')",
  "git_branch": "$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'unknown')",
  "features": {
    "arm64_jit": $ENABLE_JIT,
    "arm64_simd": $ENABLE_SIMD,
    "neon_support": $ENABLE_SIMD
  },
  "build_config": {
    "enable_jit": $ENABLE_JIT,
    "enable_simd": $ENABLE_SIMD,
    "enable_tests": $ENABLE_TESTS,
    "enable_bench": $ENABLE_BENCH,
    "cross_compile": $CROSS_COMPILE,
    "build_tags": "$BUILD_TAGS",
    "parallelism": $PARALLELISM
  }
}
EOF
    )

    echo "$version_info" > "$OUTPUT_DIR/version.json"

    # Generate build summary
    local build_summary=$(cat << EOF
ARM64 JIT Enabled Sonic Build Summary
=====================================

Build Configuration:
- Output Directory: $OUTPUT_DIR
- JIT Enabled: $ENABLE_JIT
- SIMD Enabled: $ENABLE_SIMD
- Tests Enabled: $ENABLE_TESTS
- Benchmarks Enabled: $ENABLE_BENCH
- Cross Compile: $CROSS_COMPILE
- Target OS: $TARGET_OS
- Target Arch: $TARGET_ARCH
- Build Tags: $BUILD_TAGS
- Parallelism: $PARALLELISM

Environment:
- Go Version: $(go version)
- Git Commit: $(git rev-parse HEAD 2>/dev/null || echo 'unknown')
- Build Time: $(date -u +%Y-%m-%dT%H:%M:%SZ)

Files Generated:
- $(ls -la "$OUTPUT_DIR" 2>/dev/null || echo "No files found")

EOF
    )

    echo "$build_summary" > "$OUTPUT_DIR/build_summary.txt"

    log_success "Build artifacts generated in $OUTPUT_DIR"
}

# Main build function
main() {
    log_info "Starting ARM64 JIT enabled sonic build..."

    parse_args "$@"
    check_prerequisites
    prepare_environment
    build_library
    run_tests
    run_benchmarks
    generate_artifacts

    log_success "ARM64 JIT enabled sonic build completed successfully!"
    log_info "Build artifacts available in: $OUTPUT_DIR"
    log_info "View build summary: cat $OUTPUT_DIR/build_summary.txt"
}

# Run main function with all arguments
main "$@"