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

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

// fakePipeline returns a minimal pipeline payload for the fake server.
func fakePipeline(id string, number int, state, slug, branch string) map[string]any {
	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	return map[string]any{
		"id":           id,
		"state":        state,
		"number":       number,
		"project_slug": slug,
		"created_at":   now.Format(time.RFC3339),
		"updated_at":   now.Format(time.RFC3339),
		"trigger": map[string]any{
			"type":        "webhook",
			"received_at": now.Format(time.RFC3339),
			"actor":       map[string]any{"login": "testuser", "avatar_url": ""},
		},
		"vcs": map[string]any{
			"provider_name":         "GitHub",
			"origin_repository_url": "https://github.com/testorg/testrepo",
			"target_repository_url": "https://github.com/testorg/testrepo",
			"revision":              "abc1234def5678",
			"branch":                branch,
		},
	}
}

func TestPipelineGet_ByID(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	pipelineID := "5034460f-c7c4-4c43-9457-de07e2029e7b"
	wfID := "wf-uuid-001"
	fake.AddPipeline(pipelineID, fakePipeline(pipelineID, 42, "created", "gh/testorg/testrepo", "main"))
	fake.AddPipelineWorkflows(pipelineID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID,
		fakeJob("job-uuid-1", "run-tests", 101, "gh/testorg/testrepo"),
		fakeJob("job-uuid-2", "deploy", 102, "gh/testorg/testrepo"),
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, []string{"pipeline", "get", pipelineID}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestPipelineGet_ByNumber(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	pipelineID := "5034460f-c7c4-4c43-9457-de07e2029e7b"
	wfID := "wf-uuid-002"
	slug := "gh/testorg/testrepo"
	p := fakePipeline(pipelineID, 42, "created", slug, "main")
	fake.AddPipeline(pipelineID, p)
	fake.AddProjectPipelines(slug, p)
	fake.AddPipelineWorkflows(pipelineID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID, fakeJob("job-uuid-1", "run-tests", 101, slug))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"pipeline", "get", "42", "--project", slug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestPipelineGet_ByID_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	pipelineID := "5034460f-c7c4-4c43-9457-de07e2029e7b"
	wfID := "wf-uuid-001"
	fake.AddPipeline(pipelineID, fakePipeline(pipelineID, 42, "created", "gh/testorg/testrepo", "main"))
	fake.AddPipelineWorkflows(pipelineID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID, fakeJob("job-uuid-1", "run-tests", 101, "gh/testorg/testrepo"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, []string{"pipeline", "get", "--json", pipelineID}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], pipelineID))
	assert.Check(t, cmp.Equal(out["status"], "success"))
	assert.Check(t, cmp.Equal(out["project_slug"], "gh/testorg/testrepo"))

	wfs := out["workflows"].([]any)
	assert.Check(t, cmp.Len(wfs, 1))
	jobs := wfs[0].(map[string]any)["jobs"].([]any)
	assert.Check(t, cmp.Len(jobs, 1))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["name"], "run-tests"))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["number"], float64(101)))
}

func TestPipelineGet_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"pipeline", "get", "00000000-0000-0000-0000-000000000000"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5) // ExitNotFound
	assert.Check(t, cmp.Contains(result.Stderr, "No pipeline found"), "stderr: %s", result.Stderr)
}

func TestPipelineGet_NoToken(t *testing.T) {
	env := testenv.New(t)
	// No token set

	result := binary.RunCLI(t, []string{"pipeline", "get", "any-id"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
	assert.Check(t, cmp.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}

// --- pagination ---

func TestPipelineGet_PaginatedWorkflowJobs(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	pipelineID := "paginated-pipeline-001"
	wfID := "paginated-wf-001"
	slug := "gh/testorg/testrepo"

	fake.AddPipeline(pipelineID, fakePipeline(pipelineID, 99, "created", slug, "main"))
	fake.AddPipelineWorkflows(pipelineID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID,
		fakeJob("job-1", "lint", 1, slug),
		fakeJob("job-2", "test", 2, slug),
		fakeJob("job-3", "build", 3, slug),
		fakeJob("job-4", "deploy", 4, slug),
		fakeJob("job-5", "notify", 5, slug),
	)
	fake.SetWorkflowJobsPageSize(2) // 5 jobs → 3 pages (2, 2, 1)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, []string{"pipeline", "get", "--json", pipelineID}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	wfs := out["workflows"].([]any)
	assert.Check(t, cmp.Len(wfs, 1))
	jobs := wfs[0].(map[string]any)["jobs"].([]any)
	assert.Check(t, cmp.Len(jobs, 5), "expected all 5 paginated jobs, got %d", len(jobs))

	names := make([]string, len(jobs))
	for i, j := range jobs {
		names[i] = j.(map[string]any)["name"].(string)
	}
	assert.Check(t, cmp.DeepEqual(names, []string{"lint", "test", "build", "deploy", "notify"}))
}

func TestPipelineGet_PaginatedWorkflows(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	pipelineID := "paginated-pipeline-002"
	slug := "gh/testorg/testrepo"

	fake.AddPipeline(pipelineID, fakePipeline(pipelineID, 100, "created", slug, "main"))
	fake.AddPipelineWorkflows(pipelineID,
		fakeWorkflow("wf-p-1", "build"),
		fakeWorkflow("wf-p-2", "test"),
		fakeWorkflow("wf-p-3", "deploy"),
	)
	fake.SetPipelineWorkflowsPageSize(2) // 3 workflows → 2 pages (2, 1)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, []string{"pipeline", "get", "--json", pipelineID}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	wfs := out["workflows"].([]any)
	assert.Check(t, cmp.Len(wfs, 3), "expected all 3 paginated workflows, got %d", len(wfs))

	names := make([]string, len(wfs))
	for i, w := range wfs {
		names[i] = w.(map[string]any)["name"].(string)
	}
	assert.Check(t, cmp.DeepEqual(names, []string{"build", "test", "deploy"}))
}

// --- pipeline list ---

func TestPipelineList(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := "gh/testorg/testrepo"
	fake.AddProjectPipelines(slug,
		fakePipeline("pid-1", 10, "created", slug, "main"),
		fakePipeline("pid-2", 9, "errored", slug, "feature"),
		fakePipeline("pid-3", 8, "created", slug, "main"),
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"pipeline", "list", "--project", slug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "#10"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "#9"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "errored"), "stdout: %s", result.Stdout)
}

func TestPipelineList_Limit(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := "gh/testorg/testrepo"
	fake.AddProjectPipelines(slug,
		fakePipeline("pid-1", 10, "created", slug, "main"),
		fakePipeline("pid-2", 9, "created", slug, "main"),
		fakePipeline("pid-3", 8, "created", slug, "main"),
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"pipeline", "list", "--project", slug, "--limit", "2"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "#10"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "#9"), "stdout: %s", result.Stdout)
	assert.Check(t, !strings.Contains(result.Stdout, "#8"), "should be truncated by --limit: %s", result.Stdout)
}

func TestPipelineList_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := "gh/testorg/testrepo"
	fake.AddProjectPipelines(slug,
		fakePipeline("pid-1", 10, "created", slug, "main"),
		fakePipeline("pid-2", 9, "errored", slug, "feature"),
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"pipeline", "list", "--project", slug, "--json"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["id"], "pid-1"))
	assert.Check(t, cmp.Equal(out[0]["state"], "created"))
	assert.Check(t, cmp.Equal(out[1]["state"], "errored"))
}

