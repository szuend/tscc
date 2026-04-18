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

package resolver

import (
	"path/filepath"

	"github.com/microsoft/typescript-go/tsccbridge"
)

// LiteralResolver implements tsccbridge.ModuleResolver to strictly enforce
// deterministic resolution without FS upward traversal or implicit package lookups.
type LiteralResolver struct {
	fs tsccbridge.FS
}

func NewLiteralResolver(fs tsccbridge.FS) *LiteralResolver {
	return &LiteralResolver{fs: fs}
}

// GetPackageScopeForPath always returns nil to prevent ImpliedNodeFormat leakage
// per the roadmap.
func (r *LiteralResolver) GetPackageScopeForPath(directory string) *tsccbridge.InfoCacheEntry {
	return nil
}

// ResolvePackageDirectory handles directory-based package resolution.
func (r *LiteralResolver) ResolvePackageDirectory(moduleName string, containingFile string, resolutionMode tsccbridge.ResolutionMode, redirectedReference tsccbridge.ResolvedProjectReference) *tsccbridge.ResolvedModule {
	return nil // Unused in strict literal resolution or handled via CLI maps
}

// ResolveModuleName resolves relative literal imports without upward walking.
func (r *LiteralResolver) ResolveModuleName(moduleName string, containingFile string, resolutionMode tsccbridge.ResolutionMode, redirectedReference tsccbridge.ResolvedProjectReference) (*tsccbridge.ResolvedModule, []tsccbridge.DiagAndArgs) {
	// For literal resolution, we only allow explicit relative paths or absolute paths.
	// We don't append extensions or look for index.js unless explicitly requested.
	if filepath.IsAbs(moduleName) {
		if r.fs.FileExists(moduleName) {
			return &tsccbridge.ResolvedModule{
				ResolvedFileName: moduleName,
				OriginalPath:     moduleName,
				Extension:        filepath.Ext(moduleName),
			}, nil
		}
		return nil, nil
	}

	// Basic relative resolution without probing
	if moduleName == "." || moduleName == ".." || len(moduleName) > 1 && moduleName[0] == '.' && (moduleName[1] == '/' || moduleName[1] == '\\') || len(moduleName) > 2 && moduleName[0] == '.' && moduleName[1] == '.' && (moduleName[2] == '/' || moduleName[2] == '\\') {
		dir := filepath.Dir(containingFile)
		abs := filepath.Join(dir, moduleName)
		abs = filepath.ToSlash(abs) // Normalize

		if r.fs.FileExists(abs) {
			return &tsccbridge.ResolvedModule{
				ResolvedFileName: abs,
				OriginalPath:     abs,
				Extension:        filepath.Ext(abs),
			}, nil
		}
	}

	// Unresolved
	return nil, nil
}

// ResolveTypeReferenceDirective resolves /// <reference types="..." />
func (r *LiteralResolver) ResolveTypeReferenceDirective(
	typeReferenceDirectiveName string,
	containingFile string,
	resolutionMode tsccbridge.ResolutionMode,
	redirectedReference tsccbridge.ResolvedProjectReference,
) (*tsccbridge.ResolvedTypeReferenceDirective, []tsccbridge.DiagAndArgs) {
	return nil, nil
}
