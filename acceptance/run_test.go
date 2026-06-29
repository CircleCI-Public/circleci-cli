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
	getRunID         = "5034460f-c7c4-4c43-9457-de07e2029e7b"
	testWfID         = "b0000000-0000-4000-8000-000000000001"
	runTestProjectID = "a0000000-0000-4000-8000-000000000001"

	// Shared across multiple test files.
	testPipelineID = "aaaaaaaa-0000-0000-0000-000000000001"
	testWorkflowID = "bbbbbbbb-0000-0000-0000-000000000001"
	testSlug       = "gh/testorg/testrepo"
)

var v3TimeFormat = time.RFC3339

// fakeRunV3 returns a V3 run payload for the fake server.
func fakeRunV3(id, projectID, phase, outcome, branch, revision string) map[string]any {
	createdAt := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	attrs := map[string]any{
		"phase":      phase,
		"created_at": createdAt.Format(v3TimeFormat),
		"vcs": map[string]any{
			"branch":   branch,
			"revision": revision,
		},
	}
	// The real V3 runs API reports only current_outcome, never outcome,
	// regardless of phase.
	if outcome != "" {
		attrs["current_outcome"] = outcome
	}
	return map[string]any{
		"id":         id,
		"attributes": attrs,
		"references": map[string]any{
			// VCS now lives on the event reference (and carries the tag);
			// attributes.vcs above is retained for legacy clients.
			"event": map[string]any{
				"attributes": map[string]any{
					"vcs": map[string]any{
						"branch":   branch,
						"revision": revision,
					},
				},
			},
			"trigger": map[string]any{
				"attributes": map[string]any{
					"event_source": map[string]any{"type": "webhook"},
				},
			},
			"project": map[string]any{"id": projectID},
			"user":    map[string]any{"id": "c0000000-0000-4000-8000-000000000001"},
		},
	}
}

// fakeRunV3Tag returns a V3 run payload for a tag-triggered run: no branch,
// with the tag carried on the event reference's VCS.
func fakeRunV3Tag(id, projectID, phase, outcome, tag, revision string) map[string]any {
	run := fakeRunV3(id, projectID, phase, outcome, "", revision)
	ev := run["references"].(map[string]any)["event"].(map[string]any)["attributes"].(map[string]any)
	ev["vcs"].(map[string]any)["tag"] = tag
	return run
}

