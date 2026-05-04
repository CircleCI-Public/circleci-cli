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

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

func setupDeployFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	// Register a project so --project can resolve to IDs.
	fake.AddProjectInfo("gh/myorg/alpha", map[string]any{
		"id":                "proj-uuid-1234",
		"slug":              "gh/myorg/alpha",
		"name":              "alpha",
		"organization_name": "myorg",
		"organization_slug": "gh/myorg",
		"organization_id":   "org-uuid-5678",
	})

	fake.AddRelease("proj-uuid-1234", map[string]any{
		"id":               "rel-uuid-0001",
		"project_id":       "proj-uuid-1234",
		"component_id":     "comp-uuid-1111",
		"component_name":   "web-frontend",
		"type":             "DEPLOYMENT",
		"status":           "SUCCESS",
		"target_version":   map[string]any{"name": "1.3.0"},
		"plan_is_rollback": false,
		"pipeline_id":      "pipe-uuid-aaa1",
		"workflow_id":      "wf-uuid-aaa1",
		"created_at":       "2026-04-28T14:30:00Z",
		"ended_at":         "2026-04-28T14:35:00Z",
	})
	fake.AddRelease("proj-uuid-1234", map[string]any{
		"id":               "rel-uuid-0002",
		"project_id":       "proj-uuid-1234",
		"component_id":     "comp-uuid-2222",
		"component_name":   "api-server",
		"type":             "DEPLOYMENT",
		"status":           "FAILED",
		"target_version":   map[string]any{"name": "2.0.1"},
		"failure_reason":   "timeout",
		"plan_is_rollback": false,
		"pipeline_id":      "pipe-uuid-bbb1",
		"workflow_id":      "wf-uuid-bbb1",
		"created_at":       "2026-04-27T09:15:00Z",
		"ended_at":         "2026-04-27T09:25:00Z",
	})
	fake.AddRelease("proj-uuid-1234", map[string]any{
		"id":               "rel-uuid-0003",
		"project_id":       "proj-uuid-1234",
		"component_id":     "comp-uuid-1111",
		"component_name":   "web-frontend",
		"type":             "ROLLBACK",
		"status":           "SUCCESS",
		"target_version":   map[string]any{"name": "1.2.0"},
		"plan_is_rollback": true,
		"pipeline_id":      "pipe-uuid-aaa2",
		"workflow_id":      "wf-uuid-aaa2",
		"created_at":       "2026-04-20T10:00:00Z",
		"ended_at":         "2026-04-20T10:05:00Z",
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
	fake.AddProjectInfo("gh/myorg/empty", map[string]any{
		"id":                "proj-uuid-empty",
		"slug":              "gh/myorg/empty",
		"name":              "empty",
		"organization_name": "myorg",
		"organization_slug": "gh/myorg",
		"organization_id":   "org-uuid-5678",
	})
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
