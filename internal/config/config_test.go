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

package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantConfig  *Config
		wantErrText string
	}{
		{
			name: "defaults and single input",
			args: []string{"input.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "input.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "case-sensitive-paths opt-out",
			args: []string{"--case-sensitive-paths=false", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           false,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "override flags",
			args: []string{"--target", "es2015", "--strict=false", "foo.ts"},
			wantConfig: &Config{
				Strict:                       false,
				NoImplicitAny:                false,
				StrictNullChecks:             false,
				Target:                       "es2015",
				InputPath:                    "foo.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2015"},
				UseDefineForClassFields:      false,
			},
		},
		{
			name: "negated boolean flag",
			args: []string{"--no-strict", "bar.ts"},
			wantConfig: &Config{
				Strict:                       false,
				NoImplicitAny:                false,
				StrictNullChecks:             false,
				Target:                       "es2025",
				InputPath:                    "bar.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-property-access-from-index-signature",
			args: []string{"--no-property-access-from-index-signature", "in.ts"},
			wantConfig: &Config{
				Strict:                             true,
				NoImplicitAny:                      true,
				StrictNullChecks:                   true,
				Target:                             "es2025",
				InputPath:                          "in.ts",
				CaseSensitivePaths:                 true,
				NoUncheckedSideEffectImports:       true,
				Lib:                                []string{"es2025"},
				UseDefineForClassFields:            true,
				NoPropertyAccessFromIndexSignature: true,
			},
		},
		{
			name: "negated no-property-access-from-index-signature",
			args: []string{"--no-no-property-access-from-index-signature", "in.ts"},
			wantConfig: &Config{
				Strict:                             true,
				NoImplicitAny:                      true,
				StrictNullChecks:                   true,
				Target:                             "es2025",
				InputPath:                          "in.ts",
				CaseSensitivePaths:                 true,
				NoUncheckedSideEffectImports:       true,
				Lib:                                []string{"es2025"},
				UseDefineForClassFields:            true,
				NoPropertyAccessFromIndexSignature: false,
			},
		},
		{
			name: "output flag",
			args: []string{"-o", "out.js", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				OutJSPath:                    "out.js",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "output flag long",
			args: []string{"--out-js", "dist/bundle.js", "src/main.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "src/main.ts",
				OutJSPath:                    "dist/bundle.js",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "out-deps flag",
			args: []string{"--out-deps", "dist/bundle.d", "src/main.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "src/main.ts",
				OutDepsPath:                  "dist/bundle.d",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-implicit-any follows strict",
			args: []string{"--no-strict", "in.ts"},
			wantConfig: &Config{
				Strict:                       false,
				NoImplicitAny:                false,
				StrictNullChecks:             false,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-implicit-any override follows strict",
			args: []string{"--no-strict", "--no-implicit-any=true", "in.ts"},
			wantConfig: &Config{
				Strict:                       false,
				NoImplicitAny:                true,
				StrictNullChecks:             false,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-implicit-any override follows strict long",
			args: []string{"--strict=true", "--no-implicit-any=false", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                false,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-implicit-any negated prefix override",
			args: []string{"--no-strict", "--no-no-implicit-any", "in.ts"},
			wantConfig: &Config{
				Strict:                       false,
				NoImplicitAny:                false,
				StrictNullChecks:             false,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "lib flag comma-separated",
			args: []string{"--lib", "esnext,dom", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"esnext", "dom"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-lib flag disables default lib",
			args: []string{"--no-lib", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          nil,
				NoLib:                        true,
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-lib overrides earlier lib",
			args: []string{"--lib", "es2020", "--no-lib", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          nil,
				NoLib:                        true,
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "lib overrides earlier no-lib",
			args: []string{"--no-lib", "--lib", "dom", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"dom"},
				NoLib:                        false,
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "complex lib and no-lib interleaving",
			args: []string{"--lib", "es2020", "--no-lib", "--lib", "es2025,dom", "--no-lib", "--lib", "webworker", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"webworker"},
				NoLib:                        false,
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "lib flag repeated",
			args: []string{"--lib", "esnext", "--lib", "dom", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"esnext", "dom"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "strict-null-checks follows strict false",
			args: []string{"--no-strict", "in.ts"},
			wantConfig: &Config{
				Strict:                       false,
				NoImplicitAny:                false,
				StrictNullChecks:             false,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "strict-null-checks override follows strict",
			args: []string{"--no-strict", "--strict-null-checks=true", "in.ts"},
			wantConfig: &Config{
				Strict:                       false,
				NoImplicitAny:                false,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "skip-lib-check true",
			args: []string{"--skip-lib-check", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				SkipLibCheck:                 true,
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "skip-lib-check negated",
			args: []string{"--no-skip-lib-check", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				SkipLibCheck:                 false,
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "allow-js default follows check-js",
			args: []string{"--check-js", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				CheckJs:                      true,
				AllowJs:                      true,
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "isolated-modules true",
			args: []string{"--isolated-modules", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				IsolatedModules:              true,
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "isolated-modules negated",
			args: []string{"--no-isolated-modules", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				IsolatedModules:              false,
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "allow-js override check-js default",
			args: []string{"--check-js", "--allow-js=false", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				CheckJs:                      true,
				AllowJs:                      false,
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-unchecked-side-effect-imports false",
			args: []string{"--no-unchecked-side-effect-imports=false", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: false,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-unchecked-side-effect-imports negated",
			args: []string{"--no-no-unchecked-side-effect-imports", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: false,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-emit-helpers flag",
			args: []string{"--no-emit-helpers", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				NoEmitHelpers:                true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-emit-helpers explicit false",
			args: []string{"--no-emit-helpers=false", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				NoEmitHelpers:                false,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-emit-helpers negated",
			args: []string{"--no-no-emit-helpers", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				NoEmitHelpers:                false,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-implicit-returns flag",
			args: []string{"--no-implicit-returns", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				NoImplicitReturns:            true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-implicit-returns explicit false",
			args: []string{"--no-implicit-returns=false", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				NoImplicitReturns:            false,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "no-implicit-returns negated",
			args: []string{"--no-no-implicit-returns", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				NoImplicitReturns:            false,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "use-define-for-class-fields default true for es2025",
			args: []string{"in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "use-define-for-class-fields default false for es2020",
			args: []string{"--target", "es2020", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2020",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2020"},
				UseDefineForClassFields:      false,
			},
		},
		{
			name: "use-define-for-class-fields default true for esnext",
			args: []string{"--target", "esnext", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "esnext",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"esnext"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "use-define-for-class-fields explicit true",
			args: []string{"--use-define-for-class-fields", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      true,
			},
		},
		{
			name: "use-define-for-class-fields explicit false",
			args: []string{"--use-define-for-class-fields=false", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      false,
			},
		},
		{
			name: "use-define-for-class-fields negated",
			args: []string{"--no-use-define-for-class-fields", "in.ts"},
			wantConfig: &Config{
				Strict:                       true,
				NoImplicitAny:                true,
				StrictNullChecks:             true,
				Target:                       "es2025",
				InputPath:                    "in.ts",
				CaseSensitivePaths:           true,
				NoUncheckedSideEffectImports: true,
				Lib:                          []string{"es2025"},
				UseDefineForClassFields:      false,
			},
		},
		{
			name:        "missing input",
			args:        []string{"--target", "es2022"},
			wantErrText: "no input file specified",
		},
		{
			name:        "too many inputs",
			args:        []string{"a.ts", "b.ts"},
			wantErrText: "too many input files specified: [a.ts b.ts]",
		},
		{
			name:        "path missing equals",
			args:        []string{"--path", "lib", "in.ts"},
			wantErrText: "expected NAME=PATH",
		},
		{
			name:        "path duplicate name",
			args:        []string{"--path", "lib=/a", "--path", "lib=/b", "in.ts"},
			wantErrText: "specified more than once",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args)

			if tt.wantErrText != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrText)
				}
				if !strings.Contains(err.Error(), tt.wantErrText) {
					t.Errorf("expected error %q, got %q", tt.wantErrText, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantConfig := tt.wantConfig
			if wantConfig.InputPath != "" {
				wantConfig.InputPath, _ = filepath.Abs(wantConfig.InputPath)
			}
			if wantConfig.OutJSPath != "" {
				wantConfig.OutJSPath, _ = filepath.Abs(wantConfig.OutJSPath)
			}
			if wantConfig.OutDepsPath != "" {
				wantConfig.OutDepsPath, _ = filepath.Abs(wantConfig.OutDepsPath)
			}
			if wantConfig.OutDtsPath != "" {
				wantConfig.OutDtsPath, _ = filepath.Abs(wantConfig.OutDtsPath)
			}
			if wantConfig.OutMapPath != "" {
				wantConfig.OutMapPath, _ = filepath.Abs(wantConfig.OutMapPath)
			}

			if diff := cmp.Diff(wantConfig, got, cmpopts.IgnoreUnexported(Config{}), cmpopts.IgnoreFields(Config{}, "StrictFunctionTypes")); diff != "" {
				t.Errorf("Parse() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParse_PathMap(t *testing.T) {
	cfg, err := Parse([]string{"--path", "lib=/vendor/lib.d.ts", "--path", "utils=/vendor/utils/index.d.ts", "in.ts"})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if got, want := cfg.Paths["lib"], "/vendor/lib.d.ts"; got != want {
		t.Errorf("Paths[lib]: got %q, want %q", got, want)
	}
	if got, want := cfg.Paths["utils"], "/vendor/utils/index.d.ts"; got != want {
		t.Errorf("Paths[utils]: got %q, want %q", got, want)
	}

	// Test relative path resolution
	cfgRel, err := Parse([]string{"--path", "rel=./vendor/lib.d.ts", "in.ts"})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	wantRel, _ := filepath.Abs("./vendor/lib.d.ts")
	if got := cfgRel.Paths["rel"]; got != filepath.ToSlash(wantRel) {
		t.Errorf("Paths[rel]: got %q, want %q", got, filepath.ToSlash(wantRel))
	}
}

func TestParse_AmbientTypeFile(t *testing.T) {
	cfg, err := Parse([]string{"--ambient-type-file", "/vendor/lib.d.ts", "--ambient-type-file", "./local.d.ts", "in.ts"})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(cfg.AmbientTypeFiles) != 2 {
		t.Fatalf("expected 2 ambient type files, got %d", len(cfg.AmbientTypeFiles))
	}

	if got, want := cfg.AmbientTypeFiles[0], "/vendor/lib.d.ts"; got != want {
		t.Errorf("AmbientTypeFiles[0]: got %q, want %q", got, want)
	}

	wantRel, _ := filepath.Abs("./local.d.ts")
	if got, want := cfg.AmbientTypeFiles[1], filepath.ToSlash(wantRel); got != want {
		t.Errorf("AmbientTypeFiles[1]: got %q, want %q", got, want)
	}
}
