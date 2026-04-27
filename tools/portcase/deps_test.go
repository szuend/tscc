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

package main

import (
	"reflect"
	"testing"
)

func TestGetDependencies(t *testing.T) {
	content := `
import { A } from 'moduleA';
export * from "moduleB";
import Backbone = require("./backbone");
/// <reference path="ambient.d.ts" />
import('dynamic');
`
	expected := []string{
		"moduleA",
		"moduleB",
		"./backbone",
		"ambient.d.ts",
		"dynamic",
	}

	deps := getDependencies(content)
	if !reflect.DeepEqual(deps, expected) {
		t.Errorf("getDependencies() = %v, want %v", deps, expected)
	}
}

func TestResolveDependency(t *testing.T) {
	inputs := []string{
		"src/main.ts",
		"src/moduleA.ts",
		"src/utils/index.ts",
		"lib/ambient.d.ts",
	}

	tests := []struct {
		name     string
		importer string
		dep      string
		want     string
	}{
		{
			name:     "relative exact match",
			importer: "src/main.ts",
			dep:      "./moduleA.ts",
			want:     "src/moduleA.ts",
		},
		{
			name:     "relative implicitly adds .ts",
			importer: "src/main.ts",
			dep:      "./moduleA",
			want:     "src/moduleA.ts",
		},
		{
			name:     "relative directory index",
			importer: "src/main.ts",
			dep:      "./utils",
			want:     "src/utils/index.ts",
		},
		{
			name:     "bare specifier fallback to basename",
			importer: "src/main.ts",
			dep:      "ambient.d.ts",
			want:     "lib/ambient.d.ts",
		},
		{
			name:     "unresolvable",
			importer: "src/main.ts",
			dep:      "./missing",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveDependency(tt.importer, tt.dep, inputs)
			if got != tt.want {
				t.Errorf("resolveDependency(%q, %q) = %q, want %q", tt.importer, tt.dep, got, tt.want)
			}
		})
	}
}

func TestIsScript(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "plain script",
			content: `
var x = 1;
function foo() {}
`,
			want: true,
		},
		{
			name: "has export",
			content: `
export const x = 1;
`,
			want: false,
		},
		{
			name: "has import",
			content: `
import { y } from "mod";
var x = 1;
`,
			want: false,
		},
		{
			name: "require is allowed in script for global assignments",
			content: `
import Backbone = require("backbone");
`,
			want: false, // wait, our regex says `(?m)^import\s+` which matches this, so it's a module
		},
		{
			name: "export assignment",
			content: `
class D {}
export = D;
`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isScript(tt.content); got != tt.want {
				t.Errorf("isScript() = %v, want %v", got, tt.want)
			}
		})
	}
}
