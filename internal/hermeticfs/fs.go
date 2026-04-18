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

package hermeticfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/microsoft/typescript-go/tsccbridge"
)

// FS is a file system that enforces deterministic reads and access controls.
type FS interface {
	tsccbridge.FS
	Reads() []string
}

// Options configures the hermetic FS.
type Options struct {
	Inner              tsccbridge.FS
	CaseSensitivePaths bool
}

type hermeticFS struct {
	inner         tsccbridge.FS
	caseSensitive bool

	mu    sync.Mutex
	reads []string
	seen  map[string]bool
}

// New creates a new hermetic file system.
func New(opts Options) FS {
	return &hermeticFS{
		inner:         opts.Inner,
		caseSensitive: opts.CaseSensitivePaths,
		seen:          make(map[string]bool),
	}
}

func isBlocked(path string) bool {
	base := filepath.Base(path)
	if base == "package.json" || base == "tsconfig.json" || base == "jsconfig.json" {
		return true
	}

	segments := strings.Split(filepath.ToSlash(path), "/")
	for _, s := range segments {
		if s == "node_modules" || s == "bower_components" || s == "jspm_packages" {
			return true
		}
	}
	return false
}

func (f *hermeticFS) Reads() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	res := make([]string, len(f.reads))
	copy(res, f.reads)
	return res
}

func (f *hermeticFS) UseCaseSensitiveFileNames() bool {
	return f.caseSensitive
}

func (f *hermeticFS) FileExists(path string) bool {
	if isBlocked(path) {
		return false
	}
	return f.inner.FileExists(path)
}

func (f *hermeticFS) ReadFile(path string) (string, bool) {
	if isBlocked(path) {
		return "", false
	}
	content, ok := f.inner.ReadFile(path)
	if ok {
		f.mu.Lock()
		if !f.seen[path] {
			f.seen[path] = true
			f.reads = append(f.reads, path)
		}
		f.mu.Unlock()
	}
	return content, ok
}

func (f *hermeticFS) WriteFile(path string, data string) error {
	return f.inner.WriteFile(path, data)
}

func (f *hermeticFS) Remove(path string) error {
	return f.inner.Remove(path)
}

func (f *hermeticFS) Chtimes(path string, aTime time.Time, mTime time.Time) error {
	return f.inner.Chtimes(path, aTime, mTime)
}

func (f *hermeticFS) DirectoryExists(path string) bool {
	if isBlocked(path) {
		return false
	}
	return f.inner.DirectoryExists(path)
}

func (f *hermeticFS) GetAccessibleEntries(path string) tsccbridge.Entries {
	if isBlocked(path) {
		return tsccbridge.Entries{}
	}
	entries := f.inner.GetAccessibleEntries(path)

	var filtered tsccbridge.Entries
	for _, file := range entries.Files {
		if !isBlocked(filepath.Join(path, file)) {
			filtered.Files = append(filtered.Files, file)
		}
	}
	for _, dir := range entries.Directories {
		if !isBlocked(filepath.Join(path, dir)) {
			filtered.Directories = append(filtered.Directories, dir)
		}
	}
	return filtered
}

func (f *hermeticFS) Stat(path string) fs.FileInfo {
	if isBlocked(path) {
		return nil
	}
	return f.inner.Stat(path)
}

func (f *hermeticFS) WalkDir(root string, walkFn fs.WalkDirFunc) error {
	if isBlocked(root) {
		return os.ErrNotExist
	}
	return f.inner.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if isBlocked(path) {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		return walkFn(path, d, err)
	})
}

func (f *hermeticFS) Realpath(path string) string {
	return path
}
