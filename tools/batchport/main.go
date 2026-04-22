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
	"sync"

	"github.com/spf13/pflag"
)

func main() {
	var targetCount int
	var verbose bool
	var concurrency int

	pflag.IntVarP(&targetCount, "count", "n", 10, "Target number of successful migrations")
	pflag.BoolVar(&verbose, "verbose", false, "Output detailed line items")
	pflag.IntVarP(&concurrency, "concurrency", "j", 4, "Number of concurrent workers")
	pflag.Parse()

	upstreamDir := filepath.Join("third_party", "typescript-go", "_submodules", "TypeScript", "tests", "cases", "compiler")
	testdataDir := filepath.Join("cmd", "tscc", "testdata")

	// 1. Discovery & Deduplication
	candidates, err := discoverCandidates(upstreamDir, testdataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 2. Build portcase once
	portcasePath, cleanup, err := buildPortcase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building portcase: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	// 3. Iterative Batching in Parallel
	successCount := 0
	results := make(map[string][]string) // category -> cases
	var mu sync.Mutex

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, candidate := range candidates {
		mu.Lock()
		if successCount >= targetCount {
			mu.Unlock()
			break
		}
		mu.Unlock()

		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cat := processCandidate(c, portcasePath, testdataDir)

			mu.Lock()
			results[cat] = append(results[cat], c)
			if cat == "Success" {
				successCount++
			}
			mu.Unlock()

			if verbose {
				fmt.Printf("Case %s: %s\n", c, cat)
			}
		}(candidate)
	}

	wg.Wait()

	// 4. Reporting
	reportResults(results)
}

// discoverCandidates scans upstream tests and filters out already ported ones.
func discoverCandidates(upstreamDir, testdataDir string) ([]string, error) {
	files, err := os.ReadDir(upstreamDir)
	if err != nil {
		return nil, fmt.Errorf("reading upstream dir: %w", err)
	}

	var allCases []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".ts") {
			allCases = append(allCases, strings.TrimSuffix(f.Name(), ".ts"))
		}
	}
	sort.Strings(allCases)

	existingFiles, err := os.ReadDir(testdataDir)
	if err != nil {
		return nil, fmt.Errorf("reading testdata dir: %w", err)
	}

	existingCases := make(map[string]bool)
	for _, f := range existingFiles {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".txtar") {
			path := filepath.Join(testdataDir, f.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			lines := strings.Split(string(data), "\n")
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
	return candidates, nil
}

// buildPortcase builds the portcase tool once into a workspace-local tmp directory.
func buildPortcase() (string, func(), error) {
	tmpDir := filepath.Join("tools", "batchport", "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", nil, fmt.Errorf("creating tmp dir: %w", err)
	}

	binPath := filepath.Join(tmpDir, "portcase")
	cmd := exec.Command("go", "build", "-o", binPath, "./tools/portcase")
	if err := cmd.Run(); err != nil {
		return "", nil, fmt.Errorf("building portcase: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return binPath, cleanup, nil
}

// processCandidate handles the trial migration and test execution for a single candidate.
func processCandidate(candidate, portcasePath, testdataDir string) string {
	cmd := exec.Command(portcasePath, "--case", candidate)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return parseFailureCategory(stderr.String())
	}

	capitalized := strings.ToUpper(candidate[:1]) + candidate[1:]
	testCmd := exec.Command("go", "test", "./cmd/tscc/...", "-run", "TestScript/"+capitalized)

	testErr := testCmd.Run()
	if testErr != nil {
		cleanupFailedFiles(candidate, testdataDir)
		return "Diverged"
	}

	return "Success"
}

// parseFailureCategory extracts the failure reason from portcase stderr.
func parseFailureCategory(stderrStr string) string {
	if strings.Contains(stderrStr, "unsupported directive") {
		parts := strings.Split(stderrStr, "unsupported directive @")
		if len(parts) > 1 {
			return "Unsupported: @" + strings.Fields(parts[1])[0]
		}
		return "Unsupported"
	}
	if strings.Contains(stderrStr, "unrecognized") {
		return "Unsupported: unrecognized directive"
	}
	return "Unknown Failure"
}

// cleanupFailedFiles removes generated .txtar files when test execution fails.
func cleanupFailedFiles(candidate, testdataDir string) {
	base := strings.ToUpper(candidate[:1]) + candidate[1:]

	// Exact match
	os.Remove(filepath.Join(testdataDir, base+".txtar"))

	// Pattern match for variants/multi-file
	pattern := filepath.Join(testdataDir, base+"_*.txtar")
	matches, _ := filepath.Glob(pattern)
	for _, m := range matches {
		os.Remove(m)
	}
}

// reportResults prints the summary histogram.
func reportResults(results map[string][]string) {
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
