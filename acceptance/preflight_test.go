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
	"os"
	"os/exec"
	"path/filepath"
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
	preflightSlug      = "gh/testorg/testrepo"
	preflightProjectID = "a0000000-0000-4000-8000-0000000000ff"
	preflightID        = "4b1f8e2a-6c1d-4f7e-9a3b-2d5c8e1f0a4b"
	preflightBranch    = "cci/preflight/" + preflightID
	preflightRunID     = "f0000000-0000-4000-8000-0000000000f1"
	preflightWfID      = "b0000000-0000-4000-8000-0000000000f1"
	preflightJobID     = "d0000000-0000-4000-8000-0000000000f1"
)

// setupPreflightRepo creates a checkout with one commit, an uncommitted change
// and a bare repository as origin. It returns the checkout and origin paths.
func setupPreflightRepo(t *testing.T) (workDir, origin string) {
	t.Helper()
	root := t.TempDir()
	origin = filepath.Join(root, "origin.git")
	workDir = filepath.Join(root, "work")

	preflightGit(t, root, "init", "--quiet", "--bare", "--initial-branch=main", origin)
	preflightGit(t, root, "init", "--quiet", "--initial-branch=main", workDir)
	preflightGit(t, workDir, "remote", "add", "origin", origin)
	assert.NilError(t, os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main\n"), 0o600))
	preflightGit(t, workDir, "add", ".")
	preflightGit(t, workDir, "commit", "--quiet", "-m", "initial")
	preflightGit(t, workDir, "push", "--quiet", "origin", "main")
	assert.NilError(t, os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main // changed\n"), 0o600))
	return workDir, origin
}

func preflightGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, "git %s: %s", strings.Join(args, " "), out)
	return strings.TrimSpace(string(out))
}

// remoteBranches lists the branches on origin.
func remoteBranches(t *testing.T, origin string) string {
	t.Helper()
	return preflightGit(t, origin, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
}

func newPreflightEnv(t *testing.T, fake *fakes.CircleCI) *testenv.TestEnv {
	t.Helper()
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	env.Extra = map[string]string{
		"CIRCLE_PREFLIGHT_ID": preflightID,
		"CIRCLE_SHA_WAIT_MS":  "0",
	}
	return env
}

func runPreflight(t *testing.T, env *testenv.TestEnv, workDir string, args ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"preflight"}, args...),
		Env:     env.Environ(),
		WorkDir: workDir,
	})
}

// addPreflightRun registers the run the push "triggers", on the preflight branch.
func addPreflightRun(fake *fakes.CircleCI, phase, outcome string, jobs ...fakes.JobV3) {
	addProjectBySlug(fake, preflightSlug, preflightProjectID)
	fake.AddRunV3(preflightRunID, preflightProjectID,
		fakeRunV3(preflightRunID, preflightProjectID, phase, outcome, preflightBranch, "abc1234"))
	fake.AddRunWorkflowsV3(preflightRunID,
		fakeWorkflowV3(preflightWfID, "build", preflightRunID, preflightProjectID, phase, outcome))
	fake.AddWorkflowJobsV3(preflightWfID, jobs...)
}

func failedPreflightJob() fakes.JobV3 {
	job := fakeJobV3(preflightJobID, "test", preflightWfID, preflightProjectID)
	job.Outcome = "failed"
	job.Executions = [][]fakes.JobStep{{
		{Name: "Checkout code", Type: "checkout", Num: 1, Phase: "ended", Outcome: "succeeded", StartedAt: job.StartedAt, EndedAt: job.EndedAt},
		{Name: "Run tests", Type: "run", Num: 2, Phase: "ended", Outcome: "failed", ExitCode: new(1), StartedAt: job.StartedAt, EndedAt: job.EndedAt},
	}}
	return job
}

func TestPreflightRun_Succeeded(t *testing.T) {
	workDir, origin := setupPreflightRepo(t)
	fake := fakes.NewCircleCI(t)
	addPreflightRun(fake, "ended", "succeeded",
		fakeJobV3(preflightJobID, "test", preflightWfID, preflightProjectID))

	result := runPreflight(t, newPreflightEnv(t, fake), workDir, "run", "--project", preflightSlug, "--interval", "10ms")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Contains(result.Stderr, "to "+preflightBranch))
	assert.Check(t, cmp.Contains(result.Stderr, "Run "+preflightRunID+" succeeded"))
	assert.Check(t, cmp.Contains(result.Stderr, "Deleted branch "+preflightBranch))

	t.Run("branch is deleted from origin", func(t *testing.T) {
		assert.Check(t, cmp.Equal(remoteBranches(t, origin), "main"))
	})

	t.Run("working tree is untouched", func(t *testing.T) {
		assert.Check(t, cmp.Equal(preflightGit(t, workDir, "status", "--porcelain"), "M main.go"))
		assert.Check(t, cmp.Equal(preflightGit(t, workDir, "log", "--format=%s"), "initial"))
	})
}

