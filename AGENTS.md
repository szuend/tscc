# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## North Star

`tscc` is a unix-style TypeScript compiler. Every invocation follows the form:

```
tscc [OPTIONS] FILE
```

**All inputs, outputs, and options are explicit on the command line.** There is no tsconfig.json discovery, no package.json lookup, no walking up directory trees, no ambient project state. What you pass in is what gets compiled. What you specify as output is what gets written. The build system (Bazel, Make, whatever) owns the coordination; `tscc` owns only compilation.

## Commands

```bash
# One-time setup
git submodule update --init --recursive

# Build
go generate ./...      # regenerates third_party/typescript-go/tsccbridge/bridge.go (required before first build and after submodule updates)
go build ./cmd/tscc    # produces ./tscc in the repo root

# Test
go test ./...                                        # all tests
go test ./cmd/tscc/... -run TestScript/stub          # single end-to-end test case by name
TSCC_UPDATE_TESTDATA=1 go test ./cmd/tscc/...        # regenerate golden sections in .txtar files

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

Flags live in `internal/config/config.go`, defined via `github.com/spf13/pflag` and organized into named groups (`languageGroup`, `typeCheckingGroup`, `outputGroup`, …) that drive the `--help` layout. Add a flag to the matching group function, or create a new group if none fits. The goal is a flag for every TypeScript compiler option a caller must specify explicitly — no defaults inferred from the environment.

### Test corpus

The upstream TypeScript compiler and its Go port are both available as submodules and are the source of truth for expected compiler behavior:

```
third_party/typescript-go/                          # github.com/microsoft/typescript-go
    _submodules/TypeScript/                         # github.com/microsoft/TypeScript
        tests/cases/compiler/                       # ~6400 single-file compiler test inputs
        tests/baselines/reference/                  # expected outputs: <name>.js, <name>.errors.txt
    testdata/tests/cases/compiler/                  # subset (~136) that typescript-go has ported
    testdata/baselines/reference/tsc/               # typescript-go's own expected outputs
```

When adding a new test case, look up the input in `_submodules/TypeScript/tests/cases/compiler/` and the expected JS output in `_submodules/TypeScript/tests/baselines/reference/<name>.js`. Translate the `// @directive` header comments into explicit CLI flags on the `exec tscc` line.

### Testing

End-to-end tests live in `cmd/tscc/testdata/` as `.txtar` files (testscript format). Each file is self-contained: input TypeScript files, the `exec tscc` invocation, and assertions on output files or stderr. The harness in `cmd/tscc/main_test.go` runs `tscc` in-process via `testscript.Main` — no separate `go build` step required.

A test case structure:
```
# comment describing what is being tested
exec tscc --target es2022 --module esnext a.ts
! stderr .
cmp a.js a.js.golden

-- a.ts --
export const x: string = "hello";
-- a.js.golden --
export const x = "hello";
```

For error cases: `! exec tscc ...` asserts non-zero exit; `stderr 'pattern'` matches against stderr; `! exists file` asserts no output was written.

When the compiler output changes, run with `TSCC_UPDATE_TESTDATA=1` to rewrite golden sections, then review with `git diff`.

### License Headers

All hand-authored `.go` files carry an Apache 2.0 header (copyright 2026 Simon Zünd). The generated `tsccbridge/bridge.go` gets only the `// Code generated` notice — no license header.
