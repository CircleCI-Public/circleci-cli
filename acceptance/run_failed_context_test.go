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
)

// --- run get --log-failed ---

// TestRunGet_FailedContext prints condensed output for every failed step across
// a run's workflows and jobs, organised as workflow -> job -> step headers, and
// bypasses the interactive TUI even in a TTY-less run (the default acceptance
// invocation is already non-interactive, but --log-failed is what forces it).
func TestRunGet_FailedContext(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(fcRunID, runTestProjectID, fakeRunV3(fcRunID, runTestProjectID, "ended", "failed", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(fcRunID, fakeWorkflowV3(fcWfID, "build", fcRunID, runTestProjectID, "ended", "failed"))
	fake.AddWorkflowJobsV3(fcWfID,
		fakeJobV3(fcJob1ID, "run-tests", fcWfID, runTestProjectID),
		fakeJobV3(fcJob3ID, "lint", fcWfID, runTestProjectID),
	)

	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Format(v3TimeFormat)
	fake.AddJobV3(fcJob1ID, map[string]any{"data": map[string]any{
		"id": fcJob1ID,
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
			"workflow": map[string]any{"id": fcWfID},
			"project":  map[string]any{"id": runTestProjectID},
		},
	}})
	fake.AddJobStdoutCondensed(fcJob1ID, 0, 101, []byte("FAILURE: 2 tests failed\n"))

	// A succeeded job in the same workflow contributes no output at all.
	fake.AddJobV3(fcJob3ID, map[string]any{"data": map[string]any{
		"id": fcJob3ID,
		"attributes": map[string]any{
			"name": "lint", "type": "build", "phase": "ended", "outcome": "succeeded",
			"started_at": now, "ended_at": now,
			"parallel_executions": []map[string]any{{
				"steps": []map[string]any{
					{"name": "run lint", "type": "run", "num": 100, "phase": "ended", "outcome": "succeeded", "exit_code": 0, "started_at": now, "ended_at": now},
				},
			}},
		},
		"references": map[string]any{
			"workflow": map[string]any{"id": fcWfID},
			"project":  map[string]any{"id": runTestProjectID},
		},
	}})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--log-failed"},
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
	deployStep := func(outcome string, exit int) []map[string]any {
		return []map[string]any{
			{"name": "Spin up environment", "type": "spinup_environment", "num": 0, "phase": "ended", "outcome": "succeeded", "started_at": now, "ended_at": now},
			{"name": "deploy", "type": "run", "num": 50, "phase": "ended", "outcome": outcome, "exit_code": exit, "started_at": now, "ended_at": now},
		}
	}
	fake.AddJobV3(fcJob2ID, map[string]any{"data": map[string]any{
		"id": fcJob2ID,
		"attributes": map[string]any{
			"name": "deploy", "type": "build", "phase": "ended", "outcome": "failed",
			"started_at": now, "ended_at": now,
			"parallel_executions": []map[string]any{
				{"steps": deployStep("succeeded", 0)},
				{"steps": deployStep("failed", 1)},
			},
		},
		"references": map[string]any{
			"workflow": map[string]any{"id": fcWfID},
			"project":  map[string]any{"id": runTestProjectID},
		},
	}})
	fake.AddJobStdoutCondensed(fcJob2ID, 1, 50, []byte("deploy failed\n"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--log-failed"},
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
	fake.AddJobV3(fcJob3ID, map[string]any{"data": map[string]any{
		"id": fcJob3ID,
		"attributes": map[string]any{
			"name": "lint", "type": "build", "phase": "ended", "outcome": "succeeded",
			"started_at": now, "ended_at": now,
			"parallel_executions": []map[string]any{{
				"steps": []map[string]any{
					{"name": "run lint", "type": "run", "num": 100, "phase": "ended", "outcome": "succeeded", "exit_code": 0, "started_at": now, "ended_at": now},
				},
			}},
		},
		"references": map[string]any{
			"workflow": map[string]any{"id": fcWfID},
			"project":  map[string]any{"id": runTestProjectID},
		},
	}})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--log-failed"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}

// TestRunGet_LogFailed_RejectsJSON verifies that combining --log-failed with
// --json exits non-zero with a user-facing error.
func TestRunGet_LogFailed_RejectsJSON(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = "https://circleci.com" // never reached

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fcRunID, "--log-failed", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, cmp.Contains(result.Stderr, "run.log_failed_no_json"))
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
		Args:    []string{"run", "get", fcRunID, "--log-failed"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, cmp.Equal(result.Stderr, ""))
}
