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

// Package config handles command line argument parsing into a Config object
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

type Config struct {
	Strict      bool
	Target      string
	Module      string
	InputPath   string
	OutJSPath   string
	OutDepsPath string

	// CaseSensitivePaths pins the value returned by
	// UseCaseSensitiveFileNames() on every FS the compiler observes.
	// Design §6 requires this to be caller-pinned so tspath.Path keys
	// are identical across hosts (macOS/Windows sniffers disagree).
	CaseSensitivePaths bool

	// Paths maps bare import specifier → absolute file path. Populated by
	// repeated --path NAME=/abs/path flags. Design §4: bare specifiers are
	// denied unless explicitly mapped here.
	Paths map[string]string

	// rawPaths holds the unparsed --path arguments; populated by pflag and
	// consumed in Parse to build Paths.
	rawPaths []string
}

func Parse(args []string) (*Config, error) {
	cfg := &Config{}
	groups := buildGroups(cfg)

	flags := pflag.NewFlagSet("tscc", pflag.ContinueOnError)
	for _, g := range groups {
		flags.AddFlagSet(g.Set)
	}
	flags.Usage = func() {
		printUsage(os.Stderr, groups)
	}

	// Rewrite negated boolean flags e.g. --no-strict to --strict=false
	normalizedArgs := make([]string, len(args))
	copy(normalizedArgs, args)

	for i, arg := range normalizedArgs {
		if !strings.HasPrefix(arg, "--no-") || strings.Contains(arg, "=") {
			continue
		}

		potentialFlagName := strings.TrimPrefix(arg, "--no-")
		flag := flags.Lookup(potentialFlagName)
		if flag != nil && flag.Value.Type() == "bool" {
			normalizedArgs[i] = fmt.Sprintf("--%s=false", potentialFlagName)
		}
	}

	if err := flags.Parse(normalizedArgs); err != nil {
		return nil, err
	}

	remaining := flags.Args()
	if len(remaining) == 0 {
		return nil, errors.New("no input file specified")
	}
	if len(remaining) > 1 {
		return nil, fmt.Errorf("too many input files specified: %v", remaining)
	}

	absInput, err := filepath.Abs(remaining[0])
	if err != nil {
		return nil, fmt.Errorf("resolve input path: %w", err)
	}
	cfg.InputPath = absInput

	if cfg.OutJSPath != "" {
		absOutput, err := filepath.Abs(cfg.OutJSPath)
		if err != nil {
			return nil, fmt.Errorf("resolve output path: %w", err)
		}
		cfg.OutJSPath = absOutput
	}

	if cfg.OutDepsPath != "" {
		absDeps, err := filepath.Abs(cfg.OutDepsPath)
		if err != nil {
			return nil, fmt.Errorf("resolve depsfile path: %w", err)
		}
		cfg.OutDepsPath = absDeps
	}

	// --path NAME=/abs/path ; absolute targets only, duplicates rejected.
	// Design §4 requires one-to-one mappings we can audit deterministically;
	// accepting relative targets or duplicate names reintroduces ambiguity.
	paths := make(map[string]string, len(cfg.rawPaths))
	for _, raw := range cfg.rawPaths {
		eq := strings.IndexByte(raw, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("invalid --path %q: expected NAME=/abs/path", raw)
		}
		name, target := raw[:eq], raw[eq+1:]
		if name == "" || target == "" {
			return nil, fmt.Errorf("invalid --path %q: expected NAME=/abs/path", raw)
		}
		if !filepath.IsAbs(target) {
			return nil, fmt.Errorf("--path %s: target must be absolute, got %q", name, target)
		}
		if _, dup := paths[name]; dup {
			return nil, fmt.Errorf("--path %s specified more than once", name)
		}
		paths[name] = filepath.ToSlash(target)
	}
	if len(paths) > 0 {
		cfg.Paths = paths
	}
	cfg.rawPaths = nil

	return cfg, nil
}

type flagGroup struct {
	Name string
	Set  *pflag.FlagSet
}

func buildGroups(cfg *Config) []flagGroup {
	return []flagGroup{
		languageGroup(cfg),
		typeCheckingGroup(cfg),
		resolutionGroup(cfg),
		outputGroup(cfg),
	}
}

func languageGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("language", pflag.ContinueOnError)
	g.StringVar(&cfg.Target, "target", "es2025", "Set the JavaScript language `version` for emitted JavaScript (allowed: es6/es2015, es2016, es2017, es2018, es2019, es2020, es2021, es2022, es2023, es2024, es2025, esnext)")
	g.StringVar(&cfg.Module, "module", "", "Emitted module system `KIND` (allowed: none, commonjs, amd, umd, system, es2015, es2020, es2022, esnext, node16, node18, node20, nodenext, preserve). Default: esnext.")
	return flagGroup{Name: "Language and Environment", Set: g}
}

func typeCheckingGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("type-checking", pflag.ContinueOnError)
	g.BoolVar(&cfg.Strict, "strict", true, "Enable all strict type-checking options")
	return flagGroup{Name: "Type Checking", Set: g}
}

func resolutionGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("resolution", pflag.ContinueOnError)
	g.StringArrayVar(&cfg.rawPaths, "path", nil, "Map a bare import specifier to an absolute file path: `NAME=/abs/path`. Repeat for each dependency. Bare imports without a mapping are denied.")
	g.BoolVar(&cfg.CaseSensitivePaths, "case-sensitive-paths", true, "Treat filesystem paths as case-sensitive when keying the compiler's path cache. Pin this across all hosts to avoid macOS/Windows divergence")
	return flagGroup{Name: "Module Resolution", Set: g}
}

func outputGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("output", pflag.ContinueOnError)
	g.StringVarP(&cfg.OutJSPath, "out-js", "o", "", "Write JavaScript output to `FILE`")
	g.StringVar(&cfg.OutDepsPath, "out-deps", "", "Write a Make-compatible dependency snippet for the transitive input set to `FILE`")
	return flagGroup{Name: "Output", Set: g}
}
