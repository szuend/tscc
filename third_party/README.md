# Third-party

## typescript-go

`tscc` links against [typescript-go](https://github.com/microsoft/typescript-go), Microsoft's Go port of the TypeScript compiler. It is vendored as a git submodule at `third_party/typescript-go/` and pinned to a specific upstream commit. The submodule ships with its own nested submodule, `_submodules/TypeScript/`, which carries the upstream TypeScript compiler test corpus.

### How the bridge works

typescript-go exposes most of its compiler under `internal/`, which Go's module system forbids external modules from importing. To work around this, `go generate ./...` writes `third_party/typescript-go/tsccbridge/bridge.go` — a file *inside* the typescript-go module boundary that re-exports the `internal/…` symbols tscc needs. The outer `github.com/szuend/tscc` module imports the bridge via a `replace` directive in `go.mod`.

The bridge is a generated file; never hand-edit it. To add a new re-export, edit `tools/genbridge/main.go` and re-run `go generate ./...`. See [`AGENTS.md`](../AGENTS.md#the-bridge) for the architectural rationale.

`go generate` also applies `patches/inject-resolver.patch` to the submodule — a narrowly scoped hook that lets tscc supply its own module resolver. The submodule's `.gitmodules` sets `ignore = dirty` so neither the generated bridge file nor the applied patch dirties the outer repo's submodule status.

### Pinning and determinism

Git submodules pin to a commit hash by construction: the superproject tree records the exact SHA the submodule must check out. Every fresh `git submodule update --init --recursive` on every machine lands on the same commit. This is a hard pin, not a floating reference — the submodule URL in `.gitmodules` is only consulted for fetching, not for picking a commit.

The current pin is visible via:

```bash
git submodule status third_party/typescript-go
```

### Updating typescript-go

Bumping the pin is an intentional, reviewed operation. The steps:

```bash
# 1. Fetch the latest upstream commits in the submodule and pick a target commit.
cd third_party/typescript-go
git fetch origin
git log --oneline origin/main | head -20    # pick a commit; avoid the tip unless necessary

# 2. Check out the chosen commit inside the submodule.
git checkout <commit-sha>
cd ../..

# 3. Regenerate the bridge and re-apply the resolver patch against the new tree.
#    If the patch no longer applies cleanly, resolve conflicts in
#    patches/inject-resolver.patch before re-running go generate.
go generate ./...

# 4. Update the outer repo's go.sum by tidying — typescript-go's own go.mod may
#    have shifted and tscc's replace directive resolves against the new tree.
go mod tidy

# 5. Rebuild and copy the vendored TypeScript declaration files required by tests 
#    like APILibCheck.ts. These must be kept in sync with the submodule.
(cd third_party/typescript-go/_submodules/TypeScript && npm install && npm run build:compiler)
mkdir -p third_party/generated/ts
cp third_party/typescript-go/_submodules/TypeScript/built/local/typescript.d.ts \
   third_party/typescript-go/_submodules/TypeScript/built/local/typescript.internal.d.ts \
   third_party/typescript-go/_submodules/TypeScript/built/local/tsserverlibrary.d.ts \
   third_party/typescript-go/_submodules/TypeScript/built/local/tsserverlibrary.internal.d.ts \
   third_party/generated/ts/

# 6. Run the full test suite. Any failure here must be understood before landing
#    the bump — a silent baseline shift is exactly what pinning exists to prevent.
go test ./...
go vet ./...

# 7. Stage and commit. The submodule pin appears in the diff as a one-line change
#    to the superproject's tree entry.
git add third_party/typescript-go third_party/generated/ts go.sum
git commit -m "deps: bump typescript-go to <commit-sha-prefix>"
```

Things to watch for during a bump:

- **Patch drift.** `patches/inject-resolver.patch` targets specific lines in `internal/compiler/fileloader.go`, `internal/compiler/program.go`, and `internal/module/resolver.go`. Upstream refactors around these sites require the patch to be regenerated. `go generate` surfaces the failure as `git apply` error output.
- **Bridge drift.** Types or constants that `tools/genbridge/main.go` re-exports may have been renamed, moved between packages, or had their signatures changed. `go generate` produces a `bridge.go` that fails to compile in that case; fix by editing `genbridge` to track the new names.
- **Nested TypeScript submodule.** `third_party/typescript-go/_submodules/TypeScript/` is pinned *inside* typescript-go. Bumping typescript-go may move this pin, which changes the set of upstream compiler test cases available to `tools/portcase`. Treat this as a coupled change.
- **Baseline drift in `cmd/tscc/testdata/`.** If the bump changes emit behavior, the golden sections in `.txtar` fixtures will diverge. Review each one by hand with `TSCC_UPDATE_TESTDATA=1 go test ./cmd/tscc/...`; do not rubber-stamp the regeneration.

### Why we pin to a commit, not a tag

typescript-go is pre-1.0 and does not tag releases on a cadence we can rely on. Pinning to a commit gives us:

- Determinism: every checkout builds the same compiler.
- Precision: we can bump past a specific bug-fix commit without waiting for a tag.
- Auditability: the commit hash is the single source of truth visible in `git submodule status` and in the superproject diff.

If upstream starts tagging releases reliably, this policy is worth revisiting.

## TypeScript

The [TypeScript compiler test suite](https://github.com/microsoft/TypeScript) is vendored transitively through typescript-go, at `third_party/typescript-go/_submodules/TypeScript/`. `tools/portcase` (once it ships; see [`docs/design/06-portcase.md`](../docs/design/06-portcase.md)) converts selected cases from `tests/cases/compiler/` into tscc's `.txtar` e2e fixtures.

## Licenses

- **typescript-go** — Apache 2.0, © Microsoft Corporation. See `third_party/typescript-go/LICENSE`.
- **TypeScript** — Apache 2.0, © Microsoft Corporation. See `third_party/typescript-go/_submodules/TypeScript/LICENSE.txt`.
