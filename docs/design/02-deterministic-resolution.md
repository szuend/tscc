# Design Doc: Deterministic Module Resolution

## Goal
`tscc` must be a deterministic, single-file-at-a-time compiler. Its behavior must be a pure function of its command-line arguments and the files it explicitly reads. It must never perform "discovery" of ambient state (like walking up directory trees to find `package.json` or `tsconfig.json`) that changes how it interprets the code.

`tscc` is designed for strict, explicit-graph build systems (like GN or Bazel) where dependencies are fully vendored and explicitly mapped. As such, `tscc` takes an uncompromising **ESM-First** approach and intentionally breaks compatibility with legacy CommonJS ecosystem features that rely on directory-based configuration.

## The Problem
The standard TypeScript (and `typescript-go`) module resolution logic is "helpful" by default. It performs an upward walk to find:
1.  `node_modules` for bare specifier resolution.
2.  `package.json` to determine if a file is an ES module or CommonJS.
3.  `package.json` to resolve `"exports"` and `"types"` mappings.

In a build system, this discovery is dangerous. It means the same `tscc` invocation could produce different results depending on whether it's run in a developer's home directory or a sterile CI environment. Furthermore, module format inference via `package.json` relies on directory-based inheritance, which violates hermetic compilation principles.

Crucially, `typescript-go`'s `moduleResolution: bundler` mode — despite being the "simplest" built-in option — still walks `node_modules` and reads `package.json` for `exports`, `imports`, `main`, `types`, and `typesVersions` fields (see call sites throughout `internal/module/resolver.go`). Turning off `ResolvePackageJsonExports` and `ResolvePackageJsonImports` reduces the magic but does not eliminate it. There is no built-in mode that gives us what we need.

## The Strategy: ESM-First + Literal Resolution + Config Firewall

`tscc` will implement a resolution strategy inspired by `clang` and Go modules: **Literal Relative Resolution**, combined with a strict enforcement of ES Module semantics.

### 1. The ESM-First Mandate
To eliminate the need for `package.json` type discovery, `tscc` enforces that all files are parsed and emitted as ECMAScript Modules by default. This is driven by the compiler options `--module esnext` (or similar) rather than by any `package.json#type` inference.

- Legacy `.d.ts` inputs using `export = X` remain syntactically valid and are type-checked normally; modern ESM consumers interop with them through the standard `esModuleInterop` shim. This is a non-issue for first-party source that emits ESM.
- If a specific file strictly requires CommonJS semantics, it must explicitly declare its format using file extensions (`.cts`, `.d.cts`), which TypeScript understands intrinsically without needing `package.json`.

### 2. Absolute Paths Only
All file-path arguments on the `tscc` command line must be absolute. Relative paths are an error.

This eliminates the "current working directory" as an input to the compiler. Build systems (Bazel, GN) already know the absolute path of every input and output; passing those through directly is both the simplest and most honest contract. It also means that paths baked into output (source maps, `.d.ts` references, diagnostic locations) are fully determined by the command line, independent of where `tscc` is invoked from.

For portability of that output across machines, see §9 (Path Emission).

### 3. Relative Imports are Literal
When `tscc` encounters `import { x } from "./utils.js"`, it will:
- Append the specifier to the current file's (absolute) directory.
- Apply standard TypeScript extension substitution rules (stripping `.js` and probing for `.ts`, `.tsx`, `.d.ts`, etc.).
- **Never** look for a `package.json` to resolve the path or determine module format.
- **Never** walk up to parent directories to find `node_modules`.

### 4. Bare Imports are Explicit
Bare specifiers (e.g., `import { foo } from "lib"`) are **denied by default**. To use them, the user must provide an absolute mapping on the command line, mimicking `clang`'s `-I`:

```bash
tscc --path lib=/vendor/lib/index.d.ts /src/main.ts
```

This mapping is absolute and one-to-one. `tscc` will resolve `"lib"` to exactly that path and nothing else.

