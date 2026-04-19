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
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/config"
	"github.com/szuend/tscc/internal/hermeticfs"
)

// productionFS returns the hermetic FS stack the main binary builds (design
// §6 + §7). Tests go through the same stack so integration behavior matches
// what a user actually sees.
func productionFS() (jailed, raw tsccbridge.FS) {
	jailed = tsccbridge.BundledWrapFS(hermeticfs.New(hermeticfs.Options{
		Inner:              tsccbridge.OSFS(),
		CaseSensitivePaths: true,
	}))
	raw = tsccbridge.BundledWrapFS(tsccbridge.OSFS())
	return
}

// writeFile drops content at rel under dir, creating parent dirs as needed.
func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// runCompile invokes Compile with the production FS stack and returns status,
// stderr, and the contents of cfg.OutJSPath (empty if nothing was emitted).
func runCompile(t *testing.T, cfg *config.Config) (status tsccbridge.ExitStatus, stderr, output string) {
	t.Helper()
	jailed, raw := productionFS()
	var buf bytes.Buffer
	_, s, err := Compile(context.Background(), Inputs{
		Config:             cfg,
		JailedFS:           jailed,
		RawFS:              raw,
		DefaultLibraryPath: tsccbridge.DefaultLibPath(),
		CurrentDirectory:   filepath.Dir(cfg.InputPath),
		Stderr:             &buf,
	})
	if err != nil {
		t.Fatalf("Compile error: %v", err)
	}
	if cfg.OutJSPath != "" {
		if b, err := os.ReadFile(cfg.OutJSPath); err == nil {
			output = string(b)
		}
	}
	return s, buf.String(), output
}

// defaultCfg returns a Config with every field a caller must otherwise pass.
// Tests override specific fields inline.
func defaultCfg(input, output string) *config.Config {
	return &config.Config{
		InputPath:          input,
		OutJSPath:          output,
		Target:             "es2022",
		Strict:             true,
		CaseSensitivePaths: true,
	}
}

func TestCompile_SingleFile(t *testing.T) {
	dir := t.TempDir()
	in := writeFile(t, dir, "a.ts", "export const x: number = 42;")
	out := filepath.Join(dir, "a.js")

	status, stderr, emitted := runCompile(t, defaultCfg(in, out))
	if status != tsccbridge.ExitStatusSuccess {
		t.Fatalf("status: got %d, want success; stderr:\n%s", status, stderr)
	}
	if !strings.Contains(emitted, "export const x = 42;") {
		t.Errorf("emitted output missing expected declaration: %q", emitted)
	}
}

func TestCompile_RelativeImportResolvesViaLiteralResolver(t *testing.T) {
	// .js specifier must substitute to .ts under literal resolution (§3).
	// Explicit outputs only: --out-js names the primary output; the imported
	// module is walked but no b.js leaks to disk.
	dir := t.TempDir()
	in := writeFile(t, dir, "a.ts", `import { x } from "./b.js";
export const y: number = x + 1;`)
	writeFile(t, dir, "b.ts", "export const x: number = 41;")
	out := filepath.Join(dir, "a.js")

	status, stderr, emitted := runCompile(t, defaultCfg(in, out))
	if status != tsccbridge.ExitStatusSuccess {
		t.Fatalf("status: got %d, want success; stderr:\n%s", status, stderr)
	}
	if !strings.Contains(emitted, `from "./b.js"`) {
		t.Errorf(`a.js missing import "./b.js": %q`, emitted)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.js")); !os.IsNotExist(err) {
		t.Errorf("b.js must not exist without --out-js for it; stat err: %v", err)
	}
}

func TestCompile_WithoutOutJS_NoJSWritten(t *testing.T) {
	// Explicit outputs only: a type-check-only invocation (no --out-js) must
	// not clutter the source tree with default-path .js files.
	dir := t.TempDir()
	in := writeFile(t, dir, "a.ts", "export const x: number = 42;")

	status, stderr, _ := runCompile(t, defaultCfg(in, ""))
	if status != tsccbridge.ExitStatusSuccess {
		t.Fatalf("status: got %d, want success; stderr:\n%s", status, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.js")); !os.IsNotExist(err) {
		t.Errorf("a.js must not exist without --out-js; stat err: %v", err)
	}
}

func TestCompile_BareSpecifierDeniedWithoutPathMap(t *testing.T) {
	dir := t.TempDir()
	in := writeFile(t, dir, "a.ts", `import "lodash";`)
	out := filepath.Join(dir, "a.js")

	status, stderr, _ := runCompile(t, defaultCfg(in, out))
	if status == tsccbridge.ExitStatusSuccess {
		t.Fatalf("expected failure for unmapped bare specifier; got success")
	}
	if !strings.Contains(stderr, "Cannot find module") {
		t.Errorf("expected 'Cannot find module' in stderr; got %q", stderr)
	}
}

func TestCompile_PathMapUnlocksBareImport_AcrossJailedDir(t *testing.T) {
	// Dual purpose: proves --path threads through ProgramOptions.Resolver
	// AND that the dual-FS split works. The mapped target lives inside
	// node_modules/, which the jail blocks from discovery — so only the
	// raw FS (GetSourceFile) can read it (design §7).
	dir := t.TempDir()
	in := writeFile(t, dir, "src/a.ts", `import x from "lodash";
export const y: string = x.foo;`)
	dts := writeFile(t, dir, "node_modules/lodash/index.d.ts",
		"declare const x: { foo: string }; export = x;")
	out := filepath.Join(dir, "a.js")

	cfg := defaultCfg(in, out)
	cfg.Paths = map[string]string{"lodash": dts}

	status, stderr, _ := runCompile(t, cfg)
	if status != tsccbridge.ExitStatusSuccess {
		t.Fatalf("status: got %d, want success; stderr:\n%s", status, stderr)
	}
}

func TestCompile_JailBlocksPackageJsonImport(t *testing.T) {
	// Explicit ./package.json import is blocked even though the file exists,
	// because the jail (design §6) denies any probe of package.json.
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"test","type":"module"}`)
	in := writeFile(t, dir, "a.ts", `import pkg from "./package.json";
export const n = pkg;`)
	out := filepath.Join(dir, "a.js")

	status, stderr, _ := runCompile(t, defaultCfg(in, out))
	if status == tsccbridge.ExitStatusSuccess {
		t.Fatalf("expected failure; package.json must be invisible to resolution")
	}
	if !strings.Contains(stderr, "Cannot find module") {
		t.Errorf("expected 'Cannot find module' in stderr; got %q", stderr)
	}
}
