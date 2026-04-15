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
				AlwaysStrict: true,
				Target:       "esnext",
				InputPath:    "input.ts",
				OutputPath:   "",
			},
		},
		{
			name: "override flags",
			args: []string{"--target", "es2015", "--always-strict=false", "foo.ts"},
			wantConfig: &Config{
				AlwaysStrict: false,
				Target:       "es2015",
				InputPath:    "foo.ts",
				OutputPath:   "",
			},
		},
		{
			name: "negated boolean flag",
			args: []string{"--no-always-strict", "bar.ts"},
			wantConfig: &Config{
				AlwaysStrict: false,
				Target:       "esnext",
				InputPath:    "bar.ts",
				OutputPath:   "",
			},
		},
		{
			name: "output flag",
			args: []string{"-o", "out.js", "in.ts"},
			wantConfig: &Config{
				AlwaysStrict: true,
				Target:       "esnext",
				InputPath:    "in.ts",
				OutputPath:   "out.js",
			},
		},
		{
			name: "output flag long",
			args: []string{"--output", "dist/bundle.js", "src/main.ts"},
			wantConfig: &Config{
				AlwaysStrict: true,
				Target:       "esnext",
				InputPath:    "src/main.ts",
				OutputPath:   "dist/bundle.js",
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

			if got.AlwaysStrict != tt.wantConfig.AlwaysStrict {
				t.Errorf("AlwaysStrict: got %v, want %v", got.AlwaysStrict, tt.wantConfig.AlwaysStrict)
			}
			if got.Target != tt.wantConfig.Target {
				t.Errorf("Target: got %q, want %q", got.Target, tt.wantConfig.Target)
			}
			if got.InputPath != tt.wantConfig.InputPath {
				t.Errorf("InputPath: got %q, want %q", got.InputPath, tt.wantConfig.InputPath)
			}
			if got.OutputPath != tt.wantConfig.OutputPath {
				t.Errorf("OutputPath: got %q, want %q", got.OutputPath, tt.wantConfig.OutputPath)
			}
		})
	}
}
