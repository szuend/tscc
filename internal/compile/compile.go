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

package compile

import (
	"context"
	"fmt"
	"os"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/compileropts"
	"github.com/szuend/tscc/internal/config"
)

type Result struct {
	EmittedFiles []string
	// Later: diagnostics, exit status
}

// Compile is the main entry point for compilation. It builds the ParsedCommandLine,
// pins deterministic compiler options, creates the Program, and emits outputs.
func Compile(ctx context.Context, cfg *config.Config, host tsccbridge.CompilerHost) (*Result, error) {
	parsed, err := compileropts.BuildParsedCommandLine(cfg)
	if err != nil {
		return nil, fmt.Errorf("build parsed command line: %w", err)
	}

	opts := parsed.CompilerOptions()
	opts.Types = []string{}
	opts.TypeRoots = []string{}
	opts.Module = tsccbridge.ModuleKindESNext
	opts.SingleThreaded = tsccbridge.TSTrue

	program := tsccbridge.NewProgram(tsccbridge.ProgramOptions{
		Config: parsed,
		Host:   host,
	})

	var emittedFiles []string
	writeFile := func(fileName string, text string, data *tsccbridge.WriteFileData) error {
		// MVP: just write directly using os.WriteFile
		if err := os.WriteFile(fileName, []byte(text), 0644); err != nil {
			return err
		}
		emittedFiles = append(emittedFiles, fileName)
		return nil
	}

	program.Emit(ctx, tsccbridge.EmitOptions{
		WriteFile: writeFile,
	})

	return &Result{
		EmittedFiles: emittedFiles,
	}, nil
}
