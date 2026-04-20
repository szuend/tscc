# Design Doc: `--out-map` (source map emit)

> Milestone 5 of [`docs/roadmap.md`](../roadmap.md).

## Goal

Emit a v3 source map (`.js.map`) alongside the emitted `.js`, at a caller-specified path. Last of the standard output flags named in the lift roadmap. After this lands, tscc's output surface (`--out-js`, `--out-dts`, `--out-map`, `--out-deps`) covers every build-system-facing artifact.

## Public surface

```
tscc --out-map /abs/a.js.map /abs/a.ts
```

- `--out-map PATH` writes the source map to `PATH`. No shorthand alias.
- Setting `--out-map` implies `SourceMap = true`.
- Omitting `--out-map` preserves today's behavior: no source map is written, no `//# sourceMappingURL=` comment in the `.js`.
- `--out-map` is mutually exclusive with a hypothetical future `--inline-source-map`; not a concern until the latter exists.

## The URL-rewrite problem (unambiguous, not "verify")

The roadmap left this as an open question: "Verify whether typescript-go handles this automatically when the writer renames the file; if not, post-emit rewrite in `writeFile`."

**Investigation result: a post-emit rewrite is mandatory.** typescript-go's emitter computes `sourceMappingURL` *before* the `writeFile` callback runs, using the base name of the emitter's chosen `.js.map` path (`emitter.go:363` → `getSourceMappingURL`). The URL is baked into the `.js` buffer at `emitter.go:272` and only then handed to `writeFile`. Our callback can rename the `.js.map` file on disk, but the URL inside the `.js` still points at the compiler-chosen basename.

Upstream exposes the URL's byte offset as `WriteFileData.SourceMapUrlPos` (`program.go:1539`). That's the hook we use.

## How the rewrite works

Both pair-locator fields (`sourceMappingURL` in the `.js`, `file` in the `.js.map` — see §Path-emitting fields) are baked into the serialized output *before* `writeFile` runs, so both require a post-emit text rewrite in the callback.

### `.js` branch: `sourceMappingURL`

When `isJSOutput(fileName)` and `cfg.OutMapPath != ""`:

1. Compute `newURL`: the basename of `cfg.OutMapPath`.
2. Check `data.SourceMapUrlPos > 0`. If not, the `.js` has no URL comment — shouldn't happen when `SourceMap = true`, but defensive.
3. Splice `newURL` into `text` at `SourceMapUrlPos`, replacing the old URL substring. The old URL runs from `SourceMapUrlPos` to end-of-line (or end-of-file for the last write).

### `.js.map` branch: `file`

When `isMapOutput(fileName)` and `cfg.OutJSPath != ""`:

1. Compute `newFile`: `basename(cfg.OutJSPath)`.
2. Locate the `"file":"..."` field at the top of the JSON and replace its value with `newFile`.

No byte-position hint from upstream for this field, so `rewriteMapFileField` does a scoped string search. Pure string op; unit-testable without the compiler, parallel to `rewriteSourceMappingURL`.

### Sketch

```go
writeFile := func(fileName, text string, data *tsccbridge.WriteFileData) error {
    target := fileName
    switch {
    case cfg.OutJSPath != "" && isJSOutput(fileName):
        target = cfg.OutJSPath
        if cfg.OutMapPath != "" && data != nil && data.SourceMapUrlPos > 0 {
            text = rewriteSourceMappingURL(text, data.SourceMapUrlPos, sourceMappingURLFor(cfg))
        }
    case cfg.OutDtsPath != "" && isDtsOutput(fileName):
        target = cfg.OutDtsPath
    case cfg.OutMapPath != "" && isMapOutput(fileName):
        target = cfg.OutMapPath
        if cfg.OutJSPath != "" {
            text = rewriteMapFileField(text, filepath.Base(cfg.OutJSPath))
        }
    }
    …
}
```

### What `sourceMappingURLFor(cfg)` returns

A map URL embedded in a `.js` can be:

