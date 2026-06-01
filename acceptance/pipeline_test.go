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

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// keyDown is defined in login_test.go (same package).

const (
	pipelineProjectID = "proj-uuid-pipeline"
	pipelineDefID     = "pdef-uuid-0001"
	pipelineRepoID    = "987654321"
)

func fakePipelineDefPayload(id, name, configProvider, configRepoID, configFile, checkoutProvider, checkoutRepoID string) map[string]any {
	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	return map[string]any{
		"id":         id,
		"name":       name,
		"created_at": now.Format(time.RFC3339),
		"config_source": map[string]any{
			"provider":  configProvider,
			"file_path": configFile,
			"repo": map[string]any{
				"external_id": configRepoID,
				"full_name":   "myorg/myrepo",
			},
		},
		"checkout_source": map[string]any{
			"provider": checkoutProvider,
			"repo": map[string]any{
				"external_id": checkoutRepoID,
				"full_name":   "myorg/myrepo",
			},
		},
	}
}

func setupPipelineFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.AddProjectInfo("gh/myorg/myrepo", map[string]any{
		"id":   pipelineProjectID,
		"slug": "gh/myorg/myrepo",
		"name": "myrepo",
	})
	fake.SetCreatePipelineDefinitionResponse(pipelineProjectID,
		fakePipelineDefPayload(pipelineDefID, "my-pipeline", "github_app", pipelineRepoID, ".circleci/config.yml", "github_app", pipelineRepoID),
	)
	for _, d := range pipelineListFixtures {
		fake.AddPipelineDefinition(pipelineProjectID, d)
	}
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

var pipelineListFixtures = []map[string]any{
	{
		"id":         "pdef-uuid-0001",
		"name":       "main-pipeline",
		"created_at": "2024-01-01T00:00:00Z",
		"config_source": map[string]any{
			"provider":  "github_app",
			"file_path": ".circleci/config.yml",
			"repo":      map[string]any{"external_id": "111111111", "full_name": "myorg/myrepo"},
		},
		"checkout_source": map[string]any{
			"provider": "github_app",
			"repo":     map[string]any{"external_id": "111111111", "full_name": "myorg/myrepo"},
		},
	},
	{
		"id":          "pdef-uuid-0002",
		"name":        "nightly",
		"description": "Runs every night",
		"created_at":  "2024-02-01T00:00:00Z",
		"config_source": map[string]any{
			"provider":  "github_app",
			"file_path": ".circleci/nightly.yml",
			"repo":      map[string]any{"external_id": "111111111", "full_name": "myorg/myrepo"},
		},
		"checkout_source": map[string]any{
			"provider": "github_app",
			"repo":     map[string]any{"external_id": "111111111", "full_name": "myorg/myrepo"},
		},
	},
}

// --- pipeline create ---

func TestPipelineCreate(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "create",
			"--project", "gh/myorg/myrepo",
			"--name", "my-pipeline",
			"--config-provider", "github_app",
			"--config-repo-id", pipelineRepoID,
			"--config-file", ".circleci/config.yml",
			"--checkout-provider", "github_app",
			"--checkout-repo-id", pipelineRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, pipelineDefID))
}

func TestPipelineCreate_JSON(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "create",
			"--project", "gh/myorg/myrepo",
			"--name", "my-pipeline",
			"--config-provider", "github_app",
			"--config-repo-id", pipelineRepoID,
			"--config-file", ".circleci/config.yml",
			"--checkout-provider", "github_app",
			"--checkout-repo-id", pipelineRepoID,
			"--json",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], pipelineDefID))
	assert.Check(t, cmp.Equal(out["name"], "my-pipeline"))
}

func TestPipelineCreate_DirectProjectID(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "create",
			"--project-id", pipelineProjectID,
			"--name", "my-pipeline",
			"--config-provider", "github_app",
			"--config-repo-id", pipelineRepoID,
			"--config-file", ".circleci/config.yml",
			"--checkout-provider", "github_app",
			"--checkout-repo-id", pipelineRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, pipelineDefID))
}

func TestPipelineCreate_MissingName(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "create",
			"--project", "gh/myorg/myrepo",
			"--config-provider", "github_app",
			"--config-repo-id", pipelineRepoID,
			"--config-file", ".circleci/config.yml",
			"--checkout-provider", "github_app",
			"--checkout-repo-id", pipelineRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "--name"))
}

