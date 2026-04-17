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
	"reflect"
	"testing"

	"github.com/microsoft/typescript-go/tsccbridge"
	"github.com/szuend/tscc/internal/config"
)

func TestBuildParsedCommandLineRoundTrip(t *testing.T) {
	cfg := &config.Config{
		Target:    "es2022",
		Strict:    true,
		InputPath: "src/main.ts",
	}

	parsed, err := BuildParsedCommandLine(cfg)
	if err != nil {
		t.Fatalf("BuildParsedCommandLine returned error: %v", err)
	}

	opts := parsed.CompilerOptions()
	if opts == nil {
		t.Fatal("CompilerOptions() returned nil")
	}
	if opts.Target != tsccbridge.ScriptTargetES2022 {
		t.Errorf("Target = %v, want ScriptTargetES2022", opts.Target)
	}
	if opts.Strict != tsccbridge.TSTrue {
		t.Errorf("Strict = %v, want TSTrue", opts.Strict)
	}

	if got, want := parsed.FileNames(), []string{"src/main.ts"}; !reflect.DeepEqual(got, want) {
		t.Errorf("FileNames() = %v, want %v", got, want)
	}
}

func TestBuildParsedCommandLineUnknownTarget(t *testing.T) {
	_, err := BuildParsedCommandLine(&config.Config{Target: "es9999", InputPath: "a.ts"})
	if err == nil {
		t.Fatal("expected error for unknown target, got nil")
	}
}
