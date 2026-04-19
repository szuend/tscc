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
	"path/filepath"
	"strings"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/config"
)

type emitter struct {
	cfg       *config.Config
	inputStem string
	fs        tsccbridge.FS
	deferred  []deferredEmit
}

type deferredEmit struct {
	target string
	text   string
}

func newEmitter(cfg *config.Config, fs tsccbridge.FS) *emitter {
	return &emitter{
		cfg:       cfg,
		inputStem: stripExt(cfg.InputPath),
		fs:        fs,
	}
}

// WriteFile is the callback provided to typescript-go's Emit function.
// It filters the emitted files, keeping only the primary outputs that match the target paths.
func (e *emitter) WriteFile(fileName string, text string, data *tsccbridge.WriteFileData) error {
	if stripExt(fileName) != e.inputStem {
		return nil
	}
	target := ""
	switch {
	case e.cfg.OutJSPath != "" && isJSOutput(fileName):
		target = e.cfg.OutJSPath
	case e.cfg.OutDtsPath != "" && isDtsOutput(fileName):
		target = e.cfg.OutDtsPath
	}
	if target == "" {
		return nil
	}
	e.deferred = append(e.deferred, deferredEmit{target, text})
	return nil
}

// Commit writes the deferred emitted files to the filesystem and returns their paths.
func (e *emitter) Commit() ([]string, error) {
	var emittedFiles []string
	for _, def := range e.deferred {
		if err := e.fs.WriteFile(def.target, def.text); err != nil {
			return nil, err
		}
		emittedFiles = append(emittedFiles, def.target)
	}
	return emittedFiles, nil
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
