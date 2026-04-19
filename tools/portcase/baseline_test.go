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
	want := map[string]string{
		"a.ts": "var a = 1;\n",
		"a.js": "var a = 1;\n",
		"b.ts": "var b = 2;\n",
		"b.js": "var b = 2;\n//# sourceMappingURL=b.js.map\n",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("SplitBaseline() = %v, want %v", got, want)
	}
}

func TestExtractErrorCodes(t *testing.T) {
	content := `
tests/cases/compiler/foo.ts(1,1): error TS1005: ';' expected.
tests/cases/compiler/foo.ts(2,5): error TS2304: Cannot find name 'x'.
tests/cases/compiler/foo.ts(3,1): error TS1005: ';' expected.
`
	got := ExtractErrorCodes(content)
	want := []string{"TS1005", "TS2304"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractErrorCodes() = %v, want %v", got, want)
	}
}
