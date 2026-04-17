# Vision

## The invariant

> **Inputs + Flags = Output. Consistently. Deterministically. Always.**

A `tscc` invocation is a pure function. Given the same command-line arguments and the same input files, it produces byte-identical output — on any machine, in any directory, under any user account, at any time. Nothing else matters. Nothing else is allowed to matter.

This is the one rule. Every other rule in this document derives from it.

## Who this is for

`tscc` is built for hermetic build systems: Bazel, GN, Buck, Pants, Nix, Ninja with strict mode. These systems own the dependency graph. They declare every input, predict every output, and detect stale results by content, not timestamp. They cannot tolerate a compiler that makes decisions based on state the build rule did not explicitly provide.

`tscc` is **not** built for:

- Editor integration (tsserver covers this).
- Interactive development loops (`tsc --watch` covers this).
- CI pipelines that shell out to `tsc` against a checked-in `tsconfig.json` (plain `tsc` covers this, and does it well).

If your tool is fine with `tsc`, it will be fine with `tsc`. `tscc` exists for the case where `tsc`'s flexibility is a liability.

## Principles

Five principles derive from the invariant. Each answers a "why does tscc behave this way?" question that would otherwise come up repeatedly.

**1. No ambient state discovery.** `tscc` never walks up directory trees. It never looks for `tsconfig.json`, `package.json`, `node_modules`, or `@types` packages that the caller did not explicitly pass in. The same `tscc` invocation in `/home/user/project` and in `/sandbox/execroot` must behave identically, which means neither location can contribute anything the other doesn't.

**2. Every restriction exists to protect determinism.** Restrictions that don't protect determinism stay relaxed. This is the answer to "why do you forbid X but allow Y?" — if X threatens the invariant, it's forbidden; if Y doesn't, it's allowed, even if it feels similar. A restriction needs a concrete determinism argument or it doesn't belong in the design.

**3. CLI flags are inputs, not magic.** `--locale de-DE` produces byte-identical German diagnostics on any machine — it's deterministic and therefore fine. `LANG=de_DE` is ambient host state and therefore not fine. The distinction is not "localization is bad"; it's "ambient discovery is bad." Same logic applies everywhere: a flag is a declaration, an environment variable is a leak.

**4. All paths are absolute.** `tscc` never asks "where am I?" It never consults `os.Getwd()` for compilation inputs. Every file-path argument is an absolute path, every output location is an absolute path, every `--path` mapping target is an absolute path. The build system already has absolute paths for every artifact it manages; passing them through directly eliminates a whole category of cwd-dependent bugs.

**5. The build system owns incrementality.** `tscc` is single-shot: one invocation, one compilation, no cache state carried across runs. Incremental rebuilds are the build system's job, driven by `--out-depsfile` (a Makefile-compatible list of every source file the compiler transitively read). This is a division of responsibility, not a limitation. Bazel, Ninja, and Make already solve incrementality correctly. Duplicating that inside the compiler would expand the determinism surface without adding capability.

## Explicit non-goals

`tscc` will not:

- **Watch mode.** `tscc` exits after one compilation.
- **Incremental compilation.** No `tsbuildinfo`, no on-disk cache, no cross-invocation state.
- **tsconfig.json discovery.** If a build rule wants to pass a tsconfig file, it does so explicitly; `tscc` never looks for one.
- **Multi-project / composite project builds.** One root file per invocation; the build system composes compilations.
- **Language service integration.** No LSP, no IDE hooks.
- **Environment-variable configuration.** `tscc` does not consult `os.Environ()` for compilation behavior. Every configuration input is a CLI flag.
- **Module resolution via `package.json` walks.** Bare specifier resolution requires explicit `--path` mappings (see `docs/design/02-deterministic-resolution.md`).

These are not "future work." They are deliberately out of scope. Features that conflict with these non-goals will be declined.

## Positioning

- **vs `tsc` / `tsgo`**: `tscc` uses `typescript-go` as its compilation core, so type-checking semantics are identical to `tsc`. The difference is what sits around the core: `tscc` strips the project system (tsconfig discovery, file globbing, ambient types) and adds build-system outputs (`--out-depsfile`, explicit I/O flags). Use `tsc` for projects; use `tscc` for build rules.
- **vs `swc` / `esbuild`**: those tools transpile fast by skipping type checking. `tscc` does full type checking — build correctness depends on it. If you want a fast bundler, those are the right tools. If you want a build-system-native type checker, `tscc` is.
- **vs Bazel's `rules_nodejs` / `aspect_rules_ts`**: those are build rules that wrap an existing compiler. `tscc` is the compiler those rules should wrap. The two projects sit at different layers of the stack.

## Where to go next

- **Usage**: [`README.md`](../README.md) — command-line examples and installation.
- **Architecture**: [`docs/design/02-deterministic-resolution.md`](design/02-deterministic-resolution.md) — how the invariant is enforced against the upstream `typescript-go` compiler.
- **Current status**: [`docs/roadmap-lift-compilation.md`](roadmap-lift-compilation.md) — the sequenced plan for owning the compilation pipeline end-to-end.
- **Contributor guidance**: [`AGENTS.md`](../AGENTS.md) — repository conventions, build commands, and how the `typescript-go` submodule bridge works.
