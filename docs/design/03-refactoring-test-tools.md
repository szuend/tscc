# Refactoring `portcase` and `batchport`

## Motivation

As the complexity of test porting has grown (handling multi-file tests, error propagation across dependency graphs, and managing configuration variants), the `tools/portcase` and `tools/batchport` utilities have accumulated technical debt. Functions like `Porter.Port()` in `portcase` and `discoverCandidates()` in `batchport` have become monolithic, making them difficult to read, test, and maintain. 

This document outlines a proposed refactoring strategy to break down these monoliths into focused, single-responsibility components.

## Proposed Changes to `tools/portcase`

The `Porter.Port()` method in `tools/portcase/porter.go` currently spans hundreds of lines and handles everything from parsing directives to graph analysis and generating `.txtar` content.

### 1. Extract Dependency Graph Analysis
The logic responsible for mapping dependencies and propagating expected errors should be extracted.
*   **New Method:** `func (p *Porter) propagateErrors(inputs map[string]string, errorCodesMap map[string][]string, inputList []string)`
*   **Responsibility:** Uses the helpers in `deps.go` to build a directed graph of imports/references and transitively applies TS error codes from dependencies to their importers.

### 2. Extract Output Grouping
The logic that clusters expected baseline outputs (e.g., `a.js`, `a.d.ts`, `a.js.map`) by their base file stem should be isolated.
*   **New Method:** `func groupOutputs(outputs []OutputFile) map[string][]map[string]string`
*   **Responsibility:** Takes a flat list of parsed baseline files and returns them grouped by base filename and occurrence.

### 3. Extract Variant Rendering
The deepest loop in `Port()` iterates over configuration variants (e.g., different `--target` or `--module` settings). This logic should be a standalone function.
*   **New Method:** `func (p *Porter) renderVariant(variant Variant, inputFile string, inputIndex int, ...)`
*   **Responsibility:** Responsible for a single `.txtar` generation. It determines the final output filename, calculates the necessary CLI flags (including injecting `--ambient-type-file` for scripts), assigns expected/unexpected outputs, and calls `RenderTxtar`.

## Proposed Changes to `tools/batchport`

The `discoverCandidates()` function in `tools/batchport/main.go` currently mixes filesystem traversal, `txtar` parsing, and candidate filtering.

### 1. Extract Existing Case Discovery
The logic that reads the `testdata` directory and parses the headers of existing `.txtar` files to determine their upstream origins should be isolated.
*   **New Method:** `func getExistingCases(testdataDir string, suiteName string, allCases []string) (map[string]bool, []string, error)`
*   **Responsibility:** Returns a map of lowercased case names that have already been ported, and a list of their true-cased names.

### 2. Extract Candidate Filtering
The filtering logic that applies the `ignoreList` and deduplicates candidates should be separated from the filesystem walking logic.
*   **New Method:** `func filterCandidates(allCases []string, existingCases map[string]bool) []string`
*   **Responsibility:** Takes the full list of upstream cases and the list of already ported cases, applies the ignore list, and returns the final list of candidates to be ported.

## Impact

*   **Readability:** Breaking down these massive functions will make the control flow of test generation much easier to follow.
*   **Testability:** Smaller, pure functions (like `groupOutputs` and `filterCandidates`) can be unit-tested directly without needing to mock the entire filesystem or `tsccbridge`.
*   **Maintainability:** Future changes to test generation logic (e.g., handling new TS directives or output types) will be localized to specific, well-defined functions.