*(Note on naming: TypeScript's existing `compilerOptions.paths` is a superficially similar but semantically richer feature — it supports wildcards, multiple fallback targets, and `baseUrl`-relative resolution. `tscc`'s `--path` intentionally restricts this to one-to-one absolute mappings. We keep the names distinct to avoid implying full `paths` semantics.)*

*(Because this requires a `--path` flag for every transitive dependency, build systems should pass these via a response file, e.g., `tscc @deps_paths.txt`, which `tscc` must implement.)*

### 5. The Custom Resolver
`tscc` replaces `typescript-go`'s resolver with its own `LiteralResolver`, which implements §3 and §4 directly. In addition to its primary resolve methods, the replacement must:

- **Never call `Realpath`.** The FS Jail makes it an identity function anyway, but the resolver calling it at all couples our semantics to the jail's implementation. A unit test asserts no `Realpath` invocation across a representative resolution trace.
- **Stub `GetPackageScopeForPath` to return empty.** `fileloader.go:303` consults this during `loadSourceFileMetaData` to derive `PackageJsonType`, which in turn influences `ImpliedNodeFormat` and therefore emit behavior. An empty scope means "no ambient package context," which is exactly the §1 ESM-First invariant.

This requires a small patch to the `typescript-go` submodule: the resolver is currently constructed inline inside `internal/compiler/fileloader.go` (around line 149) as `module.NewResolver(...)`. We add an injection hook so callers can supply their own `*module.Resolver` (or equivalent interface).

An alternative considered was: keep the upstream resolver and rely solely on the FS Jail (§6) to make every `package.json` read fail, letting the resolver's built-in "missing file" fallback branches produce the behavior we want. We rejected this. It would define `tscc`'s resolution semantics by exception handling deep in upstream code — a contract we do not control and cannot verify. An upstream refactor that tightens one of those fallback branches could silently change our behavior. With a custom resolver, `tscc`'s semantics are defined by code we own, and resolution failures produce clear errors from our code rather than silent fallbacks.

The patch is small and narrowly scoped. Maintenance cost is real but bounded, and is lower than the cost of debugging subtle upstream behavior changes over time.

We will propose this hook upstream to `typescript-go`. Until it lands, `tscc` carries the patch. This keeps the bridge architecture's no-patch stance as the long-term target even while we accept a patch short-term. Other alternatives considered and rejected: (a) supplying a custom `ResolutionHost` — the host only controls I/O, not resolution policy, so the resolver's walks still happen; (b) extending `tools/genbridge` to re-export a private constructor — the inline construction at `fileloader.go:149` leaves nothing private to re-export.

### 6. The Configuration Firewall (FS Jail)
As defense in depth, `tscc` wraps the `vfs.FS` instance provided to the underlying `typescript-go` compiler:

- The wrapper intercepts `Stat`, `FileExists`, `ReadFile`, `GetAccessibleEntries`, and `WalkDir` for specific filenames (`package.json`, `tsconfig.json`, `jsconfig.json`) and returns "not found".
- The wrapper blocks traversal into `node_modules`, `bower_components`, and `jspm_packages`.
- `Realpath` is the identity function. Symlinks are not followed. This prevents host paths from leaking through symlink resolution and matches the semantics of clang's `-ffile-prefix-map`, which operates on literal paths rather than canonicalized ones. Note: this is *not* equivalent to `preserveSymlinks: true` at the resolver level — that option affects module identity, whereas jailed `Realpath` only affects what strings leak through the FS boundary. The `LiteralResolver` must also not invoke `Realpath` on its own; a unit test asserts this.
- `UseCaseSensitiveFileNames()` returns a caller-pinned value, controlled by `--case-sensitive-paths=true|false` (default `true`). The underlying `osvfs` sniffs this at runtime (Windows → false, Linux → true, macOS → probed via temp file), and that value flows into `tspath.Path` keys (`fileloader.go:137`, `fileloader.go:184`). Without pinning, the same inputs produce different path keys and different diagnostics on different hosts. Pinning is mandatory for the "Inputs + Flags = Output" invariant.
- `Stat` returns a wrapped `FileInfo` whose `ModTime()` reports the Unix epoch. The compiler hot path does not currently read `ModTime`, but `execute/incremental/host.go:36` does (for tsbuildinfo). `tscc` does not use incremental mode, but if upstream adds a `ModTime` read anywhere else — tracing, logging, future caching — the host's filesystem mtime would silently leak into output. Pinning here is cheap and preempts the regression.

