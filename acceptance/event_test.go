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
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

const (
	getEventID         = "5034460f-c7c4-4c43-9457-de07e2029e7b"
	testWfID           = "wf-uuid-001"
	eventTestProjectID = "proj-uuid-001"

	// Shared across multiple test files.
	testPipelineID = "aaaaaaaa-0000-0000-0000-000000000001"
	testWorkflowID = "bbbbbbbb-0000-0000-0000-000000000001"
	testSlug       = "gh/testorg/testrepo"
)

var v3TimeFormat = time.RFC3339

// fakeEventV3 returns a V3 event payload for the fake server.
func fakeEventV3(id, projectID, phase, outcome, branch, revision string) map[string]any {
	createdAt := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	attrs := map[string]any{
		"phase":      phase,
		"created_at": createdAt.Format(v3TimeFormat),
		"vcs": map[string]any{
			"branch":   branch,
			"revision": revision,
		},
	}
	// The real V3 events API reports only current_outcome, never outcome,
	// regardless of phase.
	if outcome != "" {
		attrs["current_outcome"] = outcome
	}
	return map[string]any{
		"id":         id,
		"attributes": attrs,
		"references": map[string]any{
			"project": map[string]any{"id": projectID},
			"user":    map[string]any{"id": "user-uuid-001"},
		},
	}
}

// fakeWorkflowV3 returns a V3 workflow payload for the fake server.
func fakeWorkflowV3(id, name, eventID, projectID, phase, outcome string) map[string]any {
	created := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	attrs := map[string]any{
		"name":       name,
		"phase":      phase,
		"created_at": created.Format(v3TimeFormat),
	}
	if phase == "ended" {
		attrs["outcome"] = outcome
		ended := created.Add(2*time.Minute + 34*time.Second)
		attrs["ended_at"] = ended.Format(v3TimeFormat)
	} else {
		attrs["current_outcome"] = outcome
	}
	return map[string]any{
		"id":         id,
		"attributes": attrs,
		"references": map[string]any{
			"event":   map[string]any{"id": eventID},
			"project": map[string]any{"id": projectID},
			"user":    map[string]any{"id": "user-uuid-001"},
		},
	}
}

// fakeRun returns a V2 pipeline payload — still needed for workflows/jobs
// and commands that haven't migrated to V3 yet (cancel, trigger).
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

func fakeJobV3(id, name, workflowID, projectID string) map[string]any {
	now := time.Now().UTC().Format(time.RFC3339)
	return map[string]any{
		"id": id,
		"attributes": map[string]any{
			"name":       name,
			"phase":      "ended",
			"outcome":    "succeeded",
			"type":       "build",
			"started_at": now,
			"ended_at":   now,
		},
		"references": map[string]any{
			"workflow": map[string]any{"id": workflowID},
			"project":  map[string]any{"id": projectID},
		},
	}
}

func addProjectInfo(fake *fakes.CircleCI, slug, projectID string) {
	fake.AddProjectInfo(slug, map[string]any{
		"id":                projectID,
		"slug":              slug,
		"name":              "testrepo",
		"organization_name": "testorg",
		"organization_slug": "gh/testorg",
	})
}

// --- event get (V3) ---

