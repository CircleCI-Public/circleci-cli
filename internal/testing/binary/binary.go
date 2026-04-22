// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

// Package binary builds the circleci CLI binary once per test run and provides
// helpers for running it in tests.
package binary

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

var (
	once       sync.Once
	binaryPath string
	buildErr   error
)

// BuildBinary compiles the CLI binary once and returns its path.
// Call from TestMain; on error, the binary could not be built and tests
// should be skipped rather than failed.
func BuildBinary() (string, error) {
	once.Do(func() {
		dir, err := os.MkdirTemp("", "circleci-cli-test-*")
		if err != nil {
			buildErr = fmt.Errorf("create temp dir: %w", err)
			return
		}
		binaryPath = filepath.Join(dir, "circleci")
		if runtime.GOOS == "windows" {
			binaryPath += ".exe"
		}

		// acceptance/ is one level below the module root.
		repoRoot, err := filepath.Abs(filepath.Join("..", ""))
		if err != nil {
			buildErr = fmt.Errorf("resolve repo root: %w", err)
			return
		}

		cmd := exec.Command("go", "build", "-o", binaryPath, ".")
		cmd.Dir = repoRoot
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("go build failed: %w\nstderr: %s", err, stderr.String())
			return
		}
	})
	return binaryPath, buildErr
}

// CLIResult holds the output of a CLI invocation.
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// RunCLI executes the circleci binary with the given args, env, and working directory.
func RunCLI(t *testing.T, args []string, env []string, workDir string) CLIResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = workDir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run CLI: %v", err)
		}
	}

	return CLIResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}
