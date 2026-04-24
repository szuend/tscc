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

package main

import (
	"fmt"
	"sort"
	"strings"
)

// SkipError is returned when a directive is explicitly not supported.
type SkipError struct {
	Directive string
	Reason    string
}

func (e *SkipError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("unsupported directive @%s: %s", e.Directive, e.Reason)
	}
	return fmt.Sprintf("unsupported directive @%s", e.Directive)
}

// IgnoreError is returned when a directive is permanently ignored.
type IgnoreError struct {
	Directive string
	Reason    string
}

func (e *IgnoreError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("ignored directive @%s: %s", e.Directive, e.Reason)
	}
	return fmt.Sprintf("ignored directive @%s", e.Directive)
}

// knownUnsupportedDirectives lists directives we explicitly know we don't support.
var knownUnsupportedDirectives = map[string]bool{
	"jsx":                true,
	"jsxfactory":         true,
	"jsxfragmentfactory": true,
	"jsximportsource":    true,
	"outdir":             true,
	"outfile":            true,
	"rootdir":            true,
	"traceresolution":    true,
	"listfiles":          true,
	"listemittedfiles":   true,
	"moduleresolution":   true,
	"paths":              true,
}

// TranslateDirectives translates parsed directives to tscc flags.
// outputBaseName is used for resolving generated output names like --out-dts <name>.d.ts.
func TranslateDirectives(directives map[string]string, outputBaseName string) ([]string, error) {
	var flags []string

	noEmit := false
	for k, v := range directives {
		if strings.ToLower(k) == "noemit" && strings.ToLower(v) == "true" {
			noEmit = true
			break
		}
	}

	var keys []string
	for k := range directives {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := directives[key]
		keyLower := strings.ToLower(key)

		if knownUnsupportedDirectives[keyLower] {
			return nil, &SkipError{Directive: key}
		}

		switch keyLower {
		case "target":
			if strings.Contains(value, ",") {
				return nil, &SkipError{Directive: key, Reason: "multiple values"}
			}
			if strings.ToLower(value) == "es5" {
				return nil, &IgnoreError{Directive: key, Reason: "es5 is not supported by tscc"}
			}
			flags = append(flags, "--target", value)
		case "strict":
			if strings.ToLower(value) == "true" {
				flags = append(flags, "--strict")
			} else if strings.ToLower(value) == "false" {
				flags = append(flags, "--no-strict")
			}
		case "module":
			if strings.Contains(value, ",") {
				return nil, &SkipError{Directive: key, Reason: "multiple values"}
			}
			if strings.ToLower(value) == "system" {
				return nil, &IgnoreError{Directive: key, Reason: "system module is not supported by typescript-go"}
			}
			flags = append(flags, "--module", value)
		case "declaration":
			if !noEmit && strings.ToLower(value) == "true" {
				flags = append(flags, "--out-dts", outputBaseName+".d.ts")
			}
		case "sourcemap":
			if !noEmit && strings.ToLower(value) == "true" {
				flags = append(flags, "--out-map", outputBaseName+".js.map")
			}
		case "noimplicitany":
			if strings.ToLower(value) == "true" || value == "" {
				flags = append(flags, "--no-implicit-any")
			} else if strings.ToLower(value) == "false" {
				flags = append(flags, "--no-no-implicit-any")
			}
		case "exactoptionalpropertytypes":
			if strings.ToLower(value) == "true" || value == "" {
				flags = append(flags, "--exact-optional-property-types")
			} else if strings.ToLower(value) == "false" {
				flags = append(flags, "--no-exact-optional-property-types")
			}
		case "strictnullchecks":
			if strings.ToLower(value) == "true" || value == "" {
				flags = append(flags, "--strict-null-checks")
			} else if strings.ToLower(value) == "false" {
				flags = append(flags, "--no-strict-null-checks")
			}
		case "alwaysstrict":
			if strings.ToLower(value) == "false" {
				return nil, &SkipError{Directive: key, Reason: "false is unsupported (deprecated)"}
			}
			// true or empty is the default, so we can just ignore it
		case "noemitonerror":
			if strings.ToLower(value) == "false" {
				return nil, &SkipError{Directive: key, Reason: "false is unsupported (tscc never emits on error)"}
			}
			// true is the default for tscc
		case "skiplibcheck":
			if strings.ToLower(value) == "true" || value == "" {
				flags = append(flags, "--skip-lib-check")
			} else if strings.ToLower(value) == "false" {
				flags = append(flags, "--no-skip-lib-check")
			}
		case "allowjs":
			if strings.ToLower(value) == "true" || value == "" {
				flags = append(flags, "--allow-js")
			} else if strings.ToLower(value) == "false" {
				flags = append(flags, "--no-allow-js")
			}
		case "checkjs":
			if strings.ToLower(value) == "true" || value == "" {
				flags = append(flags, "--check-js")
			} else if strings.ToLower(value) == "false" {
				flags = append(flags, "--no-check-js")
			}
		case "isolatedmodules":
			if strings.ToLower(value) == "true" || value == "" {
				flags = append(flags, "--isolated-modules")
			} else if strings.ToLower(value) == "false" {
				flags = append(flags, "--no-isolated-modules")
			}
		case "lib":
			flags = append(flags, "--lib", value)
		case "noemit":
			// Handled at top and in porter.go
			continue
		case "filename":
			// Handled separately during file block parsing.
			continue
		case "currentdirectory":
			// Safely ignore for now
			continue
		case "notypesandsymbols":
			// Safely ignore, as tscc doesn't generate type/symbol baselines
			continue
		case "emitdeclarationonly":
			// Safely ignore: porter.go explicitly checks this option to suppress the
			// default --out-js flag, enforcing the declaration-only behavior.
			continue
		default:
			// "any directive not in the translation table or the @filename structural set -> skip"
			return nil, &SkipError{Directive: key, Reason: "unrecognized"}
		}
	}

	if _, ok := directives["lib"]; !ok {
		target := "es2025"
		if t, ok := directives["target"]; ok {
			target = t
		}
		flags = append(flags, "--lib", target+",dom")
	}

	return flags, nil
}
