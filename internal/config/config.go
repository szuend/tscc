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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

type Config struct {
	Strict                     bool
	NoImplicitAny              bool
	ExactOptionalPropertyTypes bool
	Target                     string
	Module                     string
	InputPath                  string
	OutJSPath                  string
	OutDepsPath                string
	OutDtsPath                 string
	OutMapPath                 string

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

	// Lib holds the library files to include in the compilation.
	// Populated by repeated --lib flags or comma-separated values.
	Lib []string

	// StrictNullChecks enables strict null checks.
	StrictNullChecks bool

	// SkipLibCheck skips type checking of declaration files.
	SkipLibCheck bool

	// AllowJs allows JavaScript files to be a part of your program.
	AllowJs bool

	// CheckJs enables error reporting in type-checked JavaScript files.
	CheckJs bool

	// IsolatedModules ensures that each file can be safely transpiled without relying on other imports.
	IsolatedModules bool
}

func Parse(args []string) (*Config, error) {
	expanded, err := expandResponseFiles(args)
	if err != nil {
		return nil, err
	}
	args = expanded

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

	// Apply dynamic defaults for strict-mode family flags
	if !flags.Changed("no-implicit-any") {
		cfg.NoImplicitAny = cfg.Strict
	}

	if !flags.Changed("strict-null-checks") {
		cfg.StrictNullChecks = cfg.Strict
	}

	if !flags.Changed("lib") {
		cfg.Lib = []string{cfg.Target}
	}

	if !flags.Changed("allow-js") {
		cfg.AllowJs = cfg.CheckJs
	}

	if err := cfg.Validate(flags.Args()); err != nil {
		return nil, err
	}

	return cfg, nil
}

func expandResponseFiles(args []string) ([]string, error) {
	var expanded []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			content, err := os.ReadFile(arg[1:])
			if err != nil {
				return nil, fmt.Errorf("read response file: %w", err)
			}

			scanner := bufio.NewScanner(bytes.NewReader(content))
			for scanner.Scan() {
				line := scanner.Text()
				if line != "" {
					expanded = append(expanded, line)
				}
			}
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("parse response file %s: %w", arg, err)
			}
		} else {
			expanded = append(expanded, arg)
		}
	}
	return expanded, nil
}

func (cfg *Config) Validate(args []string) error {
	if len(args) == 0 {
		return errors.New("no input file specified")
	}
	if len(args) > 1 {
		return fmt.Errorf("too many input files specified: %v", args)
	}

	absInput, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve input path: %w", err)
	}
	cfg.InputPath = absInput

	if cfg.OutJSPath != "" {
		absOutput, err := filepath.Abs(cfg.OutJSPath)
		if err != nil {
			return fmt.Errorf("resolve output path: %w", err)
		}
		cfg.OutJSPath = absOutput
	}

	if cfg.OutDepsPath != "" {
		absDeps, err := filepath.Abs(cfg.OutDepsPath)
		if err != nil {
			return fmt.Errorf("resolve depsfile path: %w", err)
		}
		cfg.OutDepsPath = absDeps
	}

	if cfg.OutDtsPath != "" {
		absDts, err := filepath.Abs(cfg.OutDtsPath)
		if err != nil {
			return fmt.Errorf("resolve dts path: %w", err)
		}
		cfg.OutDtsPath = absDts
	}

	if cfg.OutMapPath != "" {
		absMap, err := filepath.Abs(cfg.OutMapPath)
		if err != nil {
			return fmt.Errorf("resolve map path: %w", err)
		}
		cfg.OutMapPath = absMap
	}

	// --path NAME=PATH ; targets resolved to absolute, duplicates rejected.
	// Design §4 requires one-to-one mappings we can audit deterministically;
	// duplicates are rejected. Relative paths are resolved to absolute.
	paths := make(map[string]string, len(cfg.rawPaths))
	for _, raw := range cfg.rawPaths {
		eq := strings.IndexByte(raw, '=')
		if eq <= 0 {
			return fmt.Errorf("invalid --path %q: expected NAME=PATH", raw)
		}
		name, target := raw[:eq], raw[eq+1:]
		if name == "" || target == "" {
			return fmt.Errorf("invalid --path %q: expected NAME=PATH", raw)
		}
		absTarget, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("resolve path target for %s: %w", name, err)
		}
		if _, dup := paths[name]; dup {
			return fmt.Errorf("--path %s specified more than once", name)
		}
		paths[name] = filepath.ToSlash(absTarget)
	}
	if len(paths) > 0 {
		cfg.Paths = paths
	}
	cfg.rawPaths = nil

	return nil
}

type flagGroup struct {
	Name string
	Set  *pflag.FlagSet
}

