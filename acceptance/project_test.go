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
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

func setupProjectFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddFollowedProject(map[string]any{
		"slug":     "gh/myorg/alpha",
		"username": "myorg",
		"reponame": "alpha",
		"vcs_type": "github",
		"name":     "alpha",
	})
	fake.AddFollowedProject(map[string]any{
		"slug":     "gh/myorg/beta",
		"username": "myorg",
		"reponame": "beta",
		"vcs_type": "github",
		"name":     "beta",
	})
	fake.AddProjectInfo("gh/myorg/alpha", map[string]any{
		"id":                "proj-uuid-1234",
		"slug":              "gh/myorg/alpha",
		"name":              "alpha",
		"organization_name": "myorg",
		"organization_slug": "gh/myorg",
		"organization_id":   "org-uuid-5678",
		"vcs_info": map[string]any{
			"provider":       "GitHub",
			"default_branch": "main",
			"vcs_url":        "https://github.com/myorg/alpha",
		},
	})

	fake.AddEnvVar("gh/myorg/alpha", "DATABASE_URL", "xxxx", nil)
	fake.AddEnvVar("gh/myorg/alpha", "SECRET_KEY", "xxxx",
		new(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- project list ---

func TestProjectList(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectList_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectList_JSON(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["slug"], "gh/myorg/alpha"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))

}

func TestProjectList_JQ(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "list", "--json", "--jq", ".[0].slug"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(strings.TrimSpace(result.Stdout), "gh/myorg/alpha"))
}

func TestProjectList_JSON_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestProjectList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestProjectList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- project follow ---

func TestProjectFollow(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "follow", "--project", "gh/myorg/newrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectFollow_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "follow", "--project", "gh/myorg/newrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectFollow_Idempotent(t *testing.T) {
	_, env := setupProjectFake(t)

	// Follow an already-followed project — should succeed.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "follow", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
}

func TestProjectFollow_InvalidSlug(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "follow", "--project", "notaslug"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- env list (top-level alias) ---

func TestEnvList(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvList_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvList_JSON(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "list", "--project", "gh/myorg/alpha", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["name"], "DATABASE_URL"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestEnvList_JSON_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "list", "--project", "gh/myorg/alpha", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestEnvList_Empty(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "list", "--project", "gh/myorg/beta"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// Also accessible via the deep path.
func TestProjectEnvList(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "envvar", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectEnvList_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "envvar", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- env set ---

func TestEnvSet(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "set", "NEW_VAR", "newvalue", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvSet_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "set", "NEW_VAR", "newvalue", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvSet_Overwrite(t *testing.T) {
	_, env := setupProjectFake(t)

	// Overwrite existing var.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "set", "DATABASE_URL", "postgres://new", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvSet_Overwrite_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "set", "DATABASE_URL", "postgres://new", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- env delete ---

func TestEnvDelete(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "delete", "--force", "DATABASE_URL", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvDelete_Color(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "delete", "--force", "DATABASE_URL", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEnvDelete_RequiresForce(t *testing.T) {
	// In non-interactive mode (no TTY), --force is required.
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "delete", "DATABASE_URL", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr) // ExitCancelled
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestEnvDelete_NotFound(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "delete", "--force", "DOES_NOT_EXIST", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestEnvDelete_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"envvar", "delete", "--force", "FOO", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
}

// --- project get ---

func TestProjectGet(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "get", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectGet_JSON(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "get", "--project", "gh/myorg/alpha", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], "proj-uuid-1234"))
	assert.Check(t, cmp.Equal(out["organization_id"], "org-uuid-5678"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestProjectGet_NotFound(t *testing.T) {
	_, env := setupProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "get", "--project", "gh/myorg/nonexistent"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- project trigger create ---

const (
	triggerProjectID     = "proj-uuid-1234"
	triggerPipelineDefID = "pdef-uuid-5678"
	triggerRepoID        = "987654321"
	triggerID            = "trig-uuid-abcd"
)

var triggerFixture = map[string]any{
	"id":         triggerID,
	"created_at": "2026-01-01T00:00:00Z",
	"event_source": map[string]any{
		"provider": "github_app",
		"repo": map[string]any{
			"external_id": triggerRepoID,
			"full_name":   "myorg/myrepo",
		},
	},
	"event_preset": "all-pushes",
}

func setupTriggerFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake, env := setupProjectFake(t)
	fake.SetCreateTriggerResponse(triggerProjectID, triggerPipelineDefID, triggerFixture)
	fake.AddTrigger(triggerProjectID, triggerPipelineDefID, triggerFixture)
	return fake, env
}

// --- project trigger list ---

func TestProjectTriggerList(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "list",
			"--project", "gh/myorg/alpha",
			"--pipeline-definition-id", triggerPipelineDefID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, triggerID))
}

func TestProjectTriggerList_JSON(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "list",
			"--project", "gh/myorg/alpha",
			"--pipeline-definition-id", triggerPipelineDefID,
			"--json",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 1))
	assert.Check(t, cmp.Equal(out[0]["id"], triggerID))
	assert.Check(t, cmp.Equal(out[0]["event_preset"], "all-pushes"))
}

func TestProjectTriggerList_Empty(t *testing.T) {
	fake, env := setupProjectFake(t)
	// Register project but no triggers
	fake.AddProjectInfo("gh/myorg/beta", map[string]any{
		"id":   "proj-uuid-beta",
		"slug": "gh/myorg/beta",
		"name": "beta",
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "list",
			"--project-id", "proj-uuid-beta",
			"--pipeline-definition-id", triggerPipelineDefID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "No triggers found."))
}

func TestProjectTriggerList_MissingPipelineDefinitionID(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "trigger", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "--pipeline-definition-id"))
}

// --- project trigger create ---

func TestProjectTriggerCreate(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "create",
			"--project", "gh/myorg/alpha",
			"--pipeline-definition-id", triggerPipelineDefID,
			"--repo-id", triggerRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, triggerID))
}

func TestProjectTriggerCreate_JSON(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "create",
			"--project", "gh/myorg/alpha",
			"--pipeline-definition-id", triggerPipelineDefID,
			"--repo-id", triggerRepoID,
			"--json",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], triggerID))
}

