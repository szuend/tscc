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

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func readBaseline(suiteName, caseName, ext string, variantName string) string {
	caseBase := filepath.Base(caseName)
	var names []string
	if variantName != "" {
		names = append(names, fmt.Sprintf("%s(%s)%s", caseBase, variantName, ext))
	}
	names = append(names, caseBase+ext)

	for _, name := range names {
		paths := []string{
			filepath.Join("third_party", "typescript-go", "testdata", "baselines", "reference", "submodule", suiteName, name),
			filepath.Join("third_party", "typescript-go", "testdata", "baselines", "reference", suiteName, name),
			filepath.Join("third_party", "typescript-go", "_submodules", "TypeScript", "tests", "baselines", "reference", name),
		}

		for _, p := range paths {
			if data, err := os.ReadFile(p); err == nil {
				return string(data)
			}
		}
	}

	return ""
}
