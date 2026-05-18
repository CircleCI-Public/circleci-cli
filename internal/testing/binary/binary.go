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
	"testing"
	"time"

	"github.com/Netflix/go-expect"
	cpty "github.com/creack/pty"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

const osWindows = "windows"

// BuildBinary compiles the CLI binary once and returns its path.
// Call from TestMain; on error, the binary could not be built and tests
// should be skipped rather than failed. The binary is built with the
// "testfixtures" build tag so acceptance tests can use env-var-based
// stubbing hooks (e.g. CIRCLECI_SCAN_FIXTURE).
func BuildBinary() (string, func(), error) {
	return BuildBinaryOptions("circleci", filepath.Join("..", ""), "testfixtures")
}

func BuildBinaryOptions(binaryName, relativeDir string, tags ...string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "circleci-cli-test-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp dir: %w", err)

	}
	binaryPath := filepath.Join(dir, binaryName)
	if runtime.GOOS == osWindows {
		binaryPath += ".exe"
	}

	// acceptance/ is one level below the module root.
	repoRoot, err := filepath.Abs(relativeDir)
	if err != nil {
		return "", func() {}, fmt.Errorf("resolve repo root: %w", err)
	}

	args := []string{"build", "-o", binaryPath}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	args = append(args, ".")
	cmd := exec.Command("go", args...) //#nosec:G204 // fixed "go build" invocation, binaryPath is a temp file path under test control
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
	// Stdin, if non-nil, is connected to the CLI's stdin (non-TTY mode only).
	// TTY mode always uses the expect pty.
	Stdin io.Reader
}

// RunCLI executes the circleci binary with the given args, env, and working directory.
func RunCLI(t *testing.T, opts RunOpts) CLIResult {
	t.Helper()

	skip.If(t, opts.TTY && runtime.GOOS == osWindows, "No good way to manage golden TTY snapshots on Windows")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullArgs := make([]string, 0, 3+len(opts.Args))
	fullArgs = append(fullArgs, "--insecure-storage", "--theme=dark")
	if !opts.TTY && !slices.Contains(opts.Args, "--quiet") {
		fullArgs = append(fullArgs, "--debug")
	}
	fullArgs = append(fullArgs, opts.Args...)
	t.Logf("Running CLI: %s %s\n", opts.Binary, fullArgs)

	exitCode := 0
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, opts.Binary, fullArgs...) //#nosec:G204 // opts.Binary is the test-built CLI binary, fullArgs are test-controlled
	cmd.Dir = opts.WorkDir
	cmd.Env = opts.Env

	if opts.TTY {
		c, err := expect.NewConsole(
			expect.WithStdout(&stdout),
			expect.WithStdout(os.Stdout),
		)
		assert.NilError(t, err)
		defer func() {
			assert.Check(t, c.Close())
		}()

		cmd.Stdin = c.Tty()
		cmd.Stdout = c.Tty()
		cmd.Stderr = c.Tty()

		err = cmd.Run()
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
		} else {
			assert.NilError(t, err)
		}

		// go-expect holds the slave PTY open, so the master never sees EOF
		// until we explicitly close it. Close slave first, then drain.
		err = c.Tty().Close()
		assert.Check(t, err)

		_, err = c.ExpectEOF()
		assert.Check(t, err)
	} else {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
		if opts.Stdin != nil {
			cmd.Stdin = opts.Stdin
		}

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

// RunCLIInteractive executes the circleci binary attached to a PTY and returns
// both the running command and the console so callers can drive interactive
// input with Expect/Send. Call cmd.Wait() after interaction is complete.
func RunCLIInteractive(t testing.TB, opts RunOpts) *expect.Console {
	t.Helper()
	skip.If(t, runtime.GOOS == osWindows, "PTY-based interactive tests are not supported on Windows")

	c, err := expect.NewConsole(
		expect.WithStdout(os.Stdout),
		expect.WithDefaultTimeout(5*time.Second),
	)
	assert.NilError(t, err)
	t.Cleanup(func() {
		err := c.Tty().Close()
		assert.Check(t, err)

		_, err = c.ExpectEOF()
		assert.Check(t, err)

		assert.Check(t, c.Close())
	})

	// go-expect creates PTYs with a 0×0 window size by default. Bubbletea
	// won't render any content until it receives a non-zero terminal size.
	// Use 500 columns so OAuth authorize URLs (≈300 chars) are never
	// hard-wrapped, keeping the URL on a single line for easy extraction.
	err = cpty.Setsize(c.Tty(), &cpty.Winsize{
		Rows: 24,
		Cols: 500,
	})
	assert.NilError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	fullArgs := make([]string, 0, 2+len(opts.Args))
	fullArgs = append(fullArgs, "--insecure-storage", "--theme=dark")
	fullArgs = append(fullArgs, opts.Args...)
	t.Logf("Running CLI: %s %s\n", opts.Binary, fullArgs)

	cmd := exec.CommandContext(ctx, opts.Binary, fullArgs...) //#nosec:G204 // opts.Binary is the test-built CLI binary, fullArgs are test-controlled
	cmd.Dir = opts.WorkDir
	cmd.Env = opts.Env
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	err = cmd.Start()
	assert.NilError(t, err)
	t.Cleanup(func() {
		if t.Failed() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return
		}

		assert.Check(t, cmd.Wait())
	})

	return c
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
