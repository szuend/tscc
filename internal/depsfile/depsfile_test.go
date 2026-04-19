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

package depsfile

import (
	"bytes"
	"strings"
	"testing"
)

func TestWrite_SingleInput(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "/abs/a.js", []string{"/abs/a.ts"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := buf.String()
	want := "/abs/a.js: /abs/a.ts\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrite_SortsLexicographically(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "/out/a.js", []string{
		"/abs/z.ts",
		"/abs/a.ts",
		"/abs/m.ts",
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := "/out/a.js: /abs/a.ts /abs/m.ts /abs/z.ts\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrite_Deduplicates(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "/out/a.js", []string{
		"/abs/a.ts",
		"/abs/b.ts",
		"/abs/a.ts",
		"/abs/b.ts",
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := "/out/a.js: /abs/a.ts /abs/b.ts\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrite_Deterministic(t *testing.T) {
	inputs := []string{"/b.ts", "/a.ts", "/c.ts"}
	var first, second bytes.Buffer
	if err := Write(&first, "/out.js", inputs); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	// Mutate the caller slice to show the output only depends on contents.
	other := []string{"/c.ts", "/a.ts", "/b.ts"}
	if err := Write(&second, "/out.js", other); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Errorf("non-deterministic output:\n%s\n---\n%s", first.String(), second.String())
	}
}

func TestWrite_EmptyInputsIsError(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "/out.js", nil)
	if err == nil {
		t.Fatal("expected error for empty inputs, got nil")
	}
	if !strings.Contains(err.Error(), "empty input set") {
		t.Errorf("error %q missing 'empty input set'", err.Error())
	}
	if buf.Len() != 0 {
		t.Errorf("buffer should be empty on error, got %q", buf.String())
	}
}

func TestWrite_EscapeTable(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // escaped form inside the prereq list
	}{
		{"space", "/abs/file with space.ts", `/abs/file\ with\ space.ts`},
		{"tab", "/abs/file\twith\ttab.ts", "/abs/file\\\twith\\\ttab.ts"},
		{"dollar", "/abs/a$b.ts", `/abs/a$$b.ts`},
		{"hash", "/abs/a#b.ts", `/abs/a\#b.ts`},
		{"backslash", `/abs/a\b.ts`, `/abs/a\\b.ts`},
		{"plain", "/abs/plain.ts", "/abs/plain.ts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Write(&buf, "/out.js", []string{tt.input}); err != nil {
				t.Fatalf("Write: %v", err)
			}
			want := "/out.js: " + tt.want + "\n"
			if got := buf.String(); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestWrite_RejectsColon(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "/out.js", []string{"bundled:///libs/lib.d.ts"})
	if err == nil {
		t.Fatal("expected error for ':' in path, got nil")
	}
	if !strings.Contains(err.Error(), ":") {
		t.Errorf("error should mention ':' character; got %q", err.Error())
	}
}

func TestWrite_RejectsNewline(t *testing.T) {
	for _, c := range []string{"/abs/line\nbreak.ts", "/abs/cr\rlf.ts"} {
		var buf bytes.Buffer
		err := Write(&buf, "/out.js", []string{c})
		if err == nil {
			t.Fatalf("expected error for %q, got nil", c)
		}
	}
}

func TestWrite_EscapesTargetToo(t *testing.T) {
	// The output target is under the same escape rules as prereqs: a build
	// system passing --out-js foo\ bar.js must still produce a parseable rule.
	var buf bytes.Buffer
	if err := Write(&buf, "/out/a b.js", []string{"/abs/a.ts"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := `/out/a\ b.js: /abs/a.ts` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrite_RejectsColonInTarget(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "/out/a:b.js", []string{"/abs/a.ts"}); err == nil {
		t.Fatal("expected error for ':' in target, got nil")
	}
}
