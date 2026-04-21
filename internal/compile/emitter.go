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
	"github.com/szuend/tscc/internal/paths"
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
		inputStem: paths.StripExt(cfg.InputPath),
		fs:        fs,
	}
}

// WriteFile is the callback provided to typescript-go's Emit function.
// It filters the emitted files, keeping only the primary outputs that match the target paths.
func (e *emitter) WriteFile(fileName string, text string, data *tsccbridge.WriteFileData) error {
	if paths.StripExt(fileName) != e.inputStem {
		return nil
	}
	target := ""
	switch {
	case e.cfg.OutJSPath != "" && paths.IsJSOutput(fileName):
		target = e.cfg.OutJSPath
		if e.cfg.OutMapPath != "" && data != nil && data.SourceMapUrlPos >= 0 {
			text = rewriteSourceMappingURL(text, data.SourceMapUrlPos, sourceMappingURLFor(e.cfg))
		}
	case e.cfg.OutDtsPath != "" && paths.IsDtsOutput(fileName):
		target = e.cfg.OutDtsPath
	case e.cfg.OutMapPath != "" && paths.IsMapOutput(fileName):
		target = e.cfg.OutMapPath
		newJSName := ""
		if e.cfg.OutJSPath != "" {
			newJSName = filepath.Base(e.cfg.OutJSPath)
		}
		text = rewriteMapJSON(text, fileName, newJSName)
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
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
