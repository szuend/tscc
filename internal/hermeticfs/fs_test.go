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

package hermeticfs_test

import (
	"io/fs"
	"testing"
	"time"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/hermeticfs"
)

type mockFS struct {
	files         map[string]string
	dirs          map[string]bool
	caseSensitive bool
}

func (m *mockFS) UseCaseSensitiveFileNames() bool { return m.caseSensitive }
func (m *mockFS) FileExists(path string) bool     { _, ok := m.files[path]; return ok }
func (m *mockFS) ReadFile(path string) (string, bool) {
	v, ok := m.files[path]
	return v, ok
}
func (m *mockFS) WriteFile(path string, data string) error                    { return nil }
func (m *mockFS) Remove(path string) error                                    { return nil }
func (m *mockFS) Chtimes(path string, aTime time.Time, mTime time.Time) error { return nil }
func (m *mockFS) DirectoryExists(path string) bool                            { _, ok := m.dirs[path]; return ok }
func (m *mockFS) GetAccessibleEntries(path string) tsccbridge.Entries {
	return tsccbridge.Entries{}
}
func (m *mockFS) Stat(path string) fs.FileInfo                     { return nil }
func (m *mockFS) WalkDir(root string, walkFn fs.WalkDirFunc) error { return nil }
func (m *mockFS) Realpath(path string) string                      { return path + "_real" }

func TestHermeticFS_Blocked(t *testing.T) {
	inner := &mockFS{
		files: map[string]string{
			"/app/package.json":                  "{}",
			"/app/tsconfig.json":                 "{}",
			"/app/jsconfig.json":                 "{}",
			"/app/src/index.ts":                  "const x = 1;",
			"/app/node_modules/foo/index.js":     "",
			"/app/bower_components/bar/index.js": "",
		},
		dirs: map[string]bool{
			"/app":              true,
			"/app/node_modules": true,
		},
	}

	hfs := hermeticfs.New(hermeticfs.Options{
		Inner:              inner,
		CaseSensitivePaths: true,
	})

	blockedFiles := []string{
		"/app/package.json",
		"/app/tsconfig.json",
		"/app/jsconfig.json",
		"/app/node_modules/foo/index.js",
		"/app/bower_components/bar/index.js",
	}

	for _, p := range blockedFiles {
		if hfs.FileExists(p) {
			t.Errorf("expected %q to be blocked (FileExists)", p)
		}
		if _, ok := hfs.ReadFile(p); ok {
			t.Errorf("expected %q to be blocked (ReadFile)", p)
		}
	}

	if hfs.DirectoryExists("/app/node_modules") {
		t.Errorf("expected /app/node_modules to be blocked (DirectoryExists)")
	}

	if !hfs.FileExists("/app/src/index.ts") {
		t.Errorf("expected /app/src/index.ts to be allowed")
	}

	if _, ok := hfs.ReadFile("/app/src/index.ts"); !ok {
		t.Errorf("expected /app/src/index.ts to be readable")
	}
}

func TestHermeticFS_Reads(t *testing.T) {
	inner := &mockFS{
		files: map[string]string{
			"/app/a.ts": "",
			"/app/b.ts": "",
		},
	}

	hfs := hermeticfs.New(hermeticfs.Options{
		Inner:              inner,
		CaseSensitivePaths: true,
	})

	hfs.ReadFile("/app/a.ts")
	hfs.ReadFile("/app/b.ts")
	hfs.ReadFile("/app/a.ts") // Duplicate

	reads := hfs.Reads()
	if len(reads) != 2 || reads[0] != "/app/a.ts" || reads[1] != "/app/b.ts" {
		t.Errorf("unexpected reads: %v", reads)
	}
}

func TestHermeticFS_Realpath(t *testing.T) {
	inner := &mockFS{}
	hfs := hermeticfs.New(hermeticfs.Options{Inner: inner})

	if res := hfs.Realpath("/app/foo"); res != "/app/foo" {
		t.Errorf("expected Realpath to be identity, got %q", res)
	}
}

func TestHermeticFS_CaseSensitive(t *testing.T) {
	inner := &mockFS{caseSensitive: false}
	hfs := hermeticfs.New(hermeticfs.Options{
		Inner:              inner,
		CaseSensitivePaths: true, // Should override inner
	})

	if !hfs.UseCaseSensitiveFileNames() {
		t.Errorf("expected UseCaseSensitiveFileNames to be true")
	}
}
