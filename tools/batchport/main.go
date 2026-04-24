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
	"context"
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
	var stopOnFailure bool
	var updateExisting bool
	var suiteName string

	pflag.IntVarP(&targetCount, "count", "n", 10, "Target number of successful migrations")
	pflag.BoolVar(&verbose, "verbose", false, "Output detailed line items")
	pflag.IntVarP(&concurrency, "concurrency", "j", 4, "Number of concurrent workers")
	pflag.BoolVarP(&stopOnFailure, "stop-on-failure", "x", false, "Stop on the first failure and print detailed error")
	pflag.BoolVarP(&updateExisting, "update-existing", "u", false, "Re-import existing automatically ported tests instead of finding new ones")
	pflag.StringVar(&suiteName, "suite", "compiler", "The upstream test suite (e.g. compiler, conformance)")
	pflag.Parse()

	upstreamDir := filepath.Join("third_party", "typescript-go", "_submodules", "TypeScript", "tests", "cases", suiteName)
	testdataDir := filepath.Join("cmd", "tscc", "testdata", suiteName)

	// 1. Discovery & Deduplication
	candidates, err := discoverCandidates(upstreamDir, testdataDir, updateExisting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 2. Build tools once
	portcasePath, testBinPath, cleanup, err := buildTools()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building tools: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	// 3. Iterative Batching in Parallel
	successCount := 0
	results := make(map[string][]string) // category -> cases
	var mu sync.Mutex

	tasks := make(chan string)
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Spawn workers
	for i := 0; i < concurrency; i++ {
		wg.Go(func() {
			for c := range tasks {
				cat, errDetail := processCandidate(ctx, c, portcasePath, testBinPath, testdataDir, updateExisting, suiteName)

				mu.Lock()
				results[cat] = append(results[cat], c)
				if cat == "Success" {
					successCount++
				}
				mu.Unlock()

				if cat != "Success" && !strings.HasPrefix(cat, "Ignored") && stopOnFailure {
					fmt.Printf("\nFailure on %s:\n%s\n", c, errDetail)
					cancel()
				}

				if verbose {
					fmt.Printf("Case %s: %s\n", c, cat)
				}
			}
		})
	}

	// Send tasks
loop:
	for _, candidate := range candidates {
		mu.Lock()
		if successCount >= targetCount {
			mu.Unlock()
			break
		}
		mu.Unlock()

		select {
		case <-ctx.Done():
			break loop
		case tasks <- candidate:
		}
	}
	close(tasks)
	wg.Wait()

	// 4. Reporting
	reportResults(results)
}

