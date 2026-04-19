# Roadmap

> The target semantics — hermetic FS, literal resolution, option pinning, build-system-oriented outputs — are specified in [`vision.md`](vision.md) and [`design/02-deterministic-resolution.md`](design/02-deterministic-resolution.md). This roadmap is the *sequencing* plan; those docs are the *what*. When they disagree, the design docs win.

## Where we are

The compilation-pipeline lift ([`roadmap-lift-compilation.md`](roadmap-lift-compilation.md)) is done. `cmd/tscc/main.go` drives `compile.Compile` directly; `tsccbridge.CommandLine` is gone. The hermetic FS, literal resolver, dual-FS host, diagnostic collection, and tsc-aligned exit status all ship. The CLI surface is `--target`, `--strict`, `--path`, `--case-sensitive-paths`, `--out-js`. Four `.txtar` e2e fixtures pass.

## What this roadmap covers

Five milestones that make `tscc` usable as a real build-system compiler and testable against the broader TypeScript corpus. Each has its own design doc — the sections below are the sequencing plan; binding specs live in the linked docs.

1. `--out-deps` — the headline feature, unscaffolded today. Spec: [`design/03-out-deps.md`](design/03-out-deps.md).
2. `--module` — `compile.go:68` silently hardcodes `ModuleKindESNext`; removing that unblocks ~hundreds of upstream tests. Spec: [`design/04-module-flag.md`](design/04-module-flag.md).
3. `--out-dts` — declaration emit; ~688 upstream cases gate on it. Spec: [`design/05-out-dts.md`](design/05-out-dts.md).
4. A corpus porting tool — 4 hand-written `.txtar` fixtures are not a scalable base. Spec: [`design/06-portcase.md`](design/06-portcase.md).
5. `--out-map` — last of the standard output flags named in the lift roadmap. Spec: [`design/07-out-map.md`](design/07-out-map.md).

Each milestone is several tiny commits. No milestone regresses the others; after each, `go test ./...` and `go vet ./...` stay green and `./tscc --help` shows the new flag.

## Milestone 1 — `--out-deps`

**Goal.** A single-shot invocation writes a Make-compatible `.d` file listing every source file the compile consumed: lib files, the input TS, and every transitive import. Build systems consume this to rebuild by content, not timestamp. This is the feature `vision.md §5` calls out as tscc's reason to exist separate from `tsc`.

**Source of truth for deps.** Walk `program.SourceFiles()`, not `hermeticfs.Reads()`. The jailed-FS read set misses `GetSourceFile` reads that go through the raw FS (design §7), including `--path`-mapped targets. `program.SourceFiles()` is the authoritative transitive set.

**Bridge delta.**
- Confirm `Program.SourceFiles()` (or equivalent accessor) is exported through `tsccbridge`; extend `tools/genbridge/main.go` if not.

**New package** `internal/depsfile`:

```go
// Write renders a Make-compatible dependency snippet: "target: dep1 dep2 …\n".
// inputs are sorted and deduplicated here so callers cannot accidentally emit
// unstable output. Escapes spaces and '$' per Make syntax.
func Write(w io.Writer, target string, inputs []string) error
```

No FS dependency; unit-testable in isolation.

**Config / compile plumbing.**
- `internal/config/config.go` — `OutDepsPath string` in `Config`; `--out-deps FILE` in `outputGroup`; absolute-path resolution mirroring `--out-js`.
- `internal/compile/compile.go` — after a successful emit (`errorCount == 0`), if `cfg.OutDepsPath != ""`, gather `program.SourceFiles()`, extract their paths, hand to `depsfile.Write`. Target name is the emitted JS path (respecting any `--out-js` remap).

**Tests.**
- Unit: `depsfile_test.go` — single input, many inputs, filenames containing spaces / `$` / `:` (Make escape corner cases), empty-inputs error.
- E2E: `cmd/tscc/testdata/depsfile.txtar` — two-file input with a relative import; assert the `.d` lists both plus the expected `lib.*.d.ts` files.
- E2E: `depsfile_pathmap.txtar` — exercises a `--path`-mapped import; confirms the mapped target appears in the deps list (the canary that rawFS reads are counted, not only jailed ones).

**Determinism.** The deps list is a serialized list (design §8). Sort by absolute path before writing. Two invocations with identical inputs must produce byte-identical `.d`.

