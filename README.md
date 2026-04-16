# tscc

TypeScript compilation as a build rule, not a project system.

```
tscc [FLAGS] FILE [FILE...]
```

Built on [typescript-go](https://github.com/microsoft/typescript-go), Microsoft's Go port of the TypeScript compiler.

## Why tscc

`tsc` is a project system: it discovers `tsconfig.json` by walking up the directory tree, infers output paths from project structure, and implicitly resolves modules through `package.json`. That works great in an editor. It works poorly in a build system.

Hermetic build systems (Bazel, Make, Ninja, Buck) need to own the dependency graph. Every input must be declared, every output must be known ahead of time, and the compiler must report which source files it actually read — so the build system knows when to recompile.

`tscc` is designed for exactly this:

- **Explicit inputs and outputs.** No tsconfig discovery, no directory walking, no ambient project state. What you pass on the command line is what gets compiled.
- **One file in, deterministic files out.** Each invocation produces only what you asked for: `.js` (`--output`), `.d.ts` (`--out-dts`), sourcemap (`--out-sourcemap`), depsfile (`--out-depsfile`), or any combination.
- **Depsfile output.** `--out-depsfile` writes a Makefile-compatible `.d` file listing every TypeScript source the compiler transitively read. Feed it to `make`, `ninja`, or your Bazel action to get correct incremental builds for free.

## Usage

> **Note:** Compilation is not yet implemented. This is a proof of concept.

```bash
# Compile a single file to JS
tscc --target es2022 --module esnext --output dist/index.js src/index.ts

# Also emit a declaration file and sourcemap
tscc --target es2022 --module esnext \
  --output dist/index.js \
  --out-dts dist/index.d.ts \
  --out-sourcemap dist/index.js.map \
  src/index.ts

# Write a depsfile so your build system tracks transitive imports
tscc --target es2022 --module esnext \
  --output dist/index.js \
  --out-depsfile dist/index.d \
  src/index.ts
```

The depsfile format is compatible with GNU Make and Ninja:

```make
dist/index.js: src/index.ts src/lib.ts node_modules/some-dep/index.d.ts
```

Flags follow Unix kebab-case conventions (`--out-dts`, `--out-sourcemap`) and also accept camelCase equivalents for compatibility with typescript-go option names.

## Building from Source

```bash
git clone https://github.com/szuend/tscc
cd tscc
git submodule update --init --recursive
go generate ./...
go build ./cmd/tscc
```

Requires Go 1.26 or later.

## License

Apache License 2.0. See [LICENSE](LICENSE).
