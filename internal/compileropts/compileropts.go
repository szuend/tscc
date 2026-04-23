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

// Package compileropts maps a *config.Config into a typescript-go
// CompilerOptions. Only the fields currently present on Config are mapped;
// everything else is left at its zero value and grows alongside Config.
package compileropts

import (
	"fmt"
	"strings"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/config"
)

var targetByName = map[string]tsccbridge.ScriptTarget{
	"es6":    tsccbridge.ScriptTargetES2015,
	"es2015": tsccbridge.ScriptTargetES2015,
	"es2016": tsccbridge.ScriptTargetES2016,
	"es2017": tsccbridge.ScriptTargetES2017,
	"es2018": tsccbridge.ScriptTargetES2018,
	"es2019": tsccbridge.ScriptTargetES2019,
	"es2020": tsccbridge.ScriptTargetES2020,
	"es2021": tsccbridge.ScriptTargetES2021,
	"es2022": tsccbridge.ScriptTargetES2022,
	"es2023": tsccbridge.ScriptTargetES2023,
	"es2024": tsccbridge.ScriptTargetES2024,
	"es2025": tsccbridge.ScriptTargetES2025,
	"esnext": tsccbridge.ScriptTargetESNext,
}

var moduleByName = map[string]tsccbridge.ModuleKind{
	"none":     tsccbridge.ModuleKindNone,
	"commonjs": tsccbridge.ModuleKindCommonJS,
	"cjs":      tsccbridge.ModuleKindCommonJS,
	"amd":      tsccbridge.ModuleKindAMD,
	"umd":      tsccbridge.ModuleKindUMD,
	"system":   tsccbridge.ModuleKindSystem,
	"es2015":   tsccbridge.ModuleKindES2015,
	"es6":      tsccbridge.ModuleKindES2015,
	"es2020":   tsccbridge.ModuleKindES2020,
	"es2022":   tsccbridge.ModuleKindES2022,
	"esnext":   tsccbridge.ModuleKindESNext,
	"node16":   tsccbridge.ModuleKindNode16,
	"node18":   tsccbridge.ModuleKindNode18,
	"node20":   tsccbridge.ModuleKindNode20,
	"nodenext": tsccbridge.ModuleKindNodeNext,
	"preserve": tsccbridge.ModuleKindPreserve,
}

// FromConfig converts cfg into a *tsccbridge.CompilerOptions. The returned
// options have Target, Strict, and Module set from cfg; every other field keeps its
// zero value. An unknown --target or --module returns an error.
func FromConfig(cfg *config.Config) (*tsccbridge.CompilerOptions, error) {
	target, ok := targetByName[strings.ToLower(cfg.Target)]
	if !ok {
		return nil, fmt.Errorf("unknown target: %q", cfg.Target)
	}

	mod := tsccbridge.ModuleKindESNext // default preserves current behavior
	if cfg.Module != "" {
		m, ok := moduleByName[strings.ToLower(cfg.Module)]
		if !ok {
			return nil, fmt.Errorf("unknown module: %q", cfg.Module)
		}
		mod = m
	}

	decl := tsccbridge.TSUnknown
	if cfg.OutDtsPath != "" {
		decl = tsccbridge.TSTrue
	}

	srcMap := tsccbridge.TSUnknown
	if cfg.OutMapPath != "" {
		srcMap = tsccbridge.TSTrue
	}

	return &tsccbridge.CompilerOptions{
		Target:                     target,
		Strict:                     boolToTristate(cfg.Strict),
		NoImplicitAny:              boolToTristate(cfg.NoImplicitAny),
		StrictNullChecks:           boolToTristate(cfg.StrictNullChecks),
		ExactOptionalPropertyTypes: boolToTristate(cfg.ExactOptionalPropertyTypes),
		Module:                     mod,
		Declaration:                decl,
		SourceMap:                  srcMap,
		Lib:                        cfg.Lib,
	}, nil
}

func boolToTristate(b bool) tsccbridge.Tristate {
	if b {
		return tsccbridge.TSTrue
	}
	return tsccbridge.TSFalse
}
