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
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

func TestOnboard_PathInvalid(t *testing.T) {
	cases := []struct {
		name        string
		path        func(t *testing.T) string
		placeholder string
	}{
		{
			name: "missing",
			path: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "does-not-exist")
			},
			placeholder: `"<MISSING_PATH>"`,
		},
		{
			name: "file",
			path: func(t *testing.T) string {
				tempDir := t.TempDir()
				filePath := filepath.Join(tempDir, "not-a-dir.txt")
				assert.NilError(t, os.WriteFile(filePath, []byte("hello"), 0o644))
				return filePath
			},
			placeholder: `"<FILE_PATH>"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.path(t)

			env := testenv.New(t)
			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    []string{"onboard", path},
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Equal(t, result.ExitCode, 2, "expected ExitBadArguments, stderr: %s", result.Stderr)

			stderr := strings.ReplaceAll(result.Stderr, strconv.Quote(path), tc.placeholder)
			assert.Check(t, golden.String(stderr, "TestOnboard_PathInvalid_"+tc.name+".stderr.txt"))
		})
	}
}

func TestOnboard_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "expected ExitBadArguments, stderr: %s", result.Stderr)

	stderr := strings.ReplaceAll(result.Stderr, strconv.Quote(dir), `"<DIR>"`)
	assert.Check(t, golden.String(stderr, t.Name()+".stderr.txt"))
}

func TestOnboard_NoArg(t *testing.T) {
	dir := t.TempDir()
	initGitDir(t, dir)

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOnboard_FailingTests_ShortCircuits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitDir(t, dir)

	env := testenv.New(t)
	addFakeDotnet(t, env, true)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, os.IsNotExist(statConfig(dir)), "config should not be created after test failure")
	assert.Check(t, !strings.Contains(result.Stderr, "Open this URL in your browser"), "signup should not start")

	stdout := normalizeOnboardOutput(result.Stdout, dir)
	assert.Check(t, golden.String(stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOnboard_ConfigAlreadyExists(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitDir(t, dir)
	configPath := filepath.Join(dir, ".circleci", "config.yml")
	assert.NilError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	assert.NilError(t, os.WriteFile(configPath, []byte("# existing config\nversion: 2.1\n"), 0o644))

	env := onboardAuthenticatedEnv(t, "testuser")
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	stdout := normalizeOnboardOutput(result.Stdout, dir)
	assert.Check(t, golden.String(stdout, t.Name()+".txt"))
}

func TestOnboard_HappyPath_AlreadyAuthenticated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitDir(t, dir)

	env := onboardAuthenticatedEnv(t, "testuser")
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	_, err := os.Stat(filepath.Join(dir, ".circleci", "config.yml"))
	assert.NilError(t, err)

	stdout := normalizeOnboardOutput(result.Stdout, dir)
	assert.Check(t, golden.String(stdout, t.Name()+".txt"))
}

func onboardAuthenticatedEnv(t *testing.T, login string) *testenv.TestEnv {
	t.Helper()

	fake := fakes.NewCircleCI(t)
	fake.SetMe(map[string]any{
		"id":    "e4a72497-7c55-400d-a72d-dadc4b92255d",
		"name":  "Test User",
		"login": login,
	})

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	env.Token = "test-token"
	return env
}

func statConfig(dir string) error {
	_, err := os.Stat(filepath.Join(dir, ".circleci", "config.yml"))
	return err
}

func initGitDir(t *testing.T, dir string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
}

func normalizeOnboardOutput(stdout, dir string) string {
	stdout = strings.ReplaceAll(stdout, dir, "<DIR>")
	stdout = strings.ReplaceAll(stdout, `\`, `/`)
	return stdout
}
