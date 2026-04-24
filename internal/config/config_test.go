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
	"slices"
	"strings"
	"testing"
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
				Strict:             true,
				NoImplicitAny:      true,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "input.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "case-sensitive-paths opt-out",
			args: []string{"--case-sensitive-paths=false", "in.ts"},
			wantConfig: &Config{
				Strict:             true,
				NoImplicitAny:      true,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: false,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "override flags",
			args: []string{"--target", "es2015", "--strict=false", "foo.ts"},
			wantConfig: &Config{
				Strict:             false,
				NoImplicitAny:      false,
				StrictNullChecks:   false,
				Target:             "es2015",
				InputPath:          "foo.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2015"},
			},
		},
		{
			name: "negated boolean flag",
			args: []string{"--no-strict", "bar.ts"},
			wantConfig: &Config{
				Strict:             false,
				NoImplicitAny:      false,
				StrictNullChecks:   false,
				Target:             "es2025",
				InputPath:          "bar.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "output flag",
			args: []string{"-o", "out.js", "in.ts"},
			wantConfig: &Config{
				Strict:             true,
				NoImplicitAny:      true,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "in.ts",
				OutJSPath:          "out.js",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "output flag long",
			args: []string{"--out-js", "dist/bundle.js", "src/main.ts"},
			wantConfig: &Config{
				Strict:             true,
				NoImplicitAny:      true,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "src/main.ts",
				OutJSPath:          "dist/bundle.js",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "out-deps flag",
			args: []string{"--out-deps", "dist/bundle.d", "src/main.ts"},
			wantConfig: &Config{
				Strict:             true,
				NoImplicitAny:      true,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "src/main.ts",
				OutDepsPath:        "dist/bundle.d",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "no-implicit-any follows strict",
			args: []string{"--no-strict", "in.ts"},
			wantConfig: &Config{
				Strict:             false,
				NoImplicitAny:      false,
				StrictNullChecks:   false,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "no-implicit-any override follows strict",
			args: []string{"--no-strict", "--no-implicit-any=true", "in.ts"},
			wantConfig: &Config{
				Strict:             false,
				NoImplicitAny:      true,
				StrictNullChecks:   false,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "no-implicit-any override follows strict long",
			args: []string{"--strict=true", "--no-implicit-any=false", "in.ts"},
			wantConfig: &Config{
				Strict:             true,
				NoImplicitAny:      false,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "no-implicit-any negated prefix override",
			args: []string{"--no-strict", "--no-no-implicit-any", "in.ts"},
			wantConfig: &Config{
				Strict:             false,
				NoImplicitAny:      false,
				StrictNullChecks:   false,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "lib flag comma-separated",
			args: []string{"--lib", "esnext,dom", "in.ts"},
			wantConfig: &Config{
				Strict:             true,
				NoImplicitAny:      true,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"esnext", "dom"},
			},
		},
		{
			name: "lib flag repeated",
			args: []string{"--lib", "esnext", "--lib", "dom", "in.ts"},
			wantConfig: &Config{
				Strict:             true,
				NoImplicitAny:      true,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"esnext", "dom"},
			},
		},
		{
			name: "strict-null-checks follows strict false",
			args: []string{"--no-strict", "in.ts"},
			wantConfig: &Config{
				Strict:             false,
				NoImplicitAny:      false,
				StrictNullChecks:   false,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "strict-null-checks override follows strict",
			args: []string{"--no-strict", "--strict-null-checks=true", "in.ts"},
			wantConfig: &Config{
				Strict:             false,
				NoImplicitAny:      false,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
			},
		},
		{
			name: "skip-lib-check true",
			args: []string{"--skip-lib-check", "in.ts"},
			wantConfig: &Config{
				Strict:             true,
				NoImplicitAny:      true,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
				SkipLibCheck:       true,
			},
		},
		{
			name: "skip-lib-check negated",
			args: []string{"--no-skip-lib-check", "in.ts"},
			wantConfig: &Config{
				Strict:             true,
				NoImplicitAny:      true,
				StrictNullChecks:   true,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: true,
				Lib:                []string{"es2025"},
				SkipLibCheck:       false,
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

			if got.Strict != tt.wantConfig.Strict {
				t.Errorf("Strict: got %v, want %v", got.Strict, tt.wantConfig.Strict)
			}
			if got.NoImplicitAny != tt.wantConfig.NoImplicitAny {
				t.Errorf("NoImplicitAny: got %v, want %v", got.NoImplicitAny, tt.wantConfig.NoImplicitAny)
			}
			if got.StrictNullChecks != tt.wantConfig.StrictNullChecks {
				t.Errorf("StrictNullChecks: got %v, want %v", got.StrictNullChecks, tt.wantConfig.StrictNullChecks)
			}
			if got.Target != tt.wantConfig.Target {
				t.Errorf("Target: got %q, want %q", got.Target, tt.wantConfig.Target)
			}
			if got.CaseSensitivePaths != tt.wantConfig.CaseSensitivePaths {
				t.Errorf("CaseSensitivePaths: got %v, want %v", got.CaseSensitivePaths, tt.wantConfig.CaseSensitivePaths)
			}
			if got.SkipLibCheck != tt.wantConfig.SkipLibCheck {
				t.Errorf("SkipLibCheck: got %v, want %v", got.SkipLibCheck, tt.wantConfig.SkipLibCheck)
			}
			if !slices.Equal(got.Lib, tt.wantConfig.Lib) {
				t.Errorf("Lib: got %v, want %v", got.Lib, tt.wantConfig.Lib)
			}

			wantInput := tt.wantConfig.InputPath
			if wantInput != "" {
				wantInput, _ = filepath.Abs(wantInput)
			}
			if got.InputPath != wantInput {
				t.Errorf("InputPath: got %q, want %q", got.InputPath, wantInput)
			}

			wantOutput := tt.wantConfig.OutJSPath
			if wantOutput != "" {
				wantOutput, _ = filepath.Abs(wantOutput)
			}
			if got.OutJSPath != wantOutput {
				t.Errorf("OutJSPath: got %q, want %q", got.OutJSPath, wantOutput)
			}

			wantDeps := tt.wantConfig.OutDepsPath
			if wantDeps != "" {
				wantDeps, _ = filepath.Abs(wantDeps)
			}
			if got.OutDepsPath != wantDeps {
				t.Errorf("OutDepsPath: got %q, want %q", got.OutDepsPath, wantDeps)
			}

			wantMap := tt.wantConfig.OutMapPath
			if wantMap != "" {
				wantMap, _ = filepath.Abs(wantMap)
			}
			if got.OutMapPath != wantMap {
				t.Errorf("OutMapPath: got %q, want %q", got.OutMapPath, wantMap)
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
