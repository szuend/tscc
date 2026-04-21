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

package paths

import (
	"testing"
)

func TestIsJSOutput(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"js", "file.js", true},
		{"mjs", "file.mjs", true},
		{"cjs", "file.cjs", true},
		{"absolute js", "/foo/bar/file.js", true},
		{"jsx is excluded", "file.jsx", false},
		{"ts is excluded", "file.ts", false},
		{"d.ts is excluded", "file.d.ts", false},
		{"no extension", "file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsJSOutput(tt.path); got != tt.want {
				t.Errorf("IsJSOutput(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsDtsOutput(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"d.ts", "file.d.ts", true},
		{"d.mts", "file.d.mts", true},
		{"d.cts", "file.d.cts", true},
		{"absolute d.ts", "/foo/bar/file.d.ts", true},
		{"ts is excluded", "file.ts", false},
		{"js is excluded", "file.js", false},
		{"no extension", "file", false},
		{"almost d.ts", "file_d.ts", false}, // filepath.Ext would be .ts
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDtsOutput(tt.path); got != tt.want {
				t.Errorf("IsDtsOutput(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsMapOutput(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"js.map", "file.js.map", true},
		{"mjs.map", "file.mjs.map", true},
		{"cjs.map", "file.cjs.map", true},
		{"absolute js.map", "/foo/bar/file.js.map", true},
		{"d.ts.map is excluded", "file.d.ts.map", false},
		{"js is excluded", "file.js", false},
		{"no extension", "file", false},
		{"almost js.map", "file_js.map", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMapOutput(tt.path); got != tt.want {
				t.Errorf("IsMapOutput(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestStripExt(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"simple ts", "file.ts", "file"},
		{"simple js", "file.js", "file"},
		{"d.ts", "file.d.ts", "file"},
		{"d.mts", "file.d.mts", "file"},
		{"d.cts", "file.d.cts", "file"},
		{"absolute path ts", "/a/b/file.ts", "/a/b/file"},
		{"absolute path d.ts", "/a/b/file.d.ts", "/a/b/file"},
		{"js.map", "file.js.map", "file"},
		{"mjs.map", "file.mjs.map", "file"},
		{"cjs.map", "file.cjs.map", "file"},
		{"multiple dots", "my.file.name.ts", "my.file.name"},
		{"multiple dots d.ts", "my.file.name.d.ts", "my.file.name"},
		{"no extension", "file", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripExt(tt.path); got != tt.want {
				t.Errorf("StripExt(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
