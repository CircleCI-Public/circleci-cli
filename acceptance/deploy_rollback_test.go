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
	"net/http"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	rollbackOrgID       = "a0000000-0000-4000-8000-0000000f0002"
	rollbackProjectID   = "a0000000-0000-4000-8000-0000000f0001"
	rollbackComponentID = "a0000000-0000-4000-8000-000000c00001"
	rollbackEnvID       = "a0000000-0000-4000-8000-000000e00001"
)

// setupRollbackFake registers one project with a component deployed to
// production at 1.3.0 and an earlier 1.2.0 to roll back to.
func setupRollbackFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddProjectBySlug("gh/myorg/alpha", rollbackProjectID, "alpha", rollbackOrgID)

	fake.AddComponent(fakes.DeployComponent{
		ID:        rollbackComponentID,
		OrgID:     rollbackOrgID,
		ProjectID: rollbackProjectID,
		Name:      "web-frontend",
		Type:      "service",
	})
	fake.AddEnvironment(fakes.DeployEnvironment{
		ID:    rollbackEnvID,
		OrgID: rollbackOrgID,
		Name:  "production",
	})

	// Most recently deployed first — the first is what --from defaults to.
	fake.AddComponentVersion(fakes.DeployComponentVersion{
		ID:            "a0000000-0000-4000-8000-000000d00001",
		ComponentID:   rollbackComponentID,
		EnvironmentID: rollbackEnvID,
		Name:          "1.3.0",
		CreatedAt:     "2026-04-28T14:00:00Z",
	})
	fake.AddComponentVersion(fakes.DeployComponentVersion{
		ID:            "a0000000-0000-4000-8000-000000d00002",
		ComponentID:   rollbackComponentID,
		EnvironmentID: rollbackEnvID,
		Name:          "1.2.0",
		CreatedAt:     "2026-04-20T10:00:00Z",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

// TestDeployRollback resolves the component and environment by name, infers the
// deployed version, and reports the pipeline run carrying the rollback out.
func TestDeployRollback(t *testing.T) {
	fake, env := setupRollbackFake(t)
	fake.SetRollback(fakes.RollbackResult{
		ID:           "a0000000-0000-4000-8000-000000f00001",
		RollbackType: "pipeline",
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", "web-frontend",
			"--environment", "production",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	var body map[string]any
	assert.NilError(t, fake.LastRequest().Decode(&body))
	assert.Check(t, cmp.DeepEqual(body, map[string]any{
		"component_id":    rollbackComponentID,
		"environment_id":  rollbackEnvID,
		"current_version": "1.3.0",
		"target_version":  "1.2.0",
	}))
}

// TestDeployRollback_AllFields sends every optional field, and shows that an
// agent rollback reports the release-agent command instead of a pipeline run.
func TestDeployRollback_AllFields(t *testing.T) {
	fake, env := setupRollbackFake(t)
	fake.SetRollback(fakes.RollbackResult{
		ID:           "a0000000-0000-4000-8000-000000a00001",
		RollbackType: "agent",
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", rollbackComponentID,
			"--environment", rollbackEnvID,
			"--from", "1.3.0",
			"--namespace", "team-a",
			"--reason", "bad release",
			"--param", "notify=true",
			"--param", "retries=3",
			"--param", "channel=ops",
			"--checkout-ref", "refs/heads/main",
			"--config-ref", "refs/heads/release",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	var body map[string]any
	assert.NilError(t, fake.LastRequest().Decode(&body))
	assert.Check(t, cmp.DeepEqual(body, map[string]any{
		"component_id":    rollbackComponentID,
		"environment_id":  rollbackEnvID,
		"namespace":       "team-a",
		"current_version": "1.3.0",
		"target_version":  "1.2.0",
		"reason":          "bad release",
		"parameters": map[string]any{
			"notify":  true,
			"retries": float64(3),
			"channel": "ops",
		},
		"checkout_ref": "refs/heads/main",
		"config_ref":   "refs/heads/release",
	}))
}

func TestDeployRollback_JSON(t *testing.T) {
	fake, env := setupRollbackFake(t)
	fake.SetRollback(fakes.RollbackResult{
		ID:           "a0000000-0000-4000-8000-000000f00001",
		RollbackType: "pipeline",
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", "web-frontend",
			"--environment", "production",
			"--force", "--json",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

// TestDeployRollback_RequiresForce covers the non-interactive path: a rollback
// changes what is deployed, so it is refused without --force.
func TestDeployRollback_RequiresForce(t *testing.T) {
	fake, env := setupRollbackFake(t)
	fake.SetRollback(fakes.RollbackResult{
		ID:           "a0000000-0000-4000-8000-000000f00001",
		RollbackType: "pipeline",
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", "web-frontend",
			"--environment", "production",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 6))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Equal(rollbacksRequested(fake), 0), "the rollback must not be sent")
}

// rollbacksRequested counts the rollback requests the fake received, so a test
// can show that a refused rollback never reached the API.
func rollbacksRequested(fake *fakes.CircleCI) int {
	n := 0
	for _, req := range fake.AllRequests() {
		if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/rollback") {
			n++
		}
	}
	return n
}

func TestDeployRollback_MissingComponent(t *testing.T) {
	_, env := setupRollbackFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--environment", "production",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestDeployRollback_MissingTargetVersion(t *testing.T) {
	_, env := setupRollbackFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback",
			"--project", "gh/myorg/alpha",
			"--component", "web-frontend",
			"--environment", "production",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestDeployRollback_UnknownComponent(t *testing.T) {
	_, env := setupRollbackFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", "no-such-component",
			"--environment", "production",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestDeployRollback_UnknownEnvironment(t *testing.T) {
	_, env := setupRollbackFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", "web-frontend",
			"--environment", "no-such-environment",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestDeployRollback_NoDeployedVersion covers an omitted --from for a component
// with no recorded version in the environment.
func TestDeployRollback_NoDeployedVersion(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectBySlug("gh/myorg/alpha", rollbackProjectID, "alpha", rollbackOrgID)
	fake.AddComponent(fakes.DeployComponent{
		ID:        rollbackComponentID,
		OrgID:     rollbackOrgID,
		ProjectID: rollbackProjectID,
		Name:      "web-frontend",
		Type:      "service",
	})
	fake.AddEnvironment(fakes.DeployEnvironment{
		ID:    rollbackEnvID,
		OrgID: rollbackOrgID,
		Name:  "production",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", "web-frontend",
			"--environment", "production",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestDeployRollback_InvalidParam(t *testing.T) {
	_, env := setupRollbackFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", "web-frontend",
			"--environment", "production",
			"--param", "notify",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestDeployRollback_Rejected covers the API disagreeing about the versions,
// which is what a stale --from produces.
func TestDeployRollback_Rejected(t *testing.T) {
	fake, env := setupRollbackFake(t)
	fake.SetRollback(fakes.RollbackResult{
		Status: http.StatusBadRequest,
		Title:  "Invalid version",
		Detail: "1.9.9 is not the version deployed to production",
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", "web-frontend",
			"--environment", "production",
			"--from", "1.9.9",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestDeployRollback_Conflict covers a component instance already handling a
// command.
func TestDeployRollback_Conflict(t *testing.T) {
	fake, env := setupRollbackFake(t)
	fake.SetRollback(fakes.RollbackResult{
		Status: http.StatusConflict,
		Title:  "Conflict",
		Detail: "a command is already being handled",
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", "web-frontend",
			"--environment", "production",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestDeployRollback_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"deploy", "rollback", "1.2.0",
			"--project", "gh/myorg/alpha",
			"--component", "web-frontend",
			"--environment", "production",
			"--force",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 3))
}
