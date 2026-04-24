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
	"testing"
)

func TestFlattenName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo", "Foo"},
		{"foo/bar", "Foo_Bar"},
		{"conformance/classes/classExpression", "Conformance_Classes_ClassExpression"},
		{"Symbols/ES5SymbolProperty5", "Symbols_ES5SymbolProperty5"},
	}

	for _, tt := range tests {
		got := FlattenName(tt.input)
		if got != tt.want {
			t.Errorf("FlattenName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPorter_Port_Nested(t *testing.T) {
	p := Porter{
		SuiteName: "conformance",
		CaseName:  "classes/classExpression",
		TsContent: "var x = class C {};",
	}

	results, err := p.Port()
	if err != nil {
		t.Fatalf("Port failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	res := results[0]
	if res.Name != "Classes_ClassExpression.txtar" {
		t.Errorf("Expected name Classes_ClassExpression.txtar, got %s", res.Name)
	}
}
