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

const (
	getRunID = "5034460f-c7c4-4c43-9457-de07e2029e7b"
	testWfID = "wf-uuid-001"
)

// fakeRun returns a minimal run payload for the fake server.
func fakeRun(id string, number int, state, slug, branch string) map[string]any {
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

func TestRunGet_ByID(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := testWfID
	fake.AddRun(runID, fakeRun(runID, 42, "created", watchSlug, "main"))
	fake.AddRunWorkflows(runID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID,
		fakeJob("job-uuid-1", "run-tests", 101, watchSlug),
		fakeJob("job-uuid-2", "deploy", 102, watchSlug),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", runID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunGet_ByID_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := testWfID
	fake.AddRun(runID, fakeRun(runID, 42, "created", watchSlug, "main"))
	fake.AddRunWorkflows(runID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID,
		fakeJob("job-uuid-1", "run-tests", 101, watchSlug),
		fakeJob("job-uuid-2", "deploy", 102, watchSlug),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", runID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunGet_ByNumber(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := "wf-uuid-002"
	slug := watchSlug
	r := fakeRun(runID, 42, "created", slug, "main")
	fake.AddRun(runID, r)
	fake.AddProjectRuns(slug, r)
	fake.AddRunWorkflows(runID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID, fakeJob("job-uuid-1", "run-tests", 101, slug))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "42", "--project", slug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunGet_ByNumber_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := "wf-uuid-002"
	slug := watchSlug
	r := fakeRun(runID, 42, "created", slug, "main")
	fake.AddRun(runID, r)
	fake.AddProjectRuns(slug, r)
	fake.AddRunWorkflows(runID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID, fakeJob("job-uuid-1", "run-tests", 101, slug))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "42", "--project", slug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunGet_ByID_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := testWfID
	fake.AddRun(runID, fakeRun(runID, 42, "created", watchSlug, "main"))
	fake.AddRunWorkflows(runID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID, fakeJob("job-uuid-1", "run-tests", 101, watchSlug))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "--json", runID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], runID))
	assert.Check(t, cmp.Equal(out["status"], "success"))
	assert.Check(t, cmp.Equal(out["project_slug"], watchSlug))

	wfs := out["workflows"].([]any)
	assert.Check(t, cmp.Len(wfs, 1))
	jobs := wfs[0].(map[string]any)["jobs"].([]any)
	assert.Check(t, cmp.Len(jobs, 1))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["name"], "run-tests"))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["number"], float64(101)))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunGet_ByID_JQ(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := testWfID
	fake.AddRun(runID, fakeRun(runID, 42, "created", watchSlug, "main"))
	fake.AddRunWorkflows(runID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID, fakeJob("job-uuid-1", "run-tests", 101, watchSlug))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "--json", "--jq", ".id", runID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(strings.TrimSpace(result.Stdout), runID))
}

