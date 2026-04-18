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

// Package hermeticfs wraps a vfs.FS to enforce the determinism invariants
// from design §6 (see docs/design/02-deterministic-resolution.md):
//
//   - configuration files (package.json, tsconfig.json, jsconfig.json) and
//     package directories (node_modules, bower_components, jspm_packages) are
//     invisible to every discovery method;
//   - Realpath is the identity function;
//   - UseCaseSensitiveFileNames returns a caller-pinned value, never sniffed
//     from the host;
//   - Stat returns a FileInfo whose ModTime() is the Unix epoch, so future
//     upstream reads of mtime cannot leak host timestamps into output;
//   - successful ReadFile calls are recorded in first-seen order so the
//     depsfile feature can enumerate inputs.
package hermeticfs

import (
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/microsoft/typescript-go/tsccbridge"
)

// FS extends tsccbridge.FS with a Reads accessor used to build a depsfile.
type FS interface {
	tsccbridge.FS
	// Reads returns absolute paths of every successful ReadFile, in
	// first-seen order, deduplicated.
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

// New wraps inner with the hermetic policy described above.
func New(opts Options) FS {
	return &hermeticFS{
		inner:         opts.Inner,
		caseSensitive: opts.CaseSensitivePaths,
		seen:          make(map[string]bool),
	}
}

// isBlocked reports whether p names a configuration file or lives under a
// package directory. typescript-go normalizes paths to forward slashes, so a
// base check plus a prefix/substring pass is correct and allocation-free.
func isBlocked(p string) bool {
	switch path.Base(p) {
	case "package.json", "tsconfig.json", "jsconfig.json":
		return true
	}
	for _, dir := range []string{"node_modules", "bower_components", "jspm_packages"} {
		if containsPathSegment(p, dir) {
			return true
		}
	}
	return false
}

// containsPathSegment reports whether seg appears as a whole slash-delimited
// segment of p (i.e. "/seg/", "seg/" at the start, or "/seg" at the end).
func containsPathSegment(p, seg string) bool {
	if p == seg {
		return true
	}
	if strings.HasPrefix(p, seg+"/") {
		return true
	}
	if strings.HasSuffix(p, "/"+seg) {
		return true
	}
	return strings.Contains(p, "/"+seg+"/")
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

func (f *hermeticFS) FileExists(p string) bool {
	if isBlocked(p) {
		return false
	}
	return f.inner.FileExists(p)
}

func (f *hermeticFS) ReadFile(p string) (string, bool) {
	if isBlocked(p) {
		return "", false
	}
	content, ok := f.inner.ReadFile(p)
	if ok {
		f.mu.Lock()
		if !f.seen[p] {
			f.seen[p] = true
			f.reads = append(f.reads, p)
		}
		f.mu.Unlock()
	}
	return content, ok
}

// WriteFile, Remove, and Chtimes pass through — the jail polices reads, not
// writes. The build system controls where output lands via CLI flags.
func (f *hermeticFS) WriteFile(p string, data string) error { return f.inner.WriteFile(p, data) }
func (f *hermeticFS) Remove(p string) error                 { return f.inner.Remove(p) }
func (f *hermeticFS) Chtimes(p string, aTime time.Time, mTime time.Time) error {
	return f.inner.Chtimes(p, aTime, mTime)
}

func (f *hermeticFS) DirectoryExists(p string) bool {
	if isBlocked(p) {
		return false
	}
	return f.inner.DirectoryExists(p)
}

func (f *hermeticFS) GetAccessibleEntries(p string) tsccbridge.Entries {
	if isBlocked(p) {
		return tsccbridge.Entries{}
	}
	entries := f.inner.GetAccessibleEntries(p)

	var filtered tsccbridge.Entries
	for _, file := range entries.Files {
		if !isBlocked(path.Join(p, file)) {
			filtered.Files = append(filtered.Files, file)
		}
	}
	for _, dir := range entries.Directories {
		if !isBlocked(path.Join(p, dir)) {
			filtered.Directories = append(filtered.Directories, dir)
		}
	}
	return filtered
}

// Stat returns nil for blocked paths. For allowed paths it wraps the inner
// FileInfo so ModTime reports the Unix epoch; every other FileInfo field is
// forwarded. The compiler hot path does not currently read ModTime, but
// upstream adding such a read (tracing, logging, a future cache) would
// otherwise silently leak host timestamps into output.
func (f *hermeticFS) Stat(p string) fs.FileInfo {
	if isBlocked(p) {
		return nil
	}
	info := f.inner.Stat(p)
	if info == nil {
		return nil
	}
	return zeroMtimeFileInfo{info}
}

func (f *hermeticFS) WalkDir(root string, walkFn fs.WalkDirFunc) error {
	if isBlocked(root) {
		return os.ErrNotExist
	}
	return f.inner.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if isBlocked(p) {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		return walkFn(p, d, err)
	})
}

// Realpath is the identity function. Symlinks are not followed; see design §6
// for the rationale (preventing host paths from leaking through canonicalization,
// and matching clang's -ffile-prefix-map semantics on literal paths).
func (f *hermeticFS) Realpath(p string) string { return p }

// zeroMtimeFileInfo wraps a FileInfo and reports ModTime as the Unix epoch.
// Every other field delegates to the wrapped value.
type zeroMtimeFileInfo struct{ fs.FileInfo }

func (zeroMtimeFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
