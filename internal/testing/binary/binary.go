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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aymanbagabas/go-pty"
	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/closer"
)

// BuildBinary compiles the CLI binary once and returns its path.
// Call from TestMain; on error, the binary could not be built and tests
// should be skipped rather than failed.
func BuildBinary() (string, func(), error) {
	dir, err := os.MkdirTemp("", "circleci-cli-test-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp dir: %w", err)

	}
	binaryPath := filepath.Join(dir, "circleci")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// acceptance/ is one level below the module root.
	repoRoot, err := filepath.Abs(filepath.Join("..", ""))
	if err != nil {
		return "", func() {}, fmt.Errorf("resolve repo root: %w", err)
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, ".") //#nosec:G204 // fixed "go build" invocation, binaryPath is a temp file path under test control
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return "", func() {}, fmt.Errorf("go build failed: %w\nstderr: %s", err, stderr.String())
	}

	return binaryPath, func() {
		_ = os.RemoveAll(dir)
	}, nil
}

// CLIResult holds the output of a CLI invocation.
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type RunOpts struct {
	Binary  string
	Args    []string
	Env     []string
	WorkDir string
	TTY     bool
}

// RunCLI executes the circleci binary with the given args, env, and working directory.
func RunCLI(t *testing.T, opts RunOpts) CLIResult {
	t.Helper()

	skip.If(t, opts.TTY && runtime.GOOS == "windows", "No good way to manage golden TTY snapshots on Windows")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullArgs := []string{
		"--insecure-storage",
		"--theme=dark",
	}
	if !opts.TTY && !slices.Contains(opts.Args, "--quiet") {
		fullArgs = append(fullArgs, "--debug")
	}
	fullArgs = append(fullArgs, opts.Args...)
	t.Logf("Running CLI: %s %s\n", opts.Binary, fullArgs)

	exitCode := 0
	var stdout, stderr bytes.Buffer

	if opts.TTY {
		p, err := pty.New()
		assert.NilError(t, err)

		cmd := p.CommandContext(ctx, opts.Binary, fullArgs...)
		cmd.Dir = opts.WorkDir
		cmd.Env = opts.Env

		err = cmd.Start()
		assert.NilError(t, err)

		// Both goroutines run concurrently to avoid deadlock:
		// - WriteTo drains output so the process never blocks on a full PTY buffer.
		// - Wait captures the exit code then closes the PTY, which unblocks WriteTo.
		var eg errgroup.Group
		eg.Go(func() (err error) {
			defer closer.ErrorHandler(p, &err)

			err = cmd.Wait()
			if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
				exitCode = exitErr.ExitCode()
				return nil
			}
			return err
		})
		eg.Go(func() (err error) {
			_, err = io.Copy(&stdout, p)
			if errors.Is(err, io.EOF) || errors.Is(err, syscall.EIO) || errors.Is(err, os.ErrClosed) {
				// Handle expected PTY close errors on Mac, Linux, Windows respectively
				return nil
			}
			return err
		})
		assert.NilError(t, eg.Wait())
	} else {
		cmd := exec.CommandContext(ctx, opts.Binary, fullArgs...) //#nosec:G204 // opts.Binary is the test-built CLI binary, fullArgs are test-controlled
		cmd.Dir = opts.WorkDir
		cmd.Env = opts.Env
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

		err := cmd.Run()
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
		} else {
			assert.NilError(t, err)
		}
	}

	return CLIResult{
		Stdout:   normalizeLocalURLs(stdout.String()),
		Stderr:   filterDebugLines(stderr.String()),
		ExitCode: exitCode,
	}
}

var localURLPortRe = regexp.MustCompile(`http://127\.0\.0\.1:\d+/`)

// normalizeLocalURLs replaces random-port localhost URLs with a fixed port so
// golden file comparisons are stable across test runs.
func normalizeLocalURLs(s string) string {
	return localURLPortRe.ReplaceAllString(s, "http://127.0.0.1:8000/")
}

// filterDebugLines removes lines beginning with "DEBU" from s.
func filterDebugLines(s string) string {
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(line, "DEBU") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