func TestPipelineList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"pipeline", "list", "--project", "gh/org/repo"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, cmp.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}

// --- pipeline trigger ---

func TestPipelineTrigger(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := "gh/testorg/testrepo"
	fake.SetTriggerResponse(slug, map[string]any{
		"id":         "new-pipeline-uuid",
		"state":      "created",
		"number":     43,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"pipeline", "trigger", "--project", slug, "--branch", "main"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "#43"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "new-pipeline-uuid"), "stdout: %s", result.Stdout)
}

func TestPipelineTrigger_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := "gh/testorg/testrepo"
	fake.SetTriggerResponse(slug, map[string]any{
		"id":         "new-pipeline-uuid",
		"state":      "created",
		"number":     43,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"pipeline", "trigger", "--project", slug, "--branch", "main", "--json"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], "new-pipeline-uuid"))
	assert.Check(t, cmp.Equal(out["state"], "created"))
	assert.Check(t, cmp.Equal(out["number"], float64(43)))
}

func TestPipelineTrigger_InvalidParam(t *testing.T) {
	env := testenv.New(t)
	env.Token = "testtoken"

	result := binary.RunCLI(t,
		[]string{"pipeline", "trigger", "--project", "gh/org/repo", "--branch", "main", "--parameter", "noequals"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, cmp.Contains(result.Stderr, "key=value"), "stderr: %s", result.Stderr)
}

func TestPipelineTrigger_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"pipeline", "trigger", "--project", "gh/org/repo", "--branch", "main"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, cmp.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}

// --- pipeline cancel ---

func TestPipelineCancel(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	pipelineID := "5034460f-c7c4-4c43-9457-de07e2029e7b"
	wfID := "wf-cancel-001"
	fake.AddPipeline(pipelineID, fakePipeline(pipelineID, 42, "created", "gh/testorg/testrepo", "main"))
	fake.AddPipelineWorkflows(pipelineID, map[string]any{"id": wfID, "name": "build", "status": "running"})
	fake.SetCancelResponse(wfID, 202)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, []string{"pipeline", "cancel", "--force", pipelineID}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, pipelineID), "stdout: %s", result.Stdout)
}

func TestPipelineCancel_RequiresForce(t *testing.T) {
	// In non-interactive mode (no TTY), --force is required.
	fake := fakes.NewCircleCI(t)
	pipelineID := "5034460f-c7c4-4c43-9457-de07e2029e7b"
	fake.AddPipeline(pipelineID, fakePipeline(pipelineID, 42, "created", "gh/testorg/testrepo", "main"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, []string{"pipeline", "cancel", pipelineID}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr) // ExitCancelled
	assert.Check(t, cmp.Contains(result.Stderr, "--force"), "stderr: %s", result.Stderr)
}

func TestPipelineCancel_AlreadyDone(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	pipelineID := "5034460f-c7c4-4c43-9457-de07e2029e7c"
	wfID := "wf-cancel-002"
	fake.AddPipeline(pipelineID, fakePipeline(pipelineID, 43, "created", "gh/testorg/testrepo", "main"))
	fake.AddPipelineWorkflows(pipelineID, map[string]any{"id": wfID, "name": "build", "status": "success"})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, []string{"pipeline", "cancel", "--force", pipelineID}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, cmp.Contains(result.Stderr, "no active workflows"), "stderr: %s", result.Stderr)
}

func TestPipelineCancel_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"pipeline", "cancel", "--force", "00000000-0000-0000-0000-000000000000"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
	assert.Check(t, cmp.Contains(result.Stderr, "No pipeline found"), "stderr: %s", result.Stderr)
}

func TestPipelineCancel_MissingArg(t *testing.T) {
	env := testenv.New(t)
	env.Token = "testtoken"

	result := binary.RunCLI(t, []string{"pipeline", "cancel"}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
}

func TestPipelineCancel_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"pipeline", "cancel", "--force", "5034460f-c7c4-4c43-9457-de07e2029e7b"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
}
