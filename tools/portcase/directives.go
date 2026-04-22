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

// knownUnsupportedDirectives lists directives we explicitly know we don't support.
var knownUnsupportedDirectives = map[string]bool{
	"jsx":                 true,
	"jsxfactory":          true,
	"jsxfragmentfactory":  true,
	"jsximportsource":     true,
	"allowjs":             true,
	"checkjs":             true,
	"lib":                 true,
	"outdir":              true,
	"outfile":             true,
	"rootdir":             true,
	"emitdeclarationonly": true,
	"traceresolution":     true,
	"listfiles":           true,
	"listemittedfiles":    true,
	"moduleresolution":    true,
	"paths":               true,
}

// TranslateDirectives translates parsed directives to tscc flags.
// outputBaseName is used for resolving generated output names like --out-dts <name>.d.ts.
func TranslateDirectives(directives map[string]string, outputBaseName string) ([]string, error) {
	var flags []string

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
			flags = append(flags, "--module", value)
		case "declaration":
			if strings.ToLower(value) == "true" {
				flags = append(flags, "--out-dts", outputBaseName+".d.ts")
			}
		case "sourcemap":
			if strings.ToLower(value) == "true" {
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
		case "filename":
			// Handled separately during file block parsing.
			continue
		case "currentdirectory":
			// Safely ignore for now
			continue
		default:
			// "any directive not in the translation table or the @filename structural set -> skip"
			return nil, &SkipError{Directive: key, Reason: "unrecognized"}
		}
	}
	return flags, nil
}
