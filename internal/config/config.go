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
	"strings"

	"github.com/spf13/pflag"
)

type Config struct {
	AlwaysStrict bool
	Target       string
	InputPath    string
	OutputPath   string
}

func Parse(args []string) (*Config, error) {
	flags := pflag.NewFlagSet("tscc", pflag.ContinueOnError)

	cfg := &Config{}

	flags.BoolVar(&cfg.AlwaysStrict, "always-strict", true, "Files are parsed in ECMAScript strict mode, and emit \"use strict\" (--no-always-strict to disable)")
	flags.StringVar(&cfg.Target, "target", "es2025", "Set the JavaScript language version for emitted JavaScript (allowed: es6/es2015, es2016, es2017, es2018, es2019, es2020, es2021, es2022, es2023, es2024, es2025, esnext)")
	flags.StringVarP(&cfg.OutputPath, "output", "o", "", "Write JavaScript output to FILE")

	// Rewrite negated boolean flags e.g. --no-always-strict to --always-strict=false
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

	cfg.InputPath = remaining[0]
	return cfg, nil
}
