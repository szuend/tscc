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
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/szuend/tscc/internal/hermeticfs"
)

func TestHermeticFS_Blocked(t *testing.T) {
	inner := &mockFS{
		files: map[string]string{
			"/app/package.json":                  "{}",
			"/app/tsconfig.json":                 "{}",
			"/app/jsconfig.json":                 "{}",
			"/app/src/index.ts":                  "const x = 1;",
			"/app/node_modules/foo/index.js":     "",
			"/app/bower_components/bar/index.js": "",
			"/app/jspm_packages/baz/index.js":    "",
		},
		dirs: map[string][]mockEntry{
			"/app":                  {{name: "src", isDir: true}, {name: "node_modules", isDir: true}, {name: "package.json"}},
			"/app/src":              {{name: "index.ts"}},
			"/app/node_modules":     {{name: "foo", isDir: true}},
			"/app/node_modules/foo": {{name: "index.js"}},
			"/app/bower_components": {{name: "bar", isDir: true}},
			"/app/jspm_packages":    {{name: "baz", isDir: true}},
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
		"/app/jspm_packages/baz/index.js",
	}

	for _, p := range blockedFiles {
		if hfs.FileExists(p) {
			t.Errorf("FileExists(%q) should be blocked", p)
		}
		if _, ok := hfs.ReadFile(p); ok {
			t.Errorf("ReadFile(%q) should be blocked", p)
		}
		if info := hfs.Stat(p); info != nil {
			t.Errorf("Stat(%q) should return nil, got %v", p, info)
		}
	}

	blockedDirs := []string{
		"/app/node_modules",
		"/app/bower_components",
		"/app/jspm_packages",
	}
	for _, d := range blockedDirs {
		if hfs.DirectoryExists(d) {
			t.Errorf("DirectoryExists(%q) should be blocked", d)
		}
		if err := hfs.WalkDir(d, func(string, fs.DirEntry, error) error { return nil }); err == nil {
			t.Errorf("WalkDir(%q) should fail, got nil", d)
		}
	}

	// GetAccessibleEntries on /app must filter out blocked siblings.
	entries := hfs.GetAccessibleEntries("/app")
	for _, f := range entries.Files {
		if f == "package.json" {
			t.Errorf("GetAccessibleEntries(/app) leaked %q", f)
		}
	}
	for _, d := range entries.Directories {
		if d == "node_modules" {
			t.Errorf("GetAccessibleEntries(/app) leaked %q", d)
		}
	}

	// The whitelisted TS file must still be fully visible.
	if !hfs.FileExists("/app/src/index.ts") {
		t.Errorf("/app/src/index.ts should be visible")
	}
	if _, ok := hfs.ReadFile("/app/src/index.ts"); !ok {
		t.Errorf("/app/src/index.ts should be readable")
	}
}

func TestHermeticFS_WalkDirSkipsBlocked(t *testing.T) {
	inner := &mockFS{
		files: map[string]string{
			"/app/a.ts":                      "",
			"/app/package.json":              "{}",
			"/app/node_modules/foo/index.js": "",
		},
		dirs: map[string][]mockEntry{
			"/app":                  {{name: "a.ts"}, {name: "package.json"}, {name: "node_modules", isDir: true}},
			"/app/node_modules":     {{name: "foo", isDir: true}},
			"/app/node_modules/foo": {{name: "index.js"}},
		},
	}
	hfs := hermeticfs.New(hermeticfs.Options{Inner: inner, CaseSensitivePaths: true})

	var visited []string
	err := hfs.WalkDir("/app", func(p string, d fs.DirEntry, err error) error {
		visited = append(visited, p)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir error: %v", err)
	}

	for _, p := range visited {
		if p == "/app/package.json" {
			t.Errorf("WalkDir surfaced blocked file %q", p)
		}
		if p == "/app/node_modules" || p == "/app/node_modules/foo" || p == "/app/node_modules/foo/index.js" {
			t.Errorf("WalkDir descended into blocked tree at %q", p)
		}
	}
}

func TestHermeticFS_StatZeroesMtime(t *testing.T) {
	inner := &mockFS{
		files: map[string]string{"/app/a.ts": ""},
		mtime: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	hfs := hermeticfs.New(hermeticfs.Options{Inner: inner, CaseSensitivePaths: true})

	info := hfs.Stat("/app/a.ts")
	if info == nil {
		t.Fatal("Stat returned nil for allowed path")
	}
	if got := info.ModTime(); !got.Equal(time.Unix(0, 0)) {
		t.Errorf("ModTime = %v, want Unix epoch", got)
	}
	if info.Size() != 0 {
		t.Errorf("Size forwarded incorrectly: got %d", info.Size())
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
	hfs.ReadFile("/app/a.ts") // duplicate — should not re-appear

	reads := hfs.Reads()
	if len(reads) != 2 || reads[0] != "/app/a.ts" || reads[1] != "/app/b.ts" {
		t.Errorf("unexpected reads: %v", reads)
	}
}

func TestHermeticFS_ReadsConcurrent(t *testing.T) {
	inner := &mockFS{files: map[string]string{}}
	for i := range 100 {
		inner.files["/app/f"+strconv.Itoa(i)+".ts"] = ""
	}
	hfs := hermeticfs.New(hermeticfs.Options{Inner: inner, CaseSensitivePaths: true})

	var wg sync.WaitGroup
	wg.Add(100)
	for i := range 100 {
		go func() {
			defer wg.Done()
			hfs.ReadFile("/app/f" + strconv.Itoa(i) + ".ts")
		}()
	}
	wg.Wait()

	if len(hfs.Reads()) != 100 {
		t.Errorf("expected 100 unique reads, got %d", len(hfs.Reads()))
	}
}

func TestHermeticFS_Realpath(t *testing.T) {
	inner := &mockFS{}
	hfs := hermeticfs.New(hermeticfs.Options{Inner: inner})

	if res := hfs.Realpath("/app/foo"); res != "/app/foo" {
		t.Errorf("Realpath should be identity, got %q", res)
	}
}

func TestHermeticFS_CaseSensitive(t *testing.T) {
	inner := &mockFS{caseSensitive: false}
	hfs := hermeticfs.New(hermeticfs.Options{
		Inner:              inner,
		CaseSensitivePaths: true, // must override inner
	})

	if !hfs.UseCaseSensitiveFileNames() {
		t.Errorf("UseCaseSensitiveFileNames should follow Options, not inner")
	}
}
