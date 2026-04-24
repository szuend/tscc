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
		CaseName:  "simple",
		TsContent: "export const x = 1;",
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

func TestPorter_Port_NoEmit(t *testing.T) {
	p := Porter{
		CaseName:  "noemit_case",
		TsContent: "// @noEmit: true\nexport const x = 1;",
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

func TestPorter_Port_Collision(t *testing.T) {
	p := Porter{
		CaseName: "collision_case",
		TsContent: `// @allowJs: true
// @filename: b.js
export const b = 2;
`,
		BaselineJs: `//// [b.js]
export const b = 2;
`,
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
	p := &Porter{
		CaseName: "multiFileOccurrence",
		TsContent: `// @filename: dir1/a.ts
export const a = 1;
// @filename: dir2/a.ts
export const a = 2;
`,
		BaselineJs: `//// [tests/cases/compiler/multiFileOccurrence.ts] ////

//// [a.js]
"use strict";
exports.a = 1;

//// [a.js]
"use strict";
exports.a = 2;
`,
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port() failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Ensure the first result gets the first a.js output and the second gets the second a.js output
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
	p := Porter{
		CaseName: "multi",
		TsContent: `// @filename: a.ts
export const a = 1;
// @filename: b.ts
export const b = 2;
`,
		BaselineJs: `//// [a.js]
export const a = 1;
//// [b.js]
export const b = 2;
`,
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Check names
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
	p := Porter{
		CaseName:  "error_case",
		TsContent: "const x: string = 1;",
		BaselineErrors: `==== error_case.ts (1 errors) ====
error_case.ts(1,7): error TS2322: Type 'number' is not assignable to type 'string'.
`,
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
		CaseName:  "unsupported",
		TsContent: "// @jsx: react\nexport const x = 1;",
	}

	_, err := p.Port()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if _, ok := err.(*SkipError); !ok {
		t.Errorf("Expected SkipError, got %T: %v", err, err)
	}
}

func TestPorter_Port_InvalidBaseline(t *testing.T) {
	p := Porter{
		CaseName:   "invalid_baseline",
		TsContent:  "export const x = 1;",
		BaselineJs: "invalid baseline content",
	}

	_, err := p.Port()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to split baseline JS content") {
		t.Errorf("Expected error about baseline split failure, got: %v", err)
	}
}

func TestPorter_Port_Variants(t *testing.T) {
	p := Porter{
		CaseName:  "variants_case",
		TsContent: "// @target: esnext, es2015\nexport const x = 1;",
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Check names
	names := map[string]bool{
		"Variants_case_esnext.txtar": false,
		"Variants_case_es2015.txtar": false,
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

	for _, res := range results {
		if res.Name == "Variants_case_esnext.txtar" {
			if !strings.Contains(res.Content, "--target esnext") {
				t.Errorf("Expected esnext content to contain --target esnext, got:\n%s", res.Content)
			}
		} else if res.Name == "Variants_case_es2015.txtar" {
			if !strings.Contains(res.Content, "--target es2015") {
				t.Errorf("Expected es2015 content to contain --target es2015, got:\n%s", res.Content)
			}
		}
	}
}

func TestPorter_Port_MultiFile_Variants(t *testing.T) {
	p := Porter{
		CaseName: "multifile_variants",
		TsContent: `// @target: esnext, es2015
// @filename: a.ts
export const a = 1;
// @filename: b.ts
export const b = 2;
`,
		BaselineJs: `//// [a.js]
export const a = 1;
//// [b.js]
export const b = 2;
`,
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	// 2 files * 2 variants = 4 results
	if len(results) != 4 {
		t.Fatalf("Expected 4 results, got %d", len(results))
	}

	// Check names
	names := map[string]bool{
		"Multifile_variants_a_esnext.txtar": false,
		"Multifile_variants_a_es2015.txtar": false,
		"Multifile_variants_b_esnext.txtar": false,
		"Multifile_variants_b_es2015.txtar": false,
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

	// Check content for specific files to ensure flags are correct
	for _, res := range results {
		switch res.Name {
		case "Multifile_variants_a_esnext.txtar":
			if !strings.Contains(res.Content, "--target esnext") {
				t.Errorf("Expected a_esnext to contain --target esnext")
			}
			if !strings.Contains(res.Content, "exec tscc --target esnext --lib esnext,dom --out-js a.js a.ts") {
				t.Errorf("Expected a_esnext to execute a.ts with command 'exec tscc --target esnext --lib esnext,dom --out-js a.js a.ts', got:\n%s", res.Content)
			}
		case "Multifile_variants_a_es2015.txtar":
			if !strings.Contains(res.Content, "--target es2015") {
				t.Errorf("Expected a_es2015 to contain --target es2015")
			}
		case "Multifile_variants_b_esnext.txtar":
			if !strings.Contains(res.Content, "exec tscc --target esnext --lib esnext,dom --out-js b.js b.ts") {
				t.Errorf("Expected b_esnext to execute b.ts with command 'exec tscc --target esnext --lib esnext,dom --out-js b.js b.ts', got:\n%s", res.Content)
			}
		}
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
	}

	_, err := p.Port()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "unrecognized package.json") {
		t.Errorf("Expected error about unrecognized package.json, got: %v", err)
	}
}

func TestPorter_Port_EmitDeclarationOnly(t *testing.T) {
	p := Porter{
		CaseName: "emit_decl",
		TsContent: `// @emitDeclarationOnly: true
// @declaration: true
export const x = 1;
`,
		BaselineJs: "//// [emit_decl.js]\nexport const x = 1;",
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

func TestPorter_Port_PathTrimming(t *testing.T) {
	p := Porter{
		CaseName: "path_trim",
		TsContent: `// @filename: /a.ts
export const a = 1;
`,
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

func TestPorter_Port_AmbientModule(t *testing.T) {
	p := Porter{
		CaseName: "ambient",
		TsContent: `// @filename: a.d.ts
declare module 'my-module' {
    export const x: number;
}
// @filename: b.ts
import { x } from 'my-module';
`,
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	res := results[0]
	if res.Name != "Ambient_b.txtar" {
		t.Errorf("Expected result name to be Ambient_b.txtar, got %s", res.Name)
	}

	if !strings.Contains(res.Content, "--path my-module=a.d.ts") {
		t.Errorf("Expected execution to contain --path my-module=a.d.ts, got:\n%s", res.Content)
	}
}
