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
	"time"
)

func TestRenderTxtar(t *testing.T) {
	args := RenderArgs{
		CaseName:   "arrowFunctionExpression1",
		Date:       time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC),
		Flags:      []string{"--target", "es2015", "--out-js", "a.js"},
		Inputs:     []string{"a.ts"},
		ErrorCodes: []string{"TS2369"},
		Files: map[string]string{
			"a.ts": "var v = (public x: string) => { };\n",
		},
		Outputs: map[string]string{
			"a.js": "\"use strict\";\nvar v = (x) => { };\n",
		},
	}

	got := RenderTxtar(args)
	want := `# Ported from tests/cases/compiler/arrowFunctionExpression1.ts by tools/portcase.
# DO NOT EDIT by hand; re-run the porter if the upstream baseline changes.
! exec tscc --target es2015 --out-js a.js a.ts
stderr 'TS2369'
! exists a.js

-- a.ts --
var v = (public x: string) => { };
`

	if got != want {
		t.Errorf("RenderTxtar() mismatch.\nGot:\n%s\nWant:\n%s", got, want)
	}

	// Test success case
	args.ErrorCodes = nil
	gotSuccess := RenderTxtar(args)
	wantSuccess := `# Ported from tests/cases/compiler/arrowFunctionExpression1.ts by tools/portcase.
# DO NOT EDIT by hand; re-run the porter if the upstream baseline changes.
exec tscc --target es2015 --out-js a.js a.ts
! stderr .
cmp a.js a.js.golden

-- a.ts --
var v = (public x: string) => { };
-- a.js.golden --
"use strict";
var v = (x) => { };
`
	if gotSuccess != wantSuccess {
		t.Errorf("RenderTxtar() mismatch on success case.\nGot:\n%s\nWant:\n%s", gotSuccess, wantSuccess)
	}
}