**Commit shape (~4):**
1. Bridge `Program.SourceFiles()` if not already exported.
2. New `internal/depsfile` package + unit tests.
3. `--out-deps` config flag + tests.
4. Wire into `compile.Compile` + e2e fixtures.

**Exit criteria.** `tscc --out-deps a.d a.ts` emits `a.d` covering the transitive input set. Re-running in a different `cwd` produces a byte-identical file.

## Milestone 2 — `--module`

**Goal.** Let callers pick the module system: `esnext`, `commonjs`, `es2015`, `es2020`, `es2022`, `node16`, `nodenext`, `amd`, `umd`, `system`, `none`. Removes the silent `compile.go:68` hardcode.

**Bridge delta.**
- Export `ModuleKindCommonJS`, `ModuleKindAMD`, `ModuleKindUMD`, `ModuleKindSystem`, `ModuleKindNode16`, `ModuleKindNodeNext`, `ModuleKindES2015`, `ModuleKindES2020`, `ModuleKindES2022`, `ModuleKindNone` via `tools/genbridge/main.go`.

**Config / mapping.**
- `internal/config/config.go` — `Module string` on `Config`; `--module` flag in `languageGroup`; validated against a lookup table like `--target`.
- `internal/compileropts/` — extend `BuildParsedCommandLine` to map the string to `CompilerOptions.Module`. Table-driven test.
- `internal/compile/compile.go:68` — **remove** the `opts.Module = tsccbridge.ModuleKindESNext` line. Default to ESNext in the mapping layer if `cfg.Module == ""` to preserve current behavior.

**Tests.**
- Unit: every documented value round-trips; case-insensitivity; unknown errors.
- E2E: port one CommonJS case from upstream (a `commonjs`-targeted fixture in `_submodules/TypeScript/tests/cases/compiler/`); assert `require()` / `exports` emit.

**Commit shape (~3):** bridge exports, config + mapping, remove hardcode + e2e.

**Exit criteria.** `--module commonjs a.ts` emits CJS; current ESNext fixtures unchanged.

## Milestone 3 — `--out-dts`

**Goal.** Emit `.d.ts` alongside (or instead of) `.js`. Required for any `rules_ts`-style consumer.

**Config / mapping.**
- `internal/config/config.go` — `OutDtsPath string` on `Config`; `--out-dts FILE` in `outputGroup`. Setting it implies `Declaration = true`.
- `internal/compileropts/` — map `cfg` → `CompilerOptions.Declaration` (Tristate).
- `internal/compile/compile.go` — extend the `writeFile` callback: when `filepath.Ext(fileName) == ".d.ts"` and `cfg.OutDtsPath != ""`, route to that path. Mirror the `.js` remap branch.

**Tests.**
- Unit: `Declaration` tristate mapping in `compileropts_test.go`.
- E2E: `dtsEmit.txtar` — exported const, two goldens (`.js` + `.d.ts`). Error case: type error present → no `.d.ts` written; exit non-zero.

**Commit shape (~3):** config + mapping, `writeFile` remap, e2e.

**Exit criteria.** `tscc --out-dts a.d.ts a.ts` produces a `.d.ts` with the exported shape. Missing `--out-dts` behaves exactly as today.

## Milestone 4 — Corpus porting tool

**Goal.** Mechanically convert upstream `tests/cases/compiler/<name>.ts` + its baselines into a `.txtar` fixture. Scales the e2e corpus from 4 → hundreds without hand-editing. Operator-driven (`go run ./tools/portcase --case <name>`), not wired into `go generate`.

**Location.** `tools/portcase/main.go` (new). Reuses `third_party/typescript-go/internal/testrunner/test_case_parser.go` for directive parsing — don't re-implement.

**Directive → flag table (starter set):**

| Upstream directive       | tscc flag                        | Requires |
|--------------------------|----------------------------------|----------|
| `@target: X`             | `--target X`                     | today    |
| `@strict: false`         | `--no-strict`                    | today    |
| `@module: X`             | `--module X`                     | M2       |
| `@declaration: true`     | `--out-dts <generated>`          | M3       |
| `@filename: X`           | `-- X --` block inside `.txtar`  | today    |

Unsupported directives (`@jsx`, `@allowJs`, `@lib`, `@outDir`, `@emitDeclarationOnly`, …) cause the tool to skip the case and emit a one-line report so the unreachable surface is visible.

