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

// Package compilehost constructs the dual-FS CompilerHost described in design
// §7: discovery goes through a jailed FS, but GetSourceFile reads resolved
// paths through an unjailed FS so explicit imports of configuration-shaped
// files (e.g. ./package.json with { type: "json" }) still work.
package compilehost

import (
	"github.com/microsoft/typescript-go/tsccbridge"
)

// Options configures a CompilerHost.
type Options struct {
	// CurrentDirectory is the cwd reported to typescript-go. Must be absolute.
	CurrentDirectory string
	// JailedFS fronts discovery: FileExists, Stat, DirectoryExists, WalkDir,
	// GetAccessibleEntries, and ReadFile probes from the resolver all flow here.
	// Production callers wrap an hermeticfs.FS; tests can pass an in-memory FS.
	JailedFS tsccbridge.FS
	// RawFS serves GetSourceFile reads of already-resolved paths. It bypasses
	// the jail so an explicit JSON import works even though the jail blocks
	// discovery of package.json / tsconfig.json.
	RawFS tsccbridge.FS
	// DefaultLibraryPath is the path under which bundled lib.*.d.ts files live.
	DefaultLibraryPath string
}

// New constructs a CompilerHost from opts.
func New(opts Options) tsccbridge.CompilerHost {
	return tsccbridge.NewDualFSHost(opts.CurrentDirectory, opts.JailedFS, opts.RawFS, opts.DefaultLibraryPath)
}
