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
	"fmt"
	"io"
	"strings"

	"github.com/spf13/pflag"
)

const usageHeader = `Usage: tscc [FLAGS] FILE

Note: All boolean flags can be negated using the '--no-' prefix.
`

const (
	flagIndent     = "  "
	shorthandWidth = 4 // "-X, " or four spaces
	descGap        = 3 // spaces between flag column and description column
)

func printUsage(w io.Writer, groups []flagGroup) {
	fmt.Fprint(w, usageHeader, "\n")

	width := maxLeftWidth(groups)
	for _, g := range groups {
		fmt.Fprintf(w, "%s:\n", g.Name)
		g.Set.VisitAll(func(f *pflag.Flag) {
			writeFlag(w, f, width)
		})
		fmt.Fprintln(w)
	}
}

func maxLeftWidth(groups []flagGroup) int {
	max := 0
	for _, g := range groups {
		g.Set.VisitAll(func(f *pflag.Flag) {
			if w := flagLeftWidth(f); w > max {
				max = w
			}
		})
	}
	return max
}

func flagLeftWidth(f *pflag.Flag) int {
	n := len(flagIndent) + shorthandWidth
	n += len("--") + len(f.Name)
	if varname, _ := pflag.UnquoteUsage(f); varname != "" {
		n += 1 + len(varname)
	}
	return n
}

func writeFlag(w io.Writer, f *pflag.Flag, width int) {
	var b strings.Builder
	b.WriteString(flagIndent)
	if f.Shorthand != "" {
		fmt.Fprintf(&b, "-%s, ", f.Shorthand)
	} else {
		b.WriteString(strings.Repeat(" ", shorthandWidth))
	}
	fmt.Fprintf(&b, "--%s", f.Name)
	varname, usage := pflag.UnquoteUsage(f)
	if varname != "" {
		fmt.Fprintf(&b, " %s", varname)
	}

	pad := width - b.Len() + descGap
	b.WriteString(strings.Repeat(" ", pad))
	b.WriteString(usage)
	writeDefault(&b, f)

	fmt.Fprintln(w, b.String())
}

func writeDefault(b *strings.Builder, f *pflag.Flag) {
	if f.DefValue == "" {
		return
	}
	if f.Value.Type() == "bool" && f.DefValue == "false" {
		return
	}
	if f.Value.Type() == "string" {
		fmt.Fprintf(b, " (default %q)", f.DefValue)
		return
	}
	fmt.Fprintf(b, " (default %s)", f.DefValue)
}
