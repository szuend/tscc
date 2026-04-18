// Copyright 2026 Simon Zünd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package resolver implements tscc's LiteralResolver — the drop-in replacement
// for typescript-go's resolver described in design §§3–5.
//
// Resolution rules:
//
//   - Relative specifiers ("./x", "../x") resolve by literal path concatenation
//     against the containing file's directory, followed by the TypeScript
//     extension-substitution table. Never walks parents; never probes package.json.
//   - Absolute specifiers resolve to themselves if the file exists.
//   - Bare specifiers resolve only via an explicit --path NAME=/abs/path mapping;
//     any unmapped bare specifier returns unresolved (typescript-go reports
//     "Cannot find module").
//   - /// <reference types="..." /> follows the same bare-specifier rule.
//   - Realpath is never invoked — coupling our semantics to the FS jail's
//     identity Realpath would let a jail change silently alter resolution.
//   - GetPackageScopeForPath returns nil so fileloader.loadSourceFileMetaData
//     cannot derive PackageJsonType → ImpliedNodeFormat (the ESM-first invariant
//     from design §1).
package resolver

import (
	"path"
	"strings"

	"github.com/microsoft/typescript-go/tsccbridge"
)

// LiteralResolver implements tsccbridge.ModuleResolver.
type LiteralResolver struct {
	fs    tsccbridge.FS
	paths map[string]string // bare specifier → absolute resolved path
}

// Options configures a LiteralResolver.
type Options struct {
	FS tsccbridge.FS
	// Paths maps a bare import specifier to an absolute file path. Keys are
	// matched exactly — no prefix or wildcard semantics (design §4).
	Paths map[string]string
}

// New constructs a LiteralResolver from opts.
func New(opts Options) *LiteralResolver {
	return &LiteralResolver{fs: opts.FS, paths: opts.Paths}
}

// GetPackageScopeForPath always returns nil (design §1/§5). An empty scope is
// interpreted downstream as "no ambient package context", which forces ESM-first
// behavior because typescript-go cannot then read package.json#type.
func (r *LiteralResolver) GetPackageScopeForPath(directory string) *tsccbridge.InfoCacheEntry {
	return nil
}

// ResolvePackageDirectory is never invoked under literal resolution because
// bare specifiers are resolved via the --path table, not by searching for a
// package directory. Returning nil is treated as "not found" upstream.
func (r *LiteralResolver) ResolvePackageDirectory(moduleName string, containingFile string, resolutionMode tsccbridge.ResolutionMode, redirectedReference tsccbridge.ResolvedProjectReference) *tsccbridge.ResolvedModule {
	return nil
}

// ResolveModuleName implements the literal resolution table from design §§3–4.
// Unresolved returns are always a non-nil ResolvedModule with an empty
// ResolvedFileName: typescript-go's includeProcessor iterates every entry in
// the resolved-modules cache and dereferences nil, so "not found" must be a
// pointer to a zero-valued struct rather than nil.
func (r *LiteralResolver) ResolveModuleName(moduleName string, containingFile string, resolutionMode tsccbridge.ResolutionMode, redirectedReference tsccbridge.ResolvedProjectReference) (*tsccbridge.ResolvedModule, []tsccbridge.DiagAndArgs) {
	// Bare specifier: require an explicit --path mapping. Per design §4 the
	// mapping target IS the resolved path — no probing, no extension
	// substitution. The GetSourceFile read goes through the unjailed raw FS
	// (design §7), so mappings that point inside jailed directories still work.
	if !isRelative(moduleName) && !path.IsAbs(moduleName) {
		target, ok := r.paths[moduleName]
		if !ok {
			return unresolved(), nil
		}
		_, ext := splitExtension(target)
		return &tsccbridge.ResolvedModule{
			ResolvedFileName: target,
			OriginalPath:     target,
			Extension:        ext,
		}, nil
	}

	// Absolute specifier: the path already is absolute; apply extension
	// substitution in case the caller supplied a .js-flavored import.
	if path.IsAbs(moduleName) {
		return r.resolveLiteralPath(moduleName, moduleName)
	}

	// Relative specifier: join against containing dir. path.Join normalizes
	// "./" and "../" and produces a forward-slash canonical form, matching
	// typescript-go's internal convention.
	containingDir := path.Dir(containingFile)
	joined := path.Join(containingDir, moduleName)
	return r.resolveLiteralPath(joined, joined)
}

