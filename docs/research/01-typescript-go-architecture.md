# Research: typescript-go Architecture

This document describes the internal architecture of `typescript-go` and how `tscc` can leverage it for explicit, side-effect-free compilation.

## Core Components

### 1. Configuration (`internal/core`, `internal/tsoptions`)
- `core.CompilerOptions`: The data structure holding all compiler flags.
- `tsoptions.ParsedCommandLine`: An interface representing the result of parsing command-line arguments.
- `tsoptions.ParseCommandLine`: The standard parser. It populates `CompilerOptions` and the list of root files.

### 2. Environment & VFS (`internal/vfs`, `internal/compiler`)
- `vfs.FS`: An interface for file system access. `osvfs.FS()` provides real OS access.
- `compiler.CompilerHost`: An interface used by the program to interact with the environment. It wraps the `vfs.FS` and provides paths to default libraries.

### 3. Program & Resolution (`internal/compiler`, `internal/module`)
- `compiler.NewProgram(opts ProgramOptions)`: The main entry point for creating a compilation context.
- `fileLoader`: Internally used by `NewProgram` to load files. It uses `module.Resolver` to follow imports.
- `module.Resolver`: Implements Node-style and Classic-style module resolution. It searches for `package.json` and `node_modules`.

### 4. Checker & Emitter (`internal/checker`, `internal/compiler`)
- `checker.NewChecker`: Performs type checking.
- `compiler.Program.Emit(ctx, opts EmitOptions)`: Orchestrates the emission of JavaScript and declaration files.

## Strategies for tscc

### Single File Compilation with Explicit Output (-o flag)
The standard `tsc` maps `input.ts` to `input.js`. To redirect output to an arbitrary path:
1. Use `compiler.Program.Emit`.
2. Provide a custom `WriteFile` callback in `EmitOptions`.
3. The callback receives the `fileName` suggested by the compiler. For a single-file compilation, we can ignore this and write to our specified `-o` path.

```go
program.Emit(ctx, compiler.EmitOptions{
    WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
        // Redirect to our -o path
        return sys.FS().WriteFile(explicitOutputPath, text)
    },
})
```

### Preventing Automatic Module Resolution
To satisfy the "no side-effects" and "explicitly specified" mandates:

#### 1. Disable Resolution via Flags
Set `NoResolve: core.TSTrue` in `CompilerOptions`. This prevents the compiler from automatically adding files to the program based on `import` or `/// <reference />` tags. 
- Note: The compiler will still attempt to *resolve* the module name to a path to see if it's already in the program.
- If `NoResolve` is true, the user must explicitly provide all dependency files as root files on the command line.

#### 2. Disable Default Libraries
Set `NoLib: core.TSTrue` to prevent the compiler from automatically including `lib.d.ts` and related files based on the `target` flag.

#### 3. Explicit Resolution via Paths
If imports are required but must be explicit:
- Populate `CompilerOptions.Paths` (a `*collections.OrderedMap[string, []string]`).
- This allows remapping module names to specific file paths on disk, bypassing standard Node lookup logic.

#### 4. Surgical VFS (The ultimate control)
To prevent the compiler from ever seeing a `package.json` or `node_modules` directory that wasn't explicitly provided:
1. Implement a `vfs.FS` that only exposes a whitelist of files (or simply denies access to any file named `package.json` or directory named `node_modules`).
2. Initialize the `CompilerHost` with this filtered VFS.
3. This ensures that even if the `module.Resolver` tries to walk up the directory tree to find a `package.json`, it will receive "not found" for everything not explicitly in the whitelist, while standard relative imports (`./foo.ts`) will resolve perfectly.

### Implementing `--std` and `--no-std` (Embedded Standard Library)
To allow users to control which standard library declarations (`lib.d.ts` files) are loaded, and to ship a standalone binary:
- **`go:embed` Support:** `typescript-go` *already* embeds all standard `lib.*.d.ts` files in the compiled Go binary via its `internal/bundled` package. By using `bundled.WrapFS(osvfs.FS())` for our `vfs.FS` and returning `bundled.LibPath()` as the `CompilerHost`'s `DefaultLibraryPath`, the compiler natively resolves standard libraries from memory without any external files on disk.
- **`--std` flag:** This maps to `core.CompilerOptions.Lib` (`[]string`). When a user passes `--std es2022,dom`, we populate this array. If omitted, `typescript-go` automatically picks the correct defaults based on the `--target` flag.
- **`--no-std` flag:** This maps to `core.CompilerOptions.NoLib = core.TSTrue`. When set, the compiler completely ignores the bundled library path and compiles against pure JS/TS without any ambient declarations.

### Implementing `-I` / `--include` (Additional Search Paths)
To allow users to provide additional directories for module resolution (similar to `gcc -I`):
- This can be implemented using `core.CompilerOptions.Paths`.
- We can add a wildcard entry to the `Paths` map: `"*": ["path1/*", "path2/*", ...]`.
- This tells the compiler that for any module name that doesn't match a more specific rule, it should look in these directories.
- This provides a flexible way to include custom standard libraries or third-party declaration files without relying on `node_modules`.

### Implementing `--extern` (Path Mapping)
To mirror `rustc --extern foo=./foo.d.ts` and manually satisfy imports without `node_modules` discovery:
- This maps perfectly to the `Paths` and `BaseUrl` fields in `core.CompilerOptions`.
- `Paths` is defined as `*collections.OrderedMap[string, []string]`.
- When parsing `--extern foo=./out/foo.d.ts`, `tscc` can programmatically populate this map: `{"foo": ["./out/foo.d.ts"]}`.
- Setting `BaseUrl` to `.` (the current working directory) ensures these paths are resolved exactly as provided.
- The internal `module.Resolver` will natively intercept imports of `"foo"` and redirect them to `"./out/foo.d.ts"` without requiring a `tsconfig.json`.

## Top-to-Bottom Compilation without top-level `CommandLine`
Instead of using `execute.CommandLine(sys, args, ...)` which performs `tsconfig.json` discovery and uses standard resolution, `tscc` should:
1. Parse flags into `core.CompilerOptions`.
2. Manually initialize a `compiler.CompilerHost`.
3. Call `compiler.NewProgram` with a `tsoptions.ParsedCommandLine` that we've constructed.
4. Manually run `program.GetSemanticDiagnostics` and `program.Emit`.
