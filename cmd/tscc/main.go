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
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/spf13/pflag"
	"github.com/szuend/tscc/internal/compile"
	"github.com/szuend/tscc/internal/config"
	"github.com/szuend/tscc/internal/hermeticfs"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "tscc: %v\n", err)
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tscc: %v\n", err)
		os.Exit(1)
	}

	// Design §6: jailed FS blocks tsconfig/package discovery. Design §7: raw FS
	// serves already-resolved paths to GetSourceFile. Both are wrapped in
	// bundled.WrapFS so typescript-go's embedded lib.*.d.ts virtual paths
	// resolve uniformly.
	jailedFS := tsccbridge.BundledWrapFS(hermeticfs.New(hermeticfs.Options{
		Inner:              tsccbridge.OSFS(),
		CaseSensitivePaths: cfg.CaseSensitivePaths,
	}))
	rawFS := tsccbridge.BundledWrapFS(tsccbridge.OSFS())

	_, status, err := compile.Compile(context.Background(), compile.Inputs{
		Config:             cfg,
		JailedFS:           jailedFS,
		RawFS:              rawFS,
		DefaultLibraryPath: tsccbridge.DefaultLibPath(),
		CurrentDirectory:   cwd,
		Stderr:             os.Stderr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "tscc: %v\n", err)
		os.Exit(1)
	}

	os.Exit(int(status))
}
