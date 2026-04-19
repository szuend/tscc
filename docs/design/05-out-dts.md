# Design Doc: `--out-dts` (declaration emit)

> Milestone 3 of [`docs/roadmap.md`](../roadmap.md).

## Goal

Emit a TypeScript declaration file (`.d.ts`) alongside (or instead of) `.js`, with the output path controlled by an explicit CLI flag. Required for any `rules_ts`-style consumer that publishes type information to downstream rules.

## Public surface

```
tscc --out-dts /abs/a.d.ts /abs/a.ts
```

- `--out-dts PATH` routes the emitted `.d.ts` to `PATH`. No shorthand alias.
- Setting `--out-dts` implies `Declaration = true` in the compiler options — the flag's existence is itself the opt-in.
- Omitting `--out-dts` preserves today's behavior: no declaration file is written. The underlying compiler option remains `Tristate::Unknown` → upstream's "don't emit" branch.

Callers that want **only** `.d.ts`, not `.js`, pass `--out-dts PATH` without `--out-js`. The `.js` is still computed but, under [M5's coming behavior](./07-out-map.md) and similar surface choices, it lands at the compiler's default path. Full `emitDeclarationOnly` support is deferred (see §Deferred).

## Error-path semantics

If any diagnostic is a type error, `.d.ts` must not be written. This matches tsc's `Declaration` emit path: when the type graph is malformed, the emitted declaration is unreliable and worse than absent.

Concretely: after diagnostics collection, if `errorCount > 0`, `compile.Compile` already sets `ExitStatus` to `DiagnosticsPresent_OutputsSkipped` and typescript-go's emit path honors this via `EmitSkipped`. Confirm in the first commit that `.d.ts` genuinely does not appear on disk in the error case — the existing `writeFile` path already short-circuits writes when emit is skipped, but an e2e fixture pins the contract.

## Config plumbing

```go
// internal/config/config.go
type Config struct {
    …
    OutDtsPath string
}

func outputGroup(cfg *Config) flagGroup {
    g := pflag.NewFlagSet("output", pflag.ContinueOnError)
    g.StringVarP(&cfg.OutJSPath, "out-js", "o", "", "Write JavaScript output to `FILE`")
    g.StringVar(&cfg.OutDtsPath, "out-dts", "", "Write TypeScript declaration output to `FILE`. Implies --declaration.")
    return flagGroup{Name: "Output", Set: g}
}
```

`filepath.Abs` resolution mirrors `--out-js` in `config.Parse`.

## Mapping layer

`compileropts.FromConfig` gains:

```go
if cfg.OutDtsPath != "" {
    opts.Declaration = tsccbridge.TSTrue
}
```

The bridge already exports `Declaration`'s type as part of `CompilerOptions`; no bridge delta is needed.

No `DeclarationMap` handling yet. That comes with M5 (source maps) or later — the roadmap lists declaration-map support under "Work deferred beyond this roadmap" in spirit. An e2e that would force the decision (a `@declarationMap:` directive in a ported upstream case) should `skip` at the port stage until we commit to adding `--out-dts-map`.

## Write-file remap

`compile.Compile`'s existing `writeFile` callback handles `.js` remap via `isJSOutput`. Extend symmetrically:

```go
writeFile := func(fileName string, text string, data *tsccbridge.WriteFileData) error {
    target := fileName
    switch {
    case cfg.OutJSPath != "" && isJSOutput(fileName):
        target = cfg.OutJSPath
    case cfg.OutDtsPath != "" && isDtsOutput(fileName):
        target = cfg.OutDtsPath
    }
    if err := in.JailedFS.WriteFile(target, text); err != nil {
        return err
    }
    emittedFiles = append(emittedFiles, target)
    return nil
}

func isDtsOutput(name string) bool {
    switch {
    case strings.HasSuffix(name, ".d.ts"):
        return true
    case strings.HasSuffix(name, ".d.mts"):
        return true
    case strings.HasSuffix(name, ".d.cts"):
        return true
    }
    return false
}
```

Order matters: `.d.ts` check *before* any hypothetical `.ts` fallback. Use `strings.HasSuffix` rather than `filepath.Ext`, because `filepath.Ext(".d.ts")` is `.ts` — a subtle trap that caused real bugs in tooling that tried the `.ts` branch for a `.d.ts` file.

The existing `isJSOutput` helper stays correct (the JS extensions have no such double-extension case).

## Interaction with declaration diagnostics

typescript-go's declaration emit can produce its own diagnostics — types that can't be serialized (`error TS4X…`). These come back through `EmitResult.Diagnostics`, which `compile.Compile` already appends to `allDiags`. Nothing to add; verify in the e2e that a known declaration-emit error (e.g., a default export with an anonymous inferred type that requires an explicit annotation) surfaces with the expected `TS4XXX` code.

## Tests

**Unit (`compileropts_test.go`):**
- `cfg.OutDtsPath == ""` → `opts.Declaration == Tristate::Unknown`.
- `cfg.OutDtsPath != ""` → `opts.Declaration == TSTrue`.

**E2E:**
- `cmd/tscc/testdata/dtsEmit.txtar`: exported `const` with a literal type. Assert two goldens (`.js` and `.d.ts`).
- `cmd/tscc/testdata/dtsEmitOnly.txtar`: `--out-dts` without `--out-js`. Assert `.d.ts` golden; `.js` lands wherever typescript-go chose (document the path in the fixture for reproducibility) or assert with `exists <computed path>` syntax.
- `cmd/tscc/testdata/dtsEmitError.txtar`: a type error (`TS23XX`-class). Assert non-zero exit, `.js.golden` *not* written, `.d.ts` *not* written, stderr contains the expected `TSxxxx`.
- `cmd/tscc/testdata/dtsDeclError.txtar`: a declaration-emit-specific error (something that type-checks but cannot serialize). Assert non-zero exit, neither output file present, stderr contains the `TS4XXX` code.

## Commit shape

1. **Config + mapping layer.** `OutDtsPath`, `--out-dts` flag, `Declaration` tristate mapping, unit tests.
2. **`writeFile` remap.** Add `isDtsOutput`, extend the callback. No behavior change until paired with `--out-dts`.
3. **E2E fixtures.** The three `.txtar` files above, plus any error-path canaries.

## Exit criteria

1. `./tscc --out-dts /tmp/a.d.ts /abs/a.ts` produces a `.d.ts` at the requested path with the exported shape.
2. Omitting `--out-dts` produces exactly today's output (no `.d.ts` written, `.js` unchanged).
3. A compile error produces no `.d.ts` and exit status ≥ 1.
4. `.d.ts`-specific diagnostics propagate to stderr with the expected `TS4XXX` codes.
5. `go test ./...` and `go vet ./...` stay green.

## Deferred

- **`--emit-declaration-only`.** If a caller passes `--out-dts` without `--out-js`, we still compute and drop `.js` on disk. A true `emitDeclarationOnly` flag is a small follow-up that sets `CompilerOptions.EmitDeclarationOnly = TSTrue` and adjusts the writeFile to skip `.js`. Land when the first corpus case requires it (M4 will surface this).
- **`--out-dts-map` (declaration source maps).** Separate flag, separate Tristate (`DeclarationMap`). Parallel shape to M5's `--out-map`.
- **Multi-input `.d.ts` emit.** Config currently accepts exactly one input; multi-input is a broader concern.
