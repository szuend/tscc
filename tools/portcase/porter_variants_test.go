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

func TestPorter_Port_Variants(t *testing.T) {
	p := Porter{
		CaseName:       "variants_case",
		TsContent:      "// @target: esnext, es2015\nexport const x = 1;",
		BaselineFinder: mockFinder("", ""),
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

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
	js := `//// [a.js]
export const a = 1;
//// [b.js]
export const b = 2;
`
	p := Porter{
		CaseName: "multifile_variants",
		TsContent: `// @target: esnext, es2015
// @filename: a.ts
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

	if len(results) != 4 {
		t.Fatalf("Expected 4 results, got %d", len(results))
	}

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

func TestComputeVariants(t *testing.T) {
	tests := []struct {
		name              string
		options           map[string]string
		wantNames         []string
		wantUpstreamNames []string
	}{
		{
			name: "single value",
			options: map[string]string{
				"target": "es2015",
			},
			wantNames:         []string{""},
			wantUpstreamNames: []string{""},
		},
		{
			name: "multi-value target",
			options: map[string]string{
				"target": "es2015, esnext",
			},
			wantNames:         []string{"es2015", "esnext"},
			wantUpstreamNames: []string{"target=es2015", "target=esnext"},
		},
		{
			name: "multi-value target and module",
			options: map[string]string{
				"target": "es2015, esnext",
				"module": "commonjs, preserve",
			},
			wantNames: []string{
				"es2015_commonjs",
				"es2015_preserve",
				"esnext_commonjs",
				"esnext_preserve",
			},
			wantUpstreamNames: []string{
				"module=commonjs,target=es2015",
				"module=preserve,target=es2015",
				"module=commonjs,target=esnext",
				"module=preserve,target=esnext",
			},
		},
		{
			name: "case insensitive multi-value keys",
			options: map[string]string{
				"Target": "es2015, esnext",
			},
			wantNames:         []string{"es2015", "esnext"},
			wantUpstreamNames: []string{"target=es2015", "target=esnext"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeVariants(tt.options)
			if len(got) != len(tt.wantNames) {
				t.Fatalf("got %d variants, want %d", len(got), len(tt.wantNames))
			}
			for i, v := range got {
				if v.Name != tt.wantNames[i] {
					t.Errorf("variant[%d].Name = %q, want %q", i, v.Name, tt.wantNames[i])
				}
				if v.UpstreamName != tt.wantUpstreamNames[i] {
					t.Errorf("variant[%d].UpstreamName = %q, want %q", i, v.UpstreamName, tt.wantUpstreamNames[i])
				}
			}
		})
	}
}
