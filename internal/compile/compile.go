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
	"path/filepath"
	"strings"

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
	cfg := in.Config

	parsed, err := compileropts.BuildParsedCommandLine(cfg)
	if err != nil {
		return nil, tsccbridge.ExitStatusInvalidProject_OutputsSkipped, fmt.Errorf("build parsed command line: %w", err)
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
		return nil, tsccbridge.ExitStatusInvalidProject_OutputsSkipped, fmt.Errorf("project references are not supported (got %d)", len(refs))
	}

	host := compilehost.New(compilehost.Options{
		CurrentDirectory:   in.CurrentDirectory,
		JailedFS:           in.JailedFS,
		RawFS:              in.RawFS,
		DefaultLibraryPath: in.DefaultLibraryPath,
	})

	res := resolver.New(resolver.Options{FS: in.JailedFS, Paths: cfg.Paths})

	program := tsccbridge.NewProgram(tsccbridge.ProgramOptions{
		Config:          parsed,
		Host:            host,
		Resolver:        res,
		SingleThreaded:  tsccbridge.TSTrue,
		TypingsLocation: "",
		ProjectName:     "",
	})

	// Collect syntactic + semantic diagnostics in tsc's order.
	allDiags := tsccbridge.GetDiagnosticsOfAnyProgram(
		ctx,
		program,
		nil,
		false,
		program.GetBindDiagnostics,
		program.GetSemanticDiagnostics,
	)

	// writeFile runs serially because we pin SingleThreaded=TSTrue above;
	// emittedFiles is therefore safe to append without a mutex. If upstream
	// ever relaxes the contract, the pin would need to change first.
	//
	// Explicit outputs only (vision.md): the emitter walks the whole program
	// and offers a .js for every non-declaration source file, but we only
	// persist the emit that corresponds to cfg.InputPath. Secondary emits
	// (imported .ts files) are dropped — a separate tscc invocation compiles
	// those. --out-dts and --out-map will follow the same rule in M3/M5.
	inputStem := stripExt(cfg.InputPath)
	var emittedFiles []string
	var deferredEmits []struct{ target, text string }

	writeFile := func(fileName string, text string, data *tsccbridge.WriteFileData) error {
		if stripExt(fileName) != inputStem {
			return nil
		}
		target := ""
		switch {
		case cfg.OutJSPath != "" && isJSOutput(fileName):
			target = cfg.OutJSPath
		case cfg.OutDtsPath != "" && isDtsOutput(fileName):
			target = cfg.OutDtsPath
		}
		if target == "" {
			return nil
		}
		deferredEmits = append(deferredEmits, struct{ target, text string }{target, text})
		return nil
	}

	emitResult := program.Emit(ctx, tsccbridge.EmitOptions{WriteFile: writeFile})
	if emitResult != nil {
		allDiags = append(allDiags, emitResult.Diagnostics...)
	}

	allDiags = tsccbridge.SortAndDeduplicateDiagnostics(allDiags)

	errorCount := 0
	for _, d := range allDiags {
		if d.Category() == tsccbridge.DiagnosticCategoryError {
			errorCount++
		}
	}

	if errorCount == 0 {
		for _, e := range deferredEmits {
			if err := in.JailedFS.WriteFile(e.target, e.text); err != nil {
				return nil, tsccbridge.ExitStatusInvalidProject_OutputsSkipped, err
			}
			emittedFiles = append(emittedFiles, e.target)
		}
	}

	if in.Stderr != nil {
		useCase := in.JailedFS.UseCaseSensitiveFileNames()
		for _, d := range allDiags {
			tsccbridge.FormatDiagnostic(in.Stderr, d, in.CurrentDirectory, useCase)
		}
	}

	// Depsfile is authoritative — either trust it or re-run. Writing a partial
	// list on a failed compile would wedge the build system into skipping
	// rebuilds (design §"Non-goals"). Only emit when the compile fully
	// succeeded: emit not skipped AND no errors.
	emitSkipped := emitResult != nil && emitResult.EmitSkipped
	if cfg.OutDepsPath != "" && errorCount == 0 && !emitSkipped {
		// Without --out-js we don't emit JS at all, so the depsfile itself is
		// the only artifact this rule produces — use its path as the Make
		// target. See design §"Target name" for the full precedence story as
		// --out-dts / --out-map land.
		target := cfg.OutJSPath
		if target == "" {
			target = cfg.OutDtsPath
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
			return nil, tsccbridge.ExitStatusInvalidProject_OutputsSkipped, fmt.Errorf("render depsfile: %w", err)
		}
		if err := in.JailedFS.WriteFile(cfg.OutDepsPath, buf.String()); err != nil {
			return nil, tsccbridge.ExitStatusInvalidProject_OutputsSkipped, fmt.Errorf("write depsfile: %w", err)
		}
	}

	status := tsccbridge.ExitStatusSuccess
	switch {
	case emitResult != nil && emitResult.EmitSkipped && errorCount > 0:
		status = tsccbridge.ExitStatusDiagnosticsPresent_OutputsSkipped
	case errorCount > 0:
		status = tsccbridge.ExitStatusDiagnosticsPresent_OutputsGenerated
	}

	return &Result{EmittedFiles: emittedFiles, Diagnostics: allDiags}, status, nil
}

// isJSOutput matches the JS emit variants eligible for --out-js. .jsx is
// deliberately excluded — emit produces at most one JS file per compile, and
// matching .jsx too would clobber output under a future config that emits
// both. Declaration and source-map outputs are dropped entirely until their
// flags (--out-dts, --out-map) land.
func isJSOutput(name string) bool {
	switch filepath.Ext(name) {
	case ".js", ".mjs", ".cjs":
		return true
	}
	return false
}

func isDtsOutput(name string) bool {
	switch {
	case strings.HasSuffix(name, ".d.ts"):
		return true
	case strings.HasSuffix(name, ".d.mts"):
		return true
	case strings.HasSuffix(name, ".d.cts"):
		return true
	}
	return false
}

// stripExt removes the final extension from p, yielding the "stem" used to
// match a primary emit against its input file. Comparing stems treats
// /abs/a.ts and /abs/a.js as the same file.
func stripExt(p string) string {
	switch {
	case strings.HasSuffix(p, ".d.ts"):
		return strings.TrimSuffix(p, ".d.ts")
	case strings.HasSuffix(p, ".d.mts"):
		return strings.TrimSuffix(p, ".d.mts")
	case strings.HasSuffix(p, ".d.cts"):
		return strings.TrimSuffix(p, ".d.cts")
	}
	return strings.TrimSuffix(p, filepath.Ext(p))
}
