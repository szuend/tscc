# Roadmap: Lift typescript-go compilation into tscc

> The target semantics — FS Jail, Literal Resolver, path prefix map, programmatic option pinning, absolute-paths-only, response files — are specified in [`design/02-deterministic-resolution.md`](design/02-deterministic-resolution.md). This roadmap is the *sequencing* plan; the design doc is the *what*. When they disagree, the design doc wins.

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
- `GetCurrentDirectory()` → `""` or `"/"` (per design §4 and Vision §4, the core only sees absolute paths. Wiring `os.Getwd()` here leaks ambient host state into diagnostics or trace paths if a relative path slips through).
- `DefaultLibraryPath()` → `tsccbridge.DefaultLibPath()`

Other `stubSys` methods stay `unimplemented()` until something actually calls them.

`tsccbridge.OSFS` and the raw `osvfs.FS()` wiring in `stubSys` is **provisional**. It only exists so the `tsccbridge.CommandLine` forwarding path keeps surfacing one stub method at a time during steps 3–6. Production reads never go through raw `OSFS` — step 4 wraps it in the FS Jail, and step 5's custom `CompilerHost` (per design §7) holds both a jailed FS and an unjailed FS for `GetSourceFile`.

**No new package this step.** The single-FS wrapper around `tsccbridge.NewCompilerHost` has the wrong shape — the design-compliant host is a custom `compiler.CompilerHost` implementation holding two filesystems, not a thin constructor. That implementation lands in step 5.

**Tests:** none. Bridge exports are re-exports (no behavior to test); `stubSys` methods are scaffolding exercised indirectly once step 4/5 are in place.

## Step 4 — FS Jail

**Goal:** Wrap `vfs.FS` to enforce the determinism invariants from design §6.

**New package** `internal/hermeticfs`:

```go
type FS interface {
    tsccbridge.FS
    Reads() []string // absolute paths of every successful ReadFile, first-seen order, deduplicated
}

type Options struct {
    Inner             tsccbridge.FS // production: osvfs.FS(); tests: an in-memory fake
    CaseSensitivePaths bool          // pinned; NOT sniffed from Inner
}

func New(opts Options) FS
```

Requirements (see design §6 for rationale):

- **Discovery block.** `FileExists`, `DirectoryExists`, `Stat`, `ReadFile`, `GetAccessibleEntries`, and `WalkDir` all return "not found" for: `package.json`, `tsconfig.json`, `jsconfig.json`, and any path under `node_modules`, `bower_components`, `jspm_packages`.
- **`Realpath` is identity.** No symlink resolution.
- **`UseCaseSensitiveFileNames()` returns `opts.CaseSensitivePaths`.** Must not delegate to the inner FS — the OS's case-sensitivity sniff is a host-dependence that breaks "Inputs + Flags = Output."
- **Read tracking.** Only successful `ReadFile` calls that bypass the firewall enter the read set (for the depsfile feature).
- **Writes** pass through unchecked.

**Production stacking:** `bundled.WrapFS(hermeticfs.New(...))` — bundled wrapper outside so `bundled://libs/...` reads are intercepted before hitting hermetic.

**Bridge delta:** none. `hermeticfs` is tscc-internal.

**Tests:** construct with an in-memory inner FS. Assert: (a) `package.json` reads return not-found (covering all six FS methods), (b) normal reads succeed and appear in `Reads()`, (c) `Realpath` is identity, (d) `UseCaseSensitiveFileNames()` returns the pinned value regardless of what the inner FS reports.

**Dead code so far:** nothing calls `hermeticfs.New` in production. Step 5 is the first caller.

## Step 4b — Literal Module Resolution

**Goal:** Replace typescript-go's resolver with the `LiteralResolver` specified in design §§3–5.

**Bridge delta:**
- Patch `third_party/typescript-go/internal/compiler/fileloader.go:149` to accept an injected resolver instead of constructing `module.NewResolver` inline. Propose the hook upstream in parallel.

**New package** `internal/resolver`:
- Literal relative resolution (no upward walking, no `package.json` probes).
- Bare specifiers resolve *only* via explicit CLI mappings.
- **Never invokes `Realpath`.** Unit test asserts this against a spy FS.
- **`GetPackageScopeForPath` returns empty.** Prevents `ImpliedNodeFormat` leakage via `fileloader.loadSourceFileMetaData`.

## Step 5 — Program creation and emit

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
- Pins compiler options programmatically per design §8 / Impl. Plan step 6: `Types = []string{}`, `TypeRoots = []string{}`, `TypingsLocation = ""`, `ProjectReferences = nil`, `SingleThreaded = true`, `Module = ModuleKindESNext`.
- Calls `NewProgram({Host: host, Config: parsed})`.
- Calls `Program.Emit` with a `WriteFile` callback that writes to `cfg.OutputPath` for the `.js` file. Ignores declaration / sourcemap outputs for now — they arrive with `--out-dts` / `--out-sourcemap`.
- Does **not** collect diagnostics yet; that's step 5.

**New package** `internal/compilehost` (first real caller of the two-FS shape from design §7):

- Custom `compiler.CompilerHost` implementation holding **two** filesystems — a jailed one reported by `FS()`, and a raw one used internally by `GetSourceFile` for reads of already-resolved paths (so explicit JSON imports work without the jail blocking them).
- Constructor takes both filesystems, cwd, and libpath.

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

## Work deferred to later roadmaps

These are specified in `design/02-deterministic-resolution.md` but don't belong in the lift roadmap — they layer on after step 6:

- **Absolute-paths-only enforcement** at argument parsing.
- **`--path-prefix-map`** and the emit-site audit (design §9).
- **Response files** (`@file`) per design §10.
- **`--case-sensitive-paths`** flag wiring (the step 4 `FS` constructor already takes the value; the CLI flag feeding it can come later).
- **Stable-sort discipline** at every serialized-list emit site (design Impl. Plan step 8).

## Open questions

- **`DefaultLibraryPath`** — `bundled.LibPath()` returns a path inside the tsgo module. Confirm the lib files are actually readable at that path at runtime (they may be embedded and require the `embedded` build tag).
- **Multi-file inputs** — `tscc` is currently single-input by design (`config.Parse` rejects multiple positional args). Revisit if/when build systems need to pass multiple roots in one invocation.
- **Incremental / watch mode** — out of scope. `tscc`'s niche is one-shot invocation from a build system; the build system handles incrementality via `--out-depsfile`.
y via `--out-depsfile`.
