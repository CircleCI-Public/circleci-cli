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

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

func TestProjectLink_WithFlag(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectInfo("gh/myorg/alpha", fakes.ProjectInfo{
		ID:               "proj-uuid-1234",
		Slug:             "gh/myorg/alpha",
		Name:             "alpha",
		OrganizationName: "myorg",
		OrganizationSlug: "gh/myorg",
		OrganizationID:   "org-uuid-5678",
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

	// New schema: organization + project as top-level keys with nested fields.
	assert.Check(t, strings.Contains(body, "organization:\n"), "got: %s", body)
	assert.Check(t, strings.Contains(body, "    id: org-uuid-5678"), "got: %s", body)
	assert.Check(t, strings.Contains(body, "    name: myorg"), "got: %s", body)
	assert.Check(t, strings.Contains(body, "project:\n"), "got: %s", body)
	assert.Check(t, strings.Contains(body, "    id: proj-uuid-1234"), "got: %s", body)
	assert.Check(t, strings.Contains(body, "    slug: gh/myorg/alpha"), "got: %s", body)
	assert.Check(t, strings.Contains(body, "    name: alpha"), "got: %s", body)
}

// Standalone-project slugs (circleci/<orgID>/<projectID>) should round-trip
// through the same code path as VCS slugs.
func TestProjectLink_StandaloneSlug(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectInfo("circleci/E6i3yYZeWZhcf8UNqcKfjN/13c8F7nusayivoSxC6GMsw", fakes.ProjectInfo{
		ID:             "13c8F7nusayivoSxC6GMsw",
		Slug:           "circleci/E6i3yYZeWZhcf8UNqcKfjN/13c8F7nusayivoSxC6GMsw",
		Name:           "standalone",
		OrganizationID: "E6i3yYZeWZhcf8UNqcKfjN",
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

// Once a checkout is linked, subsequent commands resolve the project from
// info.yml. For a CircleCI-native project that means the canonical
// "circleci/<orgID>/<projectID>" slug built from the recorded IDs, so resolution
// survives a slug change on the CircleCI side.
func TestProjectGet_UsesLinkedUUIDs(t *testing.T) {
	const canonicalSlug = "circleci/E6i3yYZeWZhcf8UNqcKfjN/13c8F7nusayivoSxC6GMsw"
	const linkedSlug = "circleci/OldOrgShortId/OldProjShortId"

	fake := fakes.NewCircleCI(t)
	// Only register the ID-form slug for the lookup under test. If `project get`
	// used the stored slug instead, the fake would 404.
	fake.AddProjectInfo(canonicalSlug, fakes.ProjectInfo{
		ID:             "13c8F7nusayivoSxC6GMsw",
		Slug:           canonicalSlug,
		Name:           "linked",
		OrganizationID: "E6i3yYZeWZhcf8UNqcKfjN",
	})
	// Register the slug passed to link, so the initial link call succeeds.
	fake.AddProjectInfo(linkedSlug, fakes.ProjectInfo{
		ID:             "13c8F7nusayivoSxC6GMsw",
		Slug:           linkedSlug,
		Name:           "linked",
		OrganizationID: "E6i3yYZeWZhcf8UNqcKfjN",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	workDir := t.TempDir()

	link := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "link", "--project", linkedSlug},
		Env:     env.Environ(),
		WorkDir: workDir,
	})
	assert.Equal(t, link.ExitCode, 0, "link stderr: %s", link.Stderr)

	// `project get` with no --project must resolve via info.yml, rebuilding the
	// ID-form slug rather than reusing the one that was linked.
	get := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "get", "--json"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})
	assert.Equal(t, get.ExitCode, 0, "get stderr: %s", get.Stderr)
	assert.Check(t, strings.Contains(get.Stdout, canonicalSlug), "stdout: %s", get.Stdout)
}

// TestProjectGet_ClassicLinkUsesOwnSlug is the counterpart: a classic VCS project
// resolves by its own "<vcs>/<org>/<repo>" slug.
//
// `circleci project link` records org and project IDs for every project type, but
// the "circleci/<orgID>/<projectID>" form addresses CircleCI-native projects only.
// The real API answers 404 for a classic project addressed that way, whatever its
// recorded IDs are — so rebuilding the slug there breaks every command that
// resolves a project from a linked checkout.
func TestProjectGet_ClassicLinkUsesOwnSlug(t *testing.T) {
	const classicSlug = "gh/myorg/alpha"

	fake := fakes.NewCircleCI(t)
	// Only the classic slug is registered, mirroring the real API: an ID-form
	// lookup for this project 404s.
	fake.AddProjectInfo(classicSlug, fakes.ProjectInfo{
		ID:             "52404b72-02fb-482e-9bd8-846bbc048eea",
		Slug:           classicSlug,
		Name:           "alpha",
		OrganizationID: "c1e89d5c-d2e5-4db2-b2d7-a35cf73160ad",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	workDir := t.TempDir()

	link := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "link", "--project", classicSlug},
		Env:     env.Environ(),
		WorkDir: workDir,
	})
	assert.Equal(t, link.ExitCode, 0, "link stderr: %s", link.Stderr)

	get := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "get", "--json"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})
	assert.Equal(t, get.ExitCode, 0, "get stderr: %s", get.Stderr)
	assert.Check(t, strings.Contains(get.Stdout, classicSlug), "stdout: %s", get.Stdout)
}

// Refuses to overwrite an existing info.yml without --force.
func TestProjectLink_PreservesExisting(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectInfo("gh/myorg/alpha", fakes.ProjectInfo{
		ID:             "proj-uuid-1234",
		Slug:           "gh/myorg/alpha",
		OrganizationID: "org-uuid-5678",
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
