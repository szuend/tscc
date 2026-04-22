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
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/microsoft/typescript-go/tsccbridge"
)

// Porter handles the translation of an upstream test case to tscc txtar format.
type Porter struct {
	CaseName       string
	TsContent      string
	BaselineJs     string // Content of .js baseline file
	BaselineErrors string // Content of .errors.txt baseline file
}

// PortedFile represents a generated file.
type PortedFile struct {
	Name    string // Filename only, e.g. "Foo.txtar"
	Content string
}

// Port processes the case and returns the generated files.
func (p *Porter) Port() ([]PortedFile, error) {
	var results []PortedFile

	_, _, _, globalOptions, parseErr := tsccbridge.ParseTestFilesAndSymlinks(
		p.TsContent,
		p.CaseName+".ts",
		func(filename string, content string, fileOptions map[string]string) (string, error) {
			return "", nil
		},
	)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing directives: %w", parseErr)
	}

	var files map[string]string
	if p.BaselineJs != "" {
		files = SplitBaseline(p.BaselineJs)
		if len(files) == 0 {
			return nil, fmt.Errorf("failed to split baseline JS content")
		}
	} else {
		files = map[string]string{
			p.CaseName + ".ts": p.TsContent,
		}
	}

	inputs := make(map[string]string)
	outputs := make(map[string]string)

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
		inputs[p.CaseName+".ts"] = p.TsContent
		inputList = append(inputList, p.CaseName+".ts")
	}

	var errorCodesMap map[string][]string
	if p.BaselineErrors != "" {
		errorCodesMap = ExtractErrorCodesPerFile(p.BaselineErrors)
	}

	for _, inputFile := range inputList {
		inputStem := strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
		currentErrorCodes := errorCodesMap[inputFile]

		flags, err := TranslateDirectives(globalOptions, inputStem)
		if err != nil {
			return nil, err
		}

		// Determine output file name
		var currentOutName string
		if len(inputList) == 1 {
			currentOutName = strings.ToUpper(p.CaseName[:1]) + p.CaseName[1:] + ".txtar"
		} else {
			base := strings.ToUpper(p.CaseName[:1]) + p.CaseName[1:]
			currentOutName = fmt.Sprintf("%s_%s.txtar", base, inputStem)
		}

		// Filter outputs
		currentOutputs := make(map[string]string)
		var currentNotExpectedOutputs []string

		for outName, content := range outputs {
			outStem := outName
			if before, ok := strings.CutSuffix(outName, ".d.ts"); ok {
				outStem = before
			} else if before, ok := strings.CutSuffix(outName, ".js.map"); ok {
				outStem = before
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
				currentOutputs[inputStem+".js"] = ""
			}
		}

		// Check .d.ts
		if _, ok := currentOutputs[inputStem+".d.ts"]; !ok {
			hasDts := slices.Contains(flags, "--out-dts")
			if !hasDts {
				currentNotExpectedOutputs = append(currentNotExpectedOutputs, inputStem+".d.ts")
			}
		}

		// Check .js.map
		if _, ok := currentOutputs[inputStem+".js.map"]; !ok {
			hasMap := slices.Contains(flags, "--out-map")
			if !hasMap {
				currentNotExpectedOutputs = append(currentNotExpectedOutputs, inputStem+".js.map")
			}
		}

		args := RenderArgs{
			CaseName:           p.CaseName,
			Date:               time.Now().UTC(),
			Flags:              flags,
			Inputs:             []string{inputFile},
			ErrorCodes:         currentErrorCodes,
			Files:              inputs,
			Outputs:            currentOutputs,
			NotExpectedOutputs: currentNotExpectedOutputs,
		}

		txtarContent := RenderTxtar(args)
		results = append(results, PortedFile{
			Name:    currentOutName,
			Content: txtarContent,
		})
	}

	return results, nil
}
