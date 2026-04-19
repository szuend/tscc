# Design Doc: `--out-deps` (Make-compatible depsfile)

> Milestone 1 of [`docs/roadmap.md`](../roadmap.md). The behavior specified here is binding; the roadmap is *sequencing*. When they disagree, this doc wins.

## Goal

After a successful compile, write a single Make-compatible dependency snippet (`target: dep1 dep2 …\n`) listing every source file the compile transitively consumed: the user's root input, all `lib.*.d.ts` files loaded by the type checker, and every transitively-imported `.ts` / `.d.ts` / JSON module. Build systems (Bazel with `rules_ts`'s depsfile mode, Ninja with `depfile =`, plain Make with `-include`) parse this to rebuild the compile by content, not timestamp.

This is the feature [`vision.md` §5](../vision.md#5-the-build-system-owns-incrementality) calls out as tscc's reason to exist separate from `tsc`: **the build system owns incrementality; tscc reports the input set.**

## Non-goals

- **No `.d` content when the compile fails.** Build systems either trust the depsfile as authoritative or re-run; a partial dep list is worse than none. If emit is skipped or `errorCount > 0`, no depsfile is written. This matches GCC's `-MD` behavior and avoids the "stale depsfile wedges the build" failure mode.
- **No relative paths in output.** Paths are absolute exactly as `program.SourceFiles()` returns them. Portability across machines is `--path-prefix-map`'s job (design §9), not `--out-deps`'s.
- **No transitive depsfile for failed resolutions.** If a bare import can't be mapped, the compile fails and no depsfile is emitted — per the rule above.

## Source of truth: `program.SourceFiles()`, not `hermeticfs.Reads()`

The jailed FS tracks successful `ReadFile` calls, but this undercounts by design: the custom `CompilerHost` (design §7) reads `GetSourceFile`'s already-resolved paths through the **unjailed** raw FS so that `--path`-mapped targets under `node_modules` (and explicit JSON imports) work. Those reads never reach the jailed tracker.

`program.SourceFiles()` is the authoritative set. It is what the emitter walks (`emitter.go:470-488`: `getSourceFilesToEmit(host, …)`), so it is by construction the exact set the compiler *used*. An e2e canary (`depsfile_pathmap.txtar`, below) pins this: the mapped target must appear in the depsfile or we regress into partial tracking.

## Public surface

```
tscc --out-deps /abs/path/to/a.d /abs/a.ts
```

- Path resolution mirrors `--out-js`: `filepath.Abs` at parse time, rejected if empty.
- No short alias; the flag is rare enough that clarity beats brevity.
- Omitting `--out-deps` leaves depsfile generation off — the zero value must be a no-op.

## File format

```
<target>: <dep1> <dep2> <dep3>
```

Exactly one line, one newline terminator. `<target>` is the emitted JS path; `<depN>` are absolute source paths, sorted lexicographically.

**Make escape rules** (per GNU Make manual §4.3 "Prerequisite Types"):

| Character | Escape    |
|-----------|-----------|
| space     | `\ `      |
| tab       | `\ `      |
| `$`       | `$$`      |
| `#`       | `\#`      |
| `\`       | `\\`      |
| `:`       | not escaped in deps; `:` inside a filename is ambiguous in Make — document as unsupported and fail if encountered |
| `*`, `?`, `[` | not escaped — Make treats them as wildcards in targets; for prerequisites, literal `*` is preserved because Make doesn't glob prerequisites at parse time |

`\n` and `\r` in filenames are rejected with an error — no safe Make escape exists, and encountering them almost always indicates a bug upstream rather than a legitimate filename.

## Target name

The target must be what the build system expects to re-produce. That is the **emitted JS path after `--out-js` remap**, not the compiler's internal output name. Concretely:

- If `--out-js /out/a.js` is passed, target = `/out/a.js`.
- If `--out-js` is not passed, target = whatever path the `writeFile` callback wrote (which, in the current code, is the compiler's chosen filename, typically `<input-basename>.js` next to the input).

When M3 (`--out-dts`) and M5 (`--out-map`) land, they add their own writes but **do not change the depsfile's target**. One depsfile, one target — the build system registers the `.js` as the primary output and the `.d.ts` / `.js.map` as sibling outputs sharing the same prerequisite set. This mirrors how `gcc -MD` produces one `.d` per translation unit regardless of `-S` / `-c` variants.

## Determinism

Every input to `depsfile.Write` must be deterministic under the [`vision.md`](../vision.md) invariant "Inputs + Flags = Output":

1. **Sort before write.** `program.SourceFiles()` returns files in loader-scheduling order. Even with `SingleThreaded = true` (design §8), the order depends on import-graph traversal, which is stable but not lexicographic. Sort by absolute path using `sort.Strings` — Unicode code-point order, the same order every filesystem uses for `ls`.
2. **Deduplicate after sort.** Defensive; `program.SourceFiles()` should already be unique, but a slip in upstream's cache (the `filesparser.go` redirect logic) could produce duplicates.
3. **Apply `--path-prefix-map` *before* sorting** (when that lands, deferred past M1). Prefix mapping changes sort keys; sorting first then mapping would produce a non-lexicographic file on disk.
4. **Two invocations with identical inputs produce byte-identical output.** Enforced by the e2e test running the same compile twice in different `cwd`s and `cmp`-ing the depsfiles.

## The `bundled://` question (open, must resolve during M1)

`Program.SourceFiles()` will include lib files (`lib.es2022.d.ts`, …). typescript-go serves these through `bundled.WrapFS`, which overlays a virtual `bundled://` namespace. The question: do lib source-file `FileName`s surface as `bundled://libs/lib.es2022.d.ts` or as real host paths?

- If `bundled://…`: the depsfile is portable (same string on every host) but Make treats `:` as a rule separator — **the URL form breaks Make parsing** and must be canonicalized to a real or synthetic path.
- If real host paths: the depsfile is host-dependent (`/home/alice/go/pkg/mod/github.com/microsoft/typescript-go@…/…`) and unusable in a hermetic build cache.

**Resolution:** the first M1 commit writes a one-shot test that logs the `FileName` of lib files and picks a canonicalization rule:

- If `bundled://`, rewrite to a stable synthetic form (e.g., `/__tscc__/lib/es2022.d.ts`) inside `depsfile.Write`. Document the rewrite at the rule's emit site so the build system understands these are synthetic.
- If real host paths, `depsfile.Write` leaves them alone but the user is responsible for passing `--path-prefix-map` (once it lands) to canonicalize. Until `--path-prefix-map` exists, this is a known gap and the depsfile is not portable across hosts.

Either way, the canonicalization rule is a property of `depsfile.Write`, not of the caller — so future callers can't forget it.

## Package layout

New package `internal/depsfile`:

```go
// Package depsfile renders a Make-compatible dependency snippet.
package depsfile

// Write renders "target: dep1 dep2 …\n" with Make-safe escaping. Inputs are
// sorted and deduplicated before emission; callers cannot accidentally produce
// unstable output. Returns an error for filenames containing newlines or ':'.
func Write(w io.Writer, target string, inputs []string) error
```

No FS dependency. Unit-testable with `bytes.Buffer`. No awareness of `Config`, `Program`, or bridge types.

## Wiring

In `internal/compile/compile.go`, after emit succeeds and `errorCount == 0`:

```go
if cfg.OutDepsPath != "" {
    target := cfg.OutJSPath
    if target == "" {
        target = primaryJSOutputOf(emittedFiles) // the .js we wrote, post-remap
    }
    inputs := make([]string, 0, len(program.SourceFiles()))
    for _, sf := range program.SourceFiles() {
        inputs = append(inputs, canonicalize(sf.FileName()))
    }
    buf := &bytes.Buffer{}
    if err := depsfile.Write(buf, target, inputs); err != nil {
        return nil, tsccbridge.ExitStatusInvalidProject_OutputsSkipped, err
    }
    if err := in.JailedFS.WriteFile(cfg.OutDepsPath, buf.String()); err != nil {
        return nil, tsccbridge.ExitStatusInvalidProject_OutputsSkipped, err
    }
}
```

Placement: **after** emit (so we only write on success), **before** the final exit-status computation (so a depsfile write failure surfaces as a hard error, not a silent `0` from a compile that left the build system stale).

## Tests

**Unit (`depsfile_test.go`):**
- `Write` with zero inputs returns an error ("empty input set").
- One input: `target: /abs/a.ts\n`.
- Many inputs: sorted lexicographically regardless of caller order.
- Duplicate inputs: deduplicated.
- Filenames with space, `$`, `#`, `\`: escape table above.
- Filename with `\n`: error.
- Filename with `:`: error.

**E2E:**
- `cmd/tscc/testdata/depsfile.txtar`: two-file input with a relative import. Assert `.d` lists both source files plus the lib files pulled in by the type checker.
- `cmd/tscc/testdata/depsfile_pathmap.txtar` **(canary)**: compile uses `--path lodash=$WORK/node_modules/lodash/index.d.ts`. Assert the mapped target appears in the depsfile. This is the regression gate against "we went back to using `hermeticfs.Reads()`".
- `cmd/tscc/testdata/depsfile_determinism.txtar`: compile twice, once from `/tmp/a` and once from `/tmp/b`, `cmp` the two depsfiles.
- `cmd/tscc/testdata/depsfile_error.txtar`: intentionally-broken input. Assert `a.d` does not exist after the failed compile.

## Commit shape

1. **`internal/depsfile` package + unit tests.** Pure function, no dependencies. Lands in isolation.
2. **`--out-deps` config flag + `config` tests.** Flag in `outputGroup`, absolute-path resolution mirroring `--out-js`.
3. **Canonicalization rule.** Write the one-shot test that resolves the `bundled://` question; add the canonicalizer with a comment citing this doc. No behavior change to existing tests.
4. **Wire into `compile.Compile` + e2e fixtures.** This commit is the one that changes behavior.

Every commit leaves `go test ./...`, `go vet ./...`, and `./tscc --help` green.

## Exit criteria

1. `./tscc --out-deps /tmp/a.d /abs/a.ts` emits `/tmp/a.d` containing the transitive input set, `.ts` source + lib files, sorted.
2. Re-running from a different `cwd` produces byte-identical `.d`.
3. The `--path`-mapped canary proves lib + path-mapped targets both appear.
4. A compile error produces no `.d` file and exit status ≥ 1.
5. `go test ./...` and `go vet ./...` stay green.
