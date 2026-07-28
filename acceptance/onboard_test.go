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
	"gotest.tools/v3/assert/cmp"
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

	_, env := onboardStandaloneEnv(t, "testuser")
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

// TestOnboard_PostSignup_FreshSignup_ContinuesToProjectSetup is a regression test
// for a fresh signup dead-ending at "Run 'circleci project create'" instead of
// continuing into project, pipeline, and trigger setup.
//
// Signup writes the token to disk part-way through the run, but the config cached
// in the context was loaded during bootstrap — before that write — so without a
// reload LoadClient sees no token and the whole setup chain degrades to manual
// guidance on the one run where it matters most.
//
// This test must NOT set env.Token: CIRCLE_TOKEN is read ahead of the cached
// config, which masks the bug — and is why the other PostSignup tests, which all
// pass a token through the environment, never caught it.
func TestOnboard_PostSignup_FreshSignup_ContinuesToProjectSetup(t *testing.T) {
	dir := t.TempDir()
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	fake := fakes.NewCircleCI(t)
	fake.SetMe(map[string]any{
		"id": "e4a72497-7c55-400d-a72d-dadc4b92255d",
		"attributes": map[string]any{
			"name":  "New User",
			"login": "newuser",
		},
	})
	fake.SetOAuthTokenResponse(map[string]any{
		"access_token": "test-signup-token",
		"token_type":   "Bearer",
		"expires_in":   int64(7776000),
	})
	fake.SetCollaborations([]any{
		map[string]any{"id": "org-uuid-1234", "name": "myorg", "slug": "circleci/myorg", "vcs_type": "circleci"},
	})
	fake.SetCreateProjectResponse(map[string]any{
		"id":                "proj-uuid-5678",
		"slug":              "circleci/myorg/my-repo",
		"name":              "my-repo",
		"organization_name": "myorg",
		"organization_slug": "circleci/myorg",
		"organization_id":   "org-uuid-1234",
	})
	fake.SetCreatePipelineDefinitionResponse("proj-uuid-5678", map[string]any{
		"id":   "pdef-uuid-1",
		"name": "my-repo",
	})
	fake.SetCreateTriggerResponse("proj-uuid-5678", "pdef-uuid-1", map[string]any{
		"id":           "trig-uuid-1",
		"event_preset": "all-pushes",
	})

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	// Deliberately no env.Token — see the note above.
	env.Extra["CIRCLE_LOGIN_TIMEOUT"] = "20s"
	// Suppress the project-name prompt so the run completes without further input.
	env.Extra["CIRCLE_NO_INTERACTIVE"] = "true"

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--signup", "--no-browser", "--repo-id", "123456789"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Assert(t, t.Run("waits for browser callback", func(t *testing.T) {
		_, err := console.ExpectString("Waiting for browser authentication")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("browser callback", func(t *testing.T) {
		callbackViaPAR(t, fake)
	}))

	assert.Assert(t, t.Run("continues into project and pipeline setup", func(t *testing.T) {
		_, err := console.ExpectString("Logged in as newuser")
		assert.NilError(t, err)
		_, err = console.ExpectString("Project created: my-repo")
		assert.NilError(t, err)
		_, err = console.ExpectString("Pipeline definition created: my-repo")
		assert.NilError(t, err)
		_, err = console.ExpectString("Trigger created: all-pushes")
		assert.NilError(t, err)
		_, err = console.ExpectString("Your project is ready!")
		assert.NilError(t, err)
	}))
}

// TestOnboard_PostSignup_KeepsExistingProjectRef pins that onboard never rewrites
// an existing .circleci/info.yml.
//
// The file is meant to be committed, and it may record a project in another
// organization that this run has no mandate to replace — `circleci project link`
// requires --force for exactly that reason.
func TestOnboard_PostSignup_KeepsExistingProjectRef(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	// A link to a project in a different organization, so it is ignored and onboard
	// proceeds to create — the path that reaches the write.
	existing := "organization:\n  id: other-org-uuid\n" +
		"project:\n  id: other-proj-uuid\n  slug: gh/myorg/my-repo\n  name: my-repo\n"
	refPath := filepath.Join(dir, ".circleci", "info.yml")
	mkErr := os.MkdirAll(filepath.Dir(refPath), 0o755)
	assert.NilError(t, mkErr)
	writeErr := os.WriteFile(refPath, []byte(existing), 0o644)
	assert.NilError(t, writeErr)

	_, env := onboardStandaloneEnv(t, "testuser")
	addFakeDotnet(t, env, false)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	body, readErr := os.ReadFile(refPath)
	assert.NilError(t, readErr)
	assert.Check(t, cmp.Equal(string(body), existing), "info.yml must be left byte-for-byte alone")
	assert.Check(t, cmp.Contains(result.Stderr, "already records a different project"))
	assert.Check(t, cmp.Contains(result.Stderr, "circleci project link --force --project"))
}