The jail is belt-and-suspenders behind §5: the custom resolver should never attempt these reads in the first place, but parts of `typescript-go` outside the resolver (the checker, emitter, or type-reference loader) might. The jail ensures they also hit a dead end.

The existing `internal/vfs/wrapvfs/` package provides the base pattern for this.

### 7. Handling Explicit JSON Imports
If a user writes `import pkg from "./package.json" with { type: "json" }`, it works because **intent** is separated from **ambient discovery**.

The stock `CompilerHost.GetSourceFile` (`internal/compiler/host.go:78`) reads through the same `FS` it was given, so a naive jail would block even explicit JSON imports. `tscc`'s custom `CompilerHost` therefore holds *two* filesystems: the jailed one it reports from `FS()` (used by everything in the compiler that probes for files), and an unjailed one it uses internally in `GetSourceFile` for reads of already-resolved paths. The `LiteralResolver` (§5) produces the absolute path; `GetSourceFile` then reads it directly.

The jail only needs to block *discovery*. Reads of already-resolved paths go through the separate, unjailed channel.

### 8. Neutralizing Secondary "Magic"
- **Automatic `@types` Inclusion**: The compiler hunts for `node_modules/@types` in all parent directories to ambiently load global types. We neutralize this three ways: the custom resolver does not perform this walk; `tscc` sets `CompilerOptions.Types = []string{}` and `TypeRoots = []string{}` *programmatically* when constructing the options (passing `--types ""` on the CLI yields `[]string{""}`, not `nil`, which does not hit the skip-guard at `resolver.go:2055`); and the FS Jail blocks any residual `node_modules/@types` probes. Global types must be explicitly passed as inputs (e.g., `tscc --global-type /vendor/@types/node/index.d.ts /src/main.ts`).
- **Automatic Typings Acquisition**: `module.NewResolver` takes a `TypingsLocation` argument (`fileloader.go:149`) that drives automatic typings acquisition. `tscc` always passes the empty string here.
- **Project References**: `fileloader.addProjectReferenceTasks` runs before resolver construction (`fileloader.go:148`). `tscc` rejects any non-empty `ProjectReferences` with a clear error at argument parsing. There are no tsconfig files to chain through, so this should never be non-empty in practice.
- **`allowJs` and `node_modules` JS descent**: `maxNodeModuleJsDepth` (`fileloader.go:129`) allows the loader to walk into `node_modules` for `.js` files when `allowJs` is set. `tscc` does not forbid `allowJs`, but the FS Jail blocks all `node_modules` traversal regardless, so the walk terminates immediately. The resolver hitting "no files found" under `allowJs` is not an error; it just stops.
- **Dynamic `import()`**: Static string-literal specifiers are routed through the `LiteralResolver` identically to static imports. Non-constant dynamic imports (variable specifiers) are not resolved at compile time, matching standard TypeScript behavior — they pass through to runtime.
- **Triple-Slash Directives**:
  - `/// <reference path="..." />` is treated as a literal relative path and allowed.
  - `/// <reference types="..." />` is treated as a bare import and denied unless an explicit mapping was provided.
- **Default Library (`lib.d.ts`)**: The built-in `lib.*.d.ts` files are resolved via the compiler's `DefaultLibraryPath()`. `tscc` sets this to a path that is either bundled with the binary or explicitly passed via CLI (`--lib-path /path/to/lib`). It is never discovered relative to `$0`.
- **Parallel file loading**: `fileloader.processAllProgramFiles` runs with `singleThreaded=false` by default (`fileloader.go:121`), and per-task resolution traces merge back into shared state in scheduling-dependent order. This leaks into `--traceResolution`, `--explainFiles`, and `tsbuildinfo`. `tscc` sets `SingleThreaded=true` programmatically when constructing the loader options. Single-threaded compilation of a single file has no meaningful performance cost for our use case, and the alternative (a deterministic post-sort at every merge site) is both more invasive and easier to get wrong.

