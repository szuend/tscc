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

func TestPorter_Port_NoEmit(t *testing.T) {
	p := Porter{
		CaseName: "noemit_case",
		TsContent: `// @noEmit: true
// @declaration: true
// @sourceMap: true
export const x = 1;
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
	if strings.Contains(res.Content, "--out-js") {
		t.Errorf("Expected content to NOT contain --out-js, got:\n%s", res.Content)
	}
	if strings.Contains(res.Content, "--out-dts") {
		t.Errorf("Expected content to NOT contain --out-dts, got:\n%s", res.Content)
	}
	if strings.Contains(res.Content, "--out-map") {
		t.Errorf("Expected content to NOT contain --out-map, got:\n%s", res.Content)
	}
}

func TestPorter_Port_SourceMap(t *testing.T) {
	js := `//// [sourcemap_case.js]
export const x = 1;
//// [sourcemap_case.js.map]
{"version":3,"file":"sourcemap_case.js"}
`
	p := Porter{
		CaseName: "sourcemap_case",
		TsContent: `// @sourceMap: true
export const x = 1;
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
	if !strings.Contains(res.Content, "--out-map sourcemap_case.js.map") {
		t.Errorf("Expected content to contain --out-map sourcemap_case.js.map, got:\n%s", res.Content)
	}
}

func TestPorter_Port_OutDir_SourceMap(t *testing.T) {
	js := `//// [dist/outdir_sourcemap_case.js]
export const x = 1;
//// [dist/outdir_sourcemap_case.js.map]
{"version":3,"file":"outdir_sourcemap_case.js"}
`
	p := Porter{
		CaseName: "outdir_sourcemap_case",
		TsContent: `// @outDir: dist
// @sourceMap: true
export const x = 1;
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
	if !strings.Contains(res.Content, "--out-map dist/outdir_sourcemap_case.js.map") {
		t.Errorf("Expected content to contain --out-map dist/outdir_sourcemap_case.js.map, got:\n%s", res.Content)
	}
}

func TestPorter_Port_OutDir_Declaration(t *testing.T) {
	js := `//// [dist/outdir_case.js]
export const x = 1;
//// [dist/outdir_case.d.ts]
export declare const x = 1;
`
	p := Porter{
		CaseName: "outdir_case",
		TsContent: `// @outDir: dist
// @declaration: true
export const x = 1;
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
	if !strings.Contains(res.Content, "--out-dts dist/outdir_case.d.ts") {
		t.Errorf("Expected content to contain --out-dts dist/outdir_case.d.ts, got:\n%s", res.Content)
	}
}

func TestPorter_Port_DeclarationDir(t *testing.T) {
	js := `//// [dist/decl_case.js]
export const x = 1;
//// [types/decl_case.d.ts]
export declare const x = 1;
`
	p := Porter{
		CaseName: "decl_case",
		TsContent: `// @outDir: dist
// @declarationDir: types
// @declaration: true
export const x = 1;
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
	if !strings.Contains(res.Content, "--out-dts types/decl_case.d.ts") {
		t.Errorf("Expected content to contain --out-dts types/decl_case.d.ts, got:\n%s", res.Content)
	}
	if !strings.Contains(res.Content, "--out-js dist/decl_case.js") {
		t.Errorf("Expected content to contain --out-js dist/decl_case.js, got:\n%s", res.Content)
	}
}

func TestPorter_Port_OutDir(t *testing.T) {
	js := `//// [dist/outdir_case.js]
export const x = 1;
`
	p := Porter{
		CaseName: "outdir_case",
		TsContent: `// @outDir: dist
export const x = 1;
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
	if !strings.Contains(res.Content, "--out-js dist/outdir_case.js") {
		t.Errorf("Expected content to contain --out-js dist/outdir_case.js, got:\n%s", res.Content)
	}
	if !strings.Contains(res.Content, "cmp dist/outdir_case.js dist/outdir_case.js.golden") {
		t.Errorf("Expected content to contain cmp dist/outdir_case.js dist/outdir_case.js.golden, got:\n%s", res.Content)
	}
}

func TestPorter_Port_EmitDeclarationOnly(t *testing.T) {
	js := "//// [emit_decl.js]\nexport const x = 1;"
	p := Porter{
		CaseName: "emit_decl",
		TsContent: `// @emitDeclarationOnly: true
// @declaration: true
export const x = 1;
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
	if strings.Contains(res.Content, "--out-js") {
		t.Errorf("Expected content NOT to contain --out-js, got:\n%s", res.Content)
	}

	if !strings.Contains(res.Content, "! exists emit_decl.js") {
		t.Errorf("Expected content to assert '! exists emit_decl.js', got:\n%s", res.Content)
	}
}
