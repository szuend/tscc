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

package config

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestPrintUsage(t *testing.T) {
	cfg := &Config{}
	groups := buildGroups(cfg)

	var buf bytes.Buffer
	printUsage(&buf, groups)
	out := buf.String()

	t.Run("includes negation note", func(t *testing.T) {
		want := "Note: All boolean flags can be negated using the '--no-' prefix."
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	})

	t.Run("groups appear in defined order", func(t *testing.T) {
		wantOrder := []string{
			"Language and Environment:",
			"Type Checking:",
			"Output:",
		}
		prev := -1
		for _, header := range wantOrder {
			i := strings.Index(out, header)
			if i == -1 {
				t.Errorf("group header %q not found in:\n%s", header, out)
				continue
			}
			if i <= prev {
				t.Errorf("group header %q appears out of order in:\n%s", header, out)
			}
			prev = i
		}
	})

	t.Run("flag appears under its group", func(t *testing.T) {
		mustContainAfter(t, out, "Type Checking:", "--strict")
		mustContainAfter(t, out, "Output:", "--out-js")
		mustContainAfter(t, out, "Language and Environment:", "--target")
	})

	t.Run("description columns aligned across groups", func(t *testing.T) {
		positions := descriptionColumns(out)
		if len(positions) < 2 {
			t.Fatalf("expected at least two flag lines, got %d:\n%s", len(positions), out)
		}
		for _, p := range positions[1:] {
			if p != positions[0] {
				t.Errorf("description columns not aligned: positions=%v\n%s", positions, out)
				return
			}
		}
	})

	t.Run("bool descriptions omit --no- form", func(t *testing.T) {
		for line := range strings.SplitSeq(out, "\n") {
			if strings.HasPrefix(line, "Note:") {
				continue
			}
			if strings.Contains(line, "--no-") {
				t.Errorf("found --no- form in non-note line: %q", line)
			}
		}
	})

	t.Run("non-zero defaults are shown", func(t *testing.T) {
		if !strings.Contains(out, `(default "es2025")`) {
			t.Errorf(`expected (default "es2025") for --target in:\n%s`, out)
		}
		if !strings.Contains(out, "(default true)") {
			t.Errorf("expected (default true) for --strict in:\n%s", out)
		}
	})

	t.Run("zero defaults are omitted", func(t *testing.T) {
		if strings.Contains(out, `(default "")`) {
			t.Errorf(`empty-string default should be omitted, got:\n%s`, out)
		}
	})
}

func TestPrintUsageGlobalAlignment(t *testing.T) {
	// Build two groups where, without global alignment, descriptions would
	// drift between groups (one group has a much longer flag name).
	cfg := struct {
		Short bool
		Long  string
	}{}
	a := pflag.NewFlagSet("a", pflag.ContinueOnError)
	a.BoolVar(&cfg.Short, "x", false, "short flag")
	b := pflag.NewFlagSet("b", pflag.ContinueOnError)
	b.StringVar(&cfg.Long, "this-is-a-very-long-flag-name", "", "long flag")

	groups := []flagGroup{
		{Name: "Group A", Set: a},
		{Name: "Group B", Set: b},
	}

	var buf bytes.Buffer
	printUsage(&buf, groups)
	out := buf.String()

	positions := descriptionColumns(out)
	if len(positions) != 2 {
		t.Fatalf("expected 2 flag lines, got %d:\n%s", len(positions), out)
	}
	if positions[0] != positions[1] {
		t.Errorf("descriptions misaligned across groups: positions=%v\n%s", positions, out)
	}
}

// descriptionColumns returns the column index where the description text
// starts on each flag line in usage output.
func descriptionColumns(out string) []int {
	descRe := regexp.MustCompile(`^  (?:-\w, |    )--[\w-]+(?:\s\S+)?\s{3,}(\S.*)$`)

	var positions []int
	for line := range strings.SplitSeq(out, "\n") {
		m := descRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		positions = append(positions, len(line)-len(m[1]))
	}
	return positions
}

func mustContainAfter(t *testing.T, out, header, needle string) {
	t.Helper()
	_, after, ok := strings.Cut(out, header)
	if !ok {
		t.Errorf("header %q not in output:\n%s", header, out)
		return
	}
	tail := after
	// stop at the next blank line (= next group)
	if end := strings.Index(tail, "\n\n"); end != -1 {
		tail = tail[:end]
	}
	if !strings.Contains(tail, needle) {
		t.Errorf("expected %q under header %q, got:\n%s", needle, header, tail)
	}
}
