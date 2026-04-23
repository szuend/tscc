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
	"maps"
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

	inputs := make(map[string]string)
	var inputList []string

	_, _, _, globalOptions, parseErr := tsccbridge.ParseTestFilesAndSymlinks(
		p.TsContent,
		p.CaseName+".ts",
		func(filename string, content string, fileOptions map[string]string) (string, error) {
			inputs[filename] = content
			inputList = append(inputList, filename)
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

	outputs := make(map[string]string)

	for name, content := range files {
		if !((strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".d.ts")) || strings.HasSuffix(name, ".tsx")) {
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

	variants := ComputeVariants(globalOptions)

	for _, inputFile := range inputList {
		inputStem := strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
		currentErrorCodes := errorCodesMap[inputFile]

		for _, variant := range variants {
			flags, err := TranslateDirectives(variant.Options, inputStem)
			if err != nil {
				return nil, err
			}

			// Determine output file name
			var currentOutName string
			base := strings.ToUpper(p.CaseName[:1]) + p.CaseName[1:]

			if len(inputList) == 1 && variant.Name == "" {
				currentOutName = base + ".txtar"
			} else if len(inputList) == 1 {
				currentOutName = fmt.Sprintf("%s_%s.txtar", base, variant.Name)
			} else if variant.Name == "" {
				currentOutName = fmt.Sprintf("%s_%s.txtar", base, inputStem)
			} else {
				currentOutName = fmt.Sprintf("%s_%s_%s.txtar", base, inputStem, variant.Name)
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

			noEmit := false
			if val, ok := variant.Options["noemit"]; ok && strings.ToLower(val) == "true" {
				noEmit = true
			}

			// Calculate flags for out outputs
			if !noEmit {
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
	}

	return results, nil
}

// Variant represents a specific configuration combination.
type Variant struct {
	Name    string
	Options map[string]string
}

// ComputeVariants computes the Cartesian product of all multi-value directives.
// For now it only supports "target" and "module".
func ComputeVariants(options map[string]string) []Variant {
	multiValueKeys := []string{"target", "module"}
	var keysWithMultipleValues []string
	var valuesLists [][]string

	for _, k := range multiValueKeys {
		val, ok := options[k]
		if !ok {
			continue
		}
		if strings.Contains(val, ",") {
			parts := strings.Split(val, ",")
			var cleaned []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					cleaned = append(cleaned, p)
				}
			}
			if len(cleaned) > 1 {
				keysWithMultipleValues = append(keysWithMultipleValues, k)
				valuesLists = append(valuesLists, cleaned)
			}
		}
	}

	if len(keysWithMultipleValues) == 0 {
		return []Variant{{Name: "", Options: options}}
	}

	var result []Variant
	var generate func(int, map[string]string, []string)
	generate = func(idx int, currentOptions map[string]string, currentNames []string) {
		if idx == len(keysWithMultipleValues) {
			optCopy := make(map[string]string)
			maps.Copy(optCopy, options)
			maps.Copy(optCopy, currentOptions)
			result = append(result, Variant{
				Name:    strings.Join(currentNames, "_"),
				Options: optCopy,
			})
			return
		}

		key := keysWithMultipleValues[idx]
		vals := valuesLists[idx]

		for _, val := range vals {
			nextOptions := make(map[string]string)
			maps.Copy(nextOptions, currentOptions)
			nextOptions[key] = val

			generate(idx+1, nextOptions, append(currentNames, val))
		}
	}

	generate(0, make(map[string]string), nil)
	return result
}
