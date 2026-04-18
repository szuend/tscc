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

package resolver_test

import (
	"io/fs"
	"testing"
	"time"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/resolver"
)

type spyFS struct {
	files        map[string]bool
	realpathCall bool
}

func (s *spyFS) UseCaseSensitiveFileNames() bool                             { return true }
func (s *spyFS) FileExists(path string) bool                                 { return s.files[path] }
func (s *spyFS) ReadFile(path string) (string, bool)                         { return "", false }
func (s *spyFS) WriteFile(path string, data string) error                    { return nil }
func (s *spyFS) Remove(path string) error                                    { return nil }
func (s *spyFS) Chtimes(path string, aTime time.Time, mTime time.Time) error { return nil }
func (s *spyFS) DirectoryExists(path string) bool                            { return false }
func (s *spyFS) GetAccessibleEntries(path string) tsccbridge.Entries         { return tsccbridge.Entries{} }
func (s *spyFS) Stat(path string) fs.FileInfo                                { return nil }
func (s *spyFS) WalkDir(root string, walkFn fs.WalkDirFunc) error            { return nil }
func (s *spyFS) Realpath(path string) string {
	s.realpathCall = true
	return path
}

func TestLiteralResolver_NoRealpath(t *testing.T) {
	fs := &spyFS{
		files: map[string]bool{
			"/app/b.ts": true,
		},
	}
	r := resolver.NewLiteralResolver(fs)

	res, _ := r.ResolveModuleName("./b.ts", "/app/a.ts", 0, nil)

	if res == nil {
		t.Fatalf("expected resolution to succeed")
	}
	if res.ResolvedFileName != "/app/b.ts" {
		t.Errorf("expected /app/b.ts, got %q", res.ResolvedFileName)
	}

	if fs.realpathCall {
		t.Errorf("expected Realpath to not be called")
	}
}

func TestLiteralResolver_PackageScope(t *testing.T) {
	r := resolver.NewLiteralResolver(&spyFS{})
	if scope := r.GetPackageScopeForPath("/app"); scope != nil {
		t.Errorf("expected GetPackageScopeForPath to return nil, got %v", scope)
	}
}
