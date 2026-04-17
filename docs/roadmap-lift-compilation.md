# Roadmap: Lift typescript-go compilation into tscc

## Goal

Today `cmd/tscc/main.go` parses flags into a `*config.Config`, discards the result (`_ = cfg`), and forwards raw `os.Args[1:]` to `tsccbridge.CommandLine`. `tscc` is therefore a thin shim around `tsgo`'s CLI — none of tscc's own flag semantics (kebab-case, banned `--outDir`, the headline `--out-depsfile`) can influence compilation because tsgo never sees them.

The end state is a `tscc` binary that owns its own compilation pipeline:

```
*config.Config
   → *core.CompilerOptions        (mapping)
   → *tsoptions.ParsedCommandLine (options + root files)
   → *compiler.Program            (+ CompilerHost, vfs.FS)
   → *compiler.EmitResult         (via Program.Emit + WriteFile)
   → diagnostics printed to stderr, files written to disk
```

Each step below is one commit. Commits are intentionally small and leave dead code behind them — a mapping function with no caller is fine if it has tests. The forwarding path in `main.go` stays intact until step 6, so `cmd/tscc/testdata/*.txtar` golden tests keep passing on every intermediate commit.

Every package added under `internal/` must be unit testable without spinning up the full `tsgo` pipeline. That means fakeable seams (vfs.FS, CompilerHost, WriteFile callbacks) and table-driven tests over pure mappings.

## Step 1 — Config → CompilerOptions mapping

**Bridge delta** (extend `tools/genbridge/main.go`, regenerate):
- `type CompilerOptions = core.CompilerOptions`
- `type ScriptTarget = core.ScriptTarget`
- `type Tristate = core.Tristate`
- Constants: `ScriptTargetES2015…ES2025`, `ScriptTargetESNext`, `TSUnknown`, `TSTrue`, `TSFalse`
- Add `"github.com/microsoft/typescript-go/internal/core"` to imports

**New package** `internal/compileropts`:

```go
func FromConfig(cfg *config.Config) (*tsccbridge.CompilerOptions, error)
```

- Maps `cfg.Target` string → `ScriptTarget` via a lookup table covering the values `config.languageGroup` advertises (`es6`/`es2015` through `es2025`, `esnext`). Unknown → error.
- Maps `cfg.Strict` bool → `Tristate` (`TSTrue`/`TSFalse`).
- Every other `CompilerOptions` field stays at its zero value for now and grows as `Config` grows.

**Tests:** table-driven — every documented `--target`, case-insensitivity, unknown-target error, `Strict` both ways, smoke-test that unset fields remain zero.

**Dead code so far:** `FromConfig` has no caller. That's expected.

## Step 2 — Root files → ParsedCommandLine

**Bridge delta:**
- `type ParsedCommandLine = tsoptions.ParsedCommandLine`
- `var NewParsedCommandLine = tsoptions.NewParsedCommandLine`
- `type ComparePathsOptions = tspath.ComparePathsOptions`
- Imports: `internal/tsoptions`, `internal/tspath`

**New code** in `internal/compileropts` (or a sibling package):

```go
func BuildParsedCommandLine(cfg *config.Config) (*tsccbridge.ParsedCommandLine, error)
```

- Calls `FromConfig` for the options.
- Uses `[cfg.InputPath]` as the root file list (still a single input — multi-input is a later concern).
- Constructs `ComparePathsOptions` from `cfg` (case sensitivity, cwd). For now hard-code `UseCaseSensitiveFileNames: true` and leave cwd empty; revisit in step 3 when the host arrives.

**Tests:** a couple of round-trip assertions — given a `Config`, confirm `.CompilerOptions()` and `.FileNames()` match expectations. No filesystem, no host.

## Step 3 — CompilerHost and VFS plumbing

**Bridge delta:**
- `var NewCompilerHost = compiler.NewCompilerHost`
- `var NewCachedFSCompilerHost = compiler.NewCachedFSCompilerHost`
- `type CompilerHost = compiler.CompilerHost`
- `var OSFS = osvfs.FS` (function, not called at import time)
- `var DefaultLibPath = bundled.LibPath`
- Imports: `internal/compiler`, `internal/vfs/osvfs`, `internal/bundled`

**Implementation of `stubSys`** in `cmd/tscc/sys.go` — surface methods one at a time matching the 92287de pattern. Step 3 lands:
- `FS()` → `tsccbridge.OSFS()`
- `GetCurrentDirectory()` → `os.Getwd()` (cached at `stubSys` construction)
- `DefaultLibraryPath()` → `tsccbridge.DefaultLibPath()`

Other `stubSys` methods stay `unimplemented()` until something actually calls them.