// TestOnboard_PostSignup_AddsPushTriggerAlongsideOthers pins that only an
// all-pushes trigger satisfies onboard. A definition carrying just a schedule
// trigger would otherwise be reported as ready, and the push onboard tells the
// user to make would build nothing.
func TestOnboard_PostSignup_AddsPushTriggerAlongsideOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	fake, env := onboardStandaloneEnv(t, "testuser")
	fake.AddPipelineDefinition("proj-uuid-5678", map[string]any{
		"id":   "pdef-uuid-1",
		"name": "my-repo",
		"config_source": map[string]any{
			"provider": "github_app",
			"repo":     map[string]any{"external_id": "123456789"},
		},
	})
	// Only a schedule trigger exists — no push would build anything.
	fake.AddTrigger("proj-uuid-5678", "pdef-uuid-1", map[string]any{
		"id":           "trig-schedule",
		"event_preset": "schedule",
	})
	fake.SetCreateTriggerResponse("proj-uuid-5678", "pdef-uuid-1", map[string]any{
		"id":           "trig-uuid-1",
		"event_preset": "all-pushes",
	})
	addFakeDotnet(t, env, false)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan", "--repo-id", "123456789"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "Trigger created: all-pushes"))
	assert.Check(t, !strings.Contains(result.Stdout, "Trigger already exists"),
		"a schedule-only definition is not ready for a push")
	assert.Check(t, cmp.Contains(result.Stdout, "Your project is ready!"))
}

// TestOnboard_SignupFlag_RecordsProjectRefAtRepoRoot pins where --signup records
// the project link when run from a subdirectory.
//
// info.yml belongs beside config.yml at the top of the checkout. Writing it into
// whatever directory the command was invoked from would leave it unreachable to
// every command that looks for it at the root — and, run somewhere that is not a
// repository at all, would litter a home directory.
func TestOnboard_SignupFlag_RecordsProjectRefAtRepoRoot(t *testing.T) {
	root := t.TempDir()
	initGitRepoWithRemote(t, root, "https://github.com/myorg/my-repo.git")
	sub := filepath.Join(root, "services", "api")
	mkErr := os.MkdirAll(sub, 0o755)
	assert.NilError(t, mkErr)

	_, env := onboardStandaloneEnv(t, "testuser")
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--signup"},
		Env:     env.Environ(),
		WorkDir: sub,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	_, rootErr := os.Stat(filepath.Join(root, ".circleci", "info.yml"))
	assert.NilError(t, rootErr, "the link belongs at the repository root")
	_, subErr := os.Stat(filepath.Join(sub, ".circleci", "info.yml"))
	assert.Check(t, os.IsNotExist(subErr), "must not record the link in the invocation directory")
}

// TestOnboard_SignupFlag_WritesNoProjectRefOutsideRepo is the other half: with no
// repository there is nothing to link, so nothing is written.
func TestOnboard_SignupFlag_WritesNoProjectRefOutsideRepo(t *testing.T) {
	dir := t.TempDir() // deliberately not a git checkout

	_, env := onboardStandaloneEnv(t, "testuser")
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--signup"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	_, statErr := os.Stat(filepath.Join(dir, ".circleci"))
	assert.Check(t, os.IsNotExist(statErr), "must not create .circleci outside a repository")
}

