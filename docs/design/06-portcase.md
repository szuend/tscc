# Design Doc: `tools/portcase` (corpus porting tool)

> Milestone 4 of [`docs/roadmap.md`](../roadmap.md).

## Goal

Mechanically convert a single upstream TypeScript compiler test case — `_submodules/TypeScript/tests/cases/compiler/<name>.ts` plus its baselines — into a `.txtar` fixture that `go test ./cmd/tscc/...` consumes unchanged. Scales the e2e corpus from 4 hand-written fixtures toward hundreds without hand-editing. The tool is operator-driven (`go run ./tools/portcase --case <name>`), not wired into `go generate`.

## Why this matters

- **The hand-written e2e coverage is not sustainable.** 4 fixtures vs. 6,537 upstream single-file compiler cases is a 0.06% sample. Everything below that noise floor is untested behavior.
- **Hand-porting bakes in author bias.** Cases that happen to land are ones the operator thought to try. Mechanical porting surfaces cases that expose real bugs nobody would think to write from scratch.
- **Determinism failures surface in bulk only.** One ported case may accidentally pass; 100 ported cases make bugs visible as a pattern (e.g., a whole family of `--module commonjs` cases failing the same way points at a mapping bug, not at the individual case).

## Non-goals

- **No automatic baseline diffing.** We translate inputs and assert on coarse signals (exit status + error codes); full-text diagnostic comparison is deferred (see §Deferred).
- **No CI integration.** The tool is for humans porting cases in batches. CI runs only the already-ported fixtures.
- **No modifications to upstream `_submodules/TypeScript/`.** Tool is read-only against the submodule; all outputs land under `cmd/tscc/testdata/`.

## Directive parsing: reuse upstream parser via bridge

The roadmap suggests reusing `third_party/typescript-go/internal/testrunner/test_case_parser.go`. **Follow this plan.** While an initial iteration attempted to use a simple regex parser, it quickly became apparent that the upstream parser handles stateful parsing (e.g. file-specific overrides like `@filename: a.ts` followed by `@module:`) and target permutations (e.g. `@target: es5, es2015`) perfectly. Re-implementing this logic is error-prone and limits the cases we can successfully port.

By updating `tools/genbridge/main.go` to re-export `testrunner.ParseTestFilesAndSymlinks` inside the generated `tsccbridge/bridge.go` file, we achieve bug-for-bug compatibility with upstream without having to maintain a fragile regex parser.

## Directive → flag translation

Starter table (expand as M2/M3/M5 land):

| Upstream directive       | tscc flag                             | Available          | Notes |
|--------------------------|---------------------------------------|--------------------|-------|
| `@target: X`             | `--target X`                          | today              | Case-insensitive match against `targetByName`. |
| `@strict: true`          | `--strict`                            | today              | Default already `true`; emit flag anyway for clarity. |
| `@strict: false`         | `--no-strict`                         | today              | |
| `@module: X`             | `--module X`                          | M2                 | 1:1 name match. If unrecognized, skip the case. |
| `@declaration: true`     | `--out-dts <generated>.d.ts`          | M3                 | Generated path is `<basename>.d.ts` inside the txtar's work area. |
| `@sourceMap: true`       | `--out-map <generated>.js.map`        | M5                 | Same pattern. |
| `@filename: X`           | `-- X --` block inside `.txtar`       | today              | Standard txtar file marker. The default unnamed file becomes `X`. |

**Unsupported directives** — skip the case, emit a one-line report (`<name>: unsupported directive @jsx`). The current blocklist (subject to expansion):

