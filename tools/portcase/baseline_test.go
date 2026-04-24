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
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitBaseline(t *testing.T) {
	content := `//// [tests/cases/compiler/multiFile.ts] ////

//// [a.ts]
var a = 1;

//// [a.js]
var a = 1;

//// [b.ts]
var b = 2;

//// [b.js]
var b = 2;
//# sourceMappingURL=b.js.map
`
	got := SplitBaseline(content)
	want := []OutputFile{
		{Name: "a.ts", Content: "var a = 1;\n"},
		{Name: "a.js", Content: "var a = 1;\n"},
		{Name: "b.ts", Content: "var b = 2;\n"},
		{Name: "b.js", Content: "var b = 2;\n//# sourceMappingURL=b.js.map\n"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("SplitBaseline() = %v, want %v", got, want)
	}
}

func TestSplitBaseline_DuplicateNames(t *testing.T) {
	content := `//// [tests/cases/compiler/multiFile.ts] ////

//// [ts.ts]
var a = 1;
//// [ts.js]
var a = 1;
//// [ts.ts]
var b = 2;
//// [ts.js]
var b = 2;
`
	got := SplitBaseline(content)
	want := []OutputFile{
		{Name: "ts.ts", Content: "var a = 1;\n"},
		{Name: "ts.js", Content: "var a = 1;\n"},
		{Name: "ts.ts", Content: "var b = 2;\n"},
		{Name: "ts.js", Content: "var b = 2;\n"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("SplitBaseline() with duplicates = %v, want %v", got, want)
	}
}

func TestExtractErrorCodes(t *testing.T) {
	content := `
tests/cases/compiler/foo.ts(1,1): error TS1005: ';' expected.
tests/cases/compiler/foo.ts(2,5): error TS2304: Cannot find name 'x'.
tests/cases/compiler/foo.ts(3,1): error TS1005: ';' expected.

==== foo.ts (2 errors) ====
    let x = 1;
    ~~~
!!! error TS1234: Some error
!!! related TS5678 foo.ts:10:1: Related info here
`
	got := ExtractErrorCodes(content)
	want := []string{"TS1005", "TS2304", "TS1234"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractErrorCodes() = %v, want %v", got, want)
	}
}

func TestReadBaseline(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// `go test` runs in tools/portcase, so repo root is ../..
	err = os.Chdir("../..")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	t.Run("SubmoduleOverridesUpstream", func(t *testing.T) {
		content := readBaseline("compiler", "accessorDeclarationEmitJs", ".js")
		submodulePath := filepath.Join("third_party", "typescript-go", "testdata", "baselines", "reference", "submodule", "compiler", "accessorDeclarationEmitJs.js")
		expected, err := os.ReadFile(submodulePath)
		if err != nil {
			t.Fatal(err)
		}
		if content != string(expected) {
			t.Errorf("expected content from submodule, got something else")
		}
	})

	t.Run("UpstreamFallback", func(t *testing.T) {
		content := readBaseline("compiler", "abstractPropertyInitializer", ".errors.txt")
		upstreamPath := filepath.Join("third_party", "typescript-go", "_submodules", "TypeScript", "tests", "baselines", "reference", "abstractPropertyInitializer.errors.txt")
		expected, err := os.ReadFile(upstreamPath)
		if err != nil {
			t.Fatal(err)
		}
		if content != string(expected) {
			t.Errorf("expected content from upstream, got something else")
		}
	})

	t.Run("Missing", func(t *testing.T) {
		content := readBaseline("compiler", "doesNotExistSomething123", ".js")
		if content != "" {
			t.Errorf("expected empty string for missing baseline, got %q", content)
		}
	})
}
