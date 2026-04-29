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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

func main() {
	var suiteName string
	var caseName string
	var outPath string
	var force bool

	pflag.StringVar(&suiteName, "suite", "compiler", "The upstream test suite (e.g. compiler, conformance)")
	pflag.StringVar(&caseName, "case", "", "The upstream test name (without path or extension)")
	pflag.StringVar(&outPath, "out", "", "Output path (defaults to cmd/tscc/testdata/<suite>/<Name>.txtar)")
	pflag.BoolVar(&force, "force", false, "Overwrite an existing fixture")
	pflag.Parse()

	if caseName == "" {
		fmt.Fprintln(os.Stderr, "error: --case is required")
		os.Exit(1)
	}

	if outPath == "" {
		outPath = filepath.Join("cmd", "tscc", "testdata", suiteName, FlattenName(caseName)+".txtar")
	}

	if !force {
		if _, err := os.Stat(outPath); err == nil {
			fmt.Fprintf(os.Stderr, "error: output file %s already exists. Use --force to overwrite.\n", outPath)
			os.Exit(1)
		}
	}

	// 1. Read the upstream .ts file
	tsPath := filepath.Join("third_party", "typescript-go", "_submodules", "TypeScript", "tests", "cases", suiteName, caseName+".ts")
	tsData, err := os.ReadFile(tsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not read upstream case: %v\n", err)
		os.Exit(1)
	}
	tsContent := string(tsData)
	tsContent = strings.TrimPrefix(tsContent, "\xef\xbb\xbf")

	baselineFinder := func(variant Variant, ext string) string {
		return readBaseline(suiteName, caseName, ext, variant.UpstreamName)
	}

	porter := Porter{
		SuiteName:      suiteName,
		CaseName:       caseName,
		TsContent:      tsContent,
		BaselineFinder: baselineFinder,
	}

	results, err := porter.Port()
	if err != nil {
		if ignoreErr, ok := err.(*IgnoreError); ok {
			fmt.Printf("%s: ignored directive @%s\n", caseName, ignoreErr.Directive)
			os.Exit(3)
		}
		if skipErr, ok := err.(*SkipError); ok {
			fmt.Printf("%s: unsupported directive @%s\n", caseName, skipErr.Directive)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "error porting case: %v\n", err)
		os.Exit(1)
	}

	for _, res := range results {
		var currentOutPath string
		if len(results) == 1 {
			currentOutPath = outPath
		} else {
			dir := filepath.Dir(outPath)
			currentOutPath = filepath.Join(dir, res.Name)
		}

		if err := os.MkdirAll(filepath.Dir(currentOutPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating directories: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(currentOutPath, []byte(res.Content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
			os.Exit(1)
		}
	}
}
