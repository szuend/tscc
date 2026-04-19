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

// Package depsfile renders a Make-compatible dependency snippet of the form
// "target: dep1 dep2 …\n". Inputs are sorted and deduplicated before emission
// so callers cannot accidentally produce unstable output. Escapes follow GNU
// Make §4.3 ("Prerequisite Types") — see the design doc at
// docs/design/03-out-deps.md for the binding spec.
package depsfile

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Write renders "target: dep1 dep2 …\n" to w. Inputs are sorted lexicographically
// and deduplicated; Make-sensitive characters in both target and inputs are
// escaped. Returns an error if inputs is empty or if any path contains a byte
// that has no safe Make encoding (':', '\n', '\r').
func Write(w io.Writer, target string, inputs []string) error {
	if len(inputs) == 0 {
		return errors.New("depsfile: empty input set")
	}

	escTarget, err := escapePath(target)
	if err != nil {
		return fmt.Errorf("depsfile: target %q: %w", target, err)
	}

	sorted := make([]string, len(inputs))
	copy(sorted, inputs)
	sort.Strings(sorted)
	sorted = dedupeSorted(sorted)

	escaped := make([]string, len(sorted))
	for i, p := range sorted {
		esc, err := escapePath(p)
		if err != nil {
			return fmt.Errorf("depsfile: input %q: %w", p, err)
		}
		escaped[i] = esc
	}

	var b strings.Builder
	b.WriteString(escTarget)
	b.WriteByte(':')
	for _, e := range escaped {
		b.WriteByte(' ')
		b.WriteString(e)
	}
	b.WriteByte('\n')

	if _, err := io.WriteString(w, b.String()); err != nil {
		return err
	}
	return nil
}

// dedupeSorted removes adjacent duplicates from a sorted slice in-place-ish.
// Defensive; program.SourceFiles() is expected to be unique, but a slip in
// upstream's cache could produce duplicates and the file must stay canonical.
func dedupeSorted(in []string) []string {
	if len(in) < 2 {
		return in
	}
	out := in[:1]
	for _, s := range in[1:] {
		if s != out[len(out)-1] {
			out = append(out, s)
		}
	}
	return out
}

// escapePath applies the GNU Make escape table from docs/design/03-out-deps.md.
// ':' and any newline are rejected: Make cannot parse ':' inside a filename
// unambiguously, and neither '\n' nor '\r' has a safe Make encoding.
func escapePath(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	var b strings.Builder
	b.Grow(len(p))
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch c {
		case '\n', '\r':
			return "", fmt.Errorf("path contains newline at byte %d", i)
		case ':':
			return "", fmt.Errorf("path contains ':' at byte %d (ambiguous in Make)", i)
		case ' ', '\t', '#':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\\':
			b.WriteString(`\\`)
		case '$':
			b.WriteString(`$$`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), nil
}
