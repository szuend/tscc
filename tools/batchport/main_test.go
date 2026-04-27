package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGetExistingCases(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy txtar file
	dummyTxtar := filepath.Join(tempDir, "dummy.txtar")
	err := os.WriteFile(dummyTxtar, []byte("# Ported from tests/cases/compiler/FooBar.ts by tools/portcase.\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write dummy txtar: %v", err)
	}

	allCases := []string{"FooBar"}
	existingCases, existingCaseNames, err := getExistingCases(tempDir, "compiler", allCases)
	if err != nil {
		t.Fatalf("getExistingCases failed: %v", err)
	}

	if !existingCases["foobar"] {
		t.Errorf("Expected existingCases to contain 'foobar', got %v", existingCases)
	}

	expectedNames := []string{"FooBar"}
	if !reflect.DeepEqual(existingCaseNames, expectedNames) {
		t.Errorf("Expected existingCaseNames to be %v, got %v", expectedNames, existingCaseNames)
	}
}

func TestFilterCandidates(t *testing.T) {
	allCases := []string{"Case1", "Case2", "transportstream", "allowJsCrossMonorepoPackage"}
	existingCases := map[string]bool{"case1": true}
	existingCaseNames := []string{"Case1"}

	// Test updateExisting = false
	candidates := filterCandidates(allCases, existingCases, existingCaseNames, false)
	expectedCandidates := []string{"Case2"}
	if !reflect.DeepEqual(candidates, expectedCandidates) {
		t.Errorf("filterCandidates (updateExisting=false) returned %v, expected %v", candidates, expectedCandidates)
	}

	// Test updateExisting = true
	candidatesUpdate := filterCandidates(allCases, existingCases, existingCaseNames, true)
	expectedUpdateCandidates := []string{"Case1"}
	if !reflect.DeepEqual(candidatesUpdate, expectedUpdateCandidates) {
		t.Errorf("filterCandidates (updateExisting=true) returned %v, expected %v", candidatesUpdate, expectedUpdateCandidates)
	}
}
