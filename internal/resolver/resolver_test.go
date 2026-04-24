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

// spyFS records every call so tests can assert that Realpath is never invoked.
type spyFS struct {
	files        map[string]bool
	realpathCall bool
	existsCalls  []string
}

func (s *spyFS) UseCaseSensitiveFileNames() bool { return true }
func (s *spyFS) FileExists(p string) bool {
	s.existsCalls = append(s.existsCalls, p)
	return s.files[p]
}
func (s *spyFS) ReadFile(string) (string, bool)             { return "", false }
func (s *spyFS) WriteFile(string, string) error             { return nil }
func (s *spyFS) Remove(string) error                        { return nil }
func (s *spyFS) Chtimes(string, time.Time, time.Time) error { return nil }
func (s *spyFS) DirectoryExists(string) bool                { return false }
func (s *spyFS) GetAccessibleEntries(string) tsccbridge.Entries {
	return tsccbridge.Entries{}
}
func (s *spyFS) Stat(string) fs.FileInfo              { return nil }
func (s *spyFS) WalkDir(string, fs.WalkDirFunc) error { return nil }
func (s *spyFS) Realpath(p string) string {
	s.realpathCall = true
	return p
}

func newResolver(files map[string]bool, paths map[string]string) (*resolver.LiteralResolver, *spyFS) {
	spy := &spyFS{files: files}
	return resolver.New(resolver.Options{FS: spy, Paths: paths}), spy
}

func TestRelativeResolvesTsSource(t *testing.T) {
	r, _ := newResolver(map[string]bool{"/app/b.ts": true}, nil)

	res, _ := r.ResolveModuleName("./b.ts", "/app/a.ts", 0, nil)
	if res == nil || res.ResolvedFileName != "/app/b.ts" {
		t.Fatalf("expected /app/b.ts, got %+v", res)
	}
	if res.Extension != ".ts" {
		t.Errorf("Extension: got %q, want .ts", res.Extension)
	}
}

func TestRelativeSubstitutesJsForTs(t *testing.T) {
	// Idiomatic TS: user writes ".js" import; real source is .ts.
	r, _ := newResolver(map[string]bool{"/app/b.ts": true}, nil)

	res, _ := r.ResolveModuleName("./b.js", "/app/a.ts", 0, nil)
	if res == nil || res.ResolvedFileName != "/app/b.ts" {
		t.Fatalf("expected /app/b.ts, got %+v", res)
	}
	if res.Extension != ".ts" {
		t.Errorf("Extension: got %q, want .ts", res.Extension)
	}
}

func TestRelativeExtensionless(t *testing.T) {
	r, _ := newResolver(map[string]bool{"/app/b.tsx": true}, nil)
	res, _ := r.ResolveModuleName("./b", "/app/a.ts", 0, nil)
	if res == nil || res.ResolvedFileName != "/app/b.tsx" {
		t.Fatalf("expected /app/b.tsx, got %+v", res)
	}
}

func TestRelativeMissingReturnsUnresolvedMarker(t *testing.T) {
	// Unresolved returns must be a non-nil zero-valued marker so the
	// typescript-go includeProcessor's iteration does not dereference nil.
	r, _ := newResolver(map[string]bool{}, nil)
	res, _ := r.ResolveModuleName("./missing.js", "/app/a.ts", 0, nil)
	if res == nil {
		t.Fatalf("expected non-nil marker, got nil")
	}
	if res.IsResolved() {
		t.Fatalf("expected IsResolved()=false, got %+v", res)
	}
}

func TestRelativeDTS(t *testing.T) {
	r, _ := newResolver(map[string]bool{"/app/types.d.ts": true}, nil)
	res, _ := r.ResolveModuleName("./types", "/app/a.ts", 0, nil)
	if res == nil || res.ResolvedFileName != "/app/types.d.ts" {
		t.Fatalf("expected /app/types.d.ts, got %+v", res)
	}
}

func TestRelativeParentWalk(t *testing.T) {
	// ../sibling.ts — normalized to a sibling dir.
	r, _ := newResolver(map[string]bool{"/proj/sibling.ts": true}, nil)
	res, _ := r.ResolveModuleName("../sibling.js", "/proj/src/a.ts", 0, nil)
	if res == nil || res.ResolvedFileName != "/proj/sibling.ts" {
		t.Fatalf("expected /proj/sibling.ts, got %+v", res)
	}
}