func TestProjectTriggerCreate_MissingPipelineDefinitionID(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "create",
			"--project", "gh/myorg/alpha",
			"--repo-id", triggerRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "--pipeline-definition-id"))
}

func TestProjectTriggerCreate_MissingRepoID(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "create",
			"--project", "gh/myorg/alpha",
			"--pipeline-definition-id", triggerPipelineDefID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "--repo-id"))
}

func TestProjectTriggerCreate_ProjectNotFound(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "create",
			"--project", "gh/myorg/nonexistent",
			"--pipeline-definition-id", triggerPipelineDefID,
			"--repo-id", triggerRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
}

func TestProjectTriggerCreate_InvalidEventPreset(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "create",
			"--project", "gh/myorg/alpha",
			"--pipeline-definition-id", triggerPipelineDefID,
			"--repo-id", triggerRepoID,
			"--event-preset", "push-only",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "push-only"))
}

func TestProjectTriggerCreate_InvalidProvider(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "create",
			"--project", "gh/myorg/alpha",
			"--pipeline-definition-id", triggerPipelineDefID,
			"--repo-id", triggerRepoID,
			"--provider", "bitbucket",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "bitbucket"))
}

func TestProjectTriggerCreate_DirectProjectID(t *testing.T) {
	_, env := setupTriggerFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"project", "trigger", "create",
			"--project-id", triggerProjectID,
			"--pipeline-definition-id", triggerPipelineDefID,
			"--repo-id", triggerRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, triggerID))
}

// --- project create ---

func setupCreateProjectFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake, env := setupProjectFake(t)
	fake.SetCreateProjectResponse(map[string]any{
		"id":                "proj-new-uuid",
		"slug":              "gh/myorg/my-new-repo",
		"name":              "my-new-repo",
		"organization_name": "myorg",
		"organization_slug": "gh/myorg",
		"organization_id":   "org-uuid-5678",
		"vcs_info": map[string]any{
			"provider":       "GitHub",
			"default_branch": "main",
			"vcs_url":        "https://github.com/myorg/my-new-repo",
		},
	})
	fake.SetCollaborations([]any{
		map[string]any{"id": "org-uuid-5678", "name": "myorg", "slug": "gh/myorg", "vcs_type": "github"},
		map[string]any{"id": "org-uuid-9999", "name": "other-org", "slug": "gh/other-org", "vcs_type": "github"},
	})
	return fake, env
}

func TestProjectCreate(t *testing.T) {
	_, env := setupCreateProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "create", "my-new-repo", "--org", "gh/myorg"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "my-new-repo"))
	assert.Check(t, strings.Contains(result.Stdout, "Pipelines:"))
	assert.Check(t, strings.Contains(result.Stdout, "/pipelines/gh/myorg/my-new-repo"))
}

func TestProjectCreate_Color(t *testing.T) {
	_, env := setupCreateProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "create", "my-new-repo", "--org", "gh/myorg"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, strings.Contains(result.Stdout, "my-new-repo"))
	assert.Check(t, strings.Contains(result.Stdout, "Pipelines:"))
	assert.Check(t, strings.Contains(result.Stdout, "/pipelines/gh/myorg/my-new-repo"))
}

func TestProjectCreate_JSON(t *testing.T) {
	_, env := setupCreateProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "create", "my-new-repo", "--org", "gh/myorg", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], "proj-new-uuid"))
	assert.Check(t, cmp.Equal(out["slug"], "gh/myorg/my-new-repo"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestProjectCreate_MissingOrg(t *testing.T) {
	_, env := setupCreateProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "create", "my-new-repo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "--org"))
}

func TestProjectCreate_InvalidOrg(t *testing.T) {
	_, env := setupCreateProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "create", "my-new-repo", "--org", "notaslug"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "notaslug"))
}

func TestProjectCreate_APIError(t *testing.T) {
	fake, env := setupProjectFake(t) // no SetCreateProjectResponse → fake returns 422
	_ = fake

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "create", "my-new-repo", "--org", "gh/myorg"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 4, "stderr: %s", result.Stderr)
	assert.Check(t, len(result.Stderr) > 0)
}

func TestProjectCreate_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "create", "my-new-repo", "--org", "gh/myorg"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
}

func TestProjectCreate_NoArgs_NoGitRepo(t *testing.T) {
	_, env := setupCreateProjectFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "create", "--org", "gh/myorg"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "project name"))
}
