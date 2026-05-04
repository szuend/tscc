# Synthetic Package Scope for ESM-First

## The Problem

`tscc`'s North Star dictates an **ESM-first mandate** ([Design §1](02-deterministic-resolution.md#1-the-esm-first-mandate)): files are parsed and emitted as ECMAScript Modules by default, without relying on ambient `package.json` discovery.

To achieve this, `tscc` replaces the upstream resolver with a `LiteralResolver` ([Design §5](02-deterministic-resolution.md#5-the-custom-resolver)) that currently stubs `GetPackageScopeForPath` to return `nil`. The assumption in earlier designs (such as [Design §4](04-module-flag.md)) was that returning no package scope would cause the upstream compiler to default to ESM for `.ts` files when using `--module node16` or `--module nodenext`.

However, this assumption is incorrect. In `typescript-go`'s internal logic (`ast.GetImpliedNodeFormatForFile`), if a file has a `.ts` extension and the `package.json` `type` field is not explicitly `"module"`, it defaults to CommonJS:

```go
// third_party/typescript-go/internal/ast/utilities.go
impliedNodeFormat = core.IfElse(packageJsonType == "module", core.ResolutionModeESM, core.ResolutionModeCommonJS)
```

Because `LiteralResolver` returns `nil` for the package scope, `packageJsonType` is empty, causing `--module node16` to emit `require()` and `module.exports` for standard `.ts` files. This violates the ESM-first mandate and forces users to use extensions like `.mts` if they want ESM output under modern Node resolutions.

## The Solution

Instead of returning `nil`, `LiteralResolver.GetPackageScopeForPath` must return a synthetic package scope that explicitly sets the `"type"` field to `"module"`.

By doing this, we:
1. **Restore the ESM-first mandate:** `.ts` files compiled with `--module node16` or `nodenext` will correctly emit as ES modules (`import` / `export`).
2. **Preserve Determinism:** We still do not perform any disk I/O to read an actual `package.json`. The scope is completely synthetic, deterministic, and identical across all environments.
3. **Align with Upstream Architecture:** We satisfy `typescript-go`'s requirement for a `package.json` context to drive its `node16`/`nodenext` logic without leaking ambient state.

### Implementation Details

The `LiteralResolver` needs to return a constructed `*tsccbridge.InfoCacheEntry` (which maps to `packagejson.InfoCacheEntry`).

```go
func (r *LiteralResolver) GetPackageScopeForPath(directory string) *tsccbridge.InfoCacheEntry {
	// Construct a synthetic PackageJson object where the "type" field is "module".
	// This forces typescript-go to treat .ts files as ESM under --module node16/nodenext,
	// upholding tscc's ESM-first mandate.
	
	// ... construct packagejson.PackageJson with type: "module" ...
	
	return syntheticScope
}
```

This synthetic scope will be applied uniformly to every file `tscc` compiles, ensuring consistent behavior.

## Interaction with `--moduleResolution`

During the investigation into this behavior, we also confirmed that `tscc` **does not need to expose a `--moduleResolution` CLI flag.**

1. **Resolution is handled by `LiteralResolver`:** `typescript-go` uses `--moduleResolution` primarily to steer its complex file-system discovery logic (e.g., recursive `node_modules` lookups, resolving `exports` maps). Since `tscc`'s `LiteralResolver` bypasses this entirely in favor of explicit `--path` mappings, the flag's primary purpose is moot.
2. **Checker validation is tied to `--module`:** The only remaining impact of `--moduleResolution` is in the Checker (e.g., enforcing explicit extensions on relative imports). However, `typescript-go` already defaults the resolution kind based on the `--module` flag (e.g., `--module node16` implies `node16` resolution rules).
3. **Older modes are mapped to `bundler`:** Modern `typescript-go` maps older resolution modes (like `classic`) to `bundler` by default, further reducing the need for explicit overrides.

Providing `--moduleResolution` would offer no functional value and might misleadingly imply that `tscc` supports ambient Node.js module resolution features.

## Action Items

1. Update `LiteralResolver.GetPackageScopeForPath` to return a synthetic scope with `"type": "module"`.
2. Update `02-deterministic-resolution.md` and `04-module-flag.md` to reflect that the resolver returns a synthetic scope rather than `nil`, and correct the misconception about tsc's fallback behavior.
