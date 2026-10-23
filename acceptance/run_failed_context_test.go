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
	"fmt"
	"net/http"
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
	fcRunID  = "e0000000-0000-4000-8000-0000000000f1"
	fcWfID   = "b0000000-0000-4000-8000-0000000000f1"
	fcJob1ID = "d0000000-0000-4000-8000-0000000000f1" // single execution, one failed step
	fcJob2ID = "d0000000-0000-4000-8000-0000000000f2" // parallel job, one of two executions failed
	fcJob3ID = "d0000000-0000-4000-8000-0000000000f3" // succeeded, no failures — should not appear
	fcJob4ID = "d0000000-0000-4000-8000-0000000000f4" // not-run job — GetJobV3 returns 404
)

// --- run get --failure-report ---

// TestRunGet_FailedContext prints condensed output for every failed step across
// a run's workflows and jobs, organised as workflow -> job -> step headers, and
// bypasses the interactive TUI even in a TTY-less run (the default acceptance
// invocation is already non-interactive, but --failure-report is what forces it).
func TestRunGet_FailedContext(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(fcRunID, runTestProjectID, fakeRunV3(fcRunID, runTestProjectID, "ended", "failed", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(fcRunID, fakeWorkflowV3(fcWfID, "build", fcRunID, runTestProjectID, "ended", "failed"))
	fake.AddWorkflowJobsV3(fcWfID,
		fakeJobV3(fcJob1ID, "run-tests", fcWfID, runTestProjectID),
		fakeJobV3(fcJob3ID, "lint", fcWfID, runTestProjectID),
	)

	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Format(v3TimeFormat)
	fake.AddJobV3(fakes.JobV3{
		ID: fcJob1ID, Name: "run-tests", Type: "build", Phase: "ended", Outcome: "failed",
		StartedAt: now, EndedAt: now, WorkflowID: fcWfID, ProjectID: runTestProjectID,
		Executions: [][]fakes.JobStep{{
			{Name: "Spin up environment", Type: "spinup_environment", Num: 0, Phase: "ended", Outcome: "succeeded", StartedAt: now, EndedAt: now},
			{Name: "run tests", Type: "run", Num: 101, Phase: "ended", Outcome: "failed", ExitCode: new(1), StartedAt: now, EndedAt: now},
		}},
	})
	fake.AddJobStdoutCondensed(fcJob1ID, 0, 101, []byte("FAILURE: 2 tests failed\n"))

	// A succeeded job in the same workflow contributes no output at all.
	fake.AddJobV3(fakes.JobV3{
		ID: fcJob3ID, Name: "lint", Type: "build", Phase: "ended", Outcome: "succeeded",
		StartedAt: now, EndedAt: now, WorkflowID: fcWfID, ProjectID: runTestProjectID,
		Executions: [][]fakes.JobStep{{
			{Name: "run lint", Type: "run", Num: 100, Phase: "ended", Outcome: "succeeded", ExitCode: new(0), StartedAt: now, EndedAt: now},
		}},
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestRunGet_FailedContext_ParallelExecution exercises a parallel job (two
// executions) where only one execution failed: the job header includes the
// execution index and an "N of M failed" count, and only the failed execution's
// steps are printed.
func TestRunGet_FailedContext_ParallelExecution(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(fcRunID, runTestProjectID, fakeRunV3(fcRunID, runTestProjectID, "ended", "failed", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(fcRunID, fakeWorkflowV3(fcWfID, "build", fcRunID, runTestProjectID, "ended", "failed"))
	fake.AddWorkflowJobsV3(fcWfID,
		fakeJobV3(fcJob2ID, "deploy", fcWfID, runTestProjectID),
	)

	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Format(v3TimeFormat)
	deployStep := func(outcome string, exit int) []fakes.JobStep {
		return []fakes.JobStep{
			{Name: "Spin up environment", Type: "spinup_environment", Num: 0, Phase: "ended", Outcome: "succeeded", StartedAt: now, EndedAt: now},
			{Name: "deploy", Type: "run", Num: 50, Phase: "ended", Outcome: outcome, ExitCode: new(exit), StartedAt: now, EndedAt: now},
		}
	}
	fake.AddJobV3(fakes.JobV3{
		ID: fcJob2ID, Name: "deploy", Type: "build", Phase: "ended", Outcome: "failed",
		StartedAt: now, EndedAt: now, WorkflowID: fcWfID, ProjectID: runTestProjectID,
		Executions: [][]fakes.JobStep{
			deployStep("succeeded", 0),
			deployStep("failed", 1),
		},
	})
	fake.AddJobStdoutCondensed(fcJob2ID, 1, 50, []byte("deploy failed\n"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestRunGet_FailedContext_NoFailures exits 0 with no output when the run has no
// failed steps.
func TestRunGet_FailedContext_NoFailures(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(fcRunID, runTestProjectID, fakeRunV3(fcRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(fcRunID, fakeWorkflowV3(fcWfID, "build", fcRunID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(fcWfID,
		fakeJobV3(fcJob3ID, "lint", fcWfID, runTestProjectID),
	)

	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Format(v3TimeFormat)
	fake.AddJobV3(fakes.JobV3{
		ID: fcJob3ID, Name: "lint", Type: "build", Phase: "ended", Outcome: "succeeded",
		StartedAt: now, EndedAt: now, WorkflowID: fcWfID, ProjectID: runTestProjectID,
		Executions: [][]fakes.JobStep{{
			{Name: "run lint", Type: "run", Num: 100, Phase: "ended", Outcome: "succeeded", ExitCode: new(0), StartedAt: now, EndedAt: now},
		}},
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestRunGet_FailedContext_RunErrors covers the report for a run that failed
// without any job failing: a dynamic-config run whose continued config was
// rejected has no failed step to condense, so the error the run carries is the
// only thing there is to report — and the report used to be empty.
func TestRunGet_FailedContext_RunErrors(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	run := fakeRunV3(fcRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678")
	run.Errors = []fakes.RunError{{
		Type:    "config",
		Message: "Error calling workflow: 'deploy'\nCannot find a definition for job named release",
	}}
	fake.AddRunV3(fcRunID, runTestProjectID, run)
	fake.AddRunWorkflowsV3(fcRunID, fakeWorkflowV3(fcWfID, "setup", fcRunID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(fcWfID, fakeJobV3(fcJob3ID, "setup-job", fcWfID, runTestProjectID))

	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Format(v3TimeFormat)
	fake.AddJobV3(fakes.JobV3{
		ID: fcJob3ID, Name: "setup-job", Type: "build", Phase: "ended", Outcome: "succeeded",
		StartedAt: now, EndedAt: now, WorkflowID: fcWfID, ProjectID: runTestProjectID,
		Executions: [][]fakes.JobStep{{
			{Name: "continue", Type: "run", Num: 100, Phase: "ended", Outcome: "succeeded", ExitCode: new(0), StartedAt: now, EndedAt: now},
		}},
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestRunGet_FailureReport_RejectsJSON verifies that combining --failure-report with
// --json exits non-zero with a user-facing error.
func TestRunGet_FailureReport_RejectsJSON(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = "https://circleci.com" // never reached

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--failure-report", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, cmp.Contains(result.Stderr, "run.failure_report_no_json"))
}

// TestRunGet_FailedContext_JobNotFound verifies that jobs whose GetJobV3 call
// returns 404 (not-run / skipped jobs) are silently skipped rather than
// aborting the report with an error. This reproduces a real-world failure where
// workflows contain jobs with no status (e.g. "deploy.release-reporting") that
// the API cannot serve because they never executed.
func TestRunGet_FailedContext_JobNotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(fcRunID, runTestProjectID, fakeRunV3(fcRunID, runTestProjectID, "ended", "failed", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(fcRunID, fakeWorkflowV3(fcWfID, "build", fcRunID, runTestProjectID, "ended", "failed"))
	// fcJob4ID is intentionally NOT registered with AddJobV3 — the fake returns
	// 404 for it, simulating a not-run job in the workflow job list.
	fake.AddWorkflowJobsV3(fcWfID,
		fakeJobV3(fcJob1ID, "run-tests", fcWfID, runTestProjectID),
		fakeJobV3(fcJob4ID, "deploy.release-reporting", fcWfID, runTestProjectID),
	)

	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Format(v3TimeFormat)
	fake.AddJobV3(fakes.JobV3{
		ID: fcJob1ID, Name: "run-tests", Type: "build", Phase: "ended", Outcome: "failed",
		StartedAt: now, EndedAt: now, WorkflowID: fcWfID, ProjectID: runTestProjectID,
		Executions: [][]fakes.JobStep{{
			{Name: "Spin up environment", Type: "spinup_environment", Num: 0, Phase: "ended", Outcome: "succeeded", StartedAt: now, EndedAt: now},
			{Name: "run tests", Type: "run", Num: 101, Phase: "ended", Outcome: "failed", ExitCode: new(1), StartedAt: now, EndedAt: now},
		}},
	})
	fake.AddJobStdoutCondensed(fcJob1ID, 0, 101, []byte("FAILURE: 2 tests failed\n"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestRunGet_FailedContext_WorkflowsNotFound exits 0 with no output when the
// run's workflows have not materialised yet (404).
func TestRunGet_FailedContext_WorkflowsNotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(fcRunID, runTestProjectID, fakeRunV3(fcRunID, runTestProjectID, "created", "", "main", "abc1234def5678"))
	fake.SetRunWorkflowsV3NotFound(fcRunID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// addFailedStepJob registers a single-execution job with one failed step and its
// condensed output, the shape the next-steps tests need many of.
func addFailedStepJob(fake *fakes.CircleCI, jobID, name, workflowID string) {
	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Format(v3TimeFormat)
	fake.AddJobV3(fakes.JobV3{
		ID: jobID, Name: name, Type: "build", Phase: "ended", Outcome: "failed",
		StartedAt: now, EndedAt: now, WorkflowID: workflowID, ProjectID: runTestProjectID,
		Executions: [][]fakes.JobStep{{
			{Name: "run tests", Type: "run", Num: 101, Phase: "ended", Outcome: "failed", ExitCode: new(1), StartedAt: now, EndedAt: now},
		}},
	})
	fake.AddJobStdoutCondensed(jobID, 0, 101, []byte(name+" failed\n"))
}

// testsRequests counts the requests the report made to the job tests endpoint.
func testsRequests(fake *fakes.CircleCI) int {
	n := 0
	for _, req := range fake.AllRequests() {
		if strings.HasPrefix(req.URL.Path, "/api/v3/jobs/") && strings.HasSuffix(req.URL.Path, "/tests") {
			n++
		}
	}
	return n
}

// TestRunGet_FailedContext_NextSteps_FailedTests points at testresult list only
// for the failed job whose tests include a failure, and reads the tests of each
// failed job once.
func TestRunGet_FailedContext_NextSteps_FailedTests(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(fcRunID, runTestProjectID, fakeRunV3(fcRunID, runTestProjectID, "ended", "failed", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(fcRunID, fakeWorkflowV3(fcWfID, "build", fcRunID, runTestProjectID, "ended", "failed"))
	fake.AddWorkflowJobsV3(fcWfID,
		fakeJobV3(fcJob1ID, "run-tests", fcWfID, runTestProjectID),
		fakeJobV3(fcJob2ID, "lint", fcWfID, runTestProjectID),
	)
	addFailedStepJob(fake, fcJob1ID, "run-tests", fcWfID)
	addFailedStepJob(fake, fcJob2ID, "lint", fcWfID)
	fake.AddJobTests(fcJob1ID,
		fakes.TestResult{Classname: "pkg", Name: "TestOK", Result: "success"},
		fakes.TestResult{Classname: "pkg", Name: "TestBroken", Result: "failure", Message: "want 1, got 2"},
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
	assert.Check(t, cmp.Equal(testsRequests(fake), 2), "one tests read per failed job")
}

// TestRunGet_FailedContext_NextSteps_TestsEndpointErrors drops the testresult
// hint when the tests endpoint fails, and the report still succeeds.
func TestRunGet_FailedContext_NextSteps_TestsEndpointErrors(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(fcRunID, runTestProjectID, fakeRunV3(fcRunID, runTestProjectID, "ended", "failed", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(fcRunID, fakeWorkflowV3(fcWfID, "build", fcRunID, runTestProjectID, "ended", "failed"))
	fake.AddWorkflowJobsV3(fcWfID, fakeJobV3(fcJob1ID, "run-tests", fcWfID, runTestProjectID))
	addFailedStepJob(fake, fcJob1ID, "run-tests", fcWfID)
	fake.AddJobTests(fcJob1ID, fakes.TestResult{Name: "TestBroken", Result: "failure"})
	fake.SetJobTestsResponse(fcJob1ID, http.StatusInternalServerError)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestRunGet_FailedContext_NextSteps_Caps lists three failed-tests and rerun
// lines and counts the rest, while the report body still covers every job.
func TestRunGet_FailedContext_NextSteps_Caps(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(fcRunID, runTestProjectID, fakeRunV3(fcRunID, runTestProjectID, "ended", "failed", "main", "abc1234def5678"))

	const failedJobs = 5
	workflows := make([]fakes.WorkflowV3, 0, failedJobs)
	for i := range failedJobs {
		wfID := fmt.Sprintf("b0000000-0000-4000-8000-0000000000c%d", i)
		jobID := fmt.Sprintf("d0000000-0000-4000-8000-0000000000c%d", i)
		wfName := fmt.Sprintf("wf-%d", i)
		jobName := fmt.Sprintf("job-%d", i)
		workflows = append(workflows, fakeWorkflowV3(wfID, wfName, fcRunID, runTestProjectID, "ended", "failed"))
		fake.AddWorkflowJobsV3(wfID, fakeJobV3(jobID, jobName, wfID, runTestProjectID))
		addFailedStepJob(fake, jobID, jobName, wfID)
		fake.AddJobTests(jobID, fakes.TestResult{Name: "TestBroken", Result: "failure"})
	}
	fake.AddRunWorkflowsV3(fcRunID, workflows...)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
	assert.Check(t, cmp.Equal(testsRequests(fake), failedJobs), "one tests read per failed job, past the cap too")
}