func TestAbsoluteResolution(t *testing.T) {
	r, _ := newResolver(map[string]bool{"/vendor/lib.d.ts": true}, nil)
	res, _ := r.ResolveModuleName("/vendor/lib.d.ts", "/app/a.ts", 0, nil)
	if res == nil || res.ResolvedFileName != "/vendor/lib.d.ts" {
		t.Fatalf("expected /vendor/lib.d.ts, got %+v", res)
	}
}

func TestBareSpecifierDeniedWithoutPath(t *testing.T) {
	r, _ := newResolver(map[string]bool{"/vendor/lodash/index.d.ts": true}, nil)
	res, _ := r.ResolveModuleName("lodash", "/app/a.ts", 0, nil)
	if res == nil || res.IsResolved() {
		t.Fatalf("bare specifier without --path must be unresolved, got %+v", res)
	}
}

func TestBareSpecifierResolvesViaPathMap(t *testing.T) {
	r, _ := newResolver(
		map[string]bool{"/vendor/lodash/index.d.ts": true},
		map[string]string{"lodash": "/vendor/lodash/index.d.ts"},
	)
	res, _ := r.ResolveModuleName("lodash", "/app/a.ts", 0, nil)
	if res == nil || res.ResolvedFileName != "/vendor/lodash/index.d.ts" {
		t.Fatalf("expected /vendor/lodash/index.d.ts, got %+v", res)
	}
}

func TestBareSpecifierWithComplexPath(t *testing.T) {
	r, _ := newResolver(
		map[string]bool{"/vendor/typescript/lib/typescript.d.ts": true},
		map[string]string{"typescript": "/vendor/typescript/lib/typescript.d.ts"},
	)
	res, _ := r.ResolveModuleName("typescript", "/app/a.ts", 0, nil)
	if res == nil || res.ResolvedFileName != "/vendor/typescript/lib/typescript.d.ts" {
		t.Fatalf("expected /vendor/typescript/lib/typescript.d.ts, got %+v", res)
	}
}
func TestNeverCallsRealpath(t *testing.T) {
	r, spy := newResolver(
		map[string]bool{
			"/app/b.ts":                 true,
			"/vendor/lodash/index.d.ts": true,
			"/vendor/types/index.d.ts":  true,
		},
		map[string]string{
			"lodash": "/vendor/lodash/index.d.ts",
			"types":  "/vendor/types/index.d.ts",
		},
	)

	// Exercise every path — relative-with-substitution, absolute, bare, type ref.
	r.ResolveModuleName("./b.js", "/app/a.ts", 0, nil)
	r.ResolveModuleName("/vendor/lodash/index.d.ts", "/app/a.ts", 0, nil)
	r.ResolveModuleName("lodash", "/app/a.ts", 0, nil)
	r.ResolveTypeReferenceDirective("types", "/app/a.ts", 0, nil)
	r.GetPackageScopeForPath("/app")

	if spy.realpathCall {
		t.Fatalf("Realpath must never be called; it was")
	}
}

func TestGetPackageScopeReturnsNil(t *testing.T) {
	r, _ := newResolver(nil, nil)
	if scope := r.GetPackageScopeForPath("/app"); scope != nil {
		t.Errorf("expected nil scope, got %v", scope)
	}
}

func TestTypeReferenceDeniedWithoutMapping(t *testing.T) {
	r, _ := newResolver(map[string]bool{}, nil)
	res, _ := r.ResolveTypeReferenceDirective("node", "/app/a.ts", 0, nil)
	if res != nil {
		t.Fatalf("type ref without --path must be unresolved, got %+v", res)
	}
}

func TestTypeReferenceViaPathMap(t *testing.T) {
	r, _ := newResolver(
		map[string]bool{"/vendor/@types/node/index.d.ts": true},
		map[string]string{"node": "/vendor/@types/node/index.d.ts"},
	)
	res, _ := r.ResolveTypeReferenceDirective("node", "/app/a.ts", 0, nil)
	if res == nil || res.ResolvedFileName != "/vendor/@types/node/index.d.ts" {
		t.Fatalf("expected /vendor/@types/node/index.d.ts, got %+v", res)
	}
}
