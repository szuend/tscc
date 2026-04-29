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
	"bufio"
	"regexp"
	"strings"
)

var (
	noiseRe   = regexp.MustCompile(`(?m)^////\s*\[tests/cases/.*?\]\s*////.*\n?`)
	markerRe  = regexp.MustCompile(`(?m)^////\s*\[([^\]]+)\]\s*$`)
	errorRe   = regexp.MustCompile(`(?:error|!!! error)\s+(TS\d{4,5})`)
	sectionRe = regexp.MustCompile(`^==== ([a-zA-Z0-9._\-/]+) \(\d+ errors\) ====`)
	fileRe    = regexp.MustCompile(`^([a-zA-Z0-9._\-/]+)\(\d+,\d+\):`)
)

type OutputFile struct {
	Name    string
	Content string
}

// SplitBaseline splits a baseline .js file into separate files.
// It ignores the noise header `//// [tests/cases/.../name.ts] ////`.
// Returns a slice of OutputFile to preserve duplicate filenames and their order.
func SplitBaseline(content string) []OutputFile {
	var result []OutputFile

	// Strip noise headers: //// [tests/cases/...] ////
	content = noiseRe.ReplaceAllString(content, "")

	// Split by file markers: //// [filename.ext]

	matches := markerRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return result
	}

	for i, match := range matches {
		filename := content[match[2]:match[3]]
		start := match[1]
		var end int
		if i+1 < len(matches) {
			end = matches[i+1][0]
		} else {
			end = len(content)
		}

		block := content[start:end]
		block = strings.TrimPrefix(block, "\n")
		block = strings.TrimPrefix(block, "\r\n")

		// Remove trailing sourceMappingURL comments
		// Remove \r from the entire block to normalize to Unix line endings
		block = strings.ReplaceAll(block, "\r\n", "\n")

		result = append(result, OutputFile{
			Name:    filename,
			Content: strings.TrimRight(block, " \n\t") + "\n",
		})
	}

	return result
}

// ExtractErrorCodes extracts 'TSxxxx' codes from a baseline .errors.txt file.
func ExtractErrorCodes(content string) []string {
	var codes []string
	seen := make(map[string]bool)

	// Match lines like:
	// file.ts(1,1): error TS1234: ...
	// !!! error TS1234: ...

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		matches := errorRe.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			code := match[1]
			if !seen[code] {
				seen[code] = true
				codes = append(codes, code)
			}
		}
	}
	return codes
}

// ExtractErrorCodesPerFile extracts 'TSxxxx' codes from a baseline .errors.txt file,
// mapping them to the specific file they belong to.
func ExtractErrorCodesPerFile(content string) map[string][]string {
	result := make(map[string][]string)
	seen := make(map[string]map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(content))
	var currentFile string

	for scanner.Scan() {
		line := scanner.Text()

		if matches := sectionRe.FindStringSubmatch(line); len(matches) == 2 {
			currentFile = strings.TrimPrefix(matches[1], "/")
			continue
		}

		if matches := errorRe.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, match := range matches {
				code := match[1]
				fileToAttr := currentFile

				if fileToAttr == "" {
					if fileMatches := fileRe.FindStringSubmatch(line); len(fileMatches) == 2 {
						fileToAttr = strings.TrimPrefix(fileMatches[1], "/")
					}
				}

				if fileToAttr != "" {
					if seen[fileToAttr] == nil {
						seen[fileToAttr] = make(map[string]bool)
					}
					if !seen[fileToAttr][code] {
						seen[fileToAttr][code] = true
						result[fileToAttr] = append(result[fileToAttr], code)
					}
				}
			}
		}
	}
	return result
}
