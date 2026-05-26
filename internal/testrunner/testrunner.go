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

// Package testrunner runs the test command detected by reposcan/env-builder.
package testrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"

	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

const outputTailLimit = 32 * 1024

// Run executes the detected test command for dir. The command comes from a
// precomputed reposcan.Result so callers that already scanned the repo do not
// need to scan again.
func Run(ctx context.Context, dir string, result *reposcan.Result) error {
	testCommand := result.SetupCommand("test")
	if testCommand == "" {
		return clierrors.New(
			"test.no_test_command",
			"No test command detected",
			"Env-builder did not identify a test command for this project's stack.",
		).WithSuggestions(
			"Add a test target to your build configuration, such as a test script in package.json or a _test.go file",
			"Re-run after adding tests",
		).WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.Printf(ctx, "Running tests ...\n")

	tail := &tailWriter{limit: outputTailLimit}
	cmd := exec.CommandContext(ctx, "sh", "-c", testCommand) //#nosec:G204 // env-builder returns shell snippets intentionally executed through the shell
	cmd.Dir = dir
	cmd.Stdin = iostream.In(ctx)
	cmd.Stdout = io.MultiWriter(iostream.Out(ctx), tail)
	cmd.Stderr = io.MultiWriter(iostream.Err(ctx), tail)

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode := exitErr.ExitCode()
			iostream.Printf(ctx, "\n%s", RenderPrompt(result.Stack, image(result), testCommand, exitCode, tail.String()))
			return clierrors.New(
				"test.tests_failed",
				"Tests failed",
				fmt.Sprintf("Test command exited with status %d.", exitCode),
			).WithSuggestions(
				"Paste the prompt above into your AI assistant",
				"Re-run circleci init once tests pass",
			).WithExitCode(clierrors.ExitGeneralError)
		}
		return clierrors.New(
			"test.runner_error",
			"Could not run tests",
			fmt.Sprintf("The test runner could not start or complete: %s.", err),
		).WithSuggestions(
			"Check that a POSIX shell is available",
			"Re-run with --debug for more details",
		).WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.Printf(ctx, "%s Tests passed\n", iostream.SymbolOK(ctx))
	return nil
}

func image(result *reposcan.Result) string {
	if result == nil {
		return ""
	}
	if result.ImageVersion == "" {
		return result.Image
	}
	return result.Image + ":" + result.ImageVersion
}

type tailWriter struct {
	limit int
	mu    sync.Mutex
	buf   []byte
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)
	if len(w.buf) > w.limit {
		w.buf = w.buf[len(w.buf)-w.limit:]
	}
	return len(p), nil
}

func (w *tailWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return string(w.buf)
}