**New code:** a small constructor in tscc (likely `internal/compileropts` or a new `internal/compilehost`) that takes a `vfs.FS` + cwd + libpath and returns a `CompilerHost`. This is the seam that makes step 4 testable — tests inject a fake `vfs.FS`, production code uses `osvfs.FS()`.

**Tests:** construct a host over an in-memory `vfs.FS` (a tiny fake in the test file); assert `FS()`, `GetCurrentDirectory()`, `DefaultLibraryPath()` round-trip.

## Step 4 — Program creation and emit

**Bridge delta:**
- `var NewProgram = compiler.NewProgram`
- `type ProgramOptions = compiler.ProgramOptions`
- `type Program = compiler.Program`
- `type EmitOptions = compiler.EmitOptions`
- `type EmitResult = compiler.EmitResult`
- `type WriteFile = compiler.WriteFile`
- `type WriteFileData = compiler.WriteFileData`

**New package** `internal/compile`:

```go
type Result struct {
    EmittedFiles []string
    // Later: diagnostics, exit status
}

func Compile(ctx context.Context, cfg *config.Config, host tsccbridge.CompilerHost) (*Result, error)
```

- Builds the `ParsedCommandLine` via step 2's helper.
- Calls `NewProgram({Host: host, Config: parsed})`.
- Calls `Program.Emit` with a `WriteFile` callback that writes to `cfg.OutputPath` for the `.js` file. Ignores declaration / sourcemap outputs for now — they arrive with `--out-dts` / `--out-sourcemap`.
- Does **not** collect diagnostics yet; that's step 5.

**Tests:** unit test with an in-memory `vfs.FS` containing `a.ts`, construct a host over it, call `Compile`, assert the callback received the expected JS text. No type-error cases yet.

`main.go` still forwards; nothing calls `Compile` in production.

## Step 5 — Diagnostics collection and formatting

**Bridge delta:**
- `type Diagnostic = ast.Diagnostic`
- Whatever `diagnosticwriter` / formatting helpers turn out to be needed — defer the exact list until implementing.

**Extend `internal/compile`:**
- Collect diagnostics from `Program.GetConfigFileParsingDiagnostics`, `GetSyntacticDiagnostics`, `GetSemanticDiagnostics`, `GetProgramDiagnostics`, and the emit result.
- If fatal diagnostics exist and `cfg` has type-checking enabled, skip writing outputs (matches `ExitStatusDiagnosticsPresent_OutputsSkipped`).
- Format diagnostics for stderr. Start with a minimal `file:line:col: message` formatter; richer formatting (colors, source snippets) can come later.
- Map to `tsccbridge.ExitStatus*`.

**Tests:** synthetic type errors via in-memory `vfs.FS`; assert the formatted output and that no files were written when the exit status is `OutputsSkipped`.

## Step 6 — Switch `main.go` over

At this point `Compile` is fully tested in isolation. This commit:
- Replaces the `tsccbridge.CommandLine(&stubSys{}, os.Args[1:], nil)` call with construction of the host + `compile.Compile`.
- Prints diagnostics and exits with the mapped `ExitStatus`.
- Updates `cmd/tscc/testdata/*.txtar` golden sections — run `TSCC_UPDATE_TESTDATA=1 go test ./cmd/tscc/...` and review the diff by hand.

After this commit, `tsccbridge.CommandLine` and `stubSys` can be removed or retained as a legacy path. Prefer removal — the build system is our only caller and we've taken ownership of the pipeline.

## How features layer on after step 6

- **New compiler flag (e.g. `--module`, `--declaration`)** — add to `internal/config`, then to `FromConfig`, then to the `CompilerOptions` mapping tests. No changes to steps 3–5 needed.
- **`--out-dts`** — a new field on `Config`, and a branch in the `WriteFile` callback that routes `.d.ts` output to the requested path.
- **`--out-sourcemap`** — same shape as `--out-dts`.
- **`--out-depsfile`** — the headline feature. After emit, walk the `Program`'s resolved source files (lib files + user files + transitive imports), write a Makefile-compatible `.d` file. This is a new module hanging off `internal/compile`; tests drive it over a fake `Program` or capture its output given a known in-memory input tree.

## Open questions / things to re-evaluate as we go

- **Case sensitivity** of file names — currently hard-coded to `true` in the step 2 sketch. Needs to match OS behavior for correctness on case-insensitive filesystems.
- **`DefaultLibraryPath`** — `bundled.LibPath()` returns a path inside the tsgo module. Confirm the lib files are actually readable at that path at runtime (they may be embedded and require the `embedded` build tag).
- **Multi-file inputs** — `tscc` is currently single-input by design (`config.Parse` rejects multiple positional args). Revisit if/when build systems need to pass multiple roots in one invocation.
- **Incremental / watch mode** — out of scope. `tscc`'s niche is one-shot invocation from a build system; the build system handles incrementality via `--out-depsfile`.
