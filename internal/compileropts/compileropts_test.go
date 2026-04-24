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

package compileropts

import (
	"testing"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/config"
)

func TestFromConfigTarget(t *testing.T) {
	tests := []struct {
		in   string
		want tsccbridge.ScriptTarget
	}{
		{"es6", tsccbridge.ScriptTargetES2015},
		{"es2015", tsccbridge.ScriptTargetES2015},
		{"es2016", tsccbridge.ScriptTargetES2016},
		{"es2017", tsccbridge.ScriptTargetES2017},
		{"es2018", tsccbridge.ScriptTargetES2018},
		{"es2019", tsccbridge.ScriptTargetES2019},
		{"es2020", tsccbridge.ScriptTargetES2020},
		{"es2021", tsccbridge.ScriptTargetES2021},
		{"es2022", tsccbridge.ScriptTargetES2022},
		{"es2023", tsccbridge.ScriptTargetES2023},
		{"es2024", tsccbridge.ScriptTargetES2024},
		{"es2025", tsccbridge.ScriptTargetES2025},
		{"esnext", tsccbridge.ScriptTargetESNext},
		{"ES2022", tsccbridge.ScriptTargetES2022},
		{"ESNext", tsccbridge.ScriptTargetESNext},
		{"EsNext", tsccbridge.ScriptTargetESNext},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := FromConfig(&config.Config{Target: tc.in})
			if err != nil {
				t.Fatalf("FromConfig returned error: %v", err)
			}
			if got.Target != tc.want {
				t.Errorf("Target = %v, want %v", got.Target, tc.want)
			}
		})
	}
}

func TestFromConfigUnknownTarget(t *testing.T) {
	_, err := FromConfig(&config.Config{Target: "es9999"})
	if err == nil {
		t.Fatal("expected error for unknown target, got nil")
	}
}

func TestFromConfigStrict(t *testing.T) {
	tests := []struct {
		in   bool
		want tsccbridge.Tristate
	}{
		{true, tsccbridge.TSTrue},
		{false, tsccbridge.TSFalse},
	}
	for _, tc := range tests {
		got, err := FromConfig(&config.Config{Target: "es2022", Strict: tc.in})
		if err != nil {
			t.Fatalf("FromConfig returned error: %v", err)
		}
		if got.Strict != tc.want {
			t.Errorf("Strict(%v) = %v, want %v", tc.in, got.Strict, tc.want)
		}
	}
}

func TestFromConfigNoImplicitAny(t *testing.T) {
	tests := []struct {
		in   bool
		want tsccbridge.Tristate
	}{
		{true, tsccbridge.TSTrue},
		{false, tsccbridge.TSFalse},
	}
	for _, tc := range tests {
		got, err := FromConfig(&config.Config{Target: "es2022", NoImplicitAny: tc.in})
		if err != nil {
			t.Fatalf("FromConfig returned error: %v", err)
		}
		if got.NoImplicitAny != tc.want {
			t.Errorf("NoImplicitAny(%v) = %v, want %v", tc.in, got.NoImplicitAny, tc.want)
		}
	}
}

func TestFromConfigSkipLibCheck(t *testing.T) {
	tests := []struct {
		in   bool
		want tsccbridge.Tristate
	}{
		{true, tsccbridge.TSTrue},
		{false, tsccbridge.TSFalse},
	}
	for _, tc := range tests {
		got, err := FromConfig(&config.Config{Target: "es2022", SkipLibCheck: tc.in})
		if err != nil {
			t.Fatalf("FromConfig returned error: %v", err)
		}
		if got.SkipLibCheck != tc.want {
			t.Errorf("SkipLibCheck(%v) = %v, want %v", tc.in, got.SkipLibCheck, tc.want)
		}
	}
}

func TestFromConfigAllowJs(t *testing.T) {
	tests := []struct {
		in   bool
		want tsccbridge.Tristate
	}{
		{true, tsccbridge.TSTrue},
		{false, tsccbridge.TSFalse},
	}
	for _, tc := range tests {
		got, err := FromConfig(&config.Config{Target: "es2022", AllowJs: tc.in})
		if err != nil {
			t.Fatalf("FromConfig returned error: %v", err)
		}
		if got.AllowJs != tc.want {
			t.Errorf("AllowJs(%v) = %v, want %v", tc.in, got.AllowJs, tc.want)
		}
	}
}

