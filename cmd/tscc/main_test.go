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
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"tscc": main,
	})
}

func TestScript(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Dir(filepath.Dir(wd))
	tsDir := filepath.Join(repoRoot, "third_party", "generated", "ts")

	// Find all directories under testdata that contain .txtar files.
	testDirs := make(map[string]bool)
	err = filepath.WalkDir("testdata", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".txtar" {
			testDirs[filepath.Dir(path)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Sort directories for deterministic run order.
	var dirs []string
	for dir := range testDirs {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		testscript.Run(t, testscript.Params{
			Dir:           dir,
			UpdateScripts: os.Getenv("TSCC_UPDATE_TESTDATA") == "1",
			Setup: func(env *testscript.Env) error {
				env.Vars = append(env.Vars, "TSCC_TS_DIR="+tsDir)
				return nil
			},
		})
	}
}