func TestPipelineCreate_MissingConfigProvider(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "create",
			"--project", "gh/myorg/myrepo",
			"--name", "my-pipeline",
			"--config-repo-id", pipelineRepoID,
			"--config-file", ".circleci/config.yml",
			"--checkout-provider", "github_app",
			"--checkout-repo-id", pipelineRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "--config-provider"))
}

func TestPipelineCreate_InvalidConfigProvider(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "create",
			"--project", "gh/myorg/myrepo",
			"--name", "my-pipeline",
			"--config-provider", "bitbucket",
			"--config-repo-id", pipelineRepoID,
			"--config-file", ".circleci/config.yml",
			"--checkout-provider", "github_app",
			"--checkout-repo-id", pipelineRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "bitbucket"))
}

func TestPipelineCreate_InvalidCheckoutProvider(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "create",
			"--project", "gh/myorg/myrepo",
			"--name", "my-pipeline",
			"--config-provider", "github_app",
			"--config-repo-id", pipelineRepoID,
			"--config-file", ".circleci/config.yml",
			"--checkout-provider", "circleci",
			"--checkout-repo-id", pipelineRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "circleci"))
}

func TestPipelineCreate_ProjectNotFound(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "create",
			"--project", "gh/myorg/nonexistent",
			"--name", "my-pipeline",
			"--config-provider", "github_app",
			"--config-repo-id", pipelineRepoID,
			"--config-file", ".circleci/config.yml",
			"--checkout-provider", "github_app",
			"--checkout-repo-id", pipelineRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
}

func TestPipelineCreate_Interactive_DefaultConfigPath(t *testing.T) {
	_, env := setupPipelineFake(t)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "create",
			"--project", "gh/myorg/myrepo",
			"--name", "my-pipeline",
			"--config-provider", "github_app",
			"--config-repo-id", pipelineRepoID,
			// Skip --config-file to trigger prompt
			"--checkout-provider", "github_app",
			"--checkout-repo-id", pipelineRepoID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// Step 1: verify the default config file path is shown in the footer and accept it.
	_, err := console.ExpectString("Config file path")
	assert.NilError(t, err)
	_, err = console.ExpectString("(default: .circleci/config.yml · enter to confirm, esc to cancel)")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("Pipeline definition created")
	assert.NilError(t, err)
}

// --- pipeline list ---

func TestPipelineList(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "list", "--project-id", pipelineProjectID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "pdef-uuid-0001"))
	assert.Check(t, strings.Contains(result.Stdout, "main-pipeline"))
	assert.Check(t, strings.Contains(result.Stdout, "pdef-uuid-0002"))
	assert.Check(t, strings.Contains(result.Stdout, "nightly"))
}

func TestPipelineList_JSON(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "list", "--project-id", pipelineProjectID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["id"], "pdef-uuid-0001"))
	assert.Check(t, cmp.Equal(out[1]["name"], "nightly"))
}

func TestPipelineList_BySlug(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "list", "--project", "gh/myorg/myrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "pdef-uuid-0001"))
}

func TestPipelineList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectInfo("gh/myorg/empty", map[string]any{
		"id":   "proj-empty",
		"slug": "gh/myorg/empty",
		"name": "empty",
	})
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "list", "--project", "gh/myorg/empty"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "No pipeline definitions found"))
}

func TestPipelineList_ProjectNotFound(t *testing.T) {
	_, env := setupPipelineFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "list", "--project", "gh/myorg/nonexistent"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
}

// --- pipeline run ---

const (
	pipelineRunID     = "run-uuid-0001"
	pipelineRunNumber = 42
)

func setupPipelineRunFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.SetTriggerPipelineRunResponse("gh/myorg/myrepo", map[string]any{
		"id":         pipelineRunID,
		"state":      "created",
		"number":     pipelineRunNumber,
		"created_at": "2024-06-01T00:00:00Z",
	})
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

func TestPipelineRun(t *testing.T) {
	_, env := setupPipelineRunFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", "gh/myorg/myrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, pipelineRunID))
	assert.Check(t, strings.Contains(result.Stdout, "42"))
}

func TestPipelineRun_JSON(t *testing.T) {
	_, env := setupPipelineRunFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", "gh/myorg/myrepo", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], pipelineRunID))
	assert.Check(t, cmp.Equal(out["state"], "created"))
	assert.Check(t, cmp.Equal(out["triggered"], true))
}

func TestPipelineRun_WithBranch(t *testing.T) {
	_, env := setupPipelineRunFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", "gh/myorg/myrepo", "--branch", "feature/my-branch"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, pipelineRunID))
}

