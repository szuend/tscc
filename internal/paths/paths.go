// Copyright 2026 Simon Zünd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package paths provides utility functions for working with TypeScript
// file paths and output variants.
package paths

import (
	"path/filepath"
	"strings"
)

// IsJSOutput matches the JS emit variants eligible for --out-js. .jsx is
// deliberately excluded — emit produces at most one JS file per compile, and
// matching .jsx too would clobber output under a future config that emits
// both. Declaration and source-map outputs are dropped entirely until their
// flags (--out-dts, --out-map) land.
func IsJSOutput(name string) bool {
	switch filepath.Ext(name) {
	case ".js", ".mjs", ".cjs":
		return true
	}
	return false
}

// IsDtsOutput matches declaration file outputs.
func IsDtsOutput(name string) bool {
	switch {
	case strings.HasSuffix(name, ".d.ts"):
		return true
	case strings.HasSuffix(name, ".d.mts"):
		return true
	case strings.HasSuffix(name, ".d.cts"):
		return true
	}
	return false
}

// StripExt removes the final extension from p, yielding the "stem" used to
// match a primary emit against its input file. Comparing stems treats
// /abs/a.ts and /abs/a.js as the same file.
func StripExt(p string) string {
	switch {
	case strings.HasSuffix(p, ".d.ts"):
		return strings.TrimSuffix(p, ".d.ts")
	case strings.HasSuffix(p, ".d.mts"):
		return strings.TrimSuffix(p, ".d.mts")
	case strings.HasSuffix(p, ".d.cts"):
		return strings.TrimSuffix(p, ".d.cts")
	}
	return strings.TrimSuffix(p, filepath.Ext(p))
}