// TestOnboard_PathArgument_UsesGivenDirectory pins that a path argument selects
// the repository onboard describes.
//
// The process runs from a *different* checkout with a different remote, so any
// detection that reads the working directory instead of the argument would name
// the project after the wrong repository and wire the pipeline to the wrong repo —
// silently, since every step still reports success.
func TestOnboard_PathArgument_UsesGivenDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	// A second, unrelated checkout to run from.
	otherDir := t.TempDir()
	initGitRepoWithRemote(t, otherDir, "https://github.com/myorg/wrong-repo.git")

	fake, env := onboardStandaloneEnv(t, "testuser")
	fake.SetCreatePipelineDefinitionResponse("proj-uuid-5678", map[string]any{
		"id":   "pdef-uuid-1",
		"name": "my-repo",
	})
	fake.SetCreateTriggerResponse("proj-uuid-5678", "pdef-uuid-1", map[string]any{
		"id":           "trig-uuid-1",
		"event_preset": "all-pushes",
	})
	addFakeDotnet(t, env, false)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan", "--repo-id", "123456789", dir},
		Env:     env.Environ(),
		WorkDir: otherDir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "Project created: my-repo"))
	assert.Check(t, cmp.Contains(result.Stdout, "Your project is ready!"))
	assert.Check(t, !strings.Contains(result.Stdout, "wrong-repo"),
		"the working directory's repository must not leak into the run")
	// The link belongs to the directory that was onboarded, not the cwd.
	_, err := os.Stat(filepath.Join(dir, ".circleci", "info.yml"))
	assert.NilError(t, err)
	_, err = os.Stat(filepath.Join(otherDir, ".circleci", "info.yml"))
	assert.Check(t, os.IsNotExist(err), "must not write into the working directory")
}

func TestOnboard_PostSignup_FirstPipelineCreated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	fake, env := onboardStandaloneEnv(t, "testuser")
	fake.SetCreatePipelineDefinitionResponse("proj-uuid-5678", map[string]any{
		"id":         "pdef-uuid-1",
		"name":       "my-repo",
		"created_at": "2026-07-23T00:00:00Z",
	})
	fake.SetCreateTriggerResponse("proj-uuid-5678", "pdef-uuid-1", map[string]any{
		"id":           "trig-uuid-1",
		"created_at":   "2026-07-23T00:00:00Z",
		"event_preset": "all-pushes",
	})
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan", "--repo-id", "123456789"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "Project created: my-repo"))
	assert.Check(t, strings.Contains(result.Stdout, "Pipeline definition created: my-repo"))
	assert.Check(t, strings.Contains(result.Stdout, "Trigger created: all-pushes"))
	assert.Check(t, strings.Contains(result.Stdout, "Your project is ready!"))
	assert.Check(t, strings.Contains(result.Stdout, "git push"))
}

func TestOnboard_PostSignup_FirstPipeline_GitHubAppResolvesRepo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	fake, env := onboardStandaloneEnv(t, "testuser")
	// GitHub App is installed for the org and can access the repo, so the repo's
	// external ID is resolved automatically — no --repo-id needed.
	fake.SetGitHubAppInstalled("org-uuid-1234", true)
	fake.AddGitHubAppRepository("org-uuid-1234", map[string]any{
		"id":             987654321,
		"repo_full_name": "myorg/my-repo",
		"repo_name":      "my-repo",
		"owner":          "myorg",
		"default_branch": "main",
		"private":        false,
	})
	fake.SetCreatePipelineDefinitionResponse("proj-uuid-5678", map[string]any{
		"id":   "pdef-uuid-1",
		"name": "my-repo",
	})
	fake.SetCreateTriggerResponse("proj-uuid-5678", "pdef-uuid-1", map[string]any{
		"id":           "trig-uuid-1",
		"event_preset": "all-pushes",
	})
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "Found repository myorg/my-repo"))
	assert.Check(t, strings.Contains(result.Stdout, "Pipeline definition created: my-repo"))
	assert.Check(t, strings.Contains(result.Stdout, "Trigger created: all-pushes"))
	assert.Check(t, strings.Contains(result.Stdout, "Your project is ready!"))
}

