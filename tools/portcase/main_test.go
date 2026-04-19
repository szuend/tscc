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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortKnownCase(t *testing.T) {
	// Re-run the tool against a known case
	outPath := filepath.Join(t.TempDir(), "ArrowFunctionExpression1.txtar")

	cmd := exec.Command("go", "run", "./tools/portcase", "--case", "ArrowFunctionExpression1", "--out", outPath)
	cmd.Dir = "." // run in tools/portcase? Wait, the tool expects to be run from repo root.
	// We need to run it from the repository root.
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("portcase failed: %v\n%s", err, out)
	}

	generatedData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	// We expect it to match the hand-ported fixture cmd/tscc/testdata/ArrowFunctionExpression1.txtar
	// Modulo header comment and whitespace.
	existingPath := filepath.Join("../..", "cmd", "tscc", "testdata", "ArrowFunctionExpression1.txtar")
	existingData, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read existing fixture: %v", err)
	}

	genLines := stripNoise(string(generatedData))
	existLines := stripNoise(string(existingData))

	if !equalLines(genLines, existLines) {
		t.Errorf("Regression failed.\nGenerated:\n%s\n\nExisting:\n%s", strings.Join(genLines, "\n"), strings.Join(existLines, "\n"))
	}
}

func stripNoise(content string) []string {
	var lines []string
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPortKnownCase_Declaration(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "DeclarationEmitFunctionDuplicateNamespace.txtar")

	cmd := exec.Command("go", "run", "./tools/portcase", "--case", "declarationEmitFunctionDuplicateNamespace", "--out", outPath)
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("portcase failed: %v\n%s", err, out)
	}

	generatedData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	// Just check if it contains the declaration file assertions
	content := string(generatedData)
	if !strings.Contains(content, "cmp declarationEmitFunctionDuplicateNamespace.d.ts declarationEmitFunctionDuplicateNamespace.d.ts.golden") {
		t.Errorf("Missing cmp assertion for .d.ts file")
	}
	if !strings.Contains(content, "-- declarationEmitFunctionDuplicateNamespace.d.ts.golden --") {
		t.Errorf("Missing .d.ts golden block")
	}
}