// unresolved returns a zero-valued ResolvedModule marker. IsResolved() is
// false because ResolvedFileName is empty.
func unresolved() *tsccbridge.ResolvedModule {
	return &tsccbridge.ResolvedModule{}
}

// ResolveTypeReferenceDirective is treated as a bare import: permitted only
// when an explicit mapping exists (design §4 / §8).
func (r *LiteralResolver) ResolveTypeReferenceDirective(
	typeReferenceDirectiveName string,
	containingFile string,
	resolutionMode tsccbridge.ResolutionMode,
	redirectedReference tsccbridge.ResolvedProjectReference,
) (*tsccbridge.ResolvedTypeReferenceDirective, []tsccbridge.DiagAndArgs) {
	target, ok := r.paths[typeReferenceDirectiveName]
	if !ok {
		return nil, nil
	}
	if !r.fs.FileExists(target) {
		return nil, nil
	}
	return &tsccbridge.ResolvedTypeReferenceDirective{
		ResolvedFileName: target,
		OriginalPath:     target,
	}, nil
}

// resolveLiteralPath applies the TypeScript extension-substitution table to
// candidate. On success it returns a populated ResolvedModule; on failure it
// returns the unresolved marker (non-nil, empty ResolvedFileName).
// `originalPath` is preserved separately so typescript-go's downstream caching
// sees the user's literal specifier form.
func (r *LiteralResolver) resolveLiteralPath(candidate, originalPath string) (*tsccbridge.ResolvedModule, []tsccbridge.DiagAndArgs) {
	base, ext := splitExtension(candidate)
	for _, probe := range extensionCandidates(ext) {
		full := base + probe
		if r.fs.FileExists(full) {
			return &tsccbridge.ResolvedModule{
				ResolvedFileName: full,
				OriginalPath:     originalPath,
				Extension:        probe,
			}, nil
		}
	}
	return unresolved(), nil
}

// isRelative matches "." / ".." / "./…" / "../…". Backslash handling is
// intentionally absent — typescript-go canonicalizes to forward slashes
// before the specifier reaches the resolver.
func isRelative(spec string) bool {
	return spec == "." || spec == ".." ||
		strings.HasPrefix(spec, "./") ||
		strings.HasPrefix(spec, "../")
}

// splitExtension returns (base, ext) where ext is the recognized suffix of
// spec. Double-extensions ".d.ts", ".d.mts", ".d.cts" are recognized before
// the single-dot cases.
func splitExtension(spec string) (base, ext string) {
	for _, e := range []string{".d.ts", ".d.mts", ".d.cts"} {
		if strings.HasSuffix(spec, e) {
			return spec[:len(spec)-len(e)], e
		}
	}
	slash := strings.LastIndex(spec, "/")
	dot := strings.LastIndex(spec, ".")
	if dot < 0 || dot < slash {
		return spec, ""
	}
	return spec[:dot], spec[dot:]
}

// extensionCandidates implements the standard TypeScript substitution rules:
// a .js-flavored import probes the corresponding .ts/.tsx/.d.ts first so a
// build system that vendors .js stubs alongside .ts sources still lands on
// the TypeScript source.
func extensionCandidates(ext string) []string {
	switch ext {
	case ".js":
		return []string{".ts", ".tsx", ".d.ts", ".js"}
	case ".jsx":
		return []string{".tsx", ".jsx"}
	case ".mjs":
		return []string{".mts", ".d.mts", ".mjs"}
	case ".cjs":
		return []string{".cts", ".d.cts", ".cjs"}
	case ".json":
		return []string{".json"}
	case "":
		return []string{".ts", ".tsx", ".d.ts", ".js", ".jsx"}
	default:
		// Native TS extensions (.ts, .tsx, .mts, .cts, .d.ts, …) used literally.
		return []string{ext}
	}
}