func TestEventGet_ByID(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID
	wfID := testWfID
	fake.AddEventV3(eventID, eventTestProjectID, fakeEventV3(eventID, eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventWorkflowsV3(eventID, fakeWorkflowV3(wfID, "build", eventID, eventTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID,
		fakeJobV3("job-uuid-1", "run-tests", wfID, eventTestProjectID),
		fakeJobV3("job-uuid-2", "deploy", wfID, eventTestProjectID),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "get", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEventGet_ByID_WorkflowsNotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID
	fake.AddEventV3(eventID, eventTestProjectID, fakeEventV3(eventID, eventTestProjectID, "created", "", "main", "abc1234def5678"))
	fake.SetEventWorkflowsV3NotFound(eventID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "get", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEventGet_ByID_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID
	wfID := testWfID
	fake.AddEventV3(eventID, eventTestProjectID, fakeEventV3(eventID, eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventWorkflowsV3(eventID, fakeWorkflowV3(wfID, "build", eventID, eventTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID,
		fakeJobV3("job-uuid-1", "run-tests", wfID, eventTestProjectID),
		fakeJobV3("job-uuid-2", "deploy", wfID, eventTestProjectID),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "get", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEventGet_ByID_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID
	wfID := testWfID
	fake.AddEventV3(eventID, eventTestProjectID, fakeEventV3(eventID, eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventWorkflowsV3(eventID, fakeWorkflowV3(wfID, "build", eventID, eventTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID, fakeJobV3("job-uuid-1", "run-tests", wfID, eventTestProjectID))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "get", "--json", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], eventID))
	assert.Check(t, cmp.Equal(out["phase"], "ended"))
	assert.Check(t, cmp.Equal(out["current_outcome"], "succeeded"))

	wfs := out["workflows"].([]any)
	assert.Check(t, cmp.Len(wfs, 1))
	jobs := wfs[0].(map[string]any)["jobs"].([]any)
	assert.Check(t, cmp.Len(jobs, 1))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["name"], "run-tests"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestEventGet_ByID_JQ(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID
	wfID := testWfID
	fake.AddEventV3(eventID, eventTestProjectID, fakeEventV3(eventID, eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventWorkflowsV3(eventID, fakeWorkflowV3(wfID, "build", eventID, eventTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID, fakeJobV3("job-uuid-1", "run-tests", wfID, eventTestProjectID))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "get", "--json", "--jq", ".id", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(strings.TrimSpace(result.Stdout), eventID))
}

func TestEventGet_ByID_JSON_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID
	wfID := testWfID
	fake.AddEventV3(eventID, eventTestProjectID, fakeEventV3(eventID, eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventWorkflowsV3(eventID, fakeWorkflowV3(wfID, "build", eventID, eventTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID, fakeJobV3("job-uuid-1", "run-tests", wfID, eventTestProjectID))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "get", "--json", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestEventGet_WithErrors(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID

	event := fakeEventV3(eventID, eventTestProjectID, "ended", "failed", "main", "abc1234def5678")
	event["attributes"].(map[string]any)["errors"] = []map[string]any{
		{"type": "config", "message": "Could not find config file"},
	}
	fake.AddEventV3(eventID, eventTestProjectID, event)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "get", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEventGet_WithErrors_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID

	event := fakeEventV3(eventID, eventTestProjectID, "ended", "failed", "main", "abc1234def5678")
	event["attributes"].(map[string]any)["errors"] = []map[string]any{
		{"type": "config", "message": "Could not find config file"},
	}
	fake.AddEventV3(eventID, eventTestProjectID, event)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "get", "--json", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	errs := out["errors"].([]any)
	assert.Check(t, cmp.Len(errs, 1))
	assert.Check(t, cmp.Equal(errs[0].(map[string]any)["type"], "config"))
	assert.Check(t, cmp.Equal(errs[0].(map[string]any)["message"], "Could not find config file"))
}

func TestEventGet_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "get", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestEventGet_NoToken(t *testing.T) {
	env := testenv.New(t)
	// No token set

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "get", "any-id"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- event list (V3 search) ---

func TestEventList(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectInfo(fake, slug, eventTestProjectID)
	fake.AddEventV3("pid-1", eventTestProjectID, fakeEventV3("pid-1", eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventV3("pid-2", eventTestProjectID, fakeEventV3("pid-2", eventTestProjectID, "ended", "failed", "feature", "deadbeef12345678"))
	fake.AddEventV3("pid-3", eventTestProjectID, fakeEventV3("pid-3", eventTestProjectID, "ended", "succeeded", "main", "1111111122222222"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "list", "--project", slug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEventList_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectInfo(fake, slug, eventTestProjectID)
	fake.AddEventV3("pid-1", eventTestProjectID, fakeEventV3("pid-1", eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventV3("pid-2", eventTestProjectID, fakeEventV3("pid-2", eventTestProjectID, "ended", "failed", "feature", "deadbeef12345678"))
	fake.AddEventV3("pid-3", eventTestProjectID, fakeEventV3("pid-3", eventTestProjectID, "ended", "succeeded", "main", "1111111122222222"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "list", "--project", slug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEventList_Limit(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectInfo(fake, slug, eventTestProjectID)
	fake.AddEventV3("pid-1", eventTestProjectID, fakeEventV3("pid-1", eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventV3("pid-2", eventTestProjectID, fakeEventV3("pid-2", eventTestProjectID, "ended", "succeeded", "main", "deadbeef12345678"))
	fake.AddEventV3("pid-3", eventTestProjectID, fakeEventV3("pid-3", eventTestProjectID, "ended", "succeeded", "main", "1111111122222222"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "list", "--project", slug, "--limit", "2"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEventList_Limit_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectInfo(fake, slug, eventTestProjectID)
	fake.AddEventV3("pid-1", eventTestProjectID, fakeEventV3("pid-1", eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventV3("pid-2", eventTestProjectID, fakeEventV3("pid-2", eventTestProjectID, "ended", "succeeded", "main", "deadbeef12345678"))
	fake.AddEventV3("pid-3", eventTestProjectID, fakeEventV3("pid-3", eventTestProjectID, "ended", "succeeded", "main", "1111111122222222"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "list", "--project", slug, "--limit", "2"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEventList_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectInfo(fake, slug, eventTestProjectID)
	fake.AddEventV3("pid-1", eventTestProjectID, fakeEventV3("pid-1", eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventV3("pid-2", eventTestProjectID, fakeEventV3("pid-2", eventTestProjectID, "ended", "failed", "feature", "deadbeef12345678"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "list", "--project", slug, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["id"], "pid-1"))
	assert.Check(t, cmp.Equal(out[0]["phase"], "ended"))
	assert.Check(t, cmp.Equal(out[0]["current_outcome"], "succeeded"))
	assert.Check(t, cmp.Equal(out[1]["current_outcome"], "failed"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestEventList_JSON_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectInfo(fake, slug, eventTestProjectID)
	fake.AddEventV3("pid-1", eventTestProjectID, fakeEventV3("pid-1", eventTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddEventV3("pid-2", eventTestProjectID, fakeEventV3("pid-2", eventTestProjectID, "ended", "failed", "feature", "deadbeef12345678"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "list", "--project", slug, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestEventList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "list", "--project", "gh/org/repo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- event create (legacy V2 trigger path, no pipeline definition) ---

func TestEventCreate(t *testing.T) {
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
		Args:    []string{"event", "create", "--project", slug, "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/project/" + slug + "/pipeline"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"branch":"main"}`),
		}, ignoreCommonHeaders))
	})
}

func TestEventCreate_Color(t *testing.T) {
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
		Args:    []string{"event", "create", "--project", slug, "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestEventCreate_JSON(t *testing.T) {
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
		Args:    []string{"event", "create", "--project", slug, "--branch", "main", "--json"},
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

func TestEventCreate_JSON_Color(t *testing.T) {
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
		Args:    []string{"event", "create", "--project", slug, "--branch", "main", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestEventCreate_InvalidParam(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "create", "--project", "gh/org/repo", "--branch", "main", "--param", "noequals"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestEventCreate_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "create", "--project", "gh/org/repo", "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- arg whitespace trimming (tested via run get; trimming lives in PersistentPreRunE) ---

func TestArgWhitespaceTrimmed(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID
	wfID := testWfID
	fake.AddEventV3(eventID, eventTestProjectID, fakeEventV3(eventID, eventTestProjectID, "ended", "succeeded", "main", "abc1234"))
	fake.AddEventWorkflowsV3(eventID, fakeWorkflowV3(wfID, "build", eventID, eventTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID, fakeJobV3("job-uuid-1", "run-tests", wfID, eventTestProjectID))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	for _, arg := range []string{eventID + " ", " " + eventID, " " + eventID + " ", eventID + "\t"} {
		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"event", "get", arg},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})
		assert.Equal(t, result.ExitCode, 0, "arg %q: whitespace should be trimmed; stderr: %s", arg, result.Stderr)
	}
}

// --- run cancel (still V2 for pipeline lookup) ---

func TestEventCancel(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID
	wfID := "wf-cancel-001"
	fake.AddRun(eventID, fakeRun(eventID, 42, "created", watchSlug, "main"))
	fake.AddEventWorkflowsV3(eventID, fakeWorkflowV3(wfID, "build", eventID, "proj-cancel", "started", ""))
	fake.SetCancelResponse(wfID, 202)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "cancel", "--force", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/workflows/" + wfID + "/cancel"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestEventCancel_Started(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID
	wfID := "wf-cancel-started"
	fake.AddRun(eventID, fakeRun(eventID, 42, "running", watchSlug, "main"))
	fake.AddEventWorkflowsV3(eventID, fakeWorkflowV3(wfID, "build", eventID, "proj-cancel", "started", ""))
	fake.SetCancelResponse(wfID, 202)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "cancel", "--force", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
}

func TestEventCancel_RequiresForce(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := getEventID
	fake.AddRun(eventID, fakeRun(eventID, 42, "created", watchSlug, "main"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "cancel", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr) // ExitCancelled
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestEventCancel_AlreadyDone(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	eventID := "5034460f-c7c4-4c43-9457-de07e2029e7c"
	wfID := "wf-cancel-002"
	fake.AddRun(eventID, fakeRun(eventID, 43, "created", watchSlug, "main"))
	fake.AddEventWorkflowsV3(eventID, fakeWorkflowV3(wfID, "build", eventID, "proj-cancel", "ended", "succeeded"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "cancel", "--force", eventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestEventCancel_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "cancel", "--force", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments (no active workflows)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestEventCancel_MissingArg(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "cancel"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
}

func TestEventCancel_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "cancel", "--force", getEventID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
}

// --- event create via a pipeline definition (the --definition-id path) ---

const (
	eventCreateID     = "run-uuid-0001"
	eventCreateNumber = 42
)

// setupEventCreateFake registers trigger responses for both the pipeline
// definitions endpoint and the legacy project trigger endpoint, so tests can
// exercise either path of 'event create'.
func setupEventCreateFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	resp := map[string]any{
		"id":         eventCreateID,
		"state":      "created",
		"number":     eventCreateNumber,
		"created_at": "2024-06-01T00:00:00Z",
	}
	fake.SetTriggerPipelineRunResponse("gh/myorg/myrepo", resp)
	fake.SetTriggerResponse("gh/myorg/myrepo", resp)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

// Without a definition or branch (and outside a git repo), the event is
// created through the legacy project trigger endpoint.
func TestEventCreate_NoBranch(t *testing.T) {
	fake, env := setupEventCreateFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "create", "--project", "gh/myorg/myrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, eventCreateID))
	assert.Check(t, strings.Contains(result.Stdout, "42"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/project/gh/myorg/myrepo/pipeline"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{}`),
		}, ignoreCommonHeaders))
	})
}

func TestEventCreate_WithDefinitionID(t *testing.T) {
	fake, env := setupEventCreateFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"event", "create",
			"--project", "gh/myorg/myrepo",
			"--definition-id", pipelineDefID,
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, eventCreateID))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/project/gh/myorg/myrepo/pipeline/run"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"definition_id":"` + pipelineDefID + `"}`),
		}, ignoreCommonHeaders))
	})
}

// With a definition, parameter values are sent as strings.
func TestEventCreate_DefinitionParams(t *testing.T) {
	fake, env := setupEventCreateFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"event", "create",
			"--project", "gh/myorg/myrepo",
			"--definition-id", pipelineDefID,
			"--param", "deploy_env=staging",
			"--param", "run_tests=true",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, eventCreateID))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/project/gh/myorg/myrepo/pipeline/run"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"definition_id":"` + pipelineDefID + `","parameters":{"deploy_env":"staging","run_tests":"true"}}`),
		}, ignoreCommonHeaders))
	})
}

// Without a definition, parameter values are coerced to booleans or integers
// for the legacy trigger endpoint.
func TestEventCreate_LegacyParams(t *testing.T) {
	fake, env := setupEventCreateFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"event", "create",
			"--project", "gh/myorg/myrepo",
			"--param", "deploy_env=staging",
			"--param", "run_tests=true",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/project/gh/myorg/myrepo/pipeline"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"parameters":{"deploy_env":"staging","run_tests":true}}`),
		}, ignoreCommonHeaders))
	})
}

func TestEventCreate_BranchAndTagExclusive(t *testing.T) {
	_, env := setupEventCreateFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"event", "create",
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

func TestEventCreate_TagRequiresDefinition(t *testing.T) {
	_, env := setupEventCreateFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"event", "create",
			"--project", "gh/myorg/myrepo",
			"--tag", "v1.0",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "--definition-id"))
}

func TestEventCreate_Skipped(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetTriggerPipelineRunSkipped("gh/myorg/myrepo", "Ignoring pipeline due to CI skip in the commit")
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "create", "--project", "gh/myorg/myrepo", "--definition-id", pipelineDefID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "not created"))
	assert.Check(t, strings.Contains(result.Stdout, "CI skip"))
}

func TestEventCreate_Skipped_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetTriggerPipelineRunSkipped("gh/myorg/myrepo", "Ignoring pipeline due to CI skip in the commit")
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "create", "--project", "gh/myorg/myrepo", "--definition-id", pipelineDefID, "--json"},
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

func TestEventCreate_ProjectNotFound(t *testing.T) {
	_, env := setupEventCreateFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "create", "--project", "gh/myorg/nonexistent"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
}

// --- event create interactive ---

// setupEventCreateInteractiveFake builds a fake with project info, pipeline
// definitions, and trigger responses for both paths — everything needed for
// the interactive definition-selection prompt.
func setupEventCreateInteractiveFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
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
	resp := map[string]any{
		"id":         eventCreateID,
		"state":      "created",
		"number":     eventCreateNumber,
		"created_at": "2024-06-01T00:00:00Z",
	}
	fake.SetTriggerPipelineRunResponse("gh/myorg/myrepo", resp)
	fake.SetTriggerResponse("gh/myorg/myrepo", resp)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

func TestEventCreate_Interactive_SelectFirst(t *testing.T) {
	_, env := setupEventCreateInteractiveFake(t)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "create", "--project", "gh/myorg/myrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// Step 1: select the first pipeline definition (main-pipeline).
	_, err := console.ExpectString("Select a pipeline definition")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	// Step 2: accept the default branch (main).
	_, err = console.ExpectString("Branch to fire on")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("Event #42 created")
	assert.NilError(t, err)
}

// Skipping the definition falls back to the legacy project trigger endpoint.
func TestEventCreate_Interactive_SkipDefinition(t *testing.T) {
	_, env := setupEventCreateInteractiveFake(t)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "create", "--project", "gh/myorg/myrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// Step 1: navigate past both definitions to "None" (2 × down, Enter).
	_, err := console.ExpectString("Select a pipeline definition")
	assert.NilError(t, err)
	_, err = console.Send(keyDown + keyDown + "\r")
	assert.NilError(t, err)

	// Step 2: accept the default branch (main).
	_, err = console.ExpectString("Branch to fire on")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("Event #42 created")
	assert.NilError(t, err)
}

func TestEventCreate_Interactive_CustomBranch(t *testing.T) {
	_, env := setupEventCreateInteractiveFake(t)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"event", "create", "--project", "gh/myorg/myrepo"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// Step 1: select the first pipeline definition.
	_, err := console.ExpectString("Select a pipeline definition")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	// Step 2: navigate to "Other..." and type a custom branch.
	_, err = console.ExpectString("Branch to fire on")
	assert.NilError(t, err)
	_, err = console.Send(keyDown + "\r") // move to "Other..."
	assert.NilError(t, err)

	_, err = console.ExpectString("Branch name")
	assert.NilError(t, err)
	_, err = console.Send("feature/my-branch\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("Event #42 created")
	assert.NilError(t, err)
}
