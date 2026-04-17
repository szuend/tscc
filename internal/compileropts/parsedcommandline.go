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

package compileropts

import (
	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/config"
)

// BuildParsedCommandLine bundles the CompilerOptions derived from cfg with
// the root file list and path-comparison settings that the typescript-go
// compiler needs. Case sensitivity is hard-coded to true and the current
// directory is left empty; both will be supplied by the CompilerHost in a
// later step once tscc owns the host construction.
func BuildParsedCommandLine(cfg *config.Config) (*tsccbridge.ParsedCommandLine, error) {
	opts, err := FromConfig(cfg)
	if err != nil {
		return nil, err
	}

	return tsccbridge.NewParsedCommandLine(
		opts,
		[]string{cfg.InputPath},
		tsccbridge.ComparePathsOptions{UseCaseSensitiveFileNames: true},
	), nil
}