func TestPreflightRun_NoLocalChanges(t *testing.T) {
	workDir, _ := setupPreflightRepo(t)
	preflightGit(t, workDir, "checkout", "--quiet", "--", ".")
	head := preflightGit(t, workDir, "rev-parse", "HEAD")
	fake := fakes.NewCircleCI(t)
	addPreflightRun(fake, "ended", "succeeded")

	result := runPreflight(t, newPreflightEnv(t, fake), workDir, "run", "--project", preflightSlug, "--interval", "10ms", "--json")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	first, _, _ := strings.Cut(result.Stdout, "\n")
	var pushed map[string]any
	assert.NilError(t, json.Unmarshal([]byte(first), &pushed))
	assert.Check(t, cmp.DeepEqual(pushed, map[string]any{
		"type":         "branch_pushed",
		"preflight_id": preflightID,
		"branch":       preflightBranch,
		"commit":       head,
		"base":         head,
		"base_branch":  "main",
		"changes":      false,
	}))
}

func TestPreflightRun_FailFastJSON(t *testing.T) {
	workDir, origin := setupPreflightRepo(t)
	fake := fakes.NewCircleCI(t)
	running := fakeJobV3("d0000000-0000-4000-8000-0000000000f2", "lint", preflightWfID, preflightProjectID)
	running.Phase = "started"
	running.Outcome = ""
	failed := failedPreflightJob()
	addPreflightRun(fake, "started", "", failed, running)
	fake.AddJobV3(failed)
	fake.AddJobTests(preflightJobID,
		fakes.TestResult{Classname: "pkg", Name: "TestOK", Result: "success"},
		fakes.TestResult{Classname: "pkg", Name: "TestBroken", Result: "failure", Message: "want 1, got 2"},
	)
	fake.SetCancelResponse(preflightWfID, http.StatusAccepted)

	result := runPreflight(t, newPreflightEnv(t, fake), workDir, "run", "--project", preflightSlug, "--interval", "10ms", "--json")

	assert.Check(t, cmp.Equal(result.ExitCode, 1), "stderr: %s", result.Stderr)

	var events []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(result.Stdout), "\n") {
		var e map[string]any
		assert.NilError(t, json.Unmarshal([]byte(line), &e), "line: %s", line)
		events = append(events, e)
	}
	assert.Assert(t, cmp.Len(events, 4), "stdout: %s", result.Stdout)

	t.Run("event order", func(t *testing.T) {
		types := make([]any, 0, len(events))
		for _, e := range events {
			types = append(types, e["type"])
		}
		assert.Check(t, cmp.DeepEqual(types, []any{"branch_pushed", "run_started", "job_failed", "run_summary"}))
	})

	t.Run("job_failed carries failed steps and tests", func(t *testing.T) {
		job := events[2]["job"].(map[string]any)
		assert.Check(t, cmp.Equal(job["name"], "test"))
		assert.Check(t, cmp.Equal(job["workflow"], "build"))
		assert.Check(t, cmp.DeepEqual(job["failed_steps"], []any{
			map[string]any{"execution": 0.0, "num": 2.0, "name": "Run tests", "exit_code": 1.0},
		}))
		assert.Check(t, cmp.DeepEqual(job["failed_tests"], []any{
			map[string]any{"classname": "pkg", "name": "TestBroken", "message": "want 1, got 2"},
		}))
		assert.Check(t, cmp.Equal(job["failed_test_count"], 1.0))
	})

	t.Run("run_summary reports an early stop and cleanup", func(t *testing.T) {
		s := events[3]
		assert.Check(t, cmp.Equal(s["outcome"], "failing"))
		assert.Check(t, cmp.Equal(s["ended"], false))
		assert.Check(t, cmp.Equal(s["run_cancelled"], true))
		assert.Check(t, cmp.Equal(s["branch_deleted"], true))
		assert.Check(t, cmp.Len(s["failed_jobs"], 1))
	})

	t.Run("the rest of the run is cancelled", func(t *testing.T) {
		cancels := 0
		for _, req := range fake.AllRequests() {
			if req.Method == http.MethodPost && req.URL.Path == "/api/v3/workflows/"+preflightWfID+"/cancel" {
				cancels++
			}
		}
		assert.Check(t, cmp.Equal(cancels, 1))
	})

	t.Run("branch is deleted from origin", func(t *testing.T) {
		assert.Check(t, cmp.Equal(remoteBranches(t, origin), "main"))
	})
}

