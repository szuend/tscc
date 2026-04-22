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
	}

	// 3. Read the baseline .errors.txt if present
	errorsPath := filepath.Join("third_party", "typescript-go", "_submodules", "TypeScript", "tests", "baselines", "reference", caseName+".errors.txt")
	var errorCodesMap map[string][]string
	if errData, err := os.ReadFile(errorsPath); err == nil {
		errorCodesMap = ExtractErrorCodesPerFile(string(errData))
	}

	for _, inputFile := range inputList {
		inputStem := strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
		currentErrorCodes := errorCodesMap[inputFile]

		flags, err := TranslateDirectives(globalOptions, inputStem)
		if err != nil {
			if skipErr, ok := err.(*SkipError); ok {
				fmt.Printf("%s: unsupported directive @%s\n", caseName, skipErr.Directive)
				os.Exit(2)
			}
			fmt.Fprintf(os.Stderr, "error translating directives: %v\n", err)
			os.Exit(1)
		}

		// Determine output file name
		var currentOutPath string
		if len(inputList) == 1 {
			currentOutPath = outPath
		} else {
			dir := filepath.Dir(outPath)
			base := strings.ToUpper(caseName[:1]) + caseName[1:]
			currentOutPath = filepath.Join(dir, fmt.Sprintf("%s_%s.txtar", base, inputStem))
		}

		// Filter outputs
		currentOutputs := make(map[string]string)
		var currentNotExpectedOutputs []string

		for outName, content := range outputs {
			outStem := outName
			if strings.HasSuffix(outName, ".d.ts") {
				outStem = strings.TrimSuffix(outName, ".d.ts")
			} else if strings.HasSuffix(outName, ".js.map") {
				outStem = strings.TrimSuffix(outName, ".js.map")
			} else {
				outStem = strings.TrimSuffix(outName, filepath.Ext(outName))
			}

			if outStem == inputStem {
				currentOutputs[outName] = content
			} else {
				currentNotExpectedOutputs = append(currentNotExpectedOutputs, outName)
			}
		}

		// Calculate flags for out outputs
		if len(currentErrorCodes) == 0 && len(currentOutputs) > 0 {
			for outName := range currentOutputs {
				if strings.HasSuffix(outName, ".js") {
					flags = append(flags, "--out-js", outName)
				}
			}
		} else if len(currentErrorCodes) > 0 {
			if len(currentOutputs) > 0 {
				for outName := range currentOutputs {
					if strings.HasSuffix(outName, ".js") {
						flags = append(flags, "--out-js", outName)
					}
				}
			} else {
				flags = append(flags, "--out-js", inputStem+".js")
				currentOutputs[inputStem+".js"] = "" // add an expected non-existent file
			}
		}

		// Check .d.ts
		if _, ok := currentOutputs[inputStem+".d.ts"]; !ok {
			hasDts := false
			for _, f := range flags {
				if f == "--out-dts" {
					hasDts = true
					break
				}
			}
			if !hasDts {
				currentNotExpectedOutputs = append(currentNotExpectedOutputs, inputStem+".d.ts")
			}
		}

		// Check .js.map
		if _, ok := currentOutputs[inputStem+".js.map"]; !ok {
			hasMap := false
			for _, f := range flags {
				if f == "--out-map" {
					hasMap = true
					break
				}
			}
			if !hasMap {
				currentNotExpectedOutputs = append(currentNotExpectedOutputs, inputStem+".js.map")
			}
		}

		args := RenderArgs{
			CaseName:           caseName,
			Date:               time.Now().UTC(),
			Flags:              flags,
			Inputs:             []string{inputFile},
			ErrorCodes:         currentErrorCodes,
			Files:              inputs,
			Outputs:            currentOutputs,
			NotExpectedOutputs: currentNotExpectedOutputs,
		}

		txtarContent := RenderTxtar(args)

		if err := os.MkdirAll(filepath.Dir(currentOutPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating directories: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(currentOutPath, []byte(txtarContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
			os.Exit(1)
		}
	}
}