func TestFromConfigUnsetFieldsStayZero(t *testing.T) {
	got, err := FromConfig(&config.Config{Target: "es2022"})
	if err != nil {
		t.Fatalf("FromConfig returned error: %v", err)
	}
	if got.Module != tsccbridge.ModuleKindESNext {
		t.Errorf("Module should be ESNext (default), got %v", got.Module)
	}
	if got.Declaration != tsccbridge.TSUnknown {
		t.Errorf("Declaration should be TSUnknown, got %v", got.Declaration)
	}
	if got.SourceMap != tsccbridge.TSUnknown {
		t.Errorf("SourceMap should be TSUnknown, got %v", got.SourceMap)
	}
	if got.OutFile != "" {
		t.Errorf("OutFile should be empty, got %q", got.OutFile)
	}
}

func TestFromConfigDeclaration(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want tsccbridge.Tristate
	}{
		{"unset", config.Config{Target: "es2022"}, tsccbridge.TSUnknown},
		{"set", config.Config{Target: "es2022", OutDtsPath: "a.d.ts"}, tsccbridge.TSTrue},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FromConfig(&tc.cfg)
			if err != nil {
				t.Fatalf("FromConfig returned error: %v", err)
			}
			if got.Declaration != tc.want {
				t.Errorf("Declaration = %v, want %v", got.Declaration, tc.want)
			}
		})
	}
}

func TestFromConfigSourceMap(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want tsccbridge.Tristate
	}{
		{"unset", config.Config{Target: "es2022"}, tsccbridge.TSUnknown},
		{"set", config.Config{Target: "es2022", OutMapPath: "a.js.map"}, tsccbridge.TSTrue},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FromConfig(&tc.cfg)
			if err != nil {
				t.Fatalf("FromConfig returned error: %v", err)
			}
			if got.SourceMap != tc.want {
				t.Errorf("SourceMap = %v, want %v", got.SourceMap, tc.want)
			}
		})
	}
}

func TestFromConfigModule(t *testing.T) {
	tests := []struct {
		in   string
		want tsccbridge.ModuleKind
	}{
		{"", tsccbridge.ModuleKindESNext},
		{"none", tsccbridge.ModuleKindNone},
		{"commonjs", tsccbridge.ModuleKindCommonJS},
		{"cjs", tsccbridge.ModuleKindCommonJS},
		{"amd", tsccbridge.ModuleKindAMD},
		{"umd", tsccbridge.ModuleKindUMD},
		{"system", tsccbridge.ModuleKindSystem},
		{"es2015", tsccbridge.ModuleKindES2015},
		{"es6", tsccbridge.ModuleKindES2015},
		{"es2020", tsccbridge.ModuleKindES2020},
		{"es2022", tsccbridge.ModuleKindES2022},
		{"esnext", tsccbridge.ModuleKindESNext},
		{"node16", tsccbridge.ModuleKindNode16},
		{"node18", tsccbridge.ModuleKindNode18},
		{"node20", tsccbridge.ModuleKindNode20},
		{"nodenext", tsccbridge.ModuleKindNodeNext},
		{"preserve", tsccbridge.ModuleKindPreserve},
		{"CommonJS", tsccbridge.ModuleKindCommonJS},
		{"COMMONJS", tsccbridge.ModuleKindCommonJS},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := FromConfig(&config.Config{Target: "es2022", Module: tc.in})
			if err != nil {
				t.Fatalf("FromConfig returned error: %v", err)
			}
			if got.Module != tc.want {
				t.Errorf("Module = %v, want %v", got.Module, tc.want)
			}
		})
	}
}

func TestFromConfigUnknownModule(t *testing.T) {
	_, err := FromConfig(&config.Config{Target: "es2022", Module: "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown module, got nil")
	}
}

func TestFromConfigPaths(t *testing.T) {
	cfg := &config.Config{
		Target: "es2022",
		Paths: map[string]string{
			"typescript": "/.ts/typescript.d.ts",
			"react":      "/.ts/react.d.ts",
		},
	}

	got, err := FromConfig(cfg)
	if err != nil {
		t.Fatalf("FromConfig returned error: %v", err)
	}

	if got.Paths == nil {
		t.Fatal("Paths should not be nil")
	}

	if got.Paths.Size() != 2 {
		t.Errorf("Paths.Size() = %d, want 2", got.Paths.Size())
	}

	val, ok := got.Paths.Get("typescript")
	if !ok {
		t.Error("Paths should contain 'typescript'")
	}
	if len(val) != 1 || val[0] != "/.ts/typescript.d.ts" {
		t.Errorf("Paths['typescript'] = %v, want [/.ts/typescript.d.ts]", val)
	}

	val, ok = got.Paths.Get("react")
	if !ok {
		t.Error("Paths should contain 'react'")
	}
	if len(val) != 1 || val[0] != "/.ts/react.d.ts" {
		t.Errorf("Paths['react'] = %v, want [/.ts/react.d.ts]", val)
	}
}
