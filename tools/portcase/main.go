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
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/spf13/pflag"
)

func main() {
	var caseName string
	var outPath string
	var force bool

	pflag.StringVar(&caseName, "case", "", "The upstream test name (without path or extension)")
	pflag.StringVar(&outPath, "out", "", "Output path (defaults to cmd/tscc/testdata/<Name>.txtar)")
	pflag.BoolVar(&force, "force", false, "Overwrite an existing fixture")
	pflag.Parse()

	if caseName == "" {
		fmt.Fprintln(os.Stderr, "error: --case is required")
		os.Exit(1)
	}

	if outPath == "" {
		outPath = filepath.Join("cmd", "tscc", "testdata", strings.ToUpper(caseName[:1])+caseName[1:]+".txtar")
	}

	if !force {
		if _, err := os.Stat(outPath); err == nil {
			fmt.Fprintf(os.Stderr, "error: output file %s already exists. Use --force to overwrite.\n", outPath)
			os.Exit(1)
		}
	}

	// 1. Read the upstream .ts file to parse directives
	tsPath := filepath.Join("third_party", "typescript-go", "_submodules", "TypeScript", "tests", "cases", "compiler", caseName+".ts")
	tsData, err := os.ReadFile(tsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not read upstream case: %v\n", err)
		os.Exit(1)
	}
	tsContent := string(tsData)

	_, _, _, globalOptions, parseErr := tsccbridge.ParseTestFilesAndSymlinks(
		tsContent,
		caseName+".ts",
		func(filename string, content string, fileOptions map[string]string) (string, error) {
			return "", nil
		},
	)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "error parsing directives: %v\n", parseErr)
		os.Exit(1)
	}

	flags, err := TranslateDirectives(globalOptions, caseName)
	if err != nil {
		if skipErr, ok := err.(*SkipError); ok {
			fmt.Printf("%s: unsupported directive @%s\n", caseName, skipErr.Directive)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "error translating directives: %v\n", err)
		os.Exit(1)
	}

	// 2. Read the baseline .js to extract files
	baselinePath := filepath.Join("third_party", "typescript-go", "_submodules", "TypeScript", "tests", "baselines", "reference", caseName+".js")
	baselineData, err := os.ReadFile(baselinePath)
	var files map[string]string
	if err == nil {
		files = SplitBaseline(string(baselineData))
	} else {
		// If there is no JS baseline, maybe it's just the input file
		files = map[string]string{
			caseName + ".ts": tsContent,
		}
	}

	inputs := make(map[string]string)
	outputs := make(map[string]string)

	// Filter files into inputs and outputs
	var inputList []string
	for name, content := range files {
		if (strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".d.ts")) || strings.HasSuffix(name, ".tsx") {
			inputs[name] = content
			inputList = append(inputList, name)
		} else {
			outputs[name] = content
		}
	}

	if len(inputs) == 0 {
		// Fallback if baseline doesn't contain inputs
		inputs[caseName+".ts"] = tsContent
		inputList = append(inputList, caseName+".ts")
	} else if len(inputs) > 1 {
		fmt.Printf("%s: unsupported multi-file case\n", caseName)
		os.Exit(2)
	}

	// 3. Read the baseline .errors.txt if present
	errorsPath := filepath.Join("third_party", "typescript-go", "_submodules", "TypeScript", "tests", "baselines", "reference", caseName+".errors.txt")
	var errorCodes []string
	if errData, err := os.ReadFile(errorsPath); err == nil {
		errorCodes = ExtractErrorCodes(string(errData))
	}

	// Calculate flags for out outputs
	if len(errorCodes) == 0 && len(outputs) > 0 {
		for outName := range outputs {
			if strings.HasSuffix(outName, ".js") {
				// Avoid adding --out-js if it's already there or implied?
				// tscc requires explicit outputs.
				flags = append(flags, "--out-js", outName)
			}
		}
	} else if len(errorCodes) > 0 {
		// Even for errors, we need to pass out-js to tscc so it knows what it *would* have written
		// since tscc's invariant is explicit outputs.
		// Wait, the design doc says: "If it's an error test, tscc doesn't emit any files. Assert they don't exist."
		// But do we need --out-js? Yes, `tscc --out-js a.js a.ts`.
		if len(outputs) > 0 {
			for outName := range outputs {
				if strings.HasSuffix(outName, ".js") {
					flags = append(flags, "--out-js", outName)
				}
			}
		} else {
			flags = append(flags, "--out-js", caseName+".js")
			outputs[caseName+".js"] = "" // add an expected non-existent file
		}
	}

	// We may have duplicate flags if out-js was added.
	// The `TranslateDirectives` handles --out-dts and --out-map, so we just add --out-js here.

	args := RenderArgs{
		CaseName:   caseName,
		Date:       time.Now().UTC(),
		Flags:      flags,
		Inputs:     inputList,
		ErrorCodes: errorCodes,
		Files:      inputs,
		Outputs:    outputs,
	}

	txtarContent := RenderTxtar(args)

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating directories: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, []byte(txtarContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		os.Exit(1)
	}
}