// fakeWorkflowV3 returns a V3 workflow payload for the fake server.
func fakeWorkflowV3(id, name, runID, projectID, phase, outcome string) map[string]any {
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
			"run":     map[string]any{"id": runID},
			"project": map[string]any{"id": projectID},
			"user":    map[string]any{"id": "c0000000-0000-4000-8000-000000000001"},
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

func addProjectBySlug(fake *fakes.CircleCI, slug, projectID string) {
	fake.AddProjectBySlug(slug, projectID, "testrepo", "a0000000-0000-4000-8000-0000000d0001")
}

// --- run get (V3) ---

func TestRunGet_ByID(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := testWfID
	fake.AddRunV3(runID, runTestProjectID, fakeRunV3(runID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(runID, fakeWorkflowV3(wfID, "build", runID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID,
		fakeJobV3("d0000000-0000-4000-8000-000000000001", "run-tests", wfID, runTestProjectID),
		fakeJobV3("d0000000-0000-4000-8000-000000000002", "deploy", wfID, runTestProjectID),
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
	assert.Check(t, golden.String(normalizeAppHost(result.Stdout, fake.URL()), t.Name()+".txt"))
}

func TestRunGet_ByID_WorkflowsNotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	fake.AddRunV3(runID, runTestProjectID, fakeRunV3(runID, runTestProjectID, "created", "", "main", "abc1234def5678"))
	fake.SetRunWorkflowsV3NotFound(runID)

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
	assert.Check(t, golden.String(normalizeAppHost(result.Stdout, fake.URL()), t.Name()+".txt"))
}

func TestRunGet_ByID_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := testWfID
	fake.AddRunV3(runID, runTestProjectID, fakeRunV3(runID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(runID, fakeWorkflowV3(wfID, "build", runID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID,
		fakeJobV3("d0000000-0000-4000-8000-000000000001", "run-tests", wfID, runTestProjectID),
		fakeJobV3("d0000000-0000-4000-8000-000000000002", "deploy", wfID, runTestProjectID),
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
	assert.Check(t, golden.String(normalizeAppHost(result.Stdout, fake.URL()), t.Name()+".txt"))
}

func TestRunGet_ByID_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := testWfID
	fake.AddRunV3(runID, runTestProjectID, fakeRunV3(runID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(runID, fakeWorkflowV3(wfID, "build", runID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID, fakeJobV3("d0000000-0000-4000-8000-000000000001", "run-tests", wfID, runTestProjectID))

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
	assert.Check(t, cmp.Equal(out["phase"], "ended"))
	assert.Check(t, cmp.Equal(out["current_outcome"], "succeeded"))

	wfs := out["workflows"].([]any)
	assert.Check(t, cmp.Len(wfs, 1))
	jobs := wfs[0].(map[string]any)["jobs"].([]any)
	assert.Check(t, cmp.Len(jobs, 1))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["name"], "run-tests"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunGet_ByID_JQ(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := testWfID
	fake.AddRunV3(runID, runTestProjectID, fakeRunV3(runID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(runID, fakeWorkflowV3(wfID, "build", runID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID, fakeJobV3("d0000000-0000-4000-8000-000000000001", "run-tests", wfID, runTestProjectID))

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
	fake.AddRunV3(runID, runTestProjectID, fakeRunV3(runID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(runID, fakeWorkflowV3(wfID, "build", runID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID, fakeJobV3("d0000000-0000-4000-8000-000000000001", "run-tests", wfID, runTestProjectID))

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

func TestRunGet_WithErrors(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID

	run := fakeRunV3(runID, runTestProjectID, "ended", "failed", "main", "abc1234def5678")
	run["attributes"].(map[string]any)["errors"] = []map[string]any{
		{"type": "config", "message": "Could not find config file"},
	}
	fake.AddRunV3(runID, runTestProjectID, run)

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
	assert.Check(t, golden.String(normalizeAppHost(result.Stdout, fake.URL()), t.Name()+".txt"))
}

func TestRunGet_WithErrors_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID

	run := fakeRunV3(runID, runTestProjectID, "ended", "failed", "main", "abc1234def5678")
	run["attributes"].(map[string]any)["errors"] = []map[string]any{
		{"type": "config", "message": "Could not find config file"},
	}
	fake.AddRunV3(runID, runTestProjectID, run)

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
	errs := out["errors"].([]any)
	assert.Check(t, cmp.Len(errs, 1))
	assert.Check(t, cmp.Equal(errs[0].(map[string]any)["type"], "config"))
	assert.Check(t, cmp.Equal(errs[0].(map[string]any)["message"], "Could not find config file"))
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

// --- run list (V3 search) ---

func TestRunList(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectBySlug(fake, slug, runTestProjectID)
	fake.AddRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, "ended", "failed", "feature", "deadbeef12345678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000003", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000003", runTestProjectID, "ended", "succeeded", "main", "1111111122222222"))

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

func TestRunList_Tag(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectBySlug(fake, slug, runTestProjectID)
	fake.AddRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, fakeRunV3Tag("e0000000-0000-4000-8000-000000000002", runTestProjectID, "ended", "succeeded", "v1.2.3", "deadbeef12345678"))

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
	assert.Check(t, strings.Contains(result.Stdout, "🏷 v1.2.3"))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunList_Tag_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectBySlug(fake, slug, runTestProjectID)
	fake.AddRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, fakeRunV3Tag("e0000000-0000-4000-8000-000000000002", runTestProjectID, "ended", "succeeded", "v1.2.3", "deadbeef12345678"))

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
	assert.Check(t, cmp.Len(out, 1))
	assert.Check(t, cmp.Equal(out[0]["tag"], "v1.2.3"))
	_, hasBranch := out[0]["branch"]
	assert.Check(t, !hasBranch) // omitempty: tag runs carry no branch
}

func TestRunList_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectBySlug(fake, slug, runTestProjectID)
	fake.AddRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, "ended", "failed", "feature", "deadbeef12345678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000003", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000003", runTestProjectID, "ended", "succeeded", "main", "1111111122222222"))

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
	addProjectBySlug(fake, slug, runTestProjectID)
	fake.AddRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, "ended", "succeeded", "main", "deadbeef12345678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000003", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000003", runTestProjectID, "ended", "succeeded", "main", "1111111122222222"))

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
	addProjectBySlug(fake, slug, runTestProjectID)
	fake.AddRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, "ended", "succeeded", "main", "deadbeef12345678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000003", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000003", runTestProjectID, "ended", "succeeded", "main", "1111111122222222"))

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
	addProjectBySlug(fake, slug, runTestProjectID)
	fake.AddRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, "ended", "failed", "feature", "deadbeef12345678"))

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
	assert.Check(t, cmp.Equal(out[0]["id"], "e0000000-0000-4000-8000-000000000001"))
	assert.Check(t, cmp.Equal(out[0]["phase"], "ended"))
	assert.Check(t, cmp.Equal(out[0]["current_outcome"], "succeeded"))
	assert.Check(t, cmp.Equal(out[1]["current_outcome"], "failed"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestRunList_JSON_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectBySlug(fake, slug, runTestProjectID)
	fake.AddRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000002", runTestProjectID, "ended", "failed", "feature", "deadbeef12345678"))

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

// --- run trigger (still V2) ---

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

// --- arg whitespace trimming (tested via run get; trimming lives in PersistentPreRunE) ---

func TestArgWhitespaceTrimmed(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := testWfID
	fake.AddRunV3(runID, runTestProjectID, fakeRunV3(runID, runTestProjectID, "ended", "succeeded", "main", "abc1234"))
	fake.AddRunWorkflowsV3(runID, fakeWorkflowV3(wfID, "build", runID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(wfID, fakeJobV3("d0000000-0000-4000-8000-000000000001", "run-tests", wfID, runTestProjectID))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	for _, arg := range []string{runID + " ", " " + runID, " " + runID + " ", runID + "\t"} {
		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"run", "get", arg},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})
		assert.Equal(t, result.ExitCode, 0, "arg %q: whitespace should be trimmed; stderr: %s", arg, result.Stderr)
	}
}

// --- run cancel (still V2 for pipeline lookup) ---

func TestRunCancel(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := "b0000000-0000-4000-8000-00000000c001"
	fake.AddRun(runID, fakeRun(runID, 42, "created", watchSlug, "main"))
	fake.AddRunWorkflowsV3(runID, fakeWorkflowV3(wfID, "build", runID, "a0000000-0000-4000-8000-00000000c001", "started", ""))
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

func TestRunCancel_Started(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	runID := getRunID
	wfID := "b0000000-0000-4000-8000-00000000c002"
	fake.AddRun(runID, fakeRun(runID, 42, "running", watchSlug, "main"))
	fake.AddRunWorkflowsV3(runID, fakeWorkflowV3(wfID, "build", runID, "a0000000-0000-4000-8000-00000000c001", "started", ""))
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
}

func TestRunCancel_RequiresForce(t *testing.T) {
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
	wfID := "b0000000-0000-4000-8000-00000000c003"
	fake.AddRun(runID, fakeRun(runID, 43, "created", watchSlug, "main"))
	fake.AddRunWorkflowsV3(runID, fakeWorkflowV3(wfID, "build", runID, "a0000000-0000-4000-8000-00000000c001", "ended", "succeeded"))

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

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments (no active workflows)
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