func TestRunGet_ByID_JSON_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := testWfID
	fake.AddRun(runID, fakeRun(runID, 42, "created", watchSlug, "main"))
	fake.AddRunWorkflows(runID, fakeWorkflow(wfID, "build"))
	fake.AddWorkflowJobs(wfID, fakeJob("job-uuid-1", "run-tests", 101, watchSlug))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "--json", runID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunGet_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunGet_NoToken(t *testing.T) {
	env := testenv.New(t)
	// No token set

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "any-id"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- run list ---

// fakeRunNoVCS returns a pipeline payload without a vcs field, using trigger_parameters instead.
func fakeRunNoVCS(id string, number int, state, slug, branch, revision string) map[string]any {
	r := fakeRun(id, number, state, slug, branch)
	delete(r, "vcs")
	r["trigger_parameters"] = map[string]any{
		"git": map[string]any{
			"branch":      branch,
			"checkout_sha": revision,
		},
	}
	return r
}

// fakeWorkflowWithDuration returns a workflow payload whose stopped_at is
// pipelineStart + durationSeconds, matching fakeRun's created_at baseline.
func fakeWorkflowWithDuration(id, name, status string, durationSeconds int) map[string]any {
	start := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	stopped := start.Add(time.Duration(durationSeconds) * time.Second)
	return map[string]any{
		"id":         id,
		"name":       name,
		"status":     status,
		"created_at": start.Format(time.RFC3339),
		"stopped_at": stopped.Format(time.RFC3339),
	}
}

func TestRunList(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.AddProjectRuns(slug,
		fakeRun("pid-1", 10, "created", slug, "main"),
		fakeRun("pid-2", 9, "errored", slug, "feature"),
		fakeRun("pid-3", 8, "created", slug, "main"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "list", "--project", slug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunList_Duration(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	r1 := fakeRun("pid-1", 10, "created", slug, "main")
	r2 := fakeRun("pid-2", 9, "errored", slug, "feature")
	r3 := fakeRun("pid-3", 8, "created", slug, "main")
	fake.AddProjectRuns(slug, r1, r2, r3)
	// Also register individually so the workflow endpoint is served.
	fake.AddRun("pid-1", r1)
	fake.AddRun("pid-2", r2)
	fake.AddRun("pid-3", r3)
	fake.AddRunWorkflows("pid-1", fakeWorkflowWithDuration("wf-1", "build", "success", 125)) // 2m5s
	fake.AddRunWorkflows("pid-2", fakeWorkflowWithDuration("wf-2", "build", "failed", 45))  // 45s
	// pid-3 has no workflows → duration stays -

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "list", "--project", slug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunList_TriggerParams(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.AddProjectRuns(slug,
		fakeRunNoVCS("pid-1", 10, "created", slug, "main", "abc1234def5678"),
		fakeRunNoVCS("pid-2", 9, "errored", slug, "feature", "deadbeef1234"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "list", "--project", slug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunList_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.AddProjectRuns(slug,
		fakeRun("pid-1", 10, "created", slug, "main"),
		fakeRun("pid-2", 9, "errored", slug, "feature"),
		fakeRun("pid-3", 8, "created", slug, "main"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "list", "--project", slug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunList_Limit(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.AddProjectRuns(slug,
		fakeRun("pid-1", 10, "created", slug, "main"),
		fakeRun("pid-2", 9, "created", slug, "main"),
		fakeRun("pid-3", 8, "created", slug, "main"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "list", "--project", slug, "--limit", "2"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunList_Limit_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.AddProjectRuns(slug,
		fakeRun("pid-1", 10, "created", slug, "main"),
		fakeRun("pid-2", 9, "created", slug, "main"),
		fakeRun("pid-3", 8, "created", slug, "main"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "list", "--project", slug, "--limit", "2"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunList_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.AddProjectRuns(slug,
		fakeRun("pid-1", 10, "created", slug, "main"),
		fakeRun("pid-2", 9, "errored", slug, "feature"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "list", "--project", slug, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["id"], "pid-1"))
	assert.Check(t, cmp.Equal(out[0]["state"], "created"))
	assert.Check(t, cmp.Equal(out[1]["state"], "errored"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunList_JSON_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.AddProjectRuns(slug,
		fakeRun("pid-1", 10, "created", slug, "main"),
		fakeRun("pid-2", 9, "errored", slug, "feature"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "list", "--project", slug, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "list", "--project", "gh/org/repo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- run trigger ---

func TestRunTrigger(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.SetTriggerResponse(slug, map[string]any{
		"id":         "new-run-uuid",
		"state":      "created",
		"number":     43,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "trigger", "--project", slug, "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunTrigger_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.SetTriggerResponse(slug, map[string]any{
		"id":         "new-run-uuid",
		"state":      "created",
		"number":     43,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "trigger", "--project", slug, "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunTrigger_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.SetTriggerResponse(slug, map[string]any{
		"id":         "new-run-uuid",
		"state":      "created",
		"number":     43,
		"created_at": time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC).Format(time.RFC3339),
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "trigger", "--project", slug, "--branch", "main", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], "new-run-uuid"))
	assert.Check(t, cmp.Equal(out["state"], "created"))
	assert.Check(t, cmp.Equal(out["number"], float64(43)))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunTrigger_JSON_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	fake.SetTriggerResponse(slug, map[string]any{
		"id":         "new-run-uuid",
		"state":      "created",
		"number":     43,
		"created_at": time.Date(2021, 1, 1, 1, 1, 1, 1, time.UTC).Format(time.RFC3339),
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "trigger", "--project", slug, "--branch", "main", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunTrigger_InvalidParam(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "trigger", "--project", "gh/org/repo", "--branch", "main", "--parameter", "noequals"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunTrigger_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "trigger", "--project", "gh/org/repo", "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- run cancel ---

func TestRunCancel(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := "wf-cancel-001"
	fake.AddRun(runID, fakeRun(runID, 42, "created", watchSlug, "main"))
	fake.AddRunWorkflows(runID, map[string]any{"id": wfID, "name": "build", "status": "running"})
	fake.SetCancelResponse(wfID, 202)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "cancel", "--force", runID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunCancel_RequiresForce(t *testing.T) {
	// In non-interactive mode (no TTY), --force is required.
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	fake.AddRun(runID, fakeRun(runID, 42, "created", watchSlug, "main"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "cancel", runID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr) // ExitCancelled
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunCancel_AlreadyDone(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := "5034460f-c7c4-4c43-9457-de07e2029e7c"
	wfID := "wf-cancel-002"
	fake.AddRun(runID, fakeRun(runID, 43, "created", watchSlug, "main"))
	fake.AddRunWorkflows(runID, map[string]any{"id": wfID, "name": "build", "status": "success"})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "cancel", "--force", runID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunCancel_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "cancel", "--force", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunCancel_MissingArg(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "cancel"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
}

func TestRunCancel_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "cancel", "--force", getRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
}
