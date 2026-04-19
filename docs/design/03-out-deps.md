# Design Doc: `--out-deps` (Make-compatible depsfile)

> Milestone 1 of [`docs/roadmap.md`](../roadmap.md). The behavior specified here is binding; the roadmap is *sequencing*. When they disagree, this doc wins.

## Goal

After a successful compile, write a single Make-compatible dependency snippet (`target: dep1 dep2 …\n`) listing every on-disk source file the compile transitively consumed: the user's root input, every transitively-imported `.ts` / `.d.ts` / JSON module, and any `--path`-mapped targets. Build systems (Bazel with `rules_ts`'s depsfile mode, Ninja with `depfile =`, plain Make with `-include`) parse this to rebuild the compile by content, not timestamp.

This is the feature [`vision.md` §5](../vision.md#5-the-build-system-owns-incrementality) calls out as tscc's reason to exist separate from `tsc`: **the build system owns incrementality; tscc reports the input set.**

## Non-goals

- **No `.d` content when the compile fails.** Build systems either trust the depsfile as authoritative or re-run; a partial dep list is worse than none. If emit is skipped or `errorCount > 0`, no depsfile is written. This matches GCC's `-MD` behavior and avoids the "stale depsfile wedges the build" failure mode.
- **No relative paths in output.** Paths are absolute exactly as `program.SourceFiles()` returns them. Portability across machines is `--path-prefix-map`'s job (design §9), not `--out-deps`'s.
- **No transitive depsfile for failed resolutions.** If a bare import can't be mapped, the compile fails and no depsfile is emitted — per the rule above.
- **No bundled `lib.*.d.ts` entries.** typescript-go embeds its lib files into the binary via `bundled.WrapFS`; they have no on-disk path the build system can stat, and their `bundled://` filenames contain `:` which Make parses as a rule separator. The tscc binary itself is the tool dependency — the build system tracks it directly, not via the depsfile. Listing bundled libs would be redundant and unparseable.

## Source of truth: `program.SourceFiles()` minus bundled, not `hermeticfs.Reads()`

The jailed FS tracks successful `ReadFile` calls, but this undercounts by design: the custom `CompilerHost` (design §7) reads `GetSourceFile`'s already-resolved paths through the **unjailed** raw FS so that `--path`-mapped targets under `node_modules` (and explicit JSON imports) work. Those reads never reach the jailed tracker.

`program.SourceFiles()` is the authoritative set. It is what the emitter walks (`emitter.go:470-488`: `getSourceFilesToEmit(host, …)`), so it is by construction the exact set the compiler *used*. An e2e canary (`depsfile_pathmap.txtar`, below) pins this: the mapped target must appear in the depsfile or we regress into partial tracking.

One filter applies on top: **drop bundled entries before writing**. `program.SourceFiles()` includes typescript-go's embedded `lib.*.d.ts` files (served via `bundled.WrapFS`); those are a property of the tscc binary, not of the user's input set, and must not appear in the depsfile (see Non-goals). The first M1 commit writes a one-shot probe to pin down how bundled entries surface — most likely a `bundled://` prefix on `FileName`, but the probe confirms the predicate before we bake it in.

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

The target must be what the build system expects to re-produce. Concretely:

- If `--out-js /out/a.js` is passed, target = `/out/a.js`.
- If `--out-js` is not passed, no JS is emitted at all (explicit outputs only, per `vision.md`). `--out-deps` is still valid on its own: target = the depsfile path. The rule "these inputs produce this depsfile" is self-consistent, and a build system using the depsfile as a `-include`d fragment can still detect input churn. This is the "tell me the input set for `foo.ts`" use case.

An output flag that never produces a file for the user is a footgun: if `--out-js` is absent, writing `<basename>.js` next to the input clutters source trees and makes the output set ambiguous across `cwd`s. The compile stays pure type-check-plus-deps unless the user explicitly opts into emit.

When M3 (`--out-dts`) and M5 (`--out-map`) land, they add their own writes. Each output flag's absence means "don't write." The depsfile target precedence becomes: `--out-js` → `--out-dts` → `--out-map` → `--out-deps` (the first one set wins). One depsfile, one target — the build system registers the chosen primary as the rule's output and other emitted files as sibling outputs sharing the same prerequisite set. This mirrors how `gcc -MD` produces one `.d` per translation unit regardless of `-S` / `-c` variants.

## Determinism

Every input to `depsfile.Write` must be deterministic under the [`vision.md`](../vision.md) invariant "Inputs + Flags = Output":

1. **Sort before write.** `program.SourceFiles()` returns files in loader-scheduling order. Even with `SingleThreaded = true` (design §8), the order depends on import-graph traversal, which is stable but not lexicographic. Sort by absolute path using `sort.Strings` — Unicode code-point order, the same order every filesystem uses for `ls`.
2. **Deduplicate after sort.** Defensive; `program.SourceFiles()` should already be unique, but a slip in upstream's cache (the `filesparser.go` redirect logic) could produce duplicates.
3. **Apply `--path-prefix-map` *before* sorting** (when that lands, deferred past M1). Prefix mapping changes sort keys; sorting first then mapping would produce a non-lexicographic file on disk.
4. **Two invocations with identical inputs produce byte-identical output.** Enforced by the e2e test running the same compile twice in different `cwd`s and `cmp`-ing the depsfiles.

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
        target = cfg.OutDepsPath // depsfile-only use case; see "Target name"
    }
    inputs := make([]string, 0, len(program.SourceFiles()))
    for _, sf := range program.SourceFiles() {
        if isBundled(sf) { // drop embedded lib.*.d.ts; see "Source of truth" above
            continue
        }
        inputs = append(inputs, sf.FileName())
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
- `cmd/tscc/testdata/depsfile.txtar`: two-file input with a relative import, no `--out-js`. Assert `.d` lists both user source files, contains no bundled-lib entries (no `bundled://` substring, no `lib.*.d.ts` path component), and `! exists *.js` — nothing is written unless an explicit output flag is passed.
- `cmd/tscc/testdata/depsfile_with_out_js.txtar`: same two-file input with `--out-js`. Assert the `.js` is written to the requested path and the depsfile target matches that path.
- `cmd/tscc/testdata/depsfile_pathmap.txtar` **(canary)**: compile uses `--path lodash=$WORK/node_modules/lodash/index.d.ts`. Assert the mapped target appears in the depsfile. This is the regression gate against "we went back to using `hermeticfs.Reads()`".
- `cmd/tscc/testdata/depsfile_determinism.txtar`: compile twice, once from `/tmp/a` and once from `/tmp/b`, `cmp` the two depsfiles.
- `cmd/tscc/testdata/depsfile_error.txtar`: intentionally-broken input. Assert `a.d` does not exist after the failed compile.

## Commit shape

1. **`internal/depsfile` package + unit tests.** Pure function, no dependencies. Lands in isolation.
2. **`--out-deps` config flag + `config` tests.** Flag in `outputGroup`, absolute-path resolution mirroring `--out-js`.
3. **Bundled-file predicate.** Write the one-shot probe that pins down how bundled entries surface in `program.SourceFiles()`; add an `isBundled(sf)` helper with a comment citing this doc. No behavior change to existing tests.
4. **Wire into `compile.Compile` + e2e fixtures.** This commit is the one that changes behavior.

Every commit leaves `go test ./...`, `go vet ./...`, and `./tscc --help` green.

## Exit criteria

1. `./tscc --out-deps /tmp/a.d /abs/a.ts` emits `/tmp/a.d` containing the transitive user-source input set, sorted, with no bundled-lib entries.
2. Re-running from a different `cwd` produces byte-identical `.d`.
3. The `--path`-mapped canary proves path-mapped targets appear.
4. A compile error produces no `.d` file and exit status ≥ 1.
5. `go test ./...` and `go vet ./...` stay green.
