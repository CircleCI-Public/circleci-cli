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
	"os/exec"
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

	_, env := onboardAuthenticatedEnv(t, "testuser")
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

	_, env := onboardAuthenticatedEnv(t, "testuser")
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

func TestOnboard_ScanAndSignupMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	initGitDir(t, dir)

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan", "--signup", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "expected ExitBadArguments, stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOnboard_SignupFlag_AlreadyAuthenticated(t *testing.T) {
	dir := t.TempDir()

	_, env := onboardAuthenticatedEnv(t, "testuser")
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--signup"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestOnboard_SignupFlag_NotInGitRepo(t *testing.T) {
	dir := t.TempDir()

	_, env := onboardAuthenticatedEnv(t, "testuser")
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--signup"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "Already signed in"), "expected signup confirmation")
}

func TestOnboard_ScanFlag_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "expected ExitBadArguments, stderr: %s", result.Stderr)

	stderr := strings.ReplaceAll(result.Stderr, strconv.Quote(dir), `"<DIR>"`)
	assert.Check(t, golden.String(stderr, "TestOnboard_NotAGitRepo.stderr.txt"))
}

func TestOnboard_ScanFlag_ExplicitSameAsDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitDir(t, dir)

	_, env := onboardAuthenticatedEnv(t, "testuser")
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	stdout := normalizeOnboardOutput(result.Stdout, dir)
	assert.Check(t, golden.String(stdout, "TestOnboard_HappyPath_AlreadyAuthenticated.txt"))
}

func TestOnboard_PostSignup_ProjectCreated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	_, env := onboardAuthenticatedEnv(t, "testuser")
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "Project created: my-repo"))
	assert.Check(t, strings.Contains(result.Stdout, "Organization: myorg"))
	assert.Check(t, strings.Contains(result.Stdout, "Commit .circleci/config.yml"))
}

func TestOnboard_PostSignup_NoOrgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitDir(t, dir)

	fake, env := onboardAuthenticatedEnv(t, "testuser")
	fake.SetCollaborations(nil)
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "isn't part of a CircleCI organization"))
	assert.Check(t, strings.Contains(result.Stdout, "circleci project create"))
}

func TestOnboard_PostSignup_CreateFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	fake, env := onboardAuthenticatedEnv(t, "testuser")
	fake.SetCreateProjectResponse(nil)
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "Could not create project"))
	assert.Check(t, strings.Contains(result.Stdout, "circleci project create"))
}

func onboardAuthenticatedEnv(t *testing.T, login string) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()

	fake := fakes.NewCircleCI(t)
	fake.SetMe(map[string]any{
		"id": "e4a72497-7c55-400d-a72d-dadc4b92255d",
		"attributes": map[string]any{
			"name":  "Test User",
			"login": login,
		},
	})
	fake.SetCollaborations([]any{
		map[string]any{"id": "org-uuid-1234", "name": "myorg", "slug": "gh/myorg", "vcs_type": "github"},
	})
	fake.SetCreateProjectResponse(map[string]any{
		"id":                "proj-uuid-5678",
		"slug":              "gh/myorg/my-repo",
		"name":              "my-repo",
		"organization_name": "myorg",
		"organization_slug": "gh/myorg",
		"organization_id":   "org-uuid-1234",
		"vcs_info": map[string]any{
			"provider":       "GitHub",
			"default_branch": "main",
			"vcs_url":        "https://github.com/myorg/my-repo",
		},
	})

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	env.Token = "test-token"
	return fake, env
}

func statConfig(dir string) error {
	_, err := os.Stat(filepath.Join(dir, ".circleci", "config.yml"))
	return err
}

func initGitDir(t *testing.T, dir string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
}

func initGitRepoWithRemote(t *testing.T, dir, remoteURL string) {
	t.Helper()
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	run := func(d string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = d
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		assert.NilError(t, err, "command %v failed: %s", args, out)
	}

	bare := t.TempDir()
	run(bare, "git", "init", "--bare", "--initial-branch=main")

	run(dir, "git", "init", "--initial-branch=main")
	run(dir, "git", "remote", "add", "origin", bare)
	run(dir, "git", "commit", "--allow-empty", "-m", "init")
	run(dir, "git", "push", "origin", "main")
	run(dir, "git", "remote", "set-url", "origin", remoteURL)

	// Create origin/HEAD symref so gitremote.DetectFromRemote can resolve the default branch.
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".git", "refs", "remotes", "origin"), 0o755))
	assert.NilError(t, os.WriteFile(
		filepath.Join(dir, ".git", "refs", "remotes", "origin", "HEAD"),
		[]byte("ref: refs/remotes/origin/main\n"), 0o644,
	))
}

func normalizeOnboardOutput(stdout, dir string) string {
	stdout = strings.ReplaceAll(stdout, dir, "<DIR>")
	stdout = strings.ReplaceAll(stdout, `\`, `/`)
	return stdout
}