func TestPreflightRun_NoRunStarted(t *testing.T) {
	workDir, origin := setupPreflightRepo(t)
	fake := fakes.NewCircleCI(t)
	addProjectBySlug(fake, preflightSlug, preflightProjectID)

	result := runPreflight(t, newPreflightEnv(t, fake), workDir, "run", "--project", preflightSlug, "--interval", "10ms")

	assert.Check(t, cmp.Equal(result.ExitCode, 5), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "but no run started for it"))
	assert.Check(t, cmp.Equal(remoteBranches(t, origin), "main"))
}

func TestPreflightRun_NoWatch(t *testing.T) {
	workDir, origin := setupPreflightRepo(t)
	fake := fakes.NewCircleCI(t)
	addPreflightRun(fake, "started", "")

	result := runPreflight(t, newPreflightEnv(t, fake), workDir, "run", "--project", preflightSlug, "--no-watch")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "preflight cleanup --id "+preflightID))
	assert.Check(t, cmp.Equal(remoteBranches(t, origin), preflightBranch+"\nmain"))
}

func TestPreflightRun_ProjectNotFound(t *testing.T) {
	workDir, origin := setupPreflightRepo(t)
	fake := fakes.NewCircleCI(t)

	result := runPreflight(t, newPreflightEnv(t, fake), workDir, "run", "--project", preflightSlug)

	assert.Check(t, cmp.Equal(result.ExitCode, 5), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "No project found"))
	assert.Check(t, cmp.Equal(remoteBranches(t, origin), "main"), "nothing is pushed before the project is found")
}

// pushPreflightBranch pushes HEAD to the preflight branch with the given ID.
func pushPreflightBranch(t *testing.T, workDir, id string) {
	t.Helper()
	preflightGit(t, workDir, "push", "--quiet", "origin", "HEAD:refs/heads/cci/preflight/"+id)
}

func TestPreflightCleanup(t *testing.T) {
	const runningID = "5c2a9f3b-0000-4000-8000-000000000002"
	workDir, origin := setupPreflightRepo(t)
	pushPreflightBranch(t, workDir, preflightID)
	pushPreflightBranch(t, workDir, runningID)

	fake := fakes.NewCircleCI(t)
	addPreflightRun(fake, "ended", "succeeded")
	fake.AddRunV3("f0000000-0000-4000-8000-0000000000f2", preflightProjectID,
		fakeRunV3("f0000000-0000-4000-8000-0000000000f2", preflightProjectID, "started", "", "cci/preflight/"+runningID, "abc1234"))
	env := newPreflightEnv(t, fake)

	t.Run("dry run deletes nothing", func(t *testing.T) {
		result := runPreflight(t, env, workDir, "cleanup", "--project", preflightSlug, "--dry-run")
		assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, cmp.Len(strings.Split(remoteBranches(t, origin), "\n"), 3))
	})

	t.Run("deletes finished preflights and skips running ones", func(t *testing.T) {
		result := runPreflight(t, env, workDir, "cleanup", "--project", preflightSlug, "--json")
		assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)

		var entries []map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &entries))
		statuses := map[string]any{}
		for _, e := range entries {
			statuses[e["preflight_id"].(string)] = e["status"]
		}
		assert.Check(t, cmp.DeepEqual(statuses, map[string]any{
			preflightID: "deleted",
			runningID:   "skipped_running",
		}))
		assert.Check(t, cmp.Equal(remoteBranches(t, origin), "cci/preflight/"+runningID+"\nmain"))
	})

	t.Run("include-running deletes the rest", func(t *testing.T) {
		result := runPreflight(t, env, workDir, "cleanup", "--project", preflightSlug, "--id", runningID, "--include-running")
		assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, cmp.Equal(remoteBranches(t, origin), "main"))
	})
}

func TestPreflightCleanup_InvalidID(t *testing.T) {
	workDir, _ := setupPreflightRepo(t)
	fake := fakes.NewCircleCI(t)

	result := runPreflight(t, newPreflightEnv(t, fake), workDir, "cleanup", "--id", "nope")

	assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, `--id must be a preflight UUID (got: "nope")`))
}
