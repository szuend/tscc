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
	"strings"
	"testing"
)

func TestPorter_Port_Simple(t *testing.T) {
	p := Porter{
		CaseName:       "simple",
		TsContent:      "export const x = 1;",
		BaselineFinder: mockFinder("", ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	res := results[0]
	if res.Name != "Simple.txtar" {
		t.Errorf("Expected name Simple.txtar, got %s", res.Name)
	}

	if !strings.Contains(res.Content, "exec tscc --lib es2025,dom --out-js simple.js simple.ts") {
		t.Errorf("Expected content to contain exec tscc --lib es2025,dom --out-js simple.js, got:\n%s", res.Content)
	}
}

func TestPorter_Port_Collision(t *testing.T) {
	js := `//// [b.js]
export const b = 2;
//// [b.js]
export const b = 2;
`
	p := Porter{
		CaseName: "collision_case",
		TsContent: `// @allowJs: true
// @filename: b.js
export const b = 2;
`,
		BaselineFinder: mockFinder(js, ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	res := results[0]
	if !strings.Contains(res.Content, "--out-js out_b.js") {
		t.Errorf("Expected content to contain --out-js out_b.js, got:\n%s", res.Content)
	}
	if !strings.Contains(res.Content, "cmp out_b.js out_b.js.golden") {
		t.Errorf("Expected content to contain cmp out_b.js out_b.js.golden, got:\n%s", res.Content)
	}
}

func TestPorter_MultiFileOccurrence(t *testing.T) {
	js := `//// [tests/cases/compiler/multiFileOccurrence.ts] ////

//// [a.js]
"use strict";
exports.a = 1;

//// [a.js]
"use strict";
exports.a = 2;
`
	p := &Porter{
		CaseName: "multiFileOccurrence",
		TsContent: `// @filename: dir1/a.ts
export const a = 1;
// @filename: dir2/a.ts
export const a = 2;
`,
		BaselineFinder: mockFinder(js, ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port() failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	res1 := results[0]
	if !strings.Contains(res1.Name, "dir1_a.txtar") {
		t.Errorf("Expected first result name to contain dir1_a.txtar, got %s", res1.Name)
	}
	if !strings.Contains(res1.Content, "exports.a = 1;") {
		t.Errorf("Expected first result content to contain exports.a = 1;, got %s", res1.Content)
	}
	if strings.Contains(res1.Content, "exports.a = 2;") {
		t.Errorf("First result content should not contain exports.a = 2;")
	}

	res2 := results[1]
	if !strings.Contains(res2.Name, "dir2_a.txtar") {
		t.Errorf("Expected second result name to contain dir2_a.txtar, got %s", res2.Name)
	}
	if !strings.Contains(res2.Content, "exports.a = 2;") {
		t.Errorf("Expected second result content to contain exports.a = 2;, got %s", res2.Content)
	}
	if strings.Contains(res2.Content, "exports.a = 1;") {
		t.Errorf("Second result content should not contain exports.a = 1;")
	}
}

func TestPorter_Port_MultiFile(t *testing.T) {
	js := `//// [a.js]
export const a = 1;
//// [b.js]
export const b = 2;
`
	p := Porter{
		CaseName: "multi",
		TsContent: `// @filename: a.ts
export const a = 1;
// @filename: b.ts
export const b = 2;
`,
		BaselineFinder: mockFinder(js, ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	names := map[string]bool{
		"Multi_a.txtar": false,
		"Multi_b.txtar": false,
	}

	for _, res := range results {
		if _, ok := names[res.Name]; ok {
			names[res.Name] = true
		} else {
			t.Errorf("Unexpected result name: %s", res.Name)
		}
	}

	for name, found := range names {
		if !found {
			t.Errorf("Expected result %s not found", name)
		}
	}
}

func TestPorter_Port_Error(t *testing.T) {
	errors := `==== error_case.ts (1 errors) ====
error_case.ts(1,7): error TS2322: Type 'number' is not assignable to type 'string'.
`
	p := Porter{
		CaseName:  "error_case",
		TsContent: "const x: string = 1;",
		BaselineFinder: func(v Variant, ext string) string {
			if ext == ".errors.txt" {
				return errors
			}
			return ""
		},
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	res := results[0]
	if !strings.Contains(res.Content, "! exec tscc") {
		t.Errorf("Expected content to contain ! exec tscc, got:\n%s", res.Content)
	}

	if !strings.Contains(res.Content, "stderr 'TS2322'") {
		t.Errorf("Expected content to contain stderr 'TS2322', got:\n%s", res.Content)
	}
}

func TestPorter_Port_UnsupportedDirective(t *testing.T) {
	p := Porter{
		CaseName:       "unsupported",
		TsContent:      "// @jsx: react\nexport const x = 1;",
		BaselineFinder: mockFinder("", ""),
	}

	_, err := p.Port()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if _, ok := err.(*SkipError); !ok {
		t.Errorf("Expected SkipError, got %T: %v", err, err)
	}
}

func TestPorter_Port_IgnoreDeprecations(t *testing.T) {
	p := Porter{
		CaseName:       "ignored_deprecations",
		TsContent:      "// @ignoreDeprecations: 5.0\nexport const x = 1;",
		BaselineFinder: mockFinder("", ""),
	}

	_, err := p.Port()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if _, ok := err.(*IgnoreError); !ok {
		t.Errorf("Expected IgnoreError, got %T: %v", err, err)
	}
}

func TestPorter_Port_BaseUrl(t *testing.T) {
	p := Porter{
		CaseName:       "base_url",
		TsContent:      "// @baseUrl: .\nexport const x = 1;",
		BaselineFinder: mockFinder("", ""),
	}

	_, err := p.Port()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if _, ok := err.(*IgnoreError); !ok {
		t.Errorf("Expected IgnoreError, got %T: %v", err, err)
	}
}

func TestPorter_Port_InvalidBaseline(t *testing.T) {
	p := Porter{
		CaseName:  "invalid_baseline",
		TsContent: "export const x = 1;",
		BaselineFinder: func(v Variant, ext string) string {
			if ext == ".js" {
				return "invalid baseline content"
			}
			return ""
		},
	}

	_, err := p.Port()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to split baseline JS content") {
		t.Errorf("Expected error about baseline split failure, got: %v", err)
	}
}

func TestPorter_Port_PackageJson(t *testing.T) {
	p := Porter{
		CaseName: "pkg_case",
		TsContent: `// @filename: node_modules/typescript/package.json
{
    "name": "typescript",
    "types": "/.ts/typescript.d.ts"
}
// @filename: index.ts
import ts = require("typescript");
`,
		BaselineFinder: mockFinder("", ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	res := results[0]
	if !strings.Contains(res.Content, "--path typescript=$TSCC_TS_DIR/typescript.d.ts") {
		t.Errorf("Expected content to contain --path flag, got:\n%s", res.Content)
	}
}

func TestPorter_Port_PackageJson_ExtraField(t *testing.T) {
	p := Porter{
		CaseName: "pkg_fail",
		TsContent: `// @filename: node_modules/typescript/package.json
{
    "name": "typescript",
    "types": "/.ts/typescript.d.ts",
    "version": "1.0.0"
}
// @filename: index.ts
export const x = 1;
`,
		BaselineFinder: mockFinder("", ""),
	}

	_, err := p.Port()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "unrecognized package.json") {
		t.Errorf("Expected error about unrecognized package.json, got: %v", err)
	}
}

func TestPorter_Port_PathTrimming(t *testing.T) {
	p := Porter{
		CaseName: "path_trim",
		TsContent: `// @filename: /a.ts
export const a = 1;
`,
		BaselineFinder: mockFinder("", ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	res := results[0]
	if !strings.Contains(res.Content, " a.ts") {
		t.Errorf("Expected execution to use 'a.ts', got:\n%s", res.Content)
	}
	if strings.Contains(res.Content, " /a.ts") {
		t.Errorf("Expected execution NOT to use '/a.ts', got:\n%s", res.Content)
	}
	if !strings.Contains(res.Content, "-- a.ts --") {
		t.Errorf("Expected file block to be '-- a.ts --', got:\n%s", res.Content)
	}
}

func TestPorter_Port_OnlyDtsFiles(t *testing.T) {
	p := Porter{
		CaseName: "only_dts",
		TsContent: `// @filename: a.d.ts
export const x: number;
// @filename: b.d.ts
export const y: number;
`,
		BaselineFinder: mockFinder("", ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	names := map[string]bool{}
	for _, res := range results {
		names[res.Name] = true
	}

	if !names["Only_dts_a.d.txtar"] {
		t.Errorf("Expected Only_dts_a.d.txtar")
	}
	if !names["Only_dts_b.d.txtar"] {
		t.Errorf("Expected Only_dts_b.d.txtar")
	}
}

func TestApplyShortCircuitFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string][]string
		expected map[string][]string
	}{
		{
			name: "no short circuit",
			input: map[string][]string{
				"file1.ts": {"TS2503", "TS2694"},
				"file2.ts": {"TS4023"},
			},
			expected: map[string][]string{
				"file1.ts": {"TS2503", "TS2694"},
				"file2.ts": {"TS4023"},
			},
		},
		{
			name: "syntax error triggers short circuit",
			input: map[string][]string{
				"file1.ts": {"TS1003", "TS2503"},
				"file2.ts": {"TS2694"},
			},
			expected: map[string][]string{
				"file1.ts": {"TS1003"},
			},
		},
		{
			name: "program error triggers short circuit",
			input: map[string][]string{
				"file1.ts": {"TS5023", "TS2503"},
			},
			expected: map[string][]string{
				"file1.ts": {"TS5023"},
			},
		},
		{
			name: "multiple short circuit codes",
			input: map[string][]string{
				"file1.ts": {"TS1003", "TS1359", "TS2503"},
				"file2.ts": {"TS6053", "TS4023"},
			},
			expected: map[string][]string{
				"file1.ts": {"TS1003", "TS1359"},
				"file2.ts": {"TS6053"},
			},
		},
		{
			name: "TS18xxx triggers short circuit",
			input: map[string][]string{
				"file1.ts": {"TS18007", "TS2322"},
			},
			expected: map[string][]string{
				"file1.ts": {"TS18007"},
			},
		},
		{
			name: "TS8xxx triggers short circuit",
			input: map[string][]string{
				"file1.ts": {"TS8009", "TS2322"},
			},
			expected: map[string][]string{
				"file1.ts": {"TS8009"},
			},
		},
		{
			name: "unparseable codes are retained if short circuit",

			input: map[string][]string{
				"file1.ts": {"TS1003", "TSBAD"},
			},
			expected: map[string][]string{
				"file1.ts": {"TS1003", "TSBAD"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			inputCopy := make(map[string][]string)
			for k, v := range tt.input {
				inputCopy[k] = append([]string{}, v...)
			}

			applyShortCircuitFilter(inputCopy)

			if len(inputCopy) != len(tt.expected) {
				t.Errorf("Expected map of length %d, got %d. Map: %v", len(tt.expected), len(inputCopy), inputCopy)
			}

			for k, expectedVals := range tt.expected {
				actualVals, ok := inputCopy[k]
				if !ok {
					t.Errorf("Expected key %q not found", k)
					continue
				}

				if len(actualVals) != len(expectedVals) {
					t.Errorf("For key %q: Expected %v, got %v", k, expectedVals, actualVals)
					continue
				}

				for i, v := range expectedVals {
					if actualVals[i] != v {
						t.Errorf("For key %q: Expected [%d] = %q, got %q", k, i, v, actualVals[i])
					}
				}
			}
		})
	}
}
