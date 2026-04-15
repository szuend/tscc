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

//go:generate go run ../../tools/genbridge

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

func main() {
	flags := pflag.NewFlagSet("tscc", pflag.ExitOnError)

	var alwaysStrict bool

	flags.BoolVar(&alwaysStrict, "always-strict", true, "Files are parsed in ECMAScript strict mode, and emit \"use strict\" (--no-always-strict to disable)")

	args := make([]string, len(os.Args))
	copy(args, os.Args)

	// Rewrite negated boolean flags e.g. --no-always-strict to --always-strict=false
	for i, arg := range args {
		if !strings.HasPrefix(arg, "--no-") || strings.Contains(arg, "=") {
			continue
		}

		potentialFlagName := strings.TrimPrefix(arg, "--no-")
		flag := flags.Lookup(potentialFlagName)
		if flag != nil && flag.Value.Type() == "bool" {
			args[i] = fmt.Sprintf("--%s=false", potentialFlagName)
		}
	}

	_ = flags.Parse(args[1:])
	fmt.Fprintln(os.Stderr, "tscc: not yet implemented")
	os.Exit(1)
}
