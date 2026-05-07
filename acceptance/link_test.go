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
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

func TestProjectLink_WithFlag(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectInfo("gh/myorg/alpha", map[string]any{
		"id":                "proj-uuid-1234",
		"slug":              "gh/myorg/alpha",
		"name":              "alpha",
		"organization_name": "myorg",
		"organization_slug": "gh/myorg",
		"organization_id":   "org-uuid-5678",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	workDir := t.TempDir()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "link", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	data, err := os.ReadFile(filepath.Join(workDir, ".circleci", "info.yml"))
	assert.NilError(t, err)
	body := string(data)

	assert.Check(t, strings.Contains(body, "slug: gh/myorg/alpha"), "got: %s", body)
	assert.Check(t, strings.Contains(body, "project_id: proj-uuid-1234"), "got: %s", body)
	assert.Check(t, strings.Contains(body, "organization_id: org-uuid-5678"), "got: %s", body)
}

// Standalone-project slugs (circleci/<orgID>/<projectID>) should round-trip
// through the same code path as VCS slugs.
func TestProjectLink_StandaloneSlug(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectInfo("circleci/E6i3yYZeWZhcf8UNqcKfjN/13c8F7nusayivoSxC6GMsw", map[string]any{
		"id":              "13c8F7nusayivoSxC6GMsw",
		"slug":            "circleci/E6i3yYZeWZhcf8UNqcKfjN/13c8F7nusayivoSxC6GMsw",
		"name":            "standalone",
		"organization_id": "E6i3yYZeWZhcf8UNqcKfjN",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	workDir := t.TempDir()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "link", "--project", "circleci/E6i3yYZeWZhcf8UNqcKfjN/13c8F7nusayivoSxC6GMsw"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	data, err := os.ReadFile(filepath.Join(workDir, ".circleci", "info.yml"))
	assert.NilError(t, err)
	body := string(data)
	assert.Check(t, strings.Contains(body, "slug: circleci/E6i3yYZeWZhcf8UNqcKfjN/13c8F7nusayivoSxC6GMsw"), "got: %s", body)
}

// In a non-interactive environment with no git remote and no --project flag,
// the command must fail rather than block on a prompt.
func TestProjectLink_NonInteractive_NoSlug(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "link"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(), // empty temp dir → no git remote
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, strings.Contains(result.Stderr, "No project found via --project flag or git remote"), "stderr: %s", result.Stderr)
}

// Without a token configured, the command must short-circuit and tell the
// user to authenticate, not write a placeholder file.
func TestProjectLink_NoToken(t *testing.T) {
	env := testenv.New(t) // no Token

	workDir := t.TempDir()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "link", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	_, statErr := os.Stat(filepath.Join(workDir, ".circleci", "info.yml"))
	assert.Check(t, os.IsNotExist(statErr), "info.yml should not be written without a token")
}

// Once a checkout is linked, subsequent commands should resolve the project
// from info.yml — using the canonical "circleci/<orgID>/<projectID>" slug
// when info.yml carries the UUIDs, even though the git remote (if any)
// would have produced a "gh/org/repo"-style slug.
func TestProjectGet_UsesLinkedUUIDs(t *testing.T) {
	const standaloneSlug = "circleci/E6i3yYZeWZhcf8UNqcKfjN/13c8F7nusayivoSxC6GMsw"

	fake := fakes.NewCircleCI(t)
	// Only register the standalone (UUID-form) slug. If `project get` falls
	// back to a VCS-style slug for any reason, the API will return 404.
	fake.AddProjectInfo(standaloneSlug, map[string]any{
		"id":              "13c8F7nusayivoSxC6GMsw",
		"slug":            standaloneSlug,
		"name":            "linked",
		"organization_id": "E6i3yYZeWZhcf8UNqcKfjN",
	})
	// Also register the VCS slug used during link, so the initial link call succeeds.
	fake.AddProjectInfo("gh/myorg/alpha", map[string]any{
		"id":              "13c8F7nusayivoSxC6GMsw",
		"slug":            "gh/myorg/alpha",
		"name":            "alpha",
		"organization_id": "E6i3yYZeWZhcf8UNqcKfjN",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	workDir := t.TempDir()

	link := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "link", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})
	assert.Equal(t, link.ExitCode, 0, "link stderr: %s", link.Stderr)

	// Now run `project get` with no --project flag. It must resolve via
	// info.yml and hit the standalone (UUID) endpoint, NOT the VCS slug.
	get := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "get", "--json"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})
	assert.Equal(t, get.ExitCode, 0, "get stderr: %s", get.Stderr)
	assert.Check(t, strings.Contains(get.Stdout, standaloneSlug), "stdout: %s", get.Stdout)
}

// Refuses to overwrite an existing info.yml without --force.
func TestProjectLink_PreservesExisting(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectInfo("gh/myorg/alpha", map[string]any{
		"id":              "proj-uuid-1234",
		"slug":            "gh/myorg/alpha",
		"organization_id": "org-uuid-5678",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	workDir := t.TempDir()
	circleciDir := filepath.Join(workDir, ".circleci")
	assert.NilError(t, os.MkdirAll(circleciDir, 0o755))
	existing := []byte("slug: pre-existing\n")
	assert.NilError(t, os.WriteFile(filepath.Join(circleciDir, "info.yml"), existing, 0o644))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "link", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr) // ExitGeneralError
	body, err := os.ReadFile(filepath.Join(circleciDir, "info.yml"))
	assert.NilError(t, err)
	assert.Equal(t, string(body), "slug: pre-existing\n")

	// With --force, the existing file is overwritten.
	result = binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "link", "--project", "gh/myorg/alpha", "--force"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	body, err = os.ReadFile(filepath.Join(circleciDir, "info.yml"))
	assert.NilError(t, err)
	assert.Check(t, strings.Contains(string(body), "slug: gh/myorg/alpha"), "got: %s", string(body))
}
