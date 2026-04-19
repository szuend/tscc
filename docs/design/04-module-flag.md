# Design Doc: `--module` (module-kind selection)

> Milestone 2 of [`docs/roadmap.md`](../roadmap.md).

## Goal

Let the caller pick the emitted module system via an explicit `--module KIND` flag. Removes the silent `compile.go:68` line that forces `opts.Module = tsccbridge.ModuleKindESNext` regardless of user input.

## Why this matters

- **Unblocks ~hundreds of upstream compiler tests** gated by `@module:` directives. M4 (the corpus porter) cannot produce faithful fixtures for anything other than ESNext until this lands.
- **Keeps the ESM-first mandate honest.** Today tscc *advertises* ESM-first but does not let you *opt out* — the guarantee is enforced by silencing your input, which violates "Inputs + Flags = Output": two identical invocations with different `--module` flags produce identical outputs. A user discovering this would rightly conclude tscc ignores their flags.
- **ESM-first remains the default.** When `--module` is unspecified, ESNext is the default, enforced at the mapping layer, not inside `compile.Compile`. The pin moves from "always" to "when unspecified".

## Compatibility with design §1 (ESM-First Mandate)

[`design/02-deterministic-resolution.md` §1](./02-deterministic-resolution.md#1-the-esm-first-mandate) reads:

> `tscc` enforces that all files are parsed and emitted as ECMAScript Modules by default. This is driven by the compiler options `--module esnext` (or similar) rather than by any `package.json#type` inference.

That "(or similar)" sentence anticipated this milestone. The invariant the ESM-First Mandate protects is *"format is never inferred from ambient state"*, not *"format is always ESNext"*. A caller explicitly passing `--module commonjs` on the CLI satisfies the invariant: the flag is a deterministic input, just like `--target es2022`.

The resolver (design §5) continues to return `GetPackageScopeForPath` = nil regardless of `--module`, so `package.json#type` cannot leak in via `ImpliedNodeFormat` even when the user picks `node16` / `nodenext`. The module *kind* and the *format inference mechanism* are separately controlled.

## Public surface

```
tscc --module KIND … a.ts
```

Accepted `KIND` values and their behavior:

| CLI value      | `core.ModuleKind`     | Notes                                        |
|----------------|-----------------------|----------------------------------------------|
| `none`         | `ModuleKindNone`      | Emit nothing for imports; type-check only.   |
| `commonjs`, `cjs` | `ModuleKindCommonJS` | `require()` / `module.exports`.               |
| `amd`          | `ModuleKindAMD`       | `define([...], factory)`.                    |
| `umd`          | `ModuleKindUMD`       | AMD + CJS with detection prologue.           |
| `system`       | `ModuleKindSystem`    | SystemJS registry.                           |
| `es2015`, `es6` | `ModuleKindES2015`   | `import` / `export` only, no dynamic import. |
| `es2020`       | `ModuleKindES2020`    | Adds `import()`, `import.meta`.              |
| `es2022`       | `ModuleKindES2022`    | Adds top-level `await`.                      |
| `esnext`       | `ModuleKindESNext`    | Latest ESM; current default.                 |
| `node16`       | `ModuleKindNode16`    | Triggers package.json-sensitive emit upstream. |
| `node18`       | `ModuleKindNode18`    | Same family as Node16/NodeNext.              |
| `node20`       | `ModuleKindNode20`    | Same family.                                 |
| `nodenext`     | `ModuleKindNodeNext`  | Latest Node semantics.                       |
| `preserve`     | `ModuleKindPreserve`  | Leave `import`/`export` syntax as-is.        |

All values are case-insensitive (`Commonjs` and `commonjs` map identically). Unknown values return a clear error at parse time.

**On the `nodeNN` family:** these kinds normally have tsc infer per-file format from `package.json#type`. With the `LiteralResolver` stubbing `GetPackageScopeForPath` (design §5), every file is treated as if it has no ambient package scope — which tsc's upstream code falls back to as ESM. The net effect of `--module node20` under tscc is close to `--module esnext` for first-party code; where it diverges is the kinds of syntax upstream guards on per-file-format (e.g., `.cts` / `.mts` extension handling). We accept these kinds because:

1. Refusing them makes tscc feel arbitrary to users porting `tsc` invocations.
2. Their determinism posture is identical to ESNext under our resolver.
3. The divergence is a feature of the flag the user asked for, not a tscc surprise.

**On the node16/node18/node20/nodenext family naming convention:** the `@module:` directive in upstream test cases uses the same names, so M4's directive translation can pass them through 1:1.

## Removing the hardcode

[`internal/compile/compile.go`](../../internal/compile/compile.go) currently reads:

```go
opts := parsed.CompilerOptions()
opts.Types = []string{}
opts.TypeRoots = []string{}
opts.Module = tsccbridge.ModuleKindESNext  // line 68 — remove
```

`opts.Types = []string{}` and `opts.TypeRoots = []string{}` stay — they protect design §8's "Neutralizing Secondary Magic" around automatic `@types` inclusion, which is independent of module kind.

The `Module` pin moves into `compileropts.FromConfig`, which is where other options already get their defaults:

```go
// compileropts.go
func FromConfig(cfg *config.Config) (*tsccbridge.CompilerOptions, error) {
    target, ok := targetByName[strings.ToLower(cfg.Target)]
    if !ok {
        return nil, fmt.Errorf("unknown target: %q", cfg.Target)
    }

    mod := tsccbridge.ModuleKindESNext  // default preserves current behavior
    if cfg.Module != "" {
        m, ok := moduleByName[strings.ToLower(cfg.Module)]
        if !ok {
            return nil, fmt.Errorf("unknown module: %q", cfg.Module)
        }
        mod = m
    }

    return &tsccbridge.CompilerOptions{
        Target: target,
        Strict: boolToTristate(cfg.Strict),
        Module: mod,
    }, nil
}
```

The mapping layer owns defaults; `compile.Compile` just consumes what it's given.

## Bridge delta

`tools/genbridge/main.go` currently exports only `ModuleKindESNext`. Extend the const block to include the full set the mapping table needs:

```go
const (
    ModuleKindNone     ModuleKind = core.ModuleKindNone
    ModuleKindCommonJS ModuleKind = core.ModuleKindCommonJS
    ModuleKindAMD      ModuleKind = core.ModuleKindAMD
    ModuleKindUMD      ModuleKind = core.ModuleKindUMD
    ModuleKindSystem   ModuleKind = core.ModuleKindSystem
    ModuleKindES2015   ModuleKind = core.ModuleKindES2015
    ModuleKindES2020   ModuleKind = core.ModuleKindES2020
    ModuleKindES2022   ModuleKind = core.ModuleKindES2022
    ModuleKindESNext   ModuleKind = core.ModuleKindESNext
    ModuleKindNode16   ModuleKind = core.ModuleKindNode16
    ModuleKindNode18   ModuleKind = core.ModuleKindNode18
    ModuleKindNode20   ModuleKind = core.ModuleKindNode20
    ModuleKindNodeNext ModuleKind = core.ModuleKindNodeNext
    ModuleKindPreserve ModuleKind = core.ModuleKindPreserve
)
```

After editing `tools/genbridge/main.go`, run `go generate ./...`.

## Config plumbing

```go
// internal/config/config.go
type Config struct {
    …
    Module string  // empty means "use the mapping layer's default (ESNext)"
}

func languageGroup(cfg *Config) flagGroup {
    g := pflag.NewFlagSet("language", pflag.ContinueOnError)
    g.StringVar(&cfg.Target, "target", "es2025", "Set the JavaScript language `version` for emitted JavaScript (allowed: …)")
    g.StringVar(&cfg.Module, "module", "", "Emitted module system `KIND` (allowed: none, commonjs, amd, umd, system, es2015, es2020, es2022, esnext, node16, node18, node20, nodenext, preserve). Default: esnext.")
    return flagGroup{Name: "Language and Environment", Set: g}
}
```

Default-string `""` rather than `"esnext"` because:

1. The help text documents the default in prose; `--help` stays readable.
2. The mapping layer owns the default; surfacing it here would split the source of truth.
3. If the default ever changes (very unlikely), one place moves, not two.

## Tests

**Unit (`compileropts_test.go`):**
- Table-driven: every `KIND` value in the public-surface table → expected `ModuleKind` constant.
- Case-insensitivity: `CommonJS`, `COMMONJS`, `commonjs`, `cjs` all map to `ModuleKindCommonJS`.
- Empty `cfg.Module` → `ModuleKindESNext` (default preservation).
- Unknown value → error with the invalid string in the message.

**E2E:**
- `cmd/tscc/testdata/moduleCommonJS.txtar`: port one CommonJS case from `_submodules/TypeScript/tests/cases/compiler/` (e.g., `exportAssignmentTopLevel.ts` or similar). Assert `exports.` / `require()` emit.
- Existing fixtures must pass unchanged — the default is still ESNext.

## Commit shape

1. **Bridge exports.** Extend `tools/genbridge/main.go` const block, regenerate, verify `go build ./...` still passes.
2. **Config + mapping layer.** Add `Module` field, `--module` flag, `moduleByName` table, mapping-layer default. Unit tests land with this commit.
3. **Remove the hardcode + e2e.** Delete `compile.go:68`, add the CommonJS e2e fixture.

## Exit criteria

1. `./tscc --module commonjs /abs/a.ts` emits CommonJS (`require`/`exports`).
2. `./tscc --module esnext /abs/a.ts` and `./tscc /abs/a.ts` produce identical output (default preservation).
3. `./tscc --module bogus /abs/a.ts` errors with a clear message.
4. All existing `.txtar` fixtures pass unchanged.
5. Upstream `@module:` directive tests can be directly translated 1:1 when M4 lands.
