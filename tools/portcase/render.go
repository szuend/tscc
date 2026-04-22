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
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"
)

type RenderArgs struct {
	CaseName           string
	Date               time.Time
	Flags              []string
	Inputs             []string          // files to pass to tscc on command line
	ErrorCodes         []string          // TSxxxx codes
	Files              map[string]string // map of filename to content
	Outputs            map[string]string // map of expected output file to content
	NotExpectedOutputs []string          // files that should not be written
}

// RenderTxtar generates the content of a .txtar file.
func RenderTxtar(args RenderArgs) string {
	var buf bytes.Buffer

	// Header
	fmt.Fprintf(&buf, "# Ported from tests/cases/compiler/%s.ts by tools/portcase.\n", args.CaseName)
	fmt.Fprintf(&buf, "# DO NOT EDIT by hand; re-run the porter if the upstream baseline changes.\n")

	// Command line
	var execCmd strings.Builder
	execCmd.WriteString("exec tscc")
	if len(args.Flags) > 0 {
		execCmd.WriteString(" " + strings.Join(args.Flags, " "))
	}
	for _, input := range args.Inputs {
		execCmd.WriteString(" " + input)
	}

	if len(args.ErrorCodes) > 0 {
		fmt.Fprintf(&buf, "! %s\n", execCmd.String())
		for _, code := range args.ErrorCodes {
			fmt.Fprintf(&buf, "stderr '%s'\n", code)
		}

		// If it's an error test, tscc doesn't emit any files. Assert they don't exist.
		var outFiles []string
		for out := range args.Outputs {
			outFiles = append(outFiles, out)
		}
		sort.Strings(outFiles)
		for _, out := range outFiles {
			fmt.Fprintf(&buf, "! exists %s\n", out)
		}

		// Assert files that should not be written anyway
		var notExpected []string
		for _, out := range args.NotExpectedOutputs {
			if _, ok := args.Outputs[out]; !ok {
				notExpected = append(notExpected, out)
			}
		}
		sort.Strings(notExpected)
		for _, out := range notExpected {
			fmt.Fprintf(&buf, "! exists %s\n", out)
		}
	} else {
		fmt.Fprintf(&buf, "%s\n", execCmd.String())
		fmt.Fprintf(&buf, "! stderr .\n")

		// Assert output files
		var outFiles []string
		for out := range args.Outputs {
			outFiles = append(outFiles, out)
		}
		sort.Strings(outFiles)
		for _, out := range outFiles {
			fmt.Fprintf(&buf, "cmp %s %s.golden\n", out, out)
		}

		// Assert files that should not be written
		var notExpected []string
		for _, out := range args.NotExpectedOutputs {
			notExpected = append(notExpected, out)
		}
		sort.Strings(notExpected)
		for _, out := range notExpected {
			fmt.Fprintf(&buf, "! exists %s\n", out)
		}
	}

	buf.WriteString("\n")

	// Render input files
	var inFiles []string
	for f := range args.Files {
		inFiles = append(inFiles, f)
	}
	sort.Strings(inFiles)

	for _, f := range inFiles {
		fmt.Fprintf(&buf, "-- %s --\n", f)
		buf.WriteString(args.Files[f])
	}

	// Render output goldens
	if len(args.ErrorCodes) == 0 {
		var outFiles []string
		for f := range args.Outputs {
			outFiles = append(outFiles, f)
		}
		sort.Strings(outFiles)

		for _, f := range outFiles {
			fmt.Fprintf(&buf, "-- %s.golden --\n", f)
			buf.WriteString(args.Outputs[f])
		}
	}

	return buf.String()
}
