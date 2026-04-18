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

package compilehost_test

import (
	"io/fs"
	"testing"
	"time"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/compilehost"
)

// stubFS is a minimal vfs.FS whose identity we track via name.
type stubFS struct{ name string }

func (stubFS) UseCaseSensitiveFileNames() bool                          { return true }
func (stubFS) FileExists(string) bool                                   { return false }
func (stubFS) ReadFile(string) (string, bool)                           { return "", false }
func (stubFS) WriteFile(string, string) error                           { return nil }
func (stubFS) Remove(string) error                                      { return nil }
func (stubFS) Chtimes(string, time.Time, time.Time) error               { return nil }
func (stubFS) DirectoryExists(string) bool                              { return false }
func (stubFS) GetAccessibleEntries(string) tsccbridge.Entries           { return tsccbridge.Entries{} }
func (stubFS) Stat(string) fs.FileInfo                                  { return nil }
func (stubFS) WalkDir(string, fs.WalkDirFunc) error                     { return nil }
func (stubFS) Realpath(p string) string                                 { return p }

// TestNewWiresJailedFSForDiscovery verifies the dual-FS contract from design §7:
// the CompilerHost exposes the jailed FS — not the raw FS — to every caller that
// reaches it through FS(). GetSourceFile's routing to rawFS is exercised end-to-end
// by internal/compile/TestCompile_PathMapUnlocksBareImport_AcrossJailedDir.
func TestNewWiresJailedFSForDiscovery(t *testing.T) {
	jailed := stubFS{name: "jailed"}
	raw := stubFS{name: "raw"}
	h := compilehost.New(compilehost.Options{
		CurrentDirectory:   "/work",
		JailedFS:           jailed,
		RawFS:              raw,
		DefaultLibraryPath: "/libs",
	})

	if got := h.FS().(stubFS).name; got != "jailed" {
		t.Errorf("FS() = %q, want jailed — discovery must not see rawFS", got)
	}
	if got := h.GetCurrentDirectory(); got != "/work" {
		t.Errorf("GetCurrentDirectory() = %q, want /work", got)
	}
	if got := h.DefaultLibraryPath(); got != "/libs" {
		t.Errorf("DefaultLibraryPath() = %q, want /libs", got)
	}
}