func buildGroups(cfg *Config) []flagGroup {
	return []flagGroup{
		languageGroup(cfg),
		javascriptSupportGroup(cfg),
		typeCheckingGroup(cfg),
		resolutionGroup(cfg),
		completenessGroup(cfg),
		interopConstraintsGroup(cfg),
		outputGroup(cfg),
	}
}

func completenessGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("completeness", pflag.ContinueOnError)
	g.BoolVar(&cfg.SkipLibCheck, "skip-lib-check", false, "Skip type checking all .d.ts files.")
	return flagGroup{Name: "Completeness", Set: g}
}

func languageGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("language", pflag.ContinueOnError)
	g.StringVar(&cfg.Target, "target", "es2025", "Set the JavaScript language `version` for emitted JavaScript (allowed: es6/es2015, es2016, es2017, es2018, es2019, es2020, es2021, es2022, es2023, es2024, es2025, esnext)")
	g.StringVar(&cfg.Module, "module", "", "Emitted module system `KIND` (allowed: none, commonjs, amd, umd, system, es2015, es2020, es2022, esnext, node16, node18, node20, nodenext, preserve). Default: esnext.")
	g.StringSliceVar(&cfg.Lib, "lib", nil, "Specify a set of bundled library declaration files that describe the target runtime environment (allowed: es5, es6/es2015, es2016, es2017, es2018, es2019, es2020, es2021, es2022, es2023, es2024, es2025, esnext, dom, dom.iterable, dom.asynciterable, webworker, webworker.importscripts, webworker.iterable, webworker.asynciterable, scripthost, and by-feature options like es2015.core)")
	g.Lookup("lib").DefValue = "matches --target; excluding DOM"
	return flagGroup{Name: "Language and Environment", Set: g}
}

func javascriptSupportGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("javascript-support", pflag.ContinueOnError)
	g.BoolVar(&cfg.AllowJs, "allow-js", false, "Allow JavaScript files to be a part of your program. Use the 'checkJs' option to get errors from these files.")
	g.Lookup("allow-js").DefValue = "false; true if --check-js is passed"
	g.BoolVar(&cfg.CheckJs, "check-js", false, "Enable error reporting in type-checked JavaScript files.")
	return flagGroup{Name: "JavaScript Support", Set: g}
}

func typeCheckingGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("type-checking", pflag.ContinueOnError)
	g.BoolVar(&cfg.Strict, "strict", true, "Enable all strict type-checking options")
	g.BoolVar(&cfg.NoImplicitAny, "no-implicit-any", true, "Raise error on expressions and declarations with an implied 'any' type")
	g.Lookup("no-implicit-any").DefValue = "true; false if --no-strict is passed"
	g.BoolVar(&cfg.StrictNullChecks, "strict-null-checks", true, "When type checking, take into account 'null' and 'undefined'")
	g.Lookup("strict-null-checks").DefValue = "true; false if --no-strict is passed"
	g.BoolVar(&cfg.ExactOptionalPropertyTypes, "exact-optional-property-types", false, "Interpret optional property types as written, rather than adding 'undefined'")
	return flagGroup{Name: "Type Checking", Set: g}
}

func resolutionGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("resolution", pflag.ContinueOnError)
	g.StringArrayVar(&cfg.rawPaths, "path", nil, "Map a bare import specifier to a file path: `NAME=PATH`. Repeat for each dependency. Targets are resolved to absolute paths. Bare imports without a mapping are denied.")
	g.BoolVar(&cfg.CaseSensitivePaths, "case-sensitive-paths", true, "Treat filesystem paths as case-sensitive when keying the compiler's path cache. Pin this across all hosts to avoid macOS/Windows divergence")
	return flagGroup{Name: "Module Resolution", Set: g}
}

func interopConstraintsGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("interop-constraints", pflag.ContinueOnError)
	g.BoolVar(&cfg.IsolatedModules, "isolated-modules", false, "Ensure that each file can be safely transpiled without relying on other imports.")
	return flagGroup{Name: "Interop Constraints", Set: g}
}

func outputGroup(cfg *Config) flagGroup {
	g := pflag.NewFlagSet("output", pflag.ContinueOnError)
	g.StringVarP(&cfg.OutJSPath, "out-js", "o", "", "Write JavaScript output to `FILE`")
	g.StringVar(&cfg.OutDtsPath, "out-dts", "", "Write TypeScript declaration output to `FILE`.")
	g.StringVar(&cfg.OutMapPath, "out-map", "", "Write source map output to `FILE`. URL comment in emitted JS uses basename(FILE); co-locate or rewrite downstream.")
	g.StringVar(&cfg.OutDepsPath, "out-deps", "", "Write a Make-compatible dependency snippet for the transitive input set to `FILE`")
	return flagGroup{Name: "Output", Set: g}
}
