# tscc

A unix-style TypeScript compiler. Explicit inputs, explicit outputs, explicit flags — nothing inferred from the environment.

```
tscc [FLAGS] FILE [FILE...]
```

Built on top of [typescript-go](https://github.com/microsoft/typescript-go), Microsoft's Go port of the TypeScript compiler.

## Motivation

`tsc` is designed around `tsconfig.json`: it discovers configuration by walking up the directory tree, infers output paths from project structure, and consults `package.json` for module resolution. This makes it powerful for interactive development but difficult to integrate into hermetic build systems like Bazel or Make, where inputs and outputs must be fully declared upfront.

`tscc` removes that indirection. You tell it what to compile, where to put the output, and what options to use. The build system owns the rest.

## Prerequisites

- Go 1.26 or later
- Git (for submodule initialization)

## Building from Source

```bash
git clone https://github.com/szuend/tscc
cd tscc
git submodule update --init --recursive
go generate ./...
go build ./cmd/tscc
```

## Usage

> **Note:** Compilation is not yet implemented. This is a proof of concept.

```bash
# Planned usage (not yet functional):
tscc --outDir ./dist --target ES2020 --module NodeNext src/index.ts src/lib.ts
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
