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

// Package compile orchestrates a single-file TypeScript compilation. It wires
// tscc's deterministic components — hermetic FS, literal resolver, dual-FS
// CompilerHost — into typescript-go's Program and Emit pipeline.
package compile

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/compilehost"
	"github.com/szuend/tscc/internal/compileropts"
	"github.com/szuend/tscc/internal/config"
	"github.com/szuend/tscc/internal/depsfile"
	"github.com/szuend/tscc/internal/resolver"
)

// Inputs bundle everything Compile needs beyond the CLI config. Callers wire
// the FS stack; Compile is agnostic about whether discovery is jailed or
// unrestricted. Production callers wrap OS FS in hermeticfs + bundled.WrapFS
// per design §6/§7; unit tests can pass an unwrapped OS/in-memory FS.
type Inputs struct {
	Config             *config.Config
	JailedFS           tsccbridge.FS
	RawFS              tsccbridge.FS
	DefaultLibraryPath string
	CurrentDirectory   string
	// Stderr receives formatted diagnostics; may be nil to suppress output.
	Stderr io.Writer
}

// Result reports what was emitted and any diagnostics surfaced during compile.
type Result struct {
	EmittedFiles []string
	Diagnostics  []*tsccbridge.Diagnostic
}

// Compile runs a single-file compile, returning an ExitStatus that mirrors
// tsc's exit codes.
func Compile(ctx context.Context, in Inputs) (*Result, tsccbridge.ExitStatus, error) {
	parsed, err := prepareOptions(in.Config)
	if err != nil {
		return nil, tsccbridge.ExitStatusInvalidProject_OutputsSkipped, err
	}

	program := createProgram(in, parsed)

	allDiags := analyzeProgram(ctx, program)

	// Explicit outputs only (vision.md): the emitter walks the whole program
	// and offers a file for every non-declaration source file, but we only
	// persist the emit that corresponds to cfg.InputPath.
	emitter := newEmitter(in.Config, in.JailedFS)
	emitResult := program.Emit(ctx, tsccbridge.EmitOptions{WriteFile: emitter.WriteFile})
	if emitResult != nil {
		allDiags = append(allDiags, emitResult.Diagnostics...)
	}

	allDiags = tsccbridge.SortAndDeduplicateDiagnostics(allDiags)
	errorCount := countErrors(allDiags)

	var emittedFiles []string
	if errorCount == 0 {
		emittedFiles, err = emitter.Commit()
		if err != nil {
			return nil, tsccbridge.ExitStatusInvalidProject_OutputsSkipped, err
		}
	}

	printDiagnostics(in, allDiags)

	if err := generateDepsfile(in, program, emitResult, errorCount); err != nil {
		return nil, tsccbridge.ExitStatusInvalidProject_OutputsSkipped, err
	}

	return &Result{EmittedFiles: emittedFiles, Diagnostics: allDiags}, computeExitStatus(emitResult, errorCount), nil
}

func prepareOptions(cfg *config.Config) (*tsccbridge.ParsedCommandLine, error) {
	parsed, err := compileropts.BuildParsedCommandLine(cfg)
	if err != nil {
		return nil, fmt.Errorf("build parsed command line: %w", err)
	}

	// Design §8: pin compiler options so the caller cannot steer typescript-go
	// into inferring configuration from the environment.
	opts := parsed.CompilerOptions()
	opts.Types = []string{}
	opts.TypeRoots = []string{}

	// ProjectReferences must be nil (design §8). tscc has no flag that populates
	// them, but if one is added in future the assertion fires rather than
	// silently handing the resolver a non-empty list it cannot process.
	if refs := parsed.ProjectReferences(); len(refs) > 0 {
		return nil, fmt.Errorf("project references are not supported (got %d)", len(refs))
	}

	return parsed, nil
}

func createProgram(in Inputs, parsed *tsccbridge.ParsedCommandLine) *tsccbridge.Program {
	host := compilehost.New(compilehost.Options{
		CurrentDirectory:   in.CurrentDirectory,
		JailedFS:           in.JailedFS,
		RawFS:              in.RawFS,
		DefaultLibraryPath: in.DefaultLibraryPath,
	})

	res := resolver.New(resolver.Options{FS: in.JailedFS, Paths: in.Config.Paths})

	return tsccbridge.NewProgram(tsccbridge.ProgramOptions{
		Config:          parsed,
		Host:            host,
		Resolver:        res,
		SingleThreaded:  tsccbridge.TSTrue,
		TypingsLocation: "",
		ProjectName:     "",
	})
}

func analyzeProgram(ctx context.Context, program *tsccbridge.Program) []*tsccbridge.Diagnostic {
	// Collect syntactic + semantic diagnostics in tsc's order.
	return tsccbridge.GetDiagnosticsOfAnyProgram(
		ctx,
		program,
		nil,
		false,
		program.GetBindDiagnostics,
		program.GetSemanticDiagnostics,
	)
}

func countErrors(diags []*tsccbridge.Diagnostic) int {
	count := 0
	for _, d := range diags {
		if d.Category() == tsccbridge.DiagnosticCategoryError {
			count++
		}
	}
	return count
}

func printDiagnostics(in Inputs, diags []*tsccbridge.Diagnostic) {
	if in.Stderr == nil {
		return
	}
	useCase := in.JailedFS.UseCaseSensitiveFileNames()
	for _, d := range diags {
		tsccbridge.FormatDiagnostic(in.Stderr, d, in.CurrentDirectory, useCase)
	}
}

func generateDepsfile(in Inputs, program *tsccbridge.Program, emitResult *tsccbridge.EmitResult, errorCount int) error {
	cfg := in.Config
	emitSkipped := emitResult != nil && emitResult.EmitSkipped

	// Depsfile is authoritative — either trust it or re-run. Writing a partial
	// list on a failed compile would wedge the build system into skipping
	// rebuilds (design §"Non-goals"). Only emit when the compile fully
	// succeeded: emit not skipped AND no errors.
	if cfg.OutDepsPath == "" || errorCount > 0 || emitSkipped {
		return nil
	}

	// Without --out-js we don't emit JS at all, so the depsfile itself is
	// the only artifact this rule produces — use its path as the Make
	// target. See design §"Target name" for the full precedence story as
	// --out-dts / --out-map land.
	target := cfg.OutJSPath
	if target == "" {
		target = cfg.OutDtsPath
	}
	if target == "" {
		target = cfg.OutMapPath
	}
	if target == "" {
		target = cfg.OutDepsPath
	}

	inputs := make([]string, 0, len(program.SourceFiles()))
	for _, sf := range program.SourceFiles() {
		name := sf.FileName()
		if tsccbridge.IsBundled(name) {
			continue
		}
		inputs = append(inputs, name)
	}

	var buf bytes.Buffer
	if err := depsfile.Write(&buf, target, inputs); err != nil {
		return fmt.Errorf("render depsfile: %w", err)
	}
	if err := in.JailedFS.WriteFile(cfg.OutDepsPath, buf.String()); err != nil {
		return fmt.Errorf("write depsfile: %w", err)
	}
	return nil
}

func computeExitStatus(emitResult *tsccbridge.EmitResult, errorCount int) tsccbridge.ExitStatus {
	switch {
	case emitResult != nil && emitResult.EmitSkipped && errorCount > 0:
		return tsccbridge.ExitStatusDiagnosticsPresent_OutputsSkipped
	case errorCount > 0:
		return tsccbridge.ExitStatusDiagnosticsPresent_OutputsGenerated
	default:
		return tsccbridge.ExitStatusSuccess
	}
}