**Baseline ingestion.**
- `_submodules/TypeScript/tests/baselines/reference/<name>.js` — split on `//// [file.ts] ////` markers into matching golden blocks keyed to the inputs.
- `_submodules/TypeScript/tests/baselines/reference/<name>.errors.txt` — extract diagnostic codes (`TSxxxx`) for `stderr 'TSxxxx'` assertions; full-text comparison is deferred (see "Work deferred" below).

**Tests.**
- Unit: directive parser over fixture strings; baseline splitter over a known multi-file baseline.
- Regression: run `portcase --case arrowFunctionExpression1`, diff against the already-hand-ported `ArrowFunctionExpression1.txtar`. Must match modulo whitespace.

**First bulk batch.** Once the tool works, port a starter set: ~50 class cases + ~50 function cases + ~50 module cases, landed as individual commits (tiny-commits rule applies). Expect some to reveal bugs in tscc — file issues, skip the case, move on.

**Commit shape (~4 tool commits + N fixture commits):** main skeleton + directive parser, baseline ingestion, flag translation table, one or more bulk-port commits.

**Exit criteria.** `go run ./tools/portcase --case <any-simple-case>` produces a `.txtar` that `go test ./cmd/tscc/...` accepts. At least one bulk batch (~50 cases) lands.

## Milestone 5 — `--out-map`

**Goal.** Emit `.js.map` source maps. Last of the standard output flags named in the lift roadmap.

**Config / mapping.**
- `internal/config/config.go` — `OutMapPath string` on `Config`; `--out-map FILE` in `outputGroup`. Implies `SourceMap = true`.
- `internal/compileropts/` — map `cfg` → `CompilerOptions.SourceMap` (Tristate).
- `internal/compile/compile.go` — `writeFile` remap: `.js.map` → `cfg.OutMapPath`. The `sourceMappingURL=` comment inside the emitted `.js` must match the remapped path. Verify whether typescript-go handles this automatically when the writer renames the file; if not, post-emit rewrite in `writeFile`.

**Tests.**
- E2E: `sourcemapEmit.txtar` — assert `.js.map` golden; assert `.js` contains `//# sourceMappingURL=` pointing at the remapped path.

**Commit shape (~3):** config + mapping, `writeFile` remap, e2e.

**Exit criteria.** `--out-map a.js.map a.ts` produces a valid v3 source map; the emitted `.js` references it correctly.

## How features layer on after this roadmap

These are specified in the design docs but deferred beyond the five milestones above:

- **`--lib` selector.** Requires understanding how `bundled.LibPath()` indexes the embedded `.d.ts` set. Bigger than it looks.
- **Individual strict sub-flags** — `--no-implicit-any`, `--strict-null-checks`, etc. Mechanical but many; only becomes urgent once M4's bulk porting hits cases using them granularly.
- **`--jsx`.** Not explicitly out of scope per `vision.md`, but a large separate pipeline (factory, fragment, preserve / react / react-jsx emit modes). Its own roadmap when scheduled.
- **`--path-prefix-map`** (design §9). Audit all emit sites for absolute-path leakage first.
- **Response files (`@argsfile`)** per design §10. Small, independent, nice-to-have for long command lines under Bazel.
- **Rich diagnostic baseline comparison.** Normalize line endings, strip absolute paths and version strings before comparing against `.errors.txt`. Needed once M4 starts ingesting error baselines in volume; deferred until M4 actually hits the wall.

## Work explicitly out of scope

Per `vision.md` "Explicit non-goals": watch mode, incremental / `tsbuildinfo`, `tsconfig.json` discovery, multi-project / composite builds, LSP, environment-variable configuration, module resolution via `package.json` walks. Features that conflict with these are declined.

## Open questions

- **Depsfile target path under `--out-js`.** If the user passes both `--out-js foo.js` and `--out-deps foo.d`, the depsfile's target should be `foo.js`, not the default output path. Double-check this in M1 by running with and without `--out-js` and diffing.
- **Lib files in `SourceFiles()`.** Do embedded `lib.*.d.ts` paths surface as their `bundled://` virtual paths, as real host paths, or as something else? The answer determines whether the depsfile is portable across hosts (vision §1). If `bundled://` leaks, M1 must canonicalize.
- **M4 flakiness budget.** Some upstream baselines bake in typescript-go-version-specific error messages or ordering that mechanical porting will trip on. How many ported cases can we tolerate flaking on at import time before the tool needs hardening?
