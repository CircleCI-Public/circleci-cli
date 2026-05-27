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

package acceptance_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestTestRun_PathNotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"test", "run", missing},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "expected ExitBadArguments, stderr: %s", result.Stderr)

	stderr := strings.ReplaceAll(result.Stderr, strconv.Quote(missing), `"<MISSING_PATH>"`)
	assert.Check(t, golden.String(stderr, t.Name()+".stderr.txt"))
}

func TestTestRun_PassingProject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)

	env := testenv.New(t)
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"test", "run", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	stdout := normalizeTestRunOutput(result.Stdout, dir)
	assert.Check(t, golden.String(stdout, t.Name()+".txt"))
}

func TestTestRun_FailingProject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)

	env := testenv.New(t)
	addFakeDotnet(t, env, true)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"test", "run", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)

	stdout := normalizeTestRunOutput(result.Stdout, dir)
	assert.Check(t, golden.String(stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestRun_NoTestCommand(t *testing.T) {
	dir := t.TempDir()

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"test", "run", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestRun_NoArg(t *testing.T) {
	dir := t.TempDir()

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"test", "run"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTestRun_PathIsFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "not-a-dir.txt")
	assert.NilError(t, os.WriteFile(filePath, []byte("hello"), 0o644))

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"test", "run", filePath},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "expected ExitBadArguments, stderr: %s", result.Stderr)

	stderr := strings.ReplaceAll(result.Stderr, strconv.Quote(filePath), `"<FILE_PATH>"`)
	assert.Check(t, golden.String(stderr, t.Name()+".stderr.txt"))
}

func TestTestRun_EnvBuilderEmitsTestStepContract(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)

	result, err := reposcan.NewDefaultScanner().Scan(context.Background(), dir)
	assert.NilError(t, err)

	cmd := result.SetupCommand("test")
	assert.Check(t, cmd != "", "env-builder should emit a 'test' setup step for .NET projects")
	assert.Check(t, strings.HasPrefix(cmd, "dotnet test"), "expected dotnet test command, got: %s", cmd)
}

func addFakeDotnet(t *testing.T, env *testenv.TestEnv, fail bool) {
	t.Helper()
	binDir := t.TempDir()
	script := filepath.Join(binDir, "dotnet")
	body := `#!/bin/sh
printf 'fake dotnet %s\n' "$*"
if [ "${CIRCLECI_TEST_RUN_FAIL:-}" = "1" ]; then
  printf 'fake failure details\n' >&2
  exit 9
fi
exit 0
`
	assert.NilError(t, os.WriteFile(script, []byte(body), 0o755))

	env.Extra["PATH"] = binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	if fail {
		env.Extra["CIRCLECI_TEST_RUN_FAIL"] = "1"
	}
}

func normalizeTestRunOutput(stdout, dir string) string {
	stdout = strings.ReplaceAll(stdout, dir, "<DIR>")
	stdout = strings.ReplaceAll(stdout, `\`, `/`)
	return stdout
}
