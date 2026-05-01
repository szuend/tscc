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
	"sort"
	"time"

	"github.com/microsoft/typescript-go/tsccbridge"
)

// mockFS is an in-memory vfs.FS used to drive hermeticfs in tests. It preserves
// directory layout so that WalkDir and GetAccessibleEntries behave realistically.
type mockFS struct {
	files         map[string]string      // path -> contents
	dirs          map[string][]mockEntry // dir path -> entries
	caseSensitive bool
	mtime         time.Time
}

type mockEntry struct {
	name  string
	isDir bool
}

func (m *mockFS) UseCaseSensitiveFileNames() bool { return m.caseSensitive }
func (m *mockFS) FileExists(p string) bool        { _, ok := m.files[p]; return ok }
func (m *mockFS) ReadFile(p string) (string, bool) {
	v, ok := m.files[p]
	return v, ok
}
func (m *mockFS) WriteFile(p string, data string) error                    { return nil }
func (m *mockFS) Remove(p string) error                                    { return nil }
func (m *mockFS) Chtimes(p string, aTime time.Time, mTime time.Time) error { return nil }
func (m *mockFS) DirectoryExists(p string) bool {
	_, ok := m.dirs[p]
	return ok
}
func (m *mockFS) GetAccessibleEntries(p string) tsccbridge.Entries {
	entries, ok := m.dirs[p]
	if !ok {
		return tsccbridge.Entries{}
	}
	var out tsccbridge.Entries
	for _, e := range entries {
		if e.isDir {
			out.Directories = append(out.Directories, e.name)
		} else {
			out.Files = append(out.Files, e.name)
		}
	}
	return out
}
func (m *mockFS) Stat(p string) fs.FileInfo {
	if _, ok := m.files[p]; ok {
		return mockFileInfo{name: p, mode: 0, mtime: m.mtime, size: int64(len(m.files[p]))}
	}
	if _, ok := m.dirs[p]; ok {
		return mockFileInfo{name: p, mode: fs.ModeDir, mtime: m.mtime}
	}
	return nil
}
func (m *mockFS) WalkDir(root string, walkFn fs.WalkDirFunc) error {
	var visit func(dir string) error
	visit = func(dir string) error {
		entries := m.dirs[dir]
		names := make([]string, 0, len(entries))
		kind := make(map[string]bool, len(entries))
		for _, e := range entries {
			names = append(names, e.name)
			kind[e.name] = e.isDir
		}
		sort.Strings(names)
		for _, name := range names {
			child := dir + "/" + name
			d := mockDirEntry{name: name, isDir: kind[name]}
			if err := walkFn(child, d, nil); err != nil {
				if err == fs.SkipDir {
					continue
				}
				return err
			}
			if kind[name] {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if _, ok := m.dirs[root]; !ok {
		return fs.ErrNotExist
	}
	if err := walkFn(root, mockDirEntry{name: root, isDir: true}, nil); err != nil {
		if err == fs.SkipDir {
			return nil
		}
		return err
	}
	return visit(root)
}
func (m *mockFS) Realpath(p string) string { return p + "_real" }

type mockFileInfo struct {
	name  string
	mode  fs.FileMode
	mtime time.Time
	size  int64
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.mtime }
func (m mockFileInfo) IsDir() bool        { return m.mode.IsDir() }
func (m mockFileInfo) Sys() any           { return nil }

type mockDirEntry struct {
	name  string
	isDir bool
}

func (m mockDirEntry) Name() string               { return m.name }
func (m mockDirEntry) IsDir() bool                { return m.isDir }
func (m mockDirEntry) Type() fs.FileMode          { return 0 }
func (m mockDirEntry) Info() (fs.FileInfo, error) { return nil, nil }