// TestOnboard_PostSignup_Rerun_Idempotent runs onboard twice over the same
// repository. The first run creates the project and records it in
// .circleci/info.yml; the second resolves it from that file instead of
// attempting a create that could only conflict.
//
// That local record is what makes a re-run recoverable at all: a CircleCI-native
// project slug is circleci/<orgID>/<projectID> — opaque IDs, not the repository
// name — and no API maps a project name to its ID within an org.
func TestOnboard_PostSignup_Rerun_Idempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	fake, env := onboardStandaloneEnv(t, "testuser")
	fake.SetGitHubAppInstalled("org-uuid-1234", true)
	fake.AddGitHubAppRepository("org-uuid-1234", map[string]any{
		"id":             987654321,
		"repo_full_name": "myorg/my-repo",
		"repo_name":      "my-repo",
		"owner":          "myorg",
	})
	fake.SetCreatePipelineDefinitionResponse("proj-uuid-5678", map[string]any{
		"id":   "pdef-uuid-1",
		"name": "my-repo",
	})
	fake.SetCreateTriggerResponse("proj-uuid-5678", "pdef-uuid-1", map[string]any{
		"id":           "trig-uuid-1",
		"event_preset": "all-pushes",
	})
	// The second run looks the project up by the slug projectref derives from the
	// recorded UUIDs. The real API accepts that form and canonicalises it to the
	// short-ID slug.
	fake.AddProjectInfo("circleci/org-uuid-1234/proj-uuid-5678", map[string]any{
		"id":                "proj-uuid-5678",
		"slug":              "circleci/Org1234ShortId/Proj5678ShortId",
		"name":              "my-repo",
		"organization_name": "myorg",
		"organization_slug": "circleci/Org1234ShortId",
		"organization_id":   "org-uuid-1234",
	})
	addFakeDotnet(t, env, false)

	first := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})
	assert.Equal(t, first.ExitCode, 0, "stderr: %s", first.Stderr)
	assert.Check(t, strings.Contains(first.Stdout, "Project created: my-repo"))
	assert.Check(t, strings.Contains(first.Stdout, "Linked this repository to the project"))
	_, err := os.Stat(filepath.Join(dir, ".circleci", "info.yml"))
	assert.NilError(t, err, "first run should record the project locally")
	// The recorded ID is unrecoverable from the project name, so the next steps
	// have to stage info.yml, not just config.yml.
	assert.Check(t, strings.Contains(first.Stdout, "git add .circleci/"))
	assert.Check(t, !strings.Contains(first.Stdout, "git add .circleci/config.yml"),
		"staging only config.yml would leave the project ID uncommitted")

	// The project now has its pipeline definition and trigger.
	fake.AddPipelineDefinition("proj-uuid-5678", map[string]any{
		"id":   "pdef-uuid-1",
		"name": "my-repo",
		"config_source": map[string]any{
			"provider": "github_app",
			"repo":     map[string]any{"external_id": "987654321"},
		},
	})
	fake.AddTrigger("proj-uuid-5678", "pdef-uuid-1", map[string]any{
		"id":           "trig-uuid-1",
		"event_preset": "all-pushes",
	})

	second := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})
	assert.Equal(t, second.ExitCode, 0, "stderr: %s", second.Stderr)
	assert.Check(t, strings.Contains(second.Stdout, "Using existing project: my-repo"))
	assert.Check(t, strings.Contains(second.Stdout, "Pipeline definition already exists: my-repo"))
	assert.Check(t, strings.Contains(second.Stdout, "Trigger already exists"))
	assert.Check(t, !strings.Contains(second.Stdout, "Project created"), "re-run should not create a second project")
	assert.Check(t, !strings.Contains(second.Stderr, "already exists"), "re-run should not surface a conflict")
}

// TestOnboard_PostSignup_LinkedProjectInAnotherOrg covers a repository already
// linked to a project in a different organization — a classic VCS project being
// onboarded into a CircleCI-native org, which is what a migration looks like.
//
// The link must not be reused: it would set up pipelines in an organization the
// user did not choose. It is ignored, onboard proceeds to create, and the name
// collision that follows points at `project link --force`, since plain
// `project link` refuses while .circleci/info.yml exists.
func TestOnboard_PostSignup_LinkedProjectInAnotherOrg(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	// A resolvable link, but to a project owned by a different org. Both IDs are
	// recorded against a classic slug, exactly as `circleci project link` writes
	// it — the slug must be used as-is rather than rebuilt into the circleci/ form.
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci"), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(dir, ".circleci", "info.yml"), []byte(
		"organization:\n  id: other-org-uuid\n  name: myorg\n"+
			"project:\n  id: other-proj-uuid\n  slug: gh/myorg/my-repo\n  name: my-repo\n",
	), 0o644))

	fake, env := onboardStandaloneEnv(t, "testuser")
	fake.AddProjectInfo("gh/myorg/my-repo", map[string]any{
		"id":                "other-proj-uuid",
		"slug":              "gh/myorg/my-repo",
		"name":              "my-repo",
		"organization_name": "myorg",
		"organization_slug": "gh/myorg",
		"organization_id":   "other-org-uuid",
	})
	fake.SetCreateProjectConflict()
	addFakeDotnet(t, env, false)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, !strings.Contains(result.Stdout, "Using existing project"),
		"a project in another organization must not be reused")
	assert.Check(t, strings.Contains(result.Stderr, `project named "my-repo" already exists`))
	// The guidance has to terminate: --force because info.yml exists, and --project
	// because otherwise link re-derives the same foreign slug from the git remote.
	assert.Check(t, strings.Contains(result.Stdout,
		"circleci project link --force --project circleci/myorg/<projectID>"))
	assert.Check(t, !strings.Contains(result.Stderr, "409"), "stderr should not leak an HTTP status")
}

