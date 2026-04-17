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
	"io"
	"os"
	"time"

	"github.com/microsoft/typescript-go/tsccbridge"
)

// stubSys satisfies tsccbridge.System with every method printing its name
// and exiting. Running tscc surfaces, one call at a time, the methods the
// typescript-go compiler actually needs — driving incremental implementation.
type stubSys struct {
	cwd string
}

func newStubSys() (*stubSys, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("determine current directory: %w", err)
	}
	return &stubSys{cwd: cwd}, nil
}

func (s *stubSys) unimplemented(method string) {
	fmt.Fprintf(os.Stderr, "tscc: System.%s not yet implemented\n", method)
	os.Exit(1)
}

func (s *stubSys) Writer() io.Writer {
	s.unimplemented("Writer")
	return nil
}

func (s *stubSys) FS() tsccbridge.FS {
	return tsccbridge.OSFS()
}

func (s *stubSys) DefaultLibraryPath() string {
	return tsccbridge.DefaultLibPath()
}

func (s *stubSys) GetCurrentDirectory() string {
	return s.cwd
}

func (s *stubSys) WriteOutputIsTTY() bool {
	s.unimplemented("WriteOutputIsTTY")
	return false
}

func (s *stubSys) GetWidthOfTerminal() int {
	s.unimplemented("GetWidthOfTerminal")
	return 0
}

func (s *stubSys) GetEnvironmentVariable(name string) string {
	s.unimplemented("GetEnvironmentVariable")
	return ""
}

func (s *stubSys) Now() time.Time {
	s.unimplemented("Now")
	return time.Time{}
}

func (s *stubSys) SinceStart() time.Duration {
	s.unimplemented("SinceStart")
	return 0
}
