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
				Target:             "es2025",
				InputPath:          "input.ts",
				OutJSPath:          "",
				CaseSensitivePaths: true,
			},
		},
		{
			name: "case-sensitive-paths opt-out",
			args: []string{"--case-sensitive-paths=false", "in.ts"},
			wantConfig: &Config{
				Strict:             true,
				Target:             "es2025",
				InputPath:          "in.ts",
				CaseSensitivePaths: false,
			},
		},
		{
			name: "override flags",
			args: []string{"--target", "es2015", "--strict=false", "foo.ts"},
			wantConfig: &Config{
				Strict:             false,
				Target:             "es2015",
				InputPath:          "foo.ts",
				OutJSPath:          "",
				CaseSensitivePaths: true,
			},
		},
		{
			name: "negated boolean flag",
			args: []string{"--no-strict", "bar.ts"},
			wantConfig: &Config{
				Strict:             false,
				Target:             "es2025",
				InputPath:          "bar.ts",
				OutJSPath:          "",
				CaseSensitivePaths: true,
			},
		},
		{
			name: "output flag",
			args: []string{"-o", "out.js", "in.ts"},
			wantConfig: &Config{
				Strict:             true,
				Target:             "es2025",
				InputPath:          "in.ts",
				OutJSPath:          "out.js",
				CaseSensitivePaths: true,
			},
		},
		{
			name: "output flag long",
			args: []string{"--out-js", "dist/bundle.js", "src/main.ts"},
			wantConfig: &Config{
				Strict:             true,
				Target:             "es2025",
				InputPath:          "src/main.ts",
				OutJSPath:          "dist/bundle.js",
				CaseSensitivePaths: true,
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
			if got.Target != tt.wantConfig.Target {
				t.Errorf("Target: got %q, want %q", got.Target, tt.wantConfig.Target)
			}
			if got.CaseSensitivePaths != tt.wantConfig.CaseSensitivePaths {
				t.Errorf("CaseSensitivePaths: got %v, want %v", got.CaseSensitivePaths, tt.wantConfig.CaseSensitivePaths)
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
		})
	}
}

