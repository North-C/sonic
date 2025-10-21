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

// Build script for ARM64 JIT enabled sonic
// This script configures and builds sonic with ARM64 JIT support

// +build ignore

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	outputDir      = flag.String("output", "", "Output directory for built binaries")
	enableJIT      = flag.Bool("jit", true, "Enable ARM64 JIT compilation")
	enableSIMD     = flag.Bool("simd", true, "Enable ARM64 SIMD optimizations")
	enableTests    = flag.Bool("tests", true, "Build and run tests")
	enableBench    = flag.Bool("bench", false, "Run benchmarks")
	verbose        = flag.Bool("verbose", false, "Verbose build output")
	crossCompile   = flag.Bool("cross", false, "Cross-compile for ARM64")
	targetOS       = flag.String("os", "linux", "Target OS for cross-compilation")
	targetArch     = flag.String("arch", "arm64", "Target architecture")
	buildTags      = flag.String("tags", "", "Additional build tags")
	ldFlags        = flag.String("ldflags", "", "Additional linker flags")
	gcFlags        = flag.String("gcflags", "", "Additional compiler flags")
)

func main() {
	flag.Parse()

	if runtime.GOARCH != "arm64" && !*crossCompile {
		log.Fatal("This build script must be run on ARM64 or with -cross flag")
	}

	// Initialize build configuration
	config := BuildConfig{
		OutputDir:    *outputDir,
		EnableJIT:    *enableJIT,
		EnableSIMD:   *enableSIMD,
		EnableTests:  *enableTests,
		EnableBench:  *enableBench,
		Verbose:      *verbose,
		CrossCompile: *crossCompile,
		TargetOS:     *targetOS,
		TargetArch:   *targetArch,
		BuildTags:    *buildTags,
		LDFlags:      *ldFlags,
		GCFlags:      *gcFlags,
	}

	// Execute build pipeline
	if err := BuildSonicARM64(config); err != nil {
		log.Fatalf("Build failed: %v", err)
	}

	fmt.Println("ARM64 JIT enabled sonic build completed successfully!")
}

// BuildConfig contains build configuration options
type BuildConfig struct {
	OutputDir    string
	EnableJIT    bool
	EnableSIMD   bool
	EnableTests  bool
	EnableBench  bool
	Verbose      bool
	CrossCompile bool
	TargetOS     string
	TargetArch   string
	BuildTags    string
	LDFlags      string
	GCFlags      string
}

// BuildSonicARM64 performs the complete build process
func BuildSonicARM64(config BuildConfig) error {
	fmt.Println("Building ARM64 JIT enabled sonic...")

	// Check prerequisites
	if err := checkPrerequisites(config); err != nil {
		return fmt.Errorf("prerequisite check failed: %v", err)
	}

	// Prepare build environment
	if err := prepareBuildEnvironment(config); err != nil {
		return fmt.Errorf("environment preparation failed: %v", err)
	}

	// Generate build tags
	tags := generateBuildTags(config)

	// Build main library
	if err := buildLibrary(config, tags); err != nil {
		return fmt.Errorf("library build failed: %v", err)
	}

	// Run tests if requested
	if config.EnableTests {
		if err := runTests(config, tags); err != nil {
			return fmt.Errorf("tests failed: %v", err)
		}
	}

	// Run benchmarks if requested
	if config.EnableBench {
		if err := runBenchmarks(config, tags); err != nil {
			return fmt.Errorf("benchmarks failed: %v", err)
		}
	}

	// Generate build artifacts
	if err := generateArtifacts(config); err != nil {
		return fmt.Errorf("artifact generation failed: %v", err)
	}

	return nil
}

// checkPrerequisites verifies build environment
func checkPrerequisites(config BuildConfig) error {
	fmt.Println("Checking prerequisites...")

	// Check Go version
	goVersion := runtime.Version()
	if !strings.Contains(goVersion, "go1.20") && !strings.Contains(goVersion, "go1.21") && !strings.Contains(goVersion, "go1.22") && !strings.Contains(goVersion, "go1.23") && !strings.Contains(goVersion, "go1.24") && !strings.Contains(goVersion, "go1.25") {
		return fmt.Errorf("Go version %s is not supported, require Go 1.20-1.25", goVersion)
	}

	// Check for required tools
	requiredTools := []string{"go", "git"}
	for _, tool := range requiredTools {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("required tool not found: %s", tool)
		}
	}

	// Check cross-compilation tools
	if config.CrossCompile {
		if config.TargetOS == "linux" {
			if _, err := exec.LookPath("aarch64-linux-gnu-gcc"); err != nil {
				fmt.Println("Warning: aarch64-linux-gnu-gcc not found, cross-compilation may fail")
			}
		}
	}

	// Check ARM64 JIT prerequisites
	if config.EnableJIT {
		if runtime.GOARCH == "arm64" {
			// Check if we can execute ARM64 instructions
			if err := checkARM64Support(); err != nil {
				return fmt.Errorf("ARM64 JIT prerequisites not met: %v", err)
			}
		}
	}

	fmt.Println("Prerequisites check passed")
	return nil
}