### 9. Path Emission (Prefix Maps)
Absolute paths in, absolute paths out — unless the build system asks for rewriting. `tscc` accepts one or more `--path-prefix-map=OLD=NEW` flags, modeled directly on `clang`'s `-ffile-prefix-map`. The transform is a dumb string-prefix substitution, not a semantic computation.

**Prefix match ordering is specified, not implicit.** When multiple prefixes match a given path (e.g., `/work=/w` and `/work/src=/ws` both match `/work/src/main.ts`), the longest matching prefix wins. Ties are broken by flag declaration order on the command line. This matters because Go map iteration is unordered, and "whichever matched first" would be a silent non-determinism.

**Emitted lists are sorted.** Any list serialized to output — `sources[]`, file lists in `tsbuildinfo`, `--listFiles`, `--explainFiles`, resolver trace entries — is sorted by a stable key (path lexicographic order, *after* prefix-map application) before emission. `typescript-go` stores these in Go `map` structures internally (e.g., `resolvedModules` at `filesparser.go:468`), whose iteration order is randomized per run; without an explicit post-sort every serialization site is non-deterministic.

The rewrite must be applied at every site where an input path can reach output. The concrete audit list:

1. **Source maps**: `sources[]`, `sourceRoot`, and `file` fields. (`sourcesContent` contains file *bytes*, not paths — unaffected.)
2. **Declaration map files (`.d.ts.map`)**: same structure as source maps, same treatment.
3. **Declaration files (`.d.ts`)**: import specifiers and `/// <reference path="..." />` directives that resolve to absolute paths must be rewritten.
4. **Diagnostic output**: file locations in text diagnostics, plus the "searched" path lists that the resolver emits on failure (e.g., `Cannot find module 'x' (searched /abs/a, /abs/b)` — these come from the resolver's tracer, not the diagnostic formatter, and must be rewritten at the trace site).
5. **`--listFiles`, `--listEmittedFiles`, `--explainFiles`** output.
6. **`tsbuildinfo`**: file lists and resolution cache entries. Note that since prefix-mapping changes the bytes of `tsbuildinfo`, any hash computed over it as a build cache key must be computed *after* the rewrite, not before. `tscc` itself does not currently emit `tsbuildinfo`; if that changes, this rule applies.
7. **JSON-serialized diagnostics** if `tscc` grows a `--diagnostics-json` mode.

Any new emit site added in the future must be added to this list. "We audit every emit path" is the rule; the list above is the current inventory.

This matches the lesson learned in C++ toolchains: DWARF, `__FILE__`, and every other path-emit site needed the same rewrite, and skipping one (as gcc did with `__FILE__` for years) produces subtle reproducibility bugs.

The DWARF precedent informs one more choice: the rewrite operates on the literal input path. Combined with §6's identity `Realpath`, this means the mapping is trivial to reason about — what you pass in on the CLI is what gets rewritten, with no hidden canonicalization step in between.

### 10. Response File Format
Because `--path` must be specified for every transitive dependency, real invocations expand into thousands of arguments. `tscc` accepts `@file` on the command line, modeled on `clang` / GCC conventions:

- Each argument in the response file is separated by whitespace (spaces, tabs, newlines).
- Arguments containing whitespace must be quoted with `"..."` or `'...'`. Backslash escapes inside quotes (`\"`, `\\`) are supported.
- Lines beginning with `#` are comments (only when `#` is the first non-whitespace character on a line).
- Response files do **not** recurse. A `@file` inside a response file is passed through as a literal argument, not expanded again. This is a deliberate restriction: recursion makes determinism harder to audit and is rarely needed.
- Response-file paths are resolved relative to `tscc`'s cwd at parse time. This is the one place `tscc` consults cwd; it is a parse-time convenience for humans invoking the tool, not a compilation input. Build systems should pass absolute paths to `@file` to preserve hermeticity.

## Explicit Non-Goals

- **No incremental / watch mode.** `tscc` is single-shot: one invocation, one compilation, no state carried across invocations. This is not just "we haven't built it" — it's a deliberate scope choice. Incremental compilation requires cache-key hashing, stale-entry detection, and mtime-sensitivity, all of which introduce their own determinism concerns (hash ordering, cache-key stability across host environments, timestamp resolution). The build system (Bazel, GN) already solves incrementality at a higher layer via content-addressed inputs and the `--out-depsfile` output; re-solving it inside the compiler would duplicate work and expand the determinism surface.
- **No environment-variable inputs.** `tscc` does not consult `os.Environ()` for compilation behavior. `stubSys.GetEnvironmentVariable` returns the empty string unconditionally. The only environment read the binary performs is whatever the Go runtime itself does (locale-independent). If a future feature needs configuration, it arrives as a CLI flag, not as an env var.
- **No host locale sniffing.** `tscc` does not read `LANG`, `LC_ALL`, or any locale env var. Diagnostic locale is controlled exclusively by the `--locale` flag passed through to the compiler; absent the flag, `typescript-go` falls back to its built-in English strings. Because the flag is a CLI input, per-locale behavior is still "Inputs + Flags = Output" — no action needed here beyond not sniffing env.

## The Build-System Contract

`tscc`'s strictness pushes work onto the caller. The expectations:

- **Pass absolute paths for every input.** Including the file to compile, every `--path` mapping target, every `--global-type`, and `--lib-path`.
- **Provide bare-specifier mappings for every transitive dependency.** Use response files (`@deps.txt`) to keep invocations manageable.
- **Patch legacy ESM/CJS mismatches in vendored `.d.ts` files** if they don't interop cleanly under `esModuleInterop`. This is rare in practice but the build system, not `tscc`, owns the fix.
- **Pass `--path-prefix-map` entries** for any path prefix that should not appear in output.

## Implementation Plan
1.  **Patch Submodule**: Add a hook to `typescript-go`'s `internal/compiler/fileloader.go` to allow providing a custom resolver instead of constructing `module.NewResolver` inline. Prepare the change as an upstream proposal in parallel.
2.  **Implement Response Files**: Add logic in `cmd/tscc/main.go` to parse `@args.txt` files per §10 and expand them into `os.Args` before `pflag` parsing.
3.  **Implement `LiteralResolver`**: A resolver in `tscc` implementing §3 and §4. Must not invoke `Realpath`.
4.  **Implement `FSJail`**: A `vfs.FS` middleware that blocks configuration files, `node_modules`-style directories (including `bower_components`, `jspm_packages`), makes `Realpath` the identity function, returns a caller-pinned `UseCaseSensitiveFileNames()`, and wraps `FileInfo` so `ModTime()` reports Unix epoch. Covers `Stat`, `FileExists`, `ReadFile`, `GetAccessibleEntries`, `WalkDir`.
5.  **Implement `CompilerHost`**: A custom host holding both a jailed and an unjailed filesystem. `FS()` returns the jailed one; `GetSourceFile` reads explicitly-resolved paths from the unjailed one.
6.  **Pin Compiler Options Programmatically**: Set `Types = []string{}`, `TypeRoots = []string{}`, `TypingsLocation = ""`, `ProjectReferences = nil`, `SingleThreaded = true`, and `Module = ModuleKindESNext` (or caller-specified ESM-compatible kind) at the point where `CompilerOptions` is constructed. CLI flags cannot be trusted to produce these values.
7.  **Implement Path Prefix Map**: A pass (or emitter/tracer hooks) that applies `--path-prefix-map` substitutions to every site enumerated in §9. Longest-match wins, ties broken by flag declaration order.
8.  **Stable Sort at Every Emit Site**: Any list derived from a Go `map` (e.g., `resolvedModules`) is sorted by path lexicographic order (post-prefix-map) before serialization.
9.  **Resolve Relative Paths**: Resolve all relative paths against `os.Getwd()` at argument parsing before passing them to the compiler core. Reject relative paths leaking into the compiler core.
cographic order (post-prefix-map) before serialization.
9.  **Enforce Absolute Paths**: Reject relative paths at argument parsing for every path-shaped flag.
