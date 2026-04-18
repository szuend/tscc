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

package compilehost

import (
	"github.com/microsoft/typescript-go/tsccbridge"
)

// New creates a minimal CompilerHost for tscc.
// This is a placeholder that delegates to the stock typescript-go CompilerHost
// until the dual-FS jail architecture from design §7 is fully implemented.
func New(cwd string, fs tsccbridge.FS, libPath string) tsccbridge.CompilerHost {
	return tsccbridge.CreateCompilerHost(cwd, fs, libPath)
}