// checkARM64Support verifies ARM64 JIT support
func checkARM64Support() error {
	// This would check for ARM64 JIT support
	// For now, just return nil
	return nil
}

// prepareBuildEnvironment prepares the build environment
func prepareBuildEnvironment(config BuildConfig) error {
	fmt.Println("Preparing build environment...")

	// Create output directory if specified
	if config.OutputDir != "" {
		if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	// Set environment variables
	if config.CrossCompile {
		os.Setenv("GOOS", config.TargetOS)
		os.Setenv("GOARCH", config.TargetArch)
	}

	// Enable JIT environment variables
	if config.EnableJIT {
		os.Setenv("SONIC_JIT_ENABLED", "1")
		os.Setenv("SONIC_ARM64_JIT", "1")
	}

	// Enable SIMD environment variables
	if config.EnableSIMD {
		os.Setenv("SONIC_SIMD_ENABLED", "1")
		os.Setenv("SONIC_ARM64_NEON", "1")
	}

	fmt.Println("Build environment prepared")
	return nil
}

// generateBuildTags generates appropriate build tags
func generateBuildTags(config BuildConfig) []string {
	var tags []string

	// Base tags
	tags = append(tags, "arm64", "go1.20", "!go1.26")

	// JIT tags
	if config.EnableJIT {
		tags = append(tags, "arm64_jit", "sonic_jit")
	}

	// SIMD tags
	if config.EnableSIMD {
		tags = append(tags, "arm64_simd", "arm64_neon")
	}

	// Additional user-specified tags
	if config.BuildTags != "" {
		userTags := strings.Split(config.BuildTags, ",")
		for _, tag := range userTags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	return tags
}

// buildLibrary builds the main sonic library
func buildLibrary(config BuildConfig, tags []string) error {
	fmt.Println("Building sonic library...")

	// Prepare build command
	args := []string{"build", "-v"}

	// Add build tags
	tagStr := strings.Join(tags, ",")
	args = append(args, "-tags", tagStr)

	// Add compiler flags
	if config.GCFlags != "" {
		args = append(args, "-gcflags", config.GCFlags)
	}

	// Add linker flags
	if config.LDFlags != "" {
		args = append(args, "-ldflags", config.LDFlags)
	}

	// Output path
	if config.OutputDir != "" {
		outputPath := filepath.Join(config.OutputDir, "sonic")
		args = append(args, "-o", outputPath)
	}

	// Build target
	args = append(args, "./...")

	// Execute build
	cmd := exec.Command("go", args...)
	if config.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build command failed: %v", err)
	}

	fmt.Println("Library build completed")
	return nil
}

// runTests runs the test suite
func runTests(config BuildConfig, tags []string) error {
	fmt.Println("Running tests...")

	// Prepare test command
	args := []string{"test", "-v"}

	// Add build tags
	tagStr := strings.Join(tags, ",")
	args = append(args, "-tags", tagStr)

	// Add test flags
	if config.Verbose {
		args = append(args, "-v")
	}

	// Test timeout
	args = append(args, "-timeout", "30m")

	// Race detection (only on native builds)
	if !config.CrossCompile {
		args = append(args, "-race")
	}

	// Test coverage
	args = append(args, "-coverprofile=coverage.out")
	args = append(args, "-covermode=atomic")

	// Test packages
	testPackages := []string{
		"./...",
		"./internal/...",
		"./internal/jit/arm64/...",
		"./internal/encoder/...",
		"./internal/decoder/...",
	}

	for _, pkg := range testPackages {
		testArgs := append(args, pkg)
		cmd := exec.Command("go", testArgs...)
		if config.Verbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tests failed for package %s: %v", pkg, err)
		}
	}

	fmt.Println("All tests passed")
	return nil
}