- A **bare basename** (`a.js.map`): relative to the `.js`'s own directory.
- A **relative path** (`maps/a.js.map`, `../maps/a.js.map`): relative to the `.js`.
- An **absolute URI** (`file:///abs/a.js.map`): never portable; forbidden.

tscc's rule, driven by the determinism invariant: **basename only**. Computation:

```go
func sourceMappingURLFor(cfg *config.Config) string {
    return filepath.Base(cfg.OutMapPath)
}
```

This ignores the relative location of `--out-js` vs `--out-map`. Build systems that need the two in different directories must either (a) accept that the source map URL assumes co-location, and place the `.map` next to the `.js` at runtime, or (b) rewrite the URL themselves after tscc runs. We declare "co-located or rewritten by the build system" because computing a correct relative path between two absolute paths injects `filepath.Rel`-style host-sensitive canonicalization that muddies the determinism story for marginal ergonomic benefit. If this proves painful in practice, a follow-up adds `--source-map-url-prefix` or similar to make the rewrite explicit.

## Config plumbing

```go
// internal/config/config.go
type Config struct {
    …
    OutMapPath string
}

func outputGroup(cfg *Config) flagGroup {
    g := pflag.NewFlagSet("output", pflag.ContinueOnError)
    g.StringVarP(&cfg.OutJSPath, "out-js", "o", "", "Write JavaScript output to `FILE`")
    g.StringVar(&cfg.OutDtsPath, "out-dts", "", "Write TypeScript declaration output to `FILE`. Implies --declaration.")
    g.StringVar(&cfg.OutMapPath, "out-map", "", "Write source map output to `FILE`. Implies --sourceMap. URL comment in emitted JS uses basename(FILE); co-locate or rewrite downstream.")
    return flagGroup{Name: "Output", Set: g}
}
```

`filepath.Abs` resolution mirrors `--out-js` / `--out-dts`.

## Mapping layer

`compileropts.FromConfig` gains:

```go
if cfg.OutMapPath != "" {
    opts.SourceMap = tsccbridge.TSTrue
}
```

## Bridge delta

None required — `SourceMap` is already a field on `CompilerOptions`, whose type is already bridged. `SourceMapUrlPos` is already on `WriteFileData`, whose type is already bridged. Verify once during implementation that no hidden field needs exporting; proceed from there.

## `isMapOutput` helper

```go
func isMapOutput(name string) bool {
    switch {
    case strings.HasSuffix(name, ".js.map"):
        return true
    case strings.HasSuffix(name, ".mjs.map"):
        return true
    case strings.HasSuffix(name, ".cjs.map"):
        return true
    }
    return false
}
```

Mirror of `isDtsOutput`. `.d.ts.map` goes through a different path (deferred; see M3 §Deferred).

## Path-emitting fields

The `.js.map` exposes four path-bearing fields. They split into two groups:

- **Content** (what were the inputs?) — flows through `--path-prefix-map` (design §9).
- **Pair-locator** (what's the sibling artifact's name?) — basename only, independent of `--path-prefix-map`. Same pattern as DWARF's `.gnu_debuglink`: the compiler names the pair; the build system places them.

| Field              | Group        | tscc rule                                                                            |
|--------------------|--------------|--------------------------------------------------------------------------------------|
| `sources[]`        | content      | Absolute by default (design §2 enforces absolute inputs). Rewritten by `--path-prefix-map`. |
| `sourceRoot`       | content      | **Unset.**                                                                           |
| `file`             | pair-locator | `basename(cfg.OutJSPath)`. Post-emit rewrite of the `.js.map` JSON.                  |
| `sourceMappingURL` | pair-locator | `basename(cfg.OutMapPath)`. Post-emit rewrite of the `.js` text (see §How the rewrite works). |
| `sourcesContent`   | —            | File bytes. No paths; nothing to rewrite.                                            |

**Why `sourceRoot` is unset.** v3 spec: `sourceRoot` is prepended to each `sources[]` entry by the consumer. With `--path-prefix-map` already controlling what `sources[]` looks like, emitting a `sourceRoot` would either duplicate or contradict that transform. If a build system needs one, it can splice it in post-emit. Simpler than exposing a flag for a rarely-useful knob.

