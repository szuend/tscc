# Design Doc: Porting Tools v2 (`portcase` improvements & `batchport`)

This document outlines the design for upgrading the `portcase` utility to handle more complex test cases, as well as the introduction of a new `batchport` harness to automate and track test migrations at scale.

## 1. `portcase` Enhancements

`portcase` translates upstream TypeScript compiler tests into `tscc`'s `txtar` format. Currently, it supports only single-input, single-run tests. We will introduce three key features to expand its coverage.

### 1.1. "Does Not Write" Checks
Because `tscc` requires explicit output flags (e.g., `--out-js`, `--out-dts`), it's crucial to assert that files are *not* written when their flags are missing.
*   **Design:** `portcase` will determine the expected outputs for a given input file (e.g., `a.js`, `a.d.ts`, `a.js.map`). If a directive like `@declaration true` is missing, it will generate a `! exists a.d.ts` assertion in the `.txtar` script.
*   **Purpose:** Ensures strict adherence to `tscc`'s explicit-output invariant and prevents output cross-contamination.

### 1.2. Multi-File Support (Matrix on Disk)
Upstream tests often contain multiple input files. Since `tscc` only accepts a single input file per invocation, we must test each input file's compilation independently.
*   **Design:** Instead of generating a single complex `.txtar` script with sequential executions, `portcase` will split multi-file tests into **multiple independent `.txtar` files**.
    *   File naming convention: `<case_name>_<input_file_stem>.txtar` (e.g., `foo_a.txtar`, `foo_b.txtar`).
    *   Each `.txtar` file populates the virtual filesystem with *all* the input `.ts` files (so the resolver can resolve cross-file imports), but only executes `tscc` against its designated input file.
    *   The "does not write" checks trivially assert that unrequested files (like the output for `b.ts` when compiling `a.ts`) are not present.
*   **Edge Case: Global Merging:** Upstream tests that rely on multiple files sharing a global scope *without explicit imports* (e.g., `tsc a.ts b.ts` where `b.ts` uses a global defined in `a.ts`) cannot be supported by `tscc`'s single-entrypoint architecture. `portcase` or `batchport` will categorize these as `Unsupported: multi-file global`.

### 1.3. Multi-Value Directives (Variants)
Some upstream tests specify multiple values for a single directive, such as `@target esnext, es2015`. These indicate that the test should be verified under multiple configurations.
*   **Design:** `portcase` will compute the Cartesian product of all multi-value directives, producing a set of configuration variants.
    *   Combined with the multi-file support, this results in a full **Matrix on Disk**.
    *   File naming convention: `<case_name>_<input_file_stem>_<variant>.txtar` (e.g., `foo_a_esnext.txtar`, `foo_a_es2015.txtar`).
    *   This provides perfect isolation and ensures that a failure in one variant (e.g., `es2015`) does not prevent the test runner from executing and reporting the status of other variants (e.g., `esnext`).

---

## 2. `batchport` Harness

The `batchport` harness will automate the invocation of `portcase` across the upstream test corpus, helping us systematically map out unsupported features and test divergence.

### 2.1. Architecture and Workflow
1.  **Discovery:** Scan `third_party/typescript-go/_submodules/TypeScript/tests/cases/compiler/*.ts` for all upstream tests.
2.  **Deduplication:** Skip tests that already have a corresponding `.txtar` file in `cmd/tscc/testdata/`.
3.  **Iterative Batching:** Select candidate tests and process them until a target of **$N$ successful migrations** is reached. Tracking successes (rather than total processed) prevents the run from stalling on a localized cluster of unsupported features.
4.  **Trial Migration:** For each candidate:
    *   Invoke `portcase`.
    *   **If `portcase` fails:** Parse its stderr to extract the failure category (e.g., `Unsupported directive @jsx`, `Unsupported directive @lib`).
    *   **If `portcase` succeeds:**
        *   Execute the resulting test: `go test ./cmd/tscc/... -run TestScript/<CaseName>`.
        *   **If the test fails:** Categorize as `Diverged` (e.g., output mismatch, unexpected diagnostics).
        *   **If the test passes:** Categorize as `Success` and increment the success counter.
5.  **Reporting:** Output a summary histogram grouping the test candidates by their outcome.

### 2.2. Command Line Interface
*   `batchport -n <count>`: The target number of *successful* cases to port before the harness stops. Defaults to a reasonable number (e.g., 10).
*   `batchport --verbose`: A flag that outputs detailed line items for every processed file and its specific failure reason, in addition to the final histogram.
