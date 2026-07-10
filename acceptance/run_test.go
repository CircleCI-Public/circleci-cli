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

	"github.com/pete-woods/go-expect"
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
	// The event VCS carries the commit (subject, url, author) — the only source
	// the client reads it from. Only runs that resolved a revision have one.
	eventVCS := map[string]any{
		"branch":   branch,
		"revision": revision,
	}
	if revision != "" {
		eventVCS["commit"] = map[string]any{
			"subject": "Fix the widget",
			"url":     "https://github.com/testorg/testrepo/commit/" + revision,
			"author":  map[string]any{"name": "Ada Lovelace", "login": "ada"},
		}
	}
	return map[string]any{
		"id":         id,
		"attributes": attrs,
		"references": map[string]any{
			// VCS now lives on the event reference (and carries the tag);
			// attributes.vcs above is retained for legacy clients.
			"event": map[string]any{
				"attributes": map[string]any{
					"vcs": eventVCS,
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

	commit := out["commit"].(map[string]any)
	assert.Check(t, cmp.Equal(commit["subject"], "Fix the widget"))
	assert.Check(t, cmp.Equal(commit["author_name"], "Ada Lovelace"))

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

// TestRunGet_ProjectDefaultsBranchToMain confirms that with --project given (and
// no --branch), the latest-run lookup defaults to the main branch without
// consulting the local git remote — the checkout is meaningless for a possibly
// different project. Run from a bare temp dir (no repo): it must not error, and
// must resolve the main-branch run rather than the feature-branch one.
func TestRunGet_ProjectDefaultsBranchToMain(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	addProjectBySlug(fake, testSlug, runTestProjectID)
	mainRunID := "e0000000-0000-4000-8000-0000000000b1"
	featureRunID := "e0000000-0000-4000-8000-0000000000b2"
	fake.AddRunV3(mainRunID, runTestProjectID, fakeRunV3(mainRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunV3(featureRunID, runTestProjectID, fakeRunV3(featureRunID, runTestProjectID, "ended", "failed", "feature", "deadbeef12345678"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "--project", testSlug, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(), // not a git repo → must not be required
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["id"], mainRunID)) // the main-branch run, not feature
}

// TestRunGet_NoInteractive_SkipsPicker confirms --no-interactive bypasses the
// picker even in a PTY: a session that would otherwise open the run picker
// instead resolves the latest run and prints its summary, which carries the
// run's full UUID (the picker's first screen never does).
func TestRunGet_NoInteractive_SkipsPicker(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	addProjectBySlug(fake, testSlug, runTestProjectID)
	runID := "e0000000-0000-4000-8000-0000000000c1"
	fake.AddRunV3(runID, runTestProjectID, fakeRunV3(runID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "--project", testSlug, "--branch", "main", "--no-interactive"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// The run summary prints the run's full UUID; the picker would show
	// "Select a run" instead.
	_, err := console.ExpectString(runID)
	assert.NilError(t, err)
}

// --- run get (interactive picker) ---

const (
	irunRun1ID = "e0000000-0000-4000-8000-0000000000a1"
	irunRun2ID = "e0000000-0000-4000-8000-0000000000a2"
	irunRun3ID = "e0000000-0000-4000-8000-0000000000a3"
	irunRun4ID = "e0000000-0000-4000-8000-0000000000a4"
	irunWfID   = "b0000000-0000-4000-8000-0000000000a1"
	irunJob1ID = "d0000000-0000-4000-8000-0000000000a1"
	irunJob2ID = "d0000000-0000-4000-8000-0000000000a2"

	// keyEsc is a lone Escape byte. In the interactive picker esc means "go
	// back a step" (except on the first picker, where it quits).
	keyEsc = "\x1b"
	keyUp  = "\x1b[A"
	// keyCtrlC is the ETX byte; in raw mode the picker decodes it as ctrl+c and
	// quits the whole flow.
	keyCtrlC = "\x03"
)

// setupRunGetInteractiveFake wires a project with two recent runs on branch
// main — one succeeded, one failed — where the first run has a "build" workflow
// containing two jobs. The "run-tests" job has a single execution with two steps
// (one failed); the "deploy" job has parallelism 2 (execution 0 succeeded,
// execution 1 failed) so it exercises the execution picker. Step output is
// registered for the failing steps. It registers the per-resource GET endpoints
// the drill-down reaches, so "see all workflows" (run summary), "all jobs"
// (workflow summary), the job report and full output report, the execution
// picker, and a single step's output all resolve.
func setupRunGetInteractiveFake(t *testing.T) *testenv.TestEnv {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	addProjectBySlug(fake, testSlug, runTestProjectID)
	fake.AddRunV3(irunRun1ID, runTestProjectID, fakeRunV3(irunRun1ID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunV3(irunRun2ID, runTestProjectID, fakeRunV3(irunRun2ID, runTestProjectID, "ended", "failed", "main", "deadbeef12345678"))
	// A run on a different branch, surfaced only when the picker is toggled to
	// "all branches" (the search filters by branch otherwise).
	fake.AddRunV3(irunRun3ID, runTestProjectID, fakeRunV3(irunRun3ID, runTestProjectID, "ended", "succeeded", "feature", "facefeed12345678"))
	// A cross-project run surfaced only under the "my runs" scope (user-filtered,
	// not branch-filtered). Its distinct revision gives the toggle a unique token.
	// It carries a repository URL, which the picker folds into its ref bracket as
	// "[org/repo:branch]".
	myRun := fakeRunV3(irunRun4ID, runTestProjectID, "ended", "succeeded", "mine", "cafed00d12345678")
	myRun["attributes"].(map[string]any)["vcs"].(map[string]any)["repository_url"] = "https://github.com/testorg/testrepo"
	fake.SetUserRuns(myRun)

	wf := fakeWorkflowV3(irunWfID, "build", irunRun1ID, runTestProjectID, "ended", "succeeded")
	fake.AddRunWorkflowsV3(irunRun1ID, wf)
	fake.AddWorkflowV3(irunWfID, wf)
	fake.AddWorkflowJobsV3(irunWfID,
		fakeJobV3(irunJob1ID, "run-tests", irunWfID, runTestProjectID),
		fakeJobV3(irunJob2ID, "deploy", irunWfID, runTestProjectID),
	)

	// The job the step picker drills into: two steps, the second failed.
	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Format(v3TimeFormat)
	fake.AddJobV3(irunJob1ID, map[string]any{"data": map[string]any{
		"id": irunJob1ID,
		"attributes": map[string]any{
			"name": "run-tests", "type": "build", "phase": "ended", "outcome": "failed",
			"started_at": now, "ended_at": now,
			"parallel_executions": []map[string]any{{
				"steps": []map[string]any{
					{"name": "Spin up environment", "type": "spinup_environment", "num": 0, "phase": "ended", "outcome": "succeeded", "started_at": now, "ended_at": now},
					{"name": "run tests", "type": "run", "num": 101, "phase": "ended", "outcome": "failed", "exit_code": 1, "started_at": now, "ended_at": now},
				},
			}},
		},
		"references": map[string]any{
			"workflow": map[string]any{"id": irunWfID},
			"project":  map[string]any{"id": runTestProjectID},
		},
	}})
	fake.AddJobStdout(irunJob1ID, 0, 0, []byte("environment ready\n"))
	fake.AddJobStderr(irunJob1ID, 0, 0, []byte(""))
	fake.AddJobStdout(irunJob1ID, 0, 101, []byte("FAILURE: 2 tests failed\n"))
	fake.AddJobStderr(irunJob1ID, 0, 101, []byte(""))
	// Test metadata for the "Failed tests" meta option (served as JSONL).
	fake.AddJobTests(irunJob1ID,
		map[string]any{"classname": "pkg/foo", "name": "TestThatFailed", "result": "failure", "run_time": 0.5, "message": "assertion failed: want 1 got 2"},
		map[string]any{"classname": "pkg/foo", "name": "TestThatPassed", "result": "success", "run_time": 0.1, "message": ""},
	)

	// A parallel job (parallelism 2): execution 0 succeeded, execution 1 failed.
	deployStep := func(outcome string, exit int) []map[string]any {
		return []map[string]any{
			{"name": "Spin up environment", "type": "spinup_environment", "num": 0, "phase": "ended", "outcome": "succeeded", "started_at": now, "ended_at": now},
			{"name": "deploy", "type": "run", "num": 50, "phase": "ended", "outcome": outcome, "exit_code": exit, "started_at": now, "ended_at": now},
		}
	}
	fake.AddJobV3(irunJob2ID, map[string]any{"data": map[string]any{
		"id": irunJob2ID,
		"attributes": map[string]any{
			"name": "deploy", "type": "build", "phase": "ended", "outcome": "failed",
			"started_at": now, "ended_at": now,
			"parallel_executions": []map[string]any{
				{"steps": deployStep("succeeded", 0)},
				{"steps": deployStep("failed", 1)},
			},
		},
		"references": map[string]any{
			"workflow": map[string]any{"id": irunWfID},
			"project":  map[string]any{"id": runTestProjectID},
		},
	}})
	fake.AddJobStdout(irunJob2ID, 1, 50, []byte("deploy failed\n"))
	fake.AddJobStderr(irunJob2ID, 1, 50, []byte(""))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return env
}

// drillToStepPicker drives the flow run → workflow (build) → job (run-tests)
// and returns once the step picker is showing.
func drillToStepPicker(t *testing.T, console *expect.Console) {
	t.Helper()
	_, err := console.ExpectString("Select a run")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("build")
	assert.NilError(t, err)
	_, err = console.Send(keyDown + "\r") // skip "see all workflows"
	assert.NilError(t, err)

	_, err = console.ExpectString("run-tests")
	assert.NilError(t, err)
	_, err = console.Send(keyDown + "\r") // skip "all jobs in workflow"
	assert.NilError(t, err)

	// The step picker is ready once a step row has rendered.
	_, err = console.ExpectString("Spin up environment")
	assert.NilError(t, err)
}

// drillToExecutionPicker drives the flow run → workflow (build) → job (deploy,
// which has parallelism 2) and returns once the execution picker is showing.
func drillToExecutionPicker(t *testing.T, console *expect.Console) {
	t.Helper()
	_, err := console.ExpectString("Select a run")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("build")
	assert.NilError(t, err)
	_, err = console.Send(keyDown + "\r") // skip "see all workflows"
	assert.NilError(t, err)

	// Pick "deploy": skip "all jobs in workflow" and "run-tests".
	_, err = console.ExpectString("deploy")
	assert.NilError(t, err)
	_, err = console.Send(keyDown + keyDown + "\r")
	assert.NilError(t, err)

	// The execution picker is ready once an execution row has rendered.
	_, err = console.ExpectString("Execution 1")
	assert.NilError(t, err)
}

// startRunGetInteractive launches "run get" with no run ID in interactive mode,
// pinning the project and branch so the flow does not depend on a git remote.
func startRunGetInteractive(t *testing.T, env *testenv.TestEnv) *expect.Console {
	t.Helper()
	return binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "--project", testSlug, "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

// Because the whole flow is one long-lived bubbletea program, its renderer
// diffs against the previous frame: a changed title line (e.g. "Select a run" →
// "Select a workflow") is emitted as a partial update and never appears
// contiguously in the PTY stream. The picker option lines, however, differ
// wholesale between stages and so are rewritten in full — so these tests assert
// on option text and final output, matching the title only on the first frame.

// TestRunGet_Interactive_SelectStep drills run → workflow → job and lands on the
// step picker, whose cursor defaults to the first failed step. Selecting it
// prints that step's raw output after the program exits.
func TestRunGet_Interactive_SelectStep(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)
	drillToStepPicker(t, console)

	// The cursor defaults to the first failed step ("run tests"); select it.
	_, err := console.Send("\r")
	assert.NilError(t, err)

	// Its output streams into the pager.
	_, err = console.ExpectString("FAILURE: 2 tests failed")
	assert.NilError(t, err)

	// esc returns to the step picker, resuming on the same step.
	_, err = console.Send(keyEsc)
	assert.NilError(t, err)
	_, err = console.ExpectString("Spin up environment")
	assert.NilError(t, err)

	// ctrl+c quits the flow.
	_, err = console.Send(keyCtrlC)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_JobReport picks the "Job report (summary)" option at the
// top of the step picker, which prints the short job summary. The summary lists
// the workflow ID, which the full output report does not — so it confirms the
// summary (not the full report) was printed.
func TestRunGet_Interactive_JobReport(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)
	drillToStepPicker(t, console)

	// Cursor starts on the failed step (index 4, below the three meta options);
	// four ups reach the first option, "Job report (summary)".
	_, err := console.Send(keyUp + keyUp + keyUp + keyUp + "\r")
	assert.NilError(t, err)

	_, err = console.ExpectString(irunWfID)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_FullOutputReport picks the "Full job report" option,
// which prints every step's rendered output.
func TestRunGet_Interactive_FullOutputReport(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)
	drillToStepPicker(t, console)

	// Cursor starts on the failed step (index 4, below the three meta options);
	// three ups reach the second option, "Full job report (including step output)".
	_, err := console.Send(keyUp + keyUp + keyUp + "\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("FAILURE: 2 tests failed")
	assert.NilError(t, err)
}

// TestRunGet_Interactive_FailedTests picks the "Failed tests" meta option, then
// selects the one failed test and reads its message in the pager.
func TestRunGet_Interactive_FailedTests(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)
	drillToStepPicker(t, console)

	// Cursor starts on the failed step (index 4); two ups reach the third option,
	// "Failed tests".
	_, err := console.Send(keyUp + keyUp + "\r")
	assert.NilError(t, err)

	// The failed-test picker lists the one failing test (passing tests excluded).
	_, err = console.ExpectString("TestThatFailed (pkg/foo)")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	// Its message opens in the pager.
	_, err = console.ExpectString("assertion failed: want 1 got 2")
	assert.NilError(t, err)

	// ctrl+c quits the flow from the pager.
	_, err = console.Send(keyCtrlC)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_ParallelExecution drills into a job with parallelism > 1,
// which inserts an execution picker before the step picker. The execution cursor
// defaults to the failed execution; its step picker then defaults to the failed
// step, whose output is printed.
func TestRunGet_Interactive_ParallelExecution(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)
	drillToExecutionPicker(t, console)

	// Cursor defaults to the failed Execution 1; select it.
	_, err := console.Send("\r")
	assert.NilError(t, err)

	// The step picker is scoped to execution 1 and has no summary options (those
	// live on the execution picker); its cursor defaults to the failed step.
	_, err = console.ExpectString("deploy [exit: 1]")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	// The step's output streams into the pager.
	_, err = console.ExpectString("deploy failed")
	assert.NilError(t, err)

	// ctrl+c quits the flow from the pager.
	_, err = console.Send(keyCtrlC)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_ParallelJobReport confirms the job summaries live on the
// execution picker for a parallel job: picking "Job report (summary)" there
// prints the short job summary (which lists the workflow ID).
func TestRunGet_Interactive_ParallelJobReport(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)
	drillToExecutionPicker(t, console)

	// Cursor starts on the failed Execution 1 (index 4, below the three meta
	// options); four ups reach the first option, "Job report (summary)".
	_, err := console.Send(keyUp + keyUp + keyUp + keyUp + "\r")
	assert.NilError(t, err)

	_, err = console.ExpectString(irunWfID)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_AllWorkflows picks a run then chooses "see all
// workflows", which prints the same run summary as "run get <id>".
func TestRunGet_Interactive_AllWorkflows(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)

	_, err := console.ExpectString("Select a run")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	_, err = console.ExpectString("See all workflows")
	assert.NilError(t, err)
	_, err = console.Send("\r") // "see all workflows" is the first option
	assert.NilError(t, err)

	// The run summary carries the run UUID, which never appears in the pickers.
	_, err = console.ExpectString(irunRun1ID)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_Back exercises esc as back-navigation: from the
// workflow picker, esc returns to the run picker (it does not quit). After
// re-selecting, choosing "all jobs in workflow" prints the workflow summary.
func TestRunGet_Interactive_Back(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)

	_, err := console.ExpectString("Select a run")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	// At the workflow picker, esc goes back to the run picker.
	_, err = console.ExpectString("See all workflows")
	assert.NilError(t, err)
	_, err = console.Send(keyEsc)
	assert.NilError(t, err)

	// The run picker is shown again rather than the program exiting; its option
	// lines are rewritten in full, so the run row reappears.
	_, err = console.ExpectString("[main] abc1234")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	// Back at the workflow picker, pick the build workflow this time.
	_, err = console.ExpectString("build")
	assert.NilError(t, err)
	_, err = console.Send(keyDown + "\r")
	assert.NilError(t, err)

	// Choose "all jobs in workflow", which prints the workflow summary.
	_, err = console.ExpectString("All jobs in workflow")
	assert.NilError(t, err)
	_, err = console.Send("\r")
	assert.NilError(t, err)

	// The workflow summary carries the workflow UUID.
	_, err = console.ExpectString(irunWfID)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_SwitchBranch toggles the run picker from the current
// branch (main) to "all branches": the feature-branch run, hidden while scoped
// to main, appears once the search is unfiltered. The switch key is the
// platform's binding — shift+tab normally, plain Tab on Windows, where the
// ConPTY/ultraviolet input stack does not deliver shift+tab.
func TestRunGet_Interactive_SwitchBranch(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)

	// Scoped to main: the main run shows, the feature run does not yet.
	_, err := console.ExpectString("[main] abc1234")
	assert.NilError(t, err)

	// Cycle main → all branches (no default branch is known here; the cycle is
	// main → all branches → my runs), re-fetching without a branch filter.
	switchSeq := "\x1b[Z" // shift+tab (CSI Z)
	if runtime.GOOS == "windows" {
		switchSeq = "\t" // Windows binds plain Tab
	}
	_, err = console.Send(switchSeq)
	assert.NilError(t, err)

	// The feature-branch run is now listed.
	_, err = console.ExpectString("[feature] facefee")
	assert.NilError(t, err)

	// esc on the first picker quits, so the program exits cleanly.
	_, err = console.Send(keyEsc)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_SwitchToMyRuns cycles the run picker past "all branches"
// to the "my runs" scope, which lists the authenticated user's runs across all
// projects (via GET /api/v3/runs?filter[user_id]=me) rather than filtering by
// branch. The cross-project run — hidden under both branch scopes — appears once
// the scope reaches "my runs".
func TestRunGet_Interactive_SwitchToMyRuns(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)

	// Scoped to main: the main run shows, the my-runs entry does not yet.
	_, err := console.ExpectString("[main] abc1234")
	assert.NilError(t, err)

	switchSeq := "\x1b[Z" // shift+tab (CSI Z)
	if runtime.GOOS == "windows" {
		switchSeq = "\t" // Windows binds plain Tab
	}
	// main → all branches → my runs.
	_, err = console.Send(switchSeq)
	assert.NilError(t, err)
	_, err = console.ExpectString("[feature] facefee")
	assert.NilError(t, err)
	_, err = console.Send(switchSeq)
	assert.NilError(t, err)

	// The user's cross-project run is now listed, its project (the "org/repo" slug
	// from its repository URL) folded into the ref bracket as "[project:branch]".
	// (The picker title also names the scope "[my runs]", asserted in the ui
	// package's flow test — the PTY renderer diffs the title line so it does not
	// appear contiguously here.)
	_, err = console.ExpectString("[testorg/testrepo:mine] cafed00")
	assert.NilError(t, err)

	// esc on the first picker quits, so the program exits cleanly.
	_, err = console.Send(keyEsc)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_NoProject runs "run get" interactively with neither a
// --project flag nor a resolvable git remote (a bare temp dir). Rather than
// erroring that no project could be inferred, the flow falls back to the
// cross-project "my runs" scope and opens directly on the user's runs.
func TestRunGet_Interactive_NoProject(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(), // not a git repo → detection fails
	})

	// The picker opens straight on the user's cross-project runs, its project
	// folded into the ref bracket as "[project:branch]".
	_, err := console.ExpectString("[testorg/testrepo:mine] cafed00")
	assert.NilError(t, err)

	// esc on the first picker quits, so the program exits cleanly.
	_, err = console.Send(keyEsc)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_ProjectNoBranch runs the interactive picker with
// --project but no --branch, outside a git repo. It must not require a remote:
// the branch defaults to main, so the project's main-branch runs load directly.
func TestRunGet_Interactive_ProjectNoBranch(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "--project", testSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(), // not a git repo → must not be required
	})

	// The defaulted main branch's run loads (not the feature-branch one).
	_, err := console.ExpectString("[main] abc1234")
	assert.NilError(t, err)

	// esc on the first picker quits, so the program exits cleanly.
	_, err = console.Send(keyEsc)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_FilterStatus presses "s" to narrow the run picker by
// pipeline status. The first status in the cycle is "canceled", which no
// main-branch run has, so the picker keeps its list and shows the empty-status
// note — confirming "s" issues a status-filtered fetch (the fake filters on
// pipeline.status).
func TestRunGet_Interactive_FilterStatus(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)

	_, err := console.ExpectString("[main] abc1234")
	assert.NilError(t, err)

	// all statuses → canceled (no canceled runs on main). Match a contiguous
	// slice of the note: the PTY renderer rewrites the line char-by-char and may
	// splice escape sequences between the first letters ("N\x1b[4ho canceled…").
	_, err = console.Send("s")
	assert.NilError(t, err)
	_, err = console.ExpectString("canceled runs on main")
	assert.NilError(t, err)

	// esc on the first picker quits, so the program exits cleanly.
	_, err = console.Send(keyEsc)
	assert.NilError(t, err)
}

// TestRunGet_Interactive_FilterStatusMyRuns filters the cross-project "my runs"
// scope by status. That scope lists via GET /api/v3/runs, which has no status
// filter — the CLI converts the status to filter[phase]/filter[current_outcome]
// (apiclient.StatusPhaseOutcome). The only user run succeeded, so filtering to
// "canceled" empties the list, exercising that conversion end-to-end.
func TestRunGet_Interactive_FilterStatusMyRuns(t *testing.T) {
	env := setupRunGetInteractiveFake(t)
	console := startRunGetInteractive(t, env)

	_, err := console.ExpectString("[main] abc1234")
	assert.NilError(t, err)

	switchSeq := "\x1b[Z" // shift+tab (CSI Z)
	if runtime.GOOS == "windows" {
		switchSeq = "\t" // Windows binds plain Tab
	}
	// main → all branches → my runs.
	_, err = console.Send(switchSeq)
	assert.NilError(t, err)
	_, err = console.ExpectString("[feature] facefee")
	assert.NilError(t, err)
	_, err = console.Send(switchSeq)
	assert.NilError(t, err)
	_, err = console.ExpectString("[testorg/testrepo:mine] cafed00")
	assert.NilError(t, err)

	// my runs, all statuses → canceled: the one user run succeeded, so the
	// phase=ended/current_outcome=canceled filter matches nothing.
	_, err = console.Send("s")
	assert.NilError(t, err)
	_, err = console.ExpectString("canceled runs in my runs")
	assert.NilError(t, err)

	_, err = console.Send(keyEsc)
	assert.NilError(t, err)
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

// TestRunList_NoVCS covers a run that resolved no commit — e.g. a not_run run
// whose config could not be fetched. The Ref and Revision cells should show "-"
// rather than blank, and the status should read "not run".
func TestRunList_NoVCS(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	slug := watchSlug
	addProjectBySlug(fake, slug, runTestProjectID)
	fake.AddRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, fakeRunV3("e0000000-0000-4000-8000-000000000001", runTestProjectID, "ended", "not_run", "", ""))

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
	assert.Check(t, strings.Contains(result.Stdout, "not run"), "status should read 'not run': %s", result.Stdout)
	assert.Check(t, strings.Contains(result.Stdout, " - "), "empty ref/revision cells should show '-': %s", result.Stdout)
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

	// The JSON carries the full commit detail (the markdown shows only the subject).
	commit := out[0]["commit"].(map[string]any)
	assert.Check(t, cmp.Equal(commit["subject"], "Fix the widget"))
	assert.Check(t, cmp.Equal(commit["author_name"], "Ada Lovelace"))
	assert.Check(t, cmp.Equal(commit["author_login"], "ada"))

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
