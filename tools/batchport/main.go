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
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/pflag"
)

func main() {
	var targetCount int
	var verbose bool

	pflag.IntVarP(&targetCount, "count", "n", 10, "Target number of successful migrations")
	pflag.BoolVar(&verbose, "verbose", false, "Output detailed line items")
	pflag.Parse()

	upstreamDir := filepath.Join("third_party", "typescript-go", "_submodules", "TypeScript", "tests", "cases", "compiler")
	testdataDir := filepath.Join("cmd", "tscc", "testdata")

	// 1. Discovery
	files, err := os.ReadDir(upstreamDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading upstream dir: %v\n", err)
		os.Exit(1)
	}

	var allCases []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".ts") {
			allCases = append(allCases, strings.TrimSuffix(f.Name(), ".ts"))
		}
	}
	sort.Strings(allCases)

	// 2. Deduplication
	existingFiles, err := os.ReadDir(testdataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading testdata dir: %v\n", err)
		os.Exit(1)
	}

	existingCases := make(map[string]bool)
	for _, f := range existingFiles {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".txtar") {
			path := filepath.Join(testdataDir, f.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not read %s: %v\n", path, err)
				continue
			}
			content := string(data)
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "# Ported from tests/cases/compiler/") {
					caseName := strings.TrimPrefix(line, "# Ported from tests/cases/compiler/")
					caseName = strings.TrimSuffix(caseName, ".ts by tools/portcase.")
					existingCases[strings.ToLower(caseName)] = true
					break
				}
			}
		}
	}

	var candidates []string
	for _, c := range allCases {
		if !existingCases[strings.ToLower(c)] {
			candidates = append(candidates, c)
		}
	}

	fmt.Printf("Found %d total cases, %d already ported, %d candidates.\n", len(allCases), len(allCases)-len(candidates), len(candidates))

	// 3. Iterative Batching
	successCount := 0
	results := make(map[string][]string) // category -> cases

	for _, candidate := range candidates {
		if successCount >= targetCount {
			break
		}

		if verbose {
			fmt.Printf("Processing %s...\n", candidate)
		}

		// Trial Migration
		// Run portcase
		cmd := exec.Command("go", "run", "./tools/portcase", "--case", candidate)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			// Failed to port
			category := "Unknown Failure"
			stderrStr := stderr.String()
			if strings.Contains(stderrStr, "unsupported directive") {
				parts := strings.Split(stderrStr, "unsupported directive @")
				if len(parts) > 1 {
					category = "Unsupported: @" + strings.Fields(parts[1])[0]
				} else {
					category = "Unsupported"
				}
			} else if strings.Contains(stderrStr, "unrecognized") {
				category = "Unsupported: unrecognized directive"
			}
			results[category] = append(results[category], candidate)
			if verbose {
				fmt.Printf("  Failed: %s\n", category)
			}
			continue
		}

		// Succeeded porting, now run the test
		capitalized := strings.ToUpper(candidate[:1]) + candidate[1:]

		testCmd := exec.Command("go", "test", "./cmd/tscc/...", "-run", "TestScript/"+capitalized)

		testErr := testCmd.Run()
		if testErr != nil {
			category := "Diverged"
			results[category] = append(results[category], candidate)
			if verbose {
				fmt.Printf("  Diverged\n")
			}

			// Delete generated files on failure so they are retried next time
			base := strings.ToUpper(candidate[:1]) + candidate[1:]

			// 1. Exact match
			exactPath := filepath.Join(testdataDir, base+".txtar")
			if err := os.Remove(exactPath); err == nil {
				if verbose {
					fmt.Printf("  Deleted %s\n", exactPath)
				}
			}

			// 2. Pattern match for variants/multi-file
			pattern := filepath.Join(testdataDir, base+"_*.txtar")
			matches, _ := filepath.Glob(pattern)
			for _, m := range matches {
				if err := os.Remove(m); err == nil {
					if verbose {
						fmt.Printf("  Deleted %s\n", m)
					}
				}
			}

			continue
		}

		// Success!
		category := "Success"
		results[category] = append(results[category], candidate)
		successCount++
		if verbose {
			fmt.Printf("  Success\n")
		}
	}

	// 5. Reporting
	fmt.Println("\nSummary:")
	var categories []string
	for k := range results {
		categories = append(categories, k)
	}
	sort.Strings(categories)

	for _, cat := range categories {
		fmt.Printf("  %s: %d\n", cat, len(results[cat]))
	}
}