// TestOnboard_PostSignup_ProjectNameConflict covers a name collision that onboard
// cannot resolve: the org already has a project with this name, but the checkout
// has no .circleci/info.yml recording its ID. Since a project cannot be looked up
// by name, onboard points at `circleci project link` rather than reporting a raw
// HTTP conflict.
func TestOnboard_PostSignup_ProjectNameConflict(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	fake, env := onboardStandaloneEnv(t, "testuser")
	fake.SetCreateProjectConflict()
	addFakeDotnet(t, env, false)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, `project named "my-repo" already exists`))
	assert.Check(t, strings.Contains(result.Stdout, "circleci project link --project circleci/myorg/<projectID>"))
	assert.Check(t, !strings.Contains(result.Stdout, "--force"), "no info.yml to overwrite")
	// No raw HTTP internals for a conflict the user can resolve with one command.
	assert.Check(t, !strings.Contains(result.Stderr, "409"), "stderr should not leak an HTTP status")
	assert.Check(t, !strings.Contains(result.Stderr, "/api/v2/"), "stderr should not leak an API path")
}

func TestOnboard_PostSignup_FirstPipeline_RepoNotAccessible(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	fake, env := onboardStandaloneEnv(t, "testuser")
	// App is installed, but the repo the user is in was not granted to it.
	fake.SetGitHubAppInstalled("org-uuid-1234", true)
	fake.AddGitHubAppRepository("org-uuid-1234", map[string]any{
		"id":             111,
		"repo_full_name": "myorg/some-other-repo",
		"repo_name":      "some-other-repo",
		"owner":          "myorg",
	})
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "can't access myorg/my-repo"))
	assert.Check(t, strings.Contains(result.Stdout, "Commit .circleci/config.yml"))
	assert.Check(t, !strings.Contains(result.Stdout, "Pipeline definition created"), "no pipeline def without a repo ID")
}

func TestOnboard_PostSignup_FirstPipeline_TriggerFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	fake, env := onboardStandaloneEnv(t, "testuser")
	fake.SetCreatePipelineDefinitionResponse("proj-uuid-5678", map[string]any{
		"id":   "pdef-uuid-1",
		"name": "my-repo",
	})
	// No trigger response registered → the trigger create returns 404 and the
	// flow degrades to manual guidance.
	addFakeDotnet(t, env, false)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan", "--repo-id", "123456789"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "Pipeline definition created: my-repo"))
	assert.Check(t, strings.Contains(result.Stderr, "Could not create trigger"))
	assert.Check(t, strings.Contains(result.Stdout, "Commit .circleci/config.yml"))
}

func TestOnboard_PostSignup_ClassicOrg_FollowsProject(t *testing.T) {
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
	assert.Check(t, strings.Contains(result.Stdout, "Project connected: my-repo"))
	assert.Check(t, strings.Contains(result.Stdout, "Organization: myorg"))
	assert.Check(t, strings.Contains(result.Stdout, "Commit and push .circleci/config.yml"))
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
	assert.Check(t, strings.Contains(result.Stdout, "circleci project create"))
}

func TestOnboard_PostSignup_CreateFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test runner uses sh -c")
	}
	dir := t.TempDir()
	copyFixture(t, "testdata/test-run/dotnet", dir)
	initGitRepoWithRemote(t, dir, "https://github.com/myorg/my-repo.git")

	fake, env := onboardStandaloneEnv(t, "testuser")
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

func onboardStandaloneEnv(t *testing.T, login string) (*fakes.CircleCI, *testenv.TestEnv) {
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
		map[string]any{"id": "org-uuid-1234", "name": "myorg", "slug": "circleci/myorg", "vcs_type": "circleci"},
	})
	// A CircleCI-native project slug embeds opaque org and project short IDs, not
	// the repository name — mirroring the real API, where a name-based slug is
	// rejected outright. The UUIDs are separate values used by the pipeline
	// definition and GitHub App calls.
	fake.SetCreateProjectResponse(map[string]any{
		"id":                "proj-uuid-5678",
		"slug":              "circleci/Org1234ShortId/Proj5678ShortId",
		"name":              "my-repo",
		"organization_name": "myorg",
		"organization_slug": "circleci/Org1234ShortId",
		"organization_id":   "org-uuid-1234",
	})

	env := testenv.New(t)
	env.CircleCIURL = fake.URL()
	env.Token = "test-token"
	return fake, env
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
