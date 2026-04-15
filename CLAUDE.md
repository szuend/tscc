# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## North Star

`tscc` is a unix-style TypeScript compiler. Every invocation follows the form:

```
tscc [FLAGS] FILE [FILE...]
```

**All inputs, outputs, and options are explicit on the command line.** There is no tsconfig.json discovery, no package.json lookup, no walking up directory trees, no ambient project state. What you pass in is what gets compiled. What you specify as output is what gets written. The build system (Bazel, Make, whatever) owns the coordination; `tscc` owns only compilation.

## Commands

```bash
# One-time setup
git submodule update --init --recursive

# Build
go generate ./...      # regenerates third_party/typescript-go/tsccbridge/bridge.go (required before first build and after submodule updates)
go build ./cmd/tscc    # produces ./tscc in the repo root

# Verify
go vet ./...
./tscc                 # should print "not yet implemented" and exit 1
```

## Architecture

```
tscc/
├── cmd/tscc/        # main binary — pflag argument parsing, delegates to execute
├── tools/genbridge/ # go:generate tool that writes the bridge package
└── third_party/
    └── typescript-go/        # git submodule (github.com/microsoft/typescript-go)
        └── tsccbridge/       # GENERATED — do not edit by hand
            └── bridge.go     # re-exports internal/execute for use by tscc
```

### The Bridge

typescript-go keeps all useful code under `internal/`, which Go's module system prevents external modules from importing. `tsccbridge/` lives *inside* the typescript-go module boundary (as an untracked file in the submodule), so it can legally re-export `internal/execute` symbols. The outer `github.com/szuend/tscc` module imports it via a `replace` directive in `go.mod`. The submodule's `.gitmodules` uses `ignore = untracked` so the generated file does not dirty the submodule's git state.

When Microsoft ships an official public API, the bridge gets replaced by direct imports. Until then, only extend `tools/genbridge/main.go` when new `internal/` symbols are needed — do not hand-edit `tsccbridge/bridge.go`.

### Adding Flags

Flags are defined in `cmd/tscc/main.go` using `github.com/spf13/pflag`. The goal is a flag for every TypeScript compiler option a caller must specify explicitly — no defaults inferred from the environment.

### License Headers

All hand-authored `.go` files carry an Apache 2.0 header (copyright 2026 Simon Zünd). The generated `tsccbridge/bridge.go` gets only the `// Code generated` notice — no license header.