**Why `file` follows the basename rule, not `--path-prefix-map`.** `file` and `sourceMappingURL` describe the same physical relationship (the `.js` ↔ `.js.map` pair) in opposite directions. Treating them asymmetrically — URL as basename, `file` as absolute or prefix-mapped — produces a map whose URL assumes co-location but whose `file` claims standalone portability. Incoherent. Use the same pair-locator rule for both. typescript-go's emitter writes `file` based on its own chosen `.js` path, so the `writeFile` callback rewrites the `"file":"..."` JSON field to `basename(cfg.OutJSPath)` before writing.

**Determinism baseline.** Two invocations with identical inputs produce byte-identical `.js.map`. An e2e asserts this by running the compile twice (in different `cwd`s) and diffing.

## Tests

**Unit (`rewriteSourceMappingURL_test.go`):**
- Splice into a canned `.js` string at a known `SourceMapUrlPos`.
- Empty-newURL case → error (defensive; tscc never produces an empty out-map path).
- `SourceMapUrlPos` at end-of-buffer → splice extends the buffer, no panic.

**Unit (`rewriteMapFileField_test.go`):**
- Replace `"file":"..."` in canned `.js.map` JSON.
- Value containing escape sequences (`\"`, `\\`) round-trips unchanged.
- Absent field → error (defensive; every emitted `.js.map` has one).

**E2E:**
- `cmd/tscc/testdata/sourcemapEmit.txtar`: simple function, one source file. Assert:
  - `.js.map` golden matches byte-for-byte.
  - `.js` contains `//# sourceMappingURL=<basename of OutMapPath>`.
  - `.js.map`'s `file` field equals `basename(--out-js)` when `--out-js` is set.
- `cmd/tscc/testdata/sourcemapDeterminism.txtar`: compile twice from different `cwd`s, `cmp` the two maps.
- `cmd/tscc/testdata/sourcemapError.txtar`: compile with a type error. Assert no `.js.map` is written, `.js.map` golden never appears, exit status ≥ 1.

## Commit shape

1. **Config + mapping layer.** `OutMapPath`, `--out-map` flag, `SourceMap` tristate mapping, unit tests.
2. **`writeFile` remap + URL rewrite.** `isMapOutput`, `rewriteSourceMappingURL`, extend the callback for the `.js` branch. Unit tests for the rewrite helper.
3. **`.js.map` `file`-field rewrite.** `rewriteMapFileField`, extend the callback for the `.js.map` branch. Unit tests for the helper.
4. **E2E fixtures.** `sourcemapEmit.txtar`, `sourcemapDeterminism.txtar`, `sourcemapError.txtar`.

## Exit criteria

1. `./tscc --out-map /tmp/a.js.map /abs/a.ts` produces a valid v3 source map at the requested path.
2. The emitted `.js` contains `//# sourceMappingURL=a.js.map` (the basename of `--out-map`).
3. The `file` field in the emitted `.js.map` equals `basename(--out-js)` when `--out-js` is set.
4. No `sourceRoot` field is emitted.
5. Omitting `--out-map` produces no `.js.map` and no `sourceMappingURL` comment.
6. Two identical invocations produce byte-identical `.js.map`.
7. A compile error produces no `.js.map` and exit status ≥ 1.
8. `go test ./...` and `go vet ./...` stay green.

## Deferred

- **`--inline-source-map`**. Separate flag, emits the base64-encoded map directly into the `.js` via the `data:` URL form (`emitter.go:367`). Small follow-up.
- **`--source-map-url-prefix` or similar** for non-co-located maps. Add only if the basename-only rule hits real friction.
- **`.d.ts.map` declaration source maps.** Requires the `DeclarationMap` tristate and a parallel writeFile path. Roadmap-deferred.
- **`--path-prefix-map` on map `sources[]`**. Part of design §9 (path emission); tracked there, not here.
