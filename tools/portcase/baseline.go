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

// SplitBaseline splits a baseline .js file into separate files.
// It ignores the noise header `//// [tests/cases/.../name.ts] ////`.
// Returns a map of filename to content.
func SplitBaseline(content string) map[string]string {
	result := make(map[string]string)

	// Strip noise headers: //// [tests/cases/...] ////
	noiseRe := regexp.MustCompile(`(?m)^////\s*\[tests/cases/.*?\]\s*////.*\n?`)
	content = noiseRe.ReplaceAllString(content, "")

	// Split by file markers: //// [filename.ext]
	markerRe := regexp.MustCompile(`(?m)^////\s*\[([^\]]+)\]\s*$`)

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
		// "They include trailing //# sourceMappingURL comments when @sourceMap is set; compare after the trailing newline."
		// Let's strip it to be safe, or leave it. Actually the instruction says:
		// "They include trailing //# sourceMappingURL comments when @sourceMap is set; compare after the trailing newline."
		// Remove \r from the entire block to normalize to Unix line endings
		block = strings.ReplaceAll(block, "\r\n", "\n")

		result[filename] = strings.TrimRight(block, " \n\t") + "\n"
	}

	return result
}

// ExtractErrorCodes extracts 'TSxxxx' codes from a baseline .errors.txt file.
func ExtractErrorCodes(content string) []string {
	var codes []string
	seen := make(map[string]bool)

	re := regexp.MustCompile(`TS\d{4,5}`)

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindAllString(line, -1)
		for _, match := range matches {
			if !seen[match] {
				seen[match] = true
				codes = append(codes, match)
			}
		}
	}
	return codes
}
