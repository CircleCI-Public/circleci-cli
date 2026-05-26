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

package testrunner

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

func testContext() (context.Context, *bytes.Buffer, *bytes.Buffer) {
	var outBuf, errBuf bytes.Buffer
	ctx := iostream.WithStreams(context.Background(), iostream.Streams{
		Out: &outBuf,
		Err: &errBuf,
		In:  strings.NewReader(""),
	})
	return ctx, &outBuf, &errBuf
}

func TestRun_NoTestCommand_ReturnsStructuredError(t *testing.T) {
	ctx, _, _ := testContext()

	err := Run(ctx, t.TempDir(), &reposcan.Result{Stack: reposcan.StackUnknown})

	assertCLIError(t, err, "test.no_test_command", clierrors.ExitGeneralError)
}

func TestRun_TestPasses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh-based test runner is not supported on Windows")
	}
	ctx, outBuf, errBuf := testContext()

	err := Run(ctx, t.TempDir(), &reposcan.Result{
		Stack:        "go",
		Image:        "cimg/go",
		ImageVersion: "1.22",
		Setup:        []reposcan.SetupStep{{Name: "test", Command: "printf 'hello\\n'"}},
	})

	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(outBuf.String(), "Running tests"))
	assert.Check(t, cmp.Contains(outBuf.String(), "hello"))
	assert.Check(t, cmp.Contains(outBuf.String(), "Tests passed"))
	assert.Equal(t, errBuf.String(), "")
}

func TestRun_TestFails_PrintsPromptAndReturnsStructuredError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh-based test runner is not supported on Windows")
	}
	ctx, outBuf, errBuf := testContext()

	err := Run(ctx, t.TempDir(), &reposcan.Result{
		Stack:        "go",
		Image:        "cimg/go",
		ImageVersion: "1.22",
		Setup:        []reposcan.SetupStep{{Name: "test", Command: "printf 'boom\\n'; printf 'details\\n' >&2; exit 7"}},
	})

	assertCLIError(t, err, "test.tests_failed", clierrors.ExitGeneralError)
	assert.Check(t, cmp.Contains(outBuf.String(), "My tests failed when running `circleci test run`."))
	assert.Check(t, cmp.Contains(outBuf.String(), "Command run: printf 'boom\\n'; printf 'details\\n' >&2; exit 7"))
	assert.Check(t, cmp.Contains(outBuf.String(), "Exit code: 7"))
	assert.Check(t, cmp.Contains(outBuf.String(), "boom"))
	assert.Check(t, cmp.Contains(outBuf.String(), "details"))
	assert.Check(t, cmp.Contains(errBuf.String(), "details"))
}

func TestRun_RunnerError_ReturnsStructuredError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh-based test runner is not supported on Windows")
	}
	ctx, _, _ := testContext()
	missingDir := filepath.Join(t.TempDir(), "missing")

	err := Run(ctx, missingDir, &reposcan.Result{
		Stack: "go",
		Image: "cimg/go",
		Setup: []reposcan.SetupStep{{Name: "test", Command: "exit 0"}},
	})

	assertCLIError(t, err, "test.runner_error", clierrors.ExitGeneralError)
}

func assertCLIError(t *testing.T, err error, code string, exitCode int) {
	t.Helper()
	var cliErr *clierrors.CLIError
	assert.Assert(t, errors.As(err, &cliErr), "expected CLIError, got %T: %v", err, err)
	assert.Equal(t, cliErr.Code, code)
	assert.Equal(t, cliErr.ExitCode, exitCode)
}
