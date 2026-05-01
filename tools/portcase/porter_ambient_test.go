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
	if res.Name != "Ambient_b.txtar" {
		t.Errorf("Expected result name to be Ambient_b.txtar, got %s", res.Name)
	}

	if !strings.Contains(res.Content, "--path my-module=a.d.ts") {
		t.Errorf("Expected execution to contain --path my-module=a.d.ts, got:\n%s", res.Content)
	}
}

func TestPorter_Port_AmbientModuleInScript(t *testing.T) {
	p := Porter{
		CaseName: "ambient_script",
		TsContent: `
declare module 'my-module' {
    export const x: number;
}
`,
		BaselineFinder: mockFinder("", ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	res := results[0]
	if !strings.Contains(res.Content, "--path my-module=ambient_script.ts") {
		t.Errorf("Expected execution to contain --path my-module=ambient_script.ts, got:\n%s", res.Content)
	}
}

func TestPorter_Port_AmbientModuleInModule(t *testing.T) {
	p := Porter{
		CaseName: "ambient_module",
		TsContent: `
export const y = 1;
declare module 'my-module' {
    export const x: number;
}
`,
		BaselineFinder: mockFinder("", ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	res := results[0]
	if strings.Contains(res.Content, "--path my-module=") {
		t.Errorf("Expected execution NOT to contain --path my-module=, got:\n%s", res.Content)
	}
}

func TestPorter_Port_AmbientModuleInDts(t *testing.T) {
	p := Porter{
		CaseName: "ambient_dts",
		TsContent: `// @filename: a.d.ts
declare module 'my-module' {
    export const x: number;
}
// @filename: b.ts
import { x } from 'my-module';
`,
		BaselineFinder: mockFinder("", ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	res := results[0]
	if !strings.Contains(res.Content, "--path my-module=a.d.ts") {
		t.Errorf("Expected execution to contain --path my-module=a.d.ts, got:\n%s", res.Content)
	}
}

func TestPorter_Port_AmbientModuleDeduplication(t *testing.T) {
	p := Porter{
		CaseName: "dedup",
		TsContent: `
declare module 'fs' { var x: number; }
declare module "fs" { var y: string; }
`,
		BaselineFinder: mockFinder("", ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	res := results[0]

	count := strings.Count(res.Content, "--path fs=")
	if count != 1 {
		t.Errorf("Expected 1 --path fs= mapping, got %d", count)
	}
}
