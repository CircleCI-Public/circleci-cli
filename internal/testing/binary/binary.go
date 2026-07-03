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

	"github.com/pete-woods/go-expect"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

const osWindows = "windows"

func Build(binaryName, relativeDir, source string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "circleci-cli-test-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp dir: %w", err)

	}
	outputPath := filepath.Join(dir, binaryName)
	if runtime.GOOS == osWindows {
		outputPath += ".exe"
	}

	// acceptance/ is one level below the module root.
	repoRoot, err := filepath.Abs(relativeDir)
	if err != nil {
		return "", func() {}, fmt.Errorf("resolve repo root: %w", err)
	}

	cmd := exec.Command("go", "build", "-o", outputPath, source) //#nosec:G204 // fixed "go build" invocation, binaryPath is a temp file path under test control
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return "", func() {}, fmt.Errorf("go build failed: %w\nstderr: %s", err, stderr.String())
	}

	return outputPath, func() {
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
			expect.WithTermSize(80, 80),
		)
		assert.NilError(t, err)
		defer func() {
			assert.Check(t, c.Close())
		}()

		err = c.Start(cmd)
		assert.NilError(t, err)

		// WaitProcess works across platforms (cmd.Wait does not work with a
		// ConPTY-attached process on Windows) and populates cmd.ProcessState.
		err = expect.WaitProcess(ctx, cmd)
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
		} else {
			assert.NilError(t, err)
			if cmd.ProcessState != nil {
				exitCode = cmd.ProcessState.ExitCode()
			}
		}

		// The console holds the PTY open, so the child exiting never produces
		// an EOF until we close it. Close, then drain.
		assert.Check(t, c.Close())

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
// the console so callers can drive interactive input with Expect/Send. Call
// cmd.Wait() after interaction is complete.
func RunCLIInteractive(t testing.TB, opts RunOpts) *expect.Console {
	t.Helper()

	// 100 columns is wide enough for the OAuth authorize URL, which is now short
	// (PAR/RFC 9126 keeps it to a client_id + request_uri, ≈80 chars, e.g.
	// https://circleci.com/oauth/authorize?request_uri=…), so it stays on a single
	// line for easy extraction. Keeping the terminal narrow also keeps full-screen
	// TUIs (the theme picker's preview, the run filter dialog) from rendering very
	// wide. (Bubbletea also needs a non-zero terminal size to render anything.)
	c, err := expect.NewConsole(
		expect.WithStdout(os.Stdout),
		expect.WithDefaultTimeout(5*time.Second),
		expect.WithTermSize(100, 80),
	)
	assert.NilError(t, err)
	t.Cleanup(func() {
		// The console holds the PTY open, so the child exiting never produces
		// an EOF until we close it. Close, then drain.
		assert.Check(t, c.Close())

		_, err := c.ExpectEOF()
		assert.Check(t, err)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	fullArgs := make([]string, 0, 2+len(opts.Args))
	fullArgs = append(fullArgs, "--insecure-storage", "--theme=dark")
	fullArgs = append(fullArgs, opts.Args...)
	t.Logf("Running CLI: %s %s\n", opts.Binary, fullArgs)

	cmd := exec.CommandContext(ctx, opts.Binary, fullArgs...) //#nosec:G204 // opts.Binary is the test-built CLI binary, fullArgs are test-controlled
	cmd.Dir = opts.WorkDir
	cmd.Env = opts.Env

	// Start attaches the command's stdin/stdout/stderr to the console's PTY on
	// all platforms (on Windows via a ConPTY pseudo console).
	err = c.Start(cmd)
	assert.NilError(t, err)
	t.Cleanup(func() {
		// WaitProcess works across platforms; cmd.Wait does not work with a
		// ConPTY-attached process on Windows.
		if t.Failed() {
			_ = cmd.Process.Kill()
			_ = expect.WaitProcess(ctx, cmd)
			return
		}

		assert.Check(t, expect.WaitProcess(ctx, cmd))
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
