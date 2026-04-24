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
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/microsoft/typescript-go/tsccbridge"
)

// Porter handles the translation of an upstream test case to tscc txtar format.
type Porter struct {
	SuiteName      string // Upstream suite, e.g. "compiler" or "conformance"
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

func applyShortCircuitFilter(errorCodesMap map[string][]string) {
	hasShortCircuit := false
	for _, codes := range errorCodesMap {
		for _, codeStr := range codes {
			code, err := strconv.Atoi(strings.TrimPrefix(codeStr, "TS"))
			if err == nil {
				if (code >= 1000 && code < 2000) || (code >= 5000 && code < 7000) || (code >= 18000 && code < 19000) {
					hasShortCircuit = true
					break
				}
			}
		}
		if hasShortCircuit {
			break
		}
	}

	if hasShortCircuit {
		for file, codes := range errorCodesMap {
			var filtered []string
			for _, codeStr := range codes {
				code, err := strconv.Atoi(strings.TrimPrefix(codeStr, "TS"))
				if err == nil {
					if (code >= 1000 && code < 2000) || (code >= 5000 && code < 7000) || (code >= 18000 && code < 19000) {
						filtered = append(filtered, codeStr)
					}
				} else {
					filtered = append(filtered, codeStr)
				}
			}
			if len(filtered) > 0 {
				errorCodesMap[file] = filtered
			} else {
				delete(errorCodesMap, file)
			}
		}
	}
}

// Port processes the case and returns the generated files.
func (p *Porter) Port() ([]PortedFile, error) {
	var results []PortedFile

	inputs := make(map[string]string)
	var inputList []string
	var pathArgs []string

	_, _, _, globalOptions, parseErr := tsccbridge.ParseTestFilesAndSymlinks(
		p.TsContent,
		p.CaseName+".ts",
		func(filename string, content string, fileOptions map[string]string) (string, error) {
			filename = strings.TrimPrefix(filename, "/")
			inputList = append(inputList, filename)

			if strings.HasSuffix(filename, "package.json") {
				var pkg map[string]any
				if err := json.Unmarshal([]byte(content), &pkg); err != nil {
					return "", fmt.Errorf("unrecognized package.json: %w", err)
				}
				if len(pkg) != 2 {
					return "", fmt.Errorf("unrecognized package.json: expected exactly 2 fields, got %d", len(pkg))
				}
				name, ok1 := pkg["name"].(string)
				types, ok2 := pkg["types"].(string)
				if !ok1 || !ok2 {
					return "", fmt.Errorf("unrecognized package.json: missing or invalid 'name' or 'types'")
				}
				if strings.HasPrefix(types, "/.ts/") {
					types = strings.Replace(types, "/.ts/", "$TSCC_TS_DIR/", 1)
				}
				pathArgs = append(pathArgs, fmt.Sprintf("%s=%s", name, types))
				return "", nil
			}

			inputs[filename] = content
			return "", nil
		},
	)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing directives: %w", parseErr)
	}

	var files []OutputFile
	if p.BaselineJs != "" {
		files = SplitBaseline(p.BaselineJs)
		if len(files) == 0 {
			return nil, fmt.Errorf("failed to split baseline JS content")
		}
	} else {
		files = []OutputFile{
			{Name: p.CaseName + ".ts", Content: p.TsContent},
		}
	}

	var outputs []OutputFile

	// Find ambient modules in all inputs deterministically
	ambientModuleRegex := regexp.MustCompile(`(?m)^\s*declare\s+module\s+['"]([^'"]+)['"]`)
	for _, filename := range inputList {
		content := inputs[filename]
		matches := ambientModuleRegex.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			// match[1] is the module name, filename is the filename
			pathArgs = append(pathArgs, fmt.Sprintf("%s=%s", match[1], filename))
		}
	}

	for _, f := range files {
		name := f.Name
		if !((strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".d.ts")) || strings.HasSuffix(name, ".tsx")) {
			if !strings.HasSuffix(name, "package.json") {
				outputs = append(outputs, f)
			}
		}
	}

	// Pre-group outputs by base outStem and occurrence.
	// An occurrence is bounded by seeing the same outName again.
	type OutputGroup map[string]string
	groupedOutputs := make(map[string][]OutputGroup)

	for _, out := range outputs {
		name := out.Name
		outStem := name
		if before, ok := strings.CutSuffix(name, ".d.ts"); ok {
			outStem = before
		} else if before, ok := strings.CutSuffix(name, ".js.map"); ok {
			outStem = before
		} else {
			outStem = strings.TrimSuffix(name, filepath.Ext(name))
		}

		baseOutStem := filepath.Base(outStem)

		groups := groupedOutputs[baseOutStem]
		if len(groups) == 0 {
			groups = append(groups, make(OutputGroup))
		}

		lastGroup := groups[len(groups)-1]
		if _, exists := lastGroup[name]; exists {
			lastGroup = make(OutputGroup)
			groups = append(groups, lastGroup)
		}

		lastGroup[name] = out.Content
		groupedOutputs[baseOutStem] = groups
	}

	if len(inputs) == 0 {
		var stripped []string
		lines := strings.Split(p.TsContent, "\n")
		optionRegex := regexp.MustCompile(`^\/{2}\s*@(\w+)\s*:\s*([^\r\n]*)`)
		for _, line := range lines {
			if optionRegex.MatchString(line) {
				continue
			}
			stripped = append(stripped, line)
		}
		inputs[p.CaseName+".ts"] = strings.Join(stripped, "\n")
		inputList = append(inputList, p.CaseName+".ts")
	}

	var errorCodesMap map[string][]string
	if p.BaselineErrors != "" {
		errorCodesMap = ExtractErrorCodesPerFile(p.BaselineErrors)
		applyShortCircuitFilter(errorCodesMap)
	}

	variants := ComputeVariants(globalOptions)

	hasNonDts := false
	for _, f := range inputList {
		if !strings.HasSuffix(f, ".d.ts") && !strings.HasSuffix(f, "package.json") {
			hasNonDts = true
			break
		}
	}

	for inputIndex, inputFile := range inputList {
		if strings.HasSuffix(inputFile, "package.json") {
			continue
		}
		if hasNonDts && strings.HasSuffix(inputFile, ".d.ts") {
			continue
		}
		inputStem := strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
		currentErrorCodes := errorCodesMap[inputFile]

		// Figure out which occurrence this input is among inputs with the same basename
		occurrenceIndex := 0
		for i := range inputIndex {
			if filepath.Base(strings.TrimSuffix(inputList[i], filepath.Ext(inputList[i]))) == filepath.Base(inputStem) {
				occurrenceIndex++
			}
		}

		for _, variant := range variants {
			flags, err := TranslateDirectives(variant.Options, inputStem)
			if err != nil {
				return nil, err
			}
			for _, arg := range pathArgs {
				flags = append(flags, "--path", arg)
			}

			// Determine output file name
			var currentOutName string
			base := strings.ToUpper(p.CaseName[:1]) + p.CaseName[1:]
			safeInputStem := strings.ReplaceAll(inputStem, "/", "_")
			safeInputStem = strings.ReplaceAll(safeInputStem, "\\", "_")

			if len(inputList) == 1 && variant.Name == "" {
				currentOutName = base + ".txtar"
			} else if len(inputList) == 1 {
				currentOutName = fmt.Sprintf("%s_%s.txtar", base, variant.Name)
			} else if variant.Name == "" {
				currentOutName = fmt.Sprintf("%s_%s.txtar", base, safeInputStem)
			} else {
				currentOutName = fmt.Sprintf("%s_%s_%s.txtar", base, safeInputStem, variant.Name)
			}

			// Assign outputs for this specific occurrence
			currentOutputs := make(map[string]string)
			var currentNotExpectedOutputs []string

			groups := groupedOutputs[filepath.Base(inputStem)]
			if occurrenceIndex < len(groups) {
				currentOutputs = groups[occurrenceIndex]
			}

			for i, group := range groups {
				if i != occurrenceIndex {
					for outName := range group {
						if _, ok := currentOutputs[outName]; !ok {
							currentNotExpectedOutputs = append(currentNotExpectedOutputs, outName)
						}
					}
				}
			}

			renameIfCollision := func(name string) string {
				if _, isInput := inputs[name]; isInput {
					dir, file := filepath.Split(name)
					if dir == "" {
						return "out_" + file
					}
					return filepath.ToSlash(filepath.Join(dir, "out_"+file))
				}
				return name
			}

			renamedOutputs := make(map[string]string)
			for outName, content := range currentOutputs {
				renamedOutputs[renameIfCollision(outName)] = content
			}
			currentOutputs = renamedOutputs

			var renamedNotExpected []string
			for _, outName := range currentNotExpectedOutputs {
				renamedNotExpected = append(renamedNotExpected, renameIfCollision(outName))
			}
			currentNotExpectedOutputs = renamedNotExpected

			noEmit := false
			if val, ok := variant.Options["noemit"]; ok && strings.ToLower(val) == "true" {
				noEmit = true
			}

			emitDeclOnly := false
			if val, ok := variant.Options["emitdeclarationonly"]; ok && strings.ToLower(val) == "true" {
				emitDeclOnly = true
			}

			// Calculate flags for out outputs
			if !noEmit {
				if len(currentOutputs) > 0 {
					hasJs := false
					var toRemove []string
					for outName := range currentOutputs {
						if strings.HasSuffix(outName, ".js") {
							if emitDeclOnly {
								toRemove = append(toRemove, outName)
								currentNotExpectedOutputs = append(currentNotExpectedOutputs, outName)
							} else {
								flags = append(flags, "--out-js", outName)
								hasJs = true
							}
						}
					}
					for _, r := range toRemove {
						delete(currentOutputs, r)
					}
					if !hasJs && !emitDeclOnly {
						outJs := renameIfCollision(inputStem + ".js")
						currentNotExpectedOutputs = append(currentNotExpectedOutputs, outJs)
					}
				} else {
					if !emitDeclOnly {
						outJs := renameIfCollision(inputStem + ".js")
						flags = append(flags, "--out-js", outJs)
						if len(currentErrorCodes) > 0 {
							currentOutputs[outJs] = "" // Keep it as an expected empty file for errors
						} else {
							currentNotExpectedOutputs = append(currentNotExpectedOutputs, outJs)
						}
					}
				}
			}

			// Check .d.ts
			outDts := renameIfCollision(inputStem + ".d.ts")
			if _, ok := currentOutputs[outDts]; !ok {
				hasDts := slices.Contains(flags, "--out-dts")
				if !hasDts {
					currentNotExpectedOutputs = append(currentNotExpectedOutputs, outDts)
				}
			}

			// Check .js.map
			outMap := renameIfCollision(inputStem + ".js.map")
			if _, ok := currentOutputs[outMap]; !ok {
				hasMap := slices.Contains(flags, "--out-map")
				if !hasMap {
					currentNotExpectedOutputs = append(currentNotExpectedOutputs, outMap)
				}
			}

			args := RenderArgs{
				SuiteName:          p.SuiteName,
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