- `@jsx`, `@jsxFactory`, `@jsxFragmentFactory`, `@jsxImportSource`
- `@allowJs`, `@checkJs`
- `@lib` (covered by a future milestone)
- `@outDir`, `@outFile`, `@rootDir`
- `@emitDeclarationOnly` (until the flag exists)
- `@traceResolution`, `@listFiles`, `@listEmittedFiles`
- `@moduleResolution` other than bundler-equivalents (literal resolver is the only mode)
- `@paths` (the upstream `paths` feature differs from tscc's `--path`; deferred until we implement the broader surface).
- Anything with per-file variants (`@filename: foo.ts` followed by file-specific `@module:` — requires stateful parsing we don't support yet).

The rule: any directive not in the translation table or the `@filename` structural set → skip. Silent drops would hide coverage gaps; the one-line report makes the unsupported surface visible and sortable.

## Baseline ingestion

Two files per upstream case:

- `_submodules/TypeScript/tests/baselines/reference/<name>.js`: may contain multiple `//// [file.ts] ////` blocks, one per input file. Split on the marker; each block becomes one golden in the txtar keyed to the matching input.
- `_submodules/TypeScript/tests/baselines/reference/<name>.errors.txt`: contains diagnostic records with file locations, code, and message. **Extract only the `TSxxxx` codes** into `stderr 'TSxxxx'` assertions for the txtar. Full-text comparison is deferred (§Deferred).

If `.errors.txt` is absent, the compile is expected to succeed: emit `! stderr .` instead of `stderr 'TSxxxx'`.

### Baseline quirks to handle

Upstream baselines include header comments (`//// [tests/cases/compiler/<name>.ts] ////`) that are noise. Strip them. They include trailing `//# sourceMappingURL` comments when `@sourceMap` is set; compare after the trailing newline.

## Generated txtar shape

```
# Ported from tests/cases/compiler/<name>.ts by tools/portcase on <date>.
# DO NOT EDIT by hand; re-run the porter if the upstream baseline changes.
<assertion line 1: exec or ! exec>
<assertion line 2: stderr match or ! stderr .>
<cmp lines>

-- <input1> --
<input1 content>
-- <input1>.golden --
<input1 baseline content>
-- <input2> --
...
```

The header comment is the single source of provenance. The regression test (`portcase --case arrowFunctionExpression1` diffed against the hand-written `ArrowFunctionExpression1.txtar`) pins the format: if a future refactor reshapes the template, it fails immediately rather than silently diverging from existing hand-ported fixtures.

## Tool skeleton

```
tools/portcase/
  main.go        # flag parsing, orchestration
  directives.go  # ParseDirectives + translation table
  baseline.go    # baseline splitter, error-code extractor
  render.go      # txtar template renderer
  main_test.go
  directives_test.go
  baseline_test.go
  render_test.go
```

Flags:

```
go run ./tools/portcase --case <name> [--out <path>] [--force]
```

- `--case <name>`: required. The upstream test name (without path or extension).
- `--out <path>`: optional. Defaults to `cmd/tscc/testdata/<Name>.txtar` (PascalCased for filesystem clarity, matching `ArrowFunctionExpression1.txtar`).
- `--force`: overwrite an existing fixture. Default is refuse.

Exit codes: `0` on success (file written), `1` on error (submodule missing, upstream case not found, I/O error), `2` on skip (unsupported directive). Skip is a dedicated exit code so bulk scripts can distinguish "couldn't port this one" from "tool broke".

## Tests

**Unit:**
- `directives_test.go`: table-driven over synthetic source strings covering every row in the translation table plus several unsupported directives.
- `baseline_test.go`: splitter over a multi-file baseline string (constructed in the test); error-code extractor over a synthetic `.errors.txt`.
- `render_test.go`: golden-compare the rendered txtar against a checked-in expected-output fixture under `tools/portcase/testdata/`.

**Regression:**
- `main_test.go`: `TestPortKnownCase` runs `portcase --case arrowFunctionExpression1`, diffs the output against the existing hand-ported `cmd/tscc/testdata/ArrowFunctionExpression1.txtar` modulo the auto-generated header comment and whitespace. If the diff is non-trivial, either the tool regressed or the hand-ported fixture drifted; either way, investigate before green-lighting.

## The bulk port, and the flakiness budget

Once the tool works, batch-port a starter set — the roadmap suggests ~50 class cases + ~50 function cases + ~50 module cases. **Open question surfaced on the roadmap:** some upstream baselines bake in tsc-version-specific error messages or diagnostic ordering that mechanical porting will trip on.

Budget rule of thumb: if more than 30% of a batch fails, stop and diagnose the common cause before continuing. Individual-case failures are expected (real tscc bugs); systemic failures indicate a tool gap.

For each failing case:
1. **Real tscc bug** → file issue, add to a `known-issues.md`, move on. Don't fix in the porting commit.
2. **Baseline noise** (version strings, path formatting) → defer until the rich-diagnostic-comparison work (see design §9 / roadmap "Work deferred"); skip via report and move on.
3. **Unsupported directive** → add to the blocklist; skip and move on.

Tiny-commits rule applies: each bulk-port commit adds ~1–10 fixtures and documents which cases were skipped and why in the commit message.

## Commit shape

1. **Tool skeleton + directive parser** (no baseline ingestion yet, no rendering; CLI shape and directive table with unit tests).
2. **Baseline ingestion** (splitter, error-code extractor, unit tests over synthetic baselines).
3. **Flag translation table + txtar rendering** (the regression test passes against `ArrowFunctionExpression1`).
4. **First bulk-port commit** (~10–20 cases; set the tone for subsequent bulk commits).
5. **Subsequent bulk-port commits**, each small enough to review.

The tool can ship before M2 and M3, but its directive table can only grow once those milestones land — schedule accordingly.

## Exit criteria

1. `go run ./tools/portcase --case <any-simple-case>` produces a `.txtar` file.
2. `go test ./cmd/tscc/...` passes against the generated fixture.
3. Regression test `portcase --case arrowFunctionExpression1` diffs clean against the hand-ported equivalent.
4. At least one bulk-port batch (~50 cases) lands, with skip rate under 50% (higher rates block further bulk ports until the tool or tscc is hardened).
5. The skip report surfaces the unsupported-directive distribution — actionable input for future milestones.

## Deferred

- **Rich diagnostic baseline comparison.** Full text of `.errors.txt` against `stderr`. Requires: line-ending normalization, stripping absolute paths (a job for `--path-prefix-map`), stripping version strings, stripping typescript-go-vs-tsc diagnostic-wording drift. This is its own project; see [roadmap.md](../roadmap.md) "Work deferred beyond these five milestones".
- **Multi-case CLI mode.** `portcase --case a,b,c` or `portcase --all-matching '<regex>'`. Useful for bulk porting but adds scope that's easy to hand-wrap in a shell loop for now.
- **Automatic re-porting on upstream bumps.** When the typescript-go submodule moves, some baselines change. A tool that detects divergence and suggests re-ports is high-value but out of M4 scope.
- **fourslash/tsserver cases.** Those live under a different directory and need a different translation pipeline entirely. Not in scope.