func TestPipelineRun_WithDefinitionID(t *testing.T) {
	_, env := setupPipelineRunFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "run",
			"--project", "gh/myorg/myrepo",
			"--definition-id", pipelineDefID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, pipelineRunID))
}

func TestPipelineRun_WithParams(t *testing.T) {
	_, env := setupPipelineRunFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "run",
			"--project", "gh/myorg/myrepo",
			"--param", "deploy_env=staging",
			"--param", "run_tests=true",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, pipelineRunID))
}

func TestPipelineRun_MutuallyExclusiveBranchAndTag(t *testing.T) {
	_, env := setupPipelineRunFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"pipeline", "run",
			"--project", "gh/myorg/myrepo",
			"--branch", "main",
			"--tag", "v1.0",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "--branch"))
	assert.Check(t, strings.Contains(result.Stderr, "--tag"))
}

func TestPipelineRun_Skipped(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetTriggerPipelineRunSkipped("gh/myorg/myrepo", "Ignoring pipeline due to CI skip in the commit")
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", "gh/myorg/myrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "not triggered"))
	assert.Check(t, strings.Contains(result.Stdout, "CI skip"))
}

func TestPipelineRun_Skipped_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetTriggerPipelineRunSkipped("gh/myorg/myrepo", "Ignoring pipeline due to CI skip in the commit")
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", "gh/myorg/myrepo", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["triggered"], false))
	assert.Check(t, strings.Contains(out["message"].(string), "CI skip"))
}

func TestPipelineRun_ProjectNotFound(t *testing.T) {
	_, env := setupPipelineRunFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", "gh/myorg/nonexistent"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
}

func TestPipelineRun_InvalidParam(t *testing.T) {
	_, env := setupPipelineRunFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", "gh/myorg/myrepo", "--param", "notakeyvalue"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
}

// --- pipeline run interactive ---

// setupPipelineRunInteractiveFake builds a fake with project info, pipeline
// definitions, and a trigger run response — everything needed for the
// interactive definition-selection prompt.
func setupPipelineRunInteractiveFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.AddProjectInfo("gh/myorg/myrepo", map[string]any{
		"id":   pipelineProjectID,
		"slug": "gh/myorg/myrepo",
		"name": "myrepo",
		"vcs_info": map[string]any{
			"provider":       "GitHub",
			"default_branch": "main",
			"vcs_url":        "https://github.com/myorg/myrepo",
		},
	})
	for _, d := range pipelineListFixtures {
		fake.AddPipelineDefinition(pipelineProjectID, d)
	}
	fake.SetTriggerPipelineRunResponse("gh/myorg/myrepo", map[string]any{
		"id":         pipelineRunID,
		"state":      "created",
		"number":     pipelineRunNumber,
		"created_at": "2024-06-01T00:00:00Z",
	})
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

func TestPipelineRun_Interactive_SelectFirst(t *testing.T) {
	_, env := setupPipelineRunInteractiveFake(t)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", "gh/myorg/myrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// Step 1: select the first pipeline definition (main-pipeline).
	_, err := console.ExpectString("Select a pipeline definition")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	// Step 2: accept the default branch (main).
	_, err = console.ExpectString("Branch to run on")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("Pipeline #42 triggered")
	assert.NilError(t, err)
}

func TestPipelineRun_Interactive_SkipDefinition(t *testing.T) {
	_, env := setupPipelineRunInteractiveFake(t)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", "gh/myorg/myrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// Step 1: navigate past both definitions to "None" (2 × down, Enter).
	_, err := console.ExpectString("Select a pipeline definition")
	assert.NilError(t, err)
	_, err = console.Send(keyDown + keyDown + "\r")
	assert.NilError(t, err)

	// Step 2: accept the default branch (main).
	_, err = console.ExpectString("Branch to run on")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("Pipeline #42 triggered")
	assert.NilError(t, err)
}

func TestPipelineRun_Interactive_CustomBranch(t *testing.T) {
	_, env := setupPipelineRunInteractiveFake(t)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", "gh/myorg/myrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// Step 1: select the first pipeline definition.
	_, err := console.ExpectString("Select a pipeline definition")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	// Step 2: navigate to "Other..." and type a custom branch.
	_, err = console.ExpectString("Branch to run on")
	assert.NilError(t, err)
	_, err = console.Send(keyDown + "\r") // move to "Other..."
	assert.NilError(t, err)

	_, err = console.ExpectString("Branch name")
	assert.NilError(t, err)
	_, err = console.Send("feature/my-branch\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("Pipeline #42 triggered")
	assert.NilError(t, err)
}
