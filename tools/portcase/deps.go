package main

import (
	"path/filepath"
	"regexp"
)

func getDependencies(content string) []string {
	var deps []string

	importRe := regexp.MustCompile(`(?:import|export)\s+.*?\s+from\s+['"]([^'"]+)['"]`)
	for _, m := range importRe.FindAllStringSubmatch(content, -1) {
		deps = append(deps, m[1])
	}

	importRequireRe := regexp.MustCompile(`import\s+.*?\s*=\s*require\(['"]([^'"]+)['"]\)`)
	for _, m := range importRequireRe.FindAllStringSubmatch(content, -1) {
		deps = append(deps, m[1])
	}

	referenceRe := regexp.MustCompile(`/\/\/\s*<\s*reference\s+path\s*=\s*['"]([^'"]+)['"]\s*/>`)
	for _, m := range referenceRe.FindAllStringSubmatch(content, -1) {
		deps = append(deps, m[1])
	}

	dynamicImportRe := regexp.MustCompile(`import\(['"]([^'"]+)['"]\)`)
	for _, m := range dynamicImportRe.FindAllStringSubmatch(content, -1) {
		deps = append(deps, m[1])
	}

	return deps
}

func resolveDependency(importer, dep string, inputList []string) string {
	dir := filepath.Dir(importer)
	joined := filepath.ToSlash(filepath.Join(dir, dep))

	candidates := []string{
		joined,
		joined + ".ts",
		joined + ".d.ts",
		joined + ".tsx",
		joined + "/index.ts",
	}

	for _, c := range candidates {
		for _, in := range inputList {
			if in == c {
				return in
			}
		}
	}

	base := filepath.Base(dep)
	for _, in := range inputList {
		if filepath.Base(in) == base || filepath.Base(in) == base+".ts" || filepath.Base(in) == base+".d.ts" {
			return in
		}
	}

	return ""
}

func isScript(content string) bool {
	// A file is a module if it has a top-level import or export.
	// We use a simplified check that matches most cases in the compiler test suite.
	hasExport := regexp.MustCompile(`(?m)^(export\s+|export\s*=|export\s*\{)`).MatchString(content)
	hasImport := regexp.MustCompile(`(?m)^(import\s+|import\s*['"]|import\s*.*?\s*=\s*require\()`).MatchString(content)
	return !hasExport && !hasImport
}