// runBenchmarks runs performance benchmarks
func runBenchmarks(config BuildConfig, tags []string) error {
	fmt.Println("Running benchmarks...")

	// Prepare benchmark command
	args := []string{"test", "-bench=.", "-benchmem"}

	// Add build tags
	tagStr := strings.Join(tags, ",")
	args = append(args, "-tags", tagStr)

	// Benchmark timeout
	args = append(args, "-timeout", "1h")

	// Benchmark output
	args = append(args, "-count", "5")
	args = append(args, "-benchtime", "10s")

	// Output file
	benchFile := "benchmark.txt"
	if config.OutputDir != "" {
		benchFile = filepath.Join(config.OutputDir, "benchmark.txt")
	}
	args = append(args, "-run=^$", "-output", benchFile)

	// Test packages
	benchPackages := []string{
		"./internal/jit/arm64/...",
		"./internal/encoder/...",
		"./internal/decoder/...",
	}

	for _, pkg := range benchPackages {
		benchArgs := append(args, pkg)
		cmd := exec.Command("go", benchArgs...)
		if config.Verbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("benchmarks failed for package %s: %v", pkg, err)
		}
	}

	fmt.Println("Benchmarks completed")
	return nil
}

// generateArtifacts generates build artifacts
func generateArtifacts(config BuildConfig) error {
	fmt.Println("Generating build artifacts...")

	// Create artifacts directory
	artifactsDir := "artifacts"
	if config.OutputDir != "" {
		artifactsDir = config.OutputDir
	}

	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return fmt.Errorf("failed to create artifacts directory: %v", err)
	}

	// Generate version information
	if err := generateVersionInfo(artifactsDir); err != nil {
		return fmt.Errorf("failed to generate version info: %v", err)
	}

	// Generate build manifest
	if err := generateBuildManifest(config, artifactsDir); err != nil {
		return fmt.Errorf("failed to generate build manifest: %v", err)
	}

	// Copy test results
	if _, err := os.Stat("coverage.out"); err == nil {
		coverageDest := filepath.Join(artifactsDir, "coverage.out")
		if err := copyFile("coverage.out", coverageDest); err != nil {
			return fmt.Errorf("failed to copy coverage file: %v", err)
		}
	}

	// Copy benchmark results
	if _, err := os.Stat("benchmark.txt"); err == nil {
		benchDest := filepath.Join(artifactsDir, "benchmark.txt")
		if err := copyFile("benchmark.txt", benchDest); err != nil {
			return fmt.Errorf("failed to copy benchmark file: %v", err)
		}
	}

	fmt.Println("Build artifacts generated")
	return nil
}

// generateVersionInfo generates version information
func generateVersionInfo(outputDir string) error {
	versionInfo := fmt.Sprintf(`{
  "version": "%s",
  "go_version": "%s",
  "goos": "%s",
  "goarch": "%s",
  "build_time": "%s",
  "git_commit": "%s",
  "features": {
    "arm64_jit": true,
    "arm64_simd": true,
    "neon_support": true
  }
}`,
		"v1.0.0-arm64-jit",
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		time.Now().Format(time.RFC3339),
		getGitCommit(),
	)

	versionFile := filepath.Join(outputDir, "version.json")
	return os.WriteFile(versionFile, []byte(versionInfo), 0644)
}

// generateBuildManifest generates build manifest
func generateBuildManifest(config BuildConfig, outputDir string) error {
	manifest := fmt.Sprintf(`{
  "build_config": {
    "output_dir": "%s",
    "enable_jit": %t,
    "enable_simd": %t,
    "enable_tests": %t,
    "enable_bench": %t,
    "verbose": %t,
    "cross_compile": %t,
    "target_os": "%s",
    "target_arch": "%s",
    "build_tags": "%s",
    "ldflags": "%s",
    "gcflags": "%s"
  },
  "build_environment": {
    "go_version": "%s",
    "go_root": "%s",
    "go_path": "%s"
  }
}`,
		config.OutputDir,
		config.EnableJIT,
		config.EnableSIMD,
		config.EnableTests,
		config.EnableBench,
		config.Verbose,
		config.CrossCompile,
		config.TargetOS,
		config.TargetArch,
		config.BuildTags,
		config.LDFlags,
		config.GCFlags,
		runtime.Version(),
		os.Getenv("GOROOT"),
		os.Getenv("GOPATH"),
	)

	manifestFile := filepath.Join(outputDir, "build_manifest.json")
	return os.WriteFile(manifestFile, []byte(manifest), 0644)
}

// getGitCommit returns current git commit hash
func getGitCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}