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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

func setupDeployFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	// Register a project so --project can resolve to IDs.
	fake.AddProjectBySlug("gh/myorg/alpha", "a0000000-0000-4000-8000-0000000f0001", "alpha", "a0000000-0000-4000-8000-0000000f0002")

	fake.AddDeployment("a0000000-0000-4000-8000-0000000f0001", map[string]any{
		"id": "a0000000-0000-4000-8000-000000000001",
		"attributes": map[string]any{
			"type":           "deployment",
			"status":         "succeeded",
			"target_version": map[string]any{"name": "1.3.0"},
			"is_rollback":    false,
			"created_at":     "2026-04-28T14:30:00Z",
			"ended_at":       "2026-04-28T14:35:00Z",
		},
		"references": map[string]any{
			"deploy_component": map[string]any{
				"id":         "a0000000-0000-4000-8000-000000c00001",
				"attributes": map[string]any{"name": "web-frontend"},
			},
			"deploy_environment": map[string]any{"id": "a0000000-0000-4000-8000-000000e00001"},
			"pipeline":           map[string]any{"id": "a0000000-0000-4000-8000-000000f00001"},
			"workflow":           map[string]any{"id": "a0000000-0000-4000-8000-000000f00002"},
		},
	})
	fake.AddDeployment("a0000000-0000-4000-8000-0000000f0001", map[string]any{
		"id": "a0000000-0000-4000-8000-000000000002",
		"attributes": map[string]any{
			"type":           "deployment",
			"status":         "failed",
			"failure_reason": "timeout",
			"target_version": map[string]any{"name": "2.0.1"},
			"is_rollback":    false,
			"created_at":     "2026-04-27T09:15:00Z",
			"ended_at":       "2026-04-27T09:25:00Z",
		},
		"references": map[string]any{
			"deploy_component": map[string]any{
				"id":         "a0000000-0000-4000-8000-000000c00002",
				"attributes": map[string]any{"name": "api-server"},
			},
			"deploy_environment": map[string]any{"id": "a0000000-0000-4000-8000-000000e00001"},
			"pipeline":           map[string]any{"id": "a0000000-0000-4000-8000-000000f00003"},
			"workflow":           map[string]any{"id": "a0000000-0000-4000-8000-000000f00004"},
		},
	})
	fake.AddDeployment("a0000000-0000-4000-8000-0000000f0001", map[string]any{
		"id": "a0000000-0000-4000-8000-000000000003",
		"attributes": map[string]any{
			"type":           "deployment",
			"status":         "succeeded",
			"target_version": map[string]any{"name": "1.2.0"},
			"is_rollback":    true,
			"created_at":     "2026-04-20T10:00:00Z",
			"ended_at":       "2026-04-20T10:05:00Z",
		},
		"references": map[string]any{
			"deploy_component": map[string]any{
				"id":         "a0000000-0000-4000-8000-000000c00001",
				"attributes": map[string]any{"name": "web-frontend"},
			},
			"deploy_environment": map[string]any{"id": "a0000000-0000-4000-8000-000000e00001"},
			"pipeline":           map[string]any{"id": "a0000000-0000-4000-8000-000000f00005"},
			"workflow":           map[string]any{"id": "a0000000-0000-4000-8000-000000f00006"},
		},
	})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return fake, env
}

func TestDeployList(t *testing.T) {
	_, env := setupDeployFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestDeployList_JSON(t *testing.T) {
	_, env := setupDeployFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "list", "--project", "gh/myorg/alpha", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(len(out), 3))
	assert.Check(t, cmp.Equal(out[0]["component_name"], "web-frontend"))
	assert.Check(t, cmp.Equal(out[0]["version"], "1.3.0"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestDeployList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectBySlug("gh/myorg/empty", "a0000000-0000-4000-8000-0000000f0003", "empty", "a0000000-0000-4000-8000-0000000f0002")
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "list", "--project", "gh/myorg/empty"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestDeployList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
}

// --- deploy environment ---

func setupDeployEnvironmentFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	const orgID = "a0000000-0000-4000-8000-0000000f0002"
	fake.AddOrg(orgID, "gh/myorg", "myorg", "github")

	fake.AddEnvironment(orgID, map[string]any{
		"id": "a0000000-0000-4000-8000-000000e00001",
		"attributes": map[string]any{
			"name": "production",
		},
		"references": map[string]any{
			"org": map[string]any{"id": orgID},
		},
	})
	fake.AddEnvironment(orgID, map[string]any{
		"id": "a0000000-0000-4000-8000-000000e00002",
		"attributes": map[string]any{
			"name": "staging",
		},
		"references": map[string]any{
			"org": map[string]any{"id": orgID},
		},
	})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return fake, env
}

func TestDeployEnvironmentList(t *testing.T) {
	_, env := setupDeployEnvironmentFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "environment", "list", "--org", "gh/myorg"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestDeployEnvironmentList_JSON(t *testing.T) {
	_, env := setupDeployEnvironmentFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "environment", "list", "--org", "gh/myorg", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestDeployEnvironmentGet(t *testing.T) {
	_, env := setupDeployEnvironmentFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "environment", "get", "a0000000-0000-4000-8000-000000e00001"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- deploy component ---

func setupDeployComponentFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	const (
		orgID     = "a0000000-0000-4000-8000-0000000f0002"
		projectID = "a0000000-0000-4000-8000-0000000f0001"
	)
	fake.AddProjectBySlug("gh/myorg/alpha", projectID, "alpha", orgID)

	fake.AddComponent(orgID, map[string]any{
		"id": "a0000000-0000-4000-8000-000000c00001",
		"attributes": map[string]any{
			"name": "web-frontend",
			"type": "service",
		},
		"references": map[string]any{
			"project": map[string]any{"id": projectID},
		},
	})
	fake.AddComponent(orgID, map[string]any{
		"id": "a0000000-0000-4000-8000-000000c00002",
		"attributes": map[string]any{
			"name": "api-server",
			"type": "service",
		},
		"references": map[string]any{
			"project": map[string]any{"id": projectID},
		},
	})

	fake.AddComponentVersion("a0000000-0000-4000-8000-000000c00001", map[string]any{
		"id": "a0000000-0000-4000-8000-000000v00001",
		"attributes": map[string]any{
			"name":       "1.3.0",
			"created_at": "2026-04-28T14:00:00Z",
		},
		"references": map[string]any{
			"component": map[string]any{"id": "a0000000-0000-4000-8000-000000c00001"},
		},
	})
	fake.AddComponentVersion("a0000000-0000-4000-8000-000000c00001", map[string]any{
		"id": "a0000000-0000-4000-8000-000000v00002",
		"attributes": map[string]any{
			"name":       "1.2.0",
			"created_at": "2026-04-20T10:00:00Z",
		},
		"references": map[string]any{
			"component": map[string]any{"id": "a0000000-0000-4000-8000-000000c00001"},
		},
	})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return fake, env
}

func TestDeployComponentList(t *testing.T) {
	_, env := setupDeployComponentFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "component", "list", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestDeployComponentList_JSON(t *testing.T) {
	_, env := setupDeployComponentFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "component", "list", "--project", "gh/myorg/alpha", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestDeployComponentGet(t *testing.T) {
	_, env := setupDeployComponentFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "component", "get", "a0000000-0000-4000-8000-000000c00001"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestDeployVersionList(t *testing.T) {
	_, env := setupDeployComponentFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "version", "list", "a0000000-0000-4000-8000-000000c00001"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- deploy settings ---

func setupDeploySettingsFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	const (
		orgID     = "a0000000-0000-4000-8000-0000000f0002"
		projectID = "a0000000-0000-4000-8000-0000000f0001"
	)
	fake.AddProjectBySlug("gh/myorg/alpha", projectID, "alpha", orgID)

	fake.SetDeploySettings(projectID, map[string]any{
		"id": "a0000000-0000-4000-8000-000000s00001",
		"attributes": map[string]any{
			"auto_cancel_redundant_deploys": true,
		},
		"references": map[string]any{
			"project": map[string]any{"id": projectID},
		},
	})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return fake, env
}

func TestDeploySettings(t *testing.T) {
	_, env := setupDeploySettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "settings", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestDeploySettings_JSON(t *testing.T) {
	_, env := setupDeploySettingsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "settings", "--project", "gh/myorg/alpha", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestDeploySettings_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectBySlug("gh/myorg/alpha", "a0000000-0000-4000-8000-0000000f0001", "alpha", "a0000000-0000-4000-8000-0000000f0002")
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"deploy", "settings", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