// discoverCandidates scans upstream tests and filters out already ported ones.
func discoverCandidates(upstreamDir, testdataDir string, updateExisting bool) ([]string, error) {
	suiteName := filepath.Base(upstreamDir) // Infer suite from directory

	var allCases []string
	err := filepath.WalkDir(upstreamDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".ts") {
			rel, err := filepath.Rel(upstreamDir, path)
			if err != nil {
				return err
			}
			allCases = append(allCases, strings.TrimSuffix(rel, ".ts"))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking upstream dir: %w", err)
	}
	sort.Strings(allCases)

	existingFiles, err := os.ReadDir(testdataDir)
	if err != nil {
		if os.IsNotExist(err) {
			// It's okay if the directory doesn't exist yet
			existingFiles = []os.DirEntry{}
		} else {
			return nil, fmt.Errorf("reading testdata dir: %w", err)
		}
	}

	existingCases := make(map[string]bool)
	var existingCaseNames []string
	for _, f := range existingFiles {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".txtar") {
			path := filepath.Join(testdataDir, f.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			lines := strings.SplitSeq(string(data), "\n")
			for line := range lines {
				prefix := fmt.Sprintf("# Ported from tests/cases/%s/", suiteName)
				if after, ok := strings.CutPrefix(line, prefix); ok {
					caseName := after
					caseName = strings.TrimSuffix(caseName, ".ts by tools/portcase.")

					// Resolve true upstream casing
					lowerCaseName := strings.ToLower(caseName)
					trueCaseName := caseName
					for _, u := range allCases {
						if strings.ToLower(u) == lowerCaseName {
							trueCaseName = u
							break
						}
					}

					existingCases[lowerCaseName] = true
					existingCaseNames = append(existingCaseNames, trueCaseName)
					break
				}
			}
		}
	}

	if updateExisting {
		unique := make(map[string]bool)
		var candidates []string
		for _, name := range existingCaseNames {
			if !unique[name] {
				unique[name] = true
				candidates = append(candidates, name)
			}
		}
		sort.Strings(candidates)
		fmt.Printf("Found %d already ported cases to update.\n", len(candidates))
		return candidates, nil
	}

	var candidates []string
	ignoreList := map[string]bool{
		"transportstream": true, // Binary input file. We need to copy the exact bytes into the .txtar for this one.

		// Manually maintained tests.
		"aliasusageinarray":                       true,
		"aliasusageinfunctionexpression":          true,
		"aliasusageingenericfunction":             true,
		"aliasusageinindexerofclass":              true,
		"aliasusageinobjectliteral":               true,
		"aliasusageinorexpression":                true,
		"aliasusageintypeargumentofextendsclause": true,
		"aliasusageinvarassignment":               true,

		// Not supported.
		"allowjscrossmonorepopackage": true,
	}

	seenCandidates := make(map[string]bool)
	for _, c := range allCases {
		lower := strings.ToLower(c)
		if ignoreList[lower] || seenCandidates[lower] {
			continue
		}
		if !existingCases[lower] {
			candidates = append(candidates, c)
			seenCandidates[lower] = true
		}
	}

	fmt.Printf("Found %d total cases, %d already ported, %d candidates.\n", len(allCases), len(allCases)-len(candidates), len(candidates))
	return candidates, nil
}

// buildTools builds the portcase tool and the tscc test binary once into a workspace-local tmp directory.
func buildTools() (string, string, func(), error) {
	tmpDir := filepath.Join("tools", "batchport", "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", "", nil, fmt.Errorf("creating tmp dir: %w", err)
	}

	portcasePath := filepath.Join(tmpDir, "portcase")
	cmd := exec.Command("go", "build", "-o", portcasePath, "./tools/portcase")
	if err := cmd.Run(); err != nil {
		return "", "", nil, fmt.Errorf("building portcase: %w", err)
	}

	testBinPath := filepath.Join(tmpDir, "tscc.test")
	cmd = exec.Command("go", "test", "-c", "-o", testBinPath, "./cmd/tscc")
	if err := cmd.Run(); err != nil {
		return "", "", nil, fmt.Errorf("building tscc test binary: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return portcasePath, testBinPath, cleanup, nil
}

func flattenName(name string) string {
	parts := strings.Split(filepath.ToSlash(name), "/")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "_")
}

// processCandidate handles the trial migration and test execution for a single candidate.
func processCandidate(ctx context.Context, candidate, portcasePath, testBinPath, testdataDir string, force bool, suiteName string) (string, string) {
	var cmd *exec.Cmd
	if force {
		cmd = exec.CommandContext(ctx, portcasePath, "--suite", suiteName, "--case", candidate, "--force")
	} else {
		cmd = exec.CommandContext(ctx, portcasePath, "--suite", suiteName, "--case", candidate)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return "Cancelled", ""
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
			return "Ignored", string(out)
		}
		return parseFailureCategory(string(out)), string(out)
	}

	capitalized := flattenName(candidate)
	// Run the pre-compiled test binary directly.
	absTestBinPath, _ := filepath.Abs(testBinPath)
	testRegex := fmt.Sprintf("^TestScript/%s(_.*)?$", capitalized)
	testCmd := exec.CommandContext(ctx, absTestBinPath, "-test.run", testRegex)
	testCmd.Dir = filepath.Join("cmd", "tscc")

	out, err = testCmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return "Cancelled", ""
		}
		cleanupFailedFiles(candidate, testdataDir)
		return "Diverged", string(out)
	}

	return "Success", ""
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
	base := flattenName(candidate)

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
