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
	"reflect"
	"testing"
)

func TestTranslateDirectives(t *testing.T) {
	tests := []struct {
		name           string
		directives     map[string]string
		outputBaseName string
		wantFlags      []string
		wantSkip       bool
	}{
		{
			name: "basic flags with injected lib",
			directives: map[string]string{
				"target": "ES2022",
				"strict": "true",
				"module": "commonjs",
			},
			wantFlags: []string{"--module", "commonjs", "--strict", "--target", "ES2022", "--lib", "ES2022,dom"}, // Injected lib goes to end
		},
		{
			name: "no-strict with injected lib",
			directives: map[string]string{
				"strict": "false",
			},
			wantFlags: []string{"--no-strict", "--lib", "es2025,dom"},
		},
		{
			name: "declaration and sourcemap with injected lib",
			directives: map[string]string{
				"declaration": "true",
				"sourcemap":   "true",
			},
			outputBaseName: "foo",
			wantFlags:      []string{"--out-dts", "foo.d.ts", "--out-map", "foo.js.map", "--lib", "es2025,dom"},
		},
		{
			name: "filename ignored, injected lib still added",
			directives: map[string]string{
				"filename": "foo.ts",
			},
			wantFlags: []string{"--lib", "es2025,dom"},
		},
		{
			name: "noImplicitAny true with injected lib",
			directives: map[string]string{
				"noImplicitAny": "true",
			},
			wantFlags: []string{"--no-implicit-any", "--lib", "es2025,dom"},
		},
		{
			name: "noImplicitAny false with injected lib",
			directives: map[string]string{
				"noImplicitAny": "false",
			},
			wantFlags: []string{"--no-no-implicit-any", "--lib", "es2025,dom"},
		},
		{
			name: "exactOptionalPropertyTypes true with injected lib",
			directives: map[string]string{
				"exactOptionalPropertyTypes": "true",
			},
			wantFlags: []string{"--exact-optional-property-types", "--lib", "es2025,dom"},
		},
		{
			name: "exactOptionalPropertyTypes false with injected lib",
			directives: map[string]string{
				"exactOptionalPropertyTypes": "false",
			},
			wantFlags: []string{"--no-exact-optional-property-types", "--lib", "es2025,dom"},
		},
		{
			name: "lib directive present",
			directives: map[string]string{
				"lib":    "esnext",
				"target": "es2022",
			},
			wantFlags: []string{"--lib", "esnext", "--target", "es2022"}, // Alphabetical order of keys
		},
		{
			name: "lib directive with commas",
			directives: map[string]string{
				"lib": "esnext,dom",
			},
			wantFlags: []string{"--lib", "esnext,dom"},
		},
		{
			name: "noTypesAndSymbols ignored",
			directives: map[string]string{
				"noTypesAndSymbols": "true",
			},
			wantFlags: []string{"--lib", "es2025,dom"}, // Injected lib still added
		},
		{
			name: "noEmit suppresses declaration and sourcemap",
			directives: map[string]string{
				"noEmit":      "true",
				"declaration": "true",
				"sourcemap":   "true",
			},
			outputBaseName: "foo",
			wantFlags:      []string{"--lib", "es2025,dom"}, // Injected lib still added, but no out-dts or out-map
		},
		{
			name: "unsupported jsx",
			directives: map[string]string{
				"jsx": "react",
			},
			wantSkip: true,
		},
		{
			name: "unrecognized directive",
			directives: map[string]string{
				"somethingMadeUp": "true",
			},
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFlags, err := TranslateDirectives(tt.directives, tt.outputBaseName)
			if tt.wantSkip {
				if err == nil {
					t.Errorf("TranslateDirectives() expected skip error, got nil")
				} else if _, ok := err.(*SkipError); !ok {
					t.Errorf("TranslateDirectives() error = %v, want SkipError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("TranslateDirectives() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(gotFlags, tt.wantFlags) {
				t.Errorf("TranslateDirectives() = %v, want %v", gotFlags, tt.wantFlags)
			}
		})
	}
}
