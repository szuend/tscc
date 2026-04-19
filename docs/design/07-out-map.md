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

Inside the `writeFile` callback, when `isJSOutput(fileName)` and `cfg.OutMapPath != ""`:

1. Compute `newURL`: the basename (or relative form, see below) of `cfg.OutMapPath`.
2. Check `data.SourceMapUrlPos > 0`. If not, the `.js` has no URL comment — shouldn't happen when `SourceMap = true`, but defensive.
3. Splice `newURL` into `text` at `SourceMapUrlPos`, replacing the old URL substring. The old URL runs from `SourceMapUrlPos` to end-of-line (or end-of-file for the last write).

Sketch:

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
    }
    …
}
```

`rewriteSourceMappingURL` is a pure string op in a small helper; unit-testable without the compiler.

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

## Determinism checks

Source maps bake in paths in two places:

1. **`sources[]`**: absolute paths of source `.ts` files. These come from typescript-go's source-map generator and are already what the compiler sees. If absolute-paths-only enforcement is honored, these are deterministic by construction. `--path-prefix-map` (design §9) rewrites them post-emit — that's separate work, already on the deferred list.
2. **`file`** field: the `.js`'s own path. typescript-go's emitter sets this to the remapped path. Verify this in the e2e; if it leaks the compiler's chosen path (i.e., ignores our `writeFile` rename), add the rewrite here too.
3. **`sourcesContent`**: the literal file contents. No paths; nothing to rewrite.

For M5, the determinism baseline is: two invocations with identical inputs produce byte-identical `.js.map`. An e2e asserts this by running the compile twice (in different `cwd`s) and diffing.

## Tests

**Unit (`rewriteSourceMappingURL_test.go`):**
- Splice into a canned `.js` string at a known `SourceMapUrlPos`.
- Empty-newURL case → error (defensive; tscc never produces an empty out-map path).
- `SourceMapUrlPos` at end-of-buffer → splice extends the buffer, no panic.

**E2E:**
- `cmd/tscc/testdata/sourcemapEmit.txtar`: simple function, one source file. Assert:
  - `.js.map` golden matches byte-for-byte.
  - `.js` contains `//# sourceMappingURL=<basename of OutMapPath>`.
  - `.js.map`'s `file` field matches the remapped `.js` basename (if `--out-js` is set).
- `cmd/tscc/testdata/sourcemapDeterminism.txtar`: compile twice from different `cwd`s, `cmp` the two maps.
- `cmd/tscc/testdata/sourcemapError.txtar`: compile with a type error. Assert no `.js.map` is written, `.js.map` golden never appears, exit status ≥ 1.

## Commit shape

1. **Config + mapping layer.** `OutMapPath`, `--out-map` flag, `SourceMap` tristate mapping, unit tests.
2. **`writeFile` remap + URL rewrite.** `isMapOutput`, `rewriteSourceMappingURL`, extend the callback. Unit tests for the rewrite helper.
3. **E2E fixtures.** `sourcemapEmit.txtar`, `sourcemapDeterminism.txtar`, `sourcemapError.txtar`.

## Exit criteria

1. `./tscc --out-map /tmp/a.js.map /abs/a.ts` produces a valid v3 source map at the requested path.
2. The emitted `.js` contains `//# sourceMappingURL=a.js.map` (the basename of `--out-map`).
3. Omitting `--out-map` produces no `.js.map` and no `sourceMappingURL` comment.
4. Two identical invocations produce byte-identical `.js.map`.
5. A compile error produces no `.js.map` and exit status ≥ 1.
6. `go test ./...` and `go vet ./...` stay green.

## Deferred

- **`--inline-source-map`**. Separate flag, emits the base64-encoded map directly into the `.js` via the `data:` URL form (`emitter.go:367`). Small follow-up.
- **`--source-map-url-prefix` or similar** for non-co-located maps. Add only if the basename-only rule hits real friction.
- **`.d.ts.map` declaration source maps.** Requires the `DeclarationMap` tristate and a parallel writeFile path. Roadmap-deferred.
- **`--path-prefix-map` on map `sources[]`**. Part of design §9 (path emission); tracked there, not here.
