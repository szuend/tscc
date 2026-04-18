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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/compilehost"
	"github.com/szuend/tscc/internal/config"
)

func TestCompile(t *testing.T) {
	tempDir := t.TempDir()

	inPath := filepath.Join(tempDir, "a.ts")
	outPath := filepath.Join(tempDir, "a.js")
	if err := os.WriteFile(inPath, []byte("export const x: number = 42;"), 0644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	cfg := &config.Config{
		InputPath: inPath,
		OutJSPath: outPath,
		Target:    "es2022",
		Strict:    true,
	}

	host := compilehost.New(tempDir, tsccbridge.OSFS(), tsccbridge.DefaultLibPath())

	res, err := Compile(context.Background(), cfg, host)
	if err != nil {
		t.Fatalf("Compile error: %v", err)
	}

	if len(res.EmittedFiles) != 1 {
		t.Errorf("expected 1 emitted file, got %d", len(res.EmittedFiles))
	}

	outBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	outStr := string(outBytes)
	if !strings.Contains(outStr, "export const x = 42;") {
		t.Errorf("unexpected output content: %q", outStr)
	}
}
