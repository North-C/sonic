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

package jit

// This file contains ARM64-specific JIT support for Sonic
// It imports and exposes ARM64-specific functionality

//go:noescape

// HasARM64JITSupport indicates if ARM64 JIT compilation is supported
const HasARM64JITSupport = true

// ARM64JITEnabled indicates if ARM64 JIT compilation is enabled
var ARM64JITEnabled = true

// EnableARM64JIT enables ARM64 JIT compilation
func EnableARM64JIT() {
	ARM64JITEnabled = true
}

// DisableARM64JIT disables ARM64 JIT compilation
func DisableARM64JIT() {
	ARM64JITEnabled = false
}

// IsARM64JITEnabled returns true if ARM64 JIT compilation is enabled
func IsARM64JITEnabled() bool {
	return ARM64JITEnabled
}

// ARM64JITVersion returns the version of ARM64 JIT support
const ARM64JITVersion = "1.0.0"