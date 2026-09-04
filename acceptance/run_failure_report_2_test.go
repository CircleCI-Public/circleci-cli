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
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	fr2RunID  = "e0000000-0000-4000-8000-0000000000f2"
	fr2WfID   = "b0000000-0000-4000-8000-0000000000f2"
	fr2Job1ID = "d0000000-0000-4000-8000-0000000000f5" // failed job, no test data
	fr2Job2ID = "d0000000-0000-4000-8000-0000000000f6" // unauthorized job
	fr2Job3ID = "d0000000-0000-4000-8000-0000000000f7" // failed job with a failed test case
)

// --- run get --failure-report-2 ---
//
// This is an experimental, hidden flag: a client-side mockup of a possible
// future GET /api/v3/runs/:id/failure-report endpoint, assembled entirely
// from existing v3 API calls. There is no such route to fake here — only the
// existing routes (runs/:id, workflows, jobs, jobs/:id, jobs/:id/tests).

// TestRunGet_FailureReport2 covers the solid-confidence kinds: failed_job,
// unauthorized_job and failed_test, plus a run-level best-effort error item.
func TestRunGet_FailureReport2(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	run := fakeRunV3(fr2RunID, runTestProjectID, "ended", "failed", "main", "abc1234def5678")
	run.Errors = []fakes.RunError{{Type: "config-fetch-error", Message: "could not fetch config"}}
	fake.AddRunV3(fr2RunID, runTestProjectID, run)
	fake.AddRunWorkflowsV3(fr2RunID, fakeWorkflowV3(fr2WfID, "build", fr2RunID, runTestProjectID, "ended", "failed"))
	fake.AddWorkflowJobsV3(fr2WfID,
		fakes.JobV3{ID: fr2Job1ID, Name: "run-tests", Type: "build", Phase: "ended", Outcome: "failed", WorkflowID: fr2WfID, ProjectID: runTestProjectID},
		fakes.JobV3{ID: fr2Job2ID, Name: "deploy", Type: "build", Phase: "ended", Outcome: "unauthorized", WorkflowID: fr2WfID, ProjectID: runTestProjectID},
		fakes.JobV3{ID: fr2Job3ID, Name: "unit-tests", Type: "build", Phase: "ended", Outcome: "failed", WorkflowID: fr2WfID, ProjectID: runTestProjectID},
	)

	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).Format(v3TimeFormat)
	fake.AddJobV3(fakes.JobV3{
		ID: fr2Job1ID, Name: "run-tests", Type: "build", Phase: "ended", Outcome: "failed",
		StartedAt: now, EndedAt: now, WorkflowID: fr2WfID, ProjectID: runTestProjectID,
		Executions: [][]fakes.JobStep{{
			{Name: "run tests", Type: "run", Num: 101, Phase: "ended", Outcome: "failed", ExitCode: new(1), StartedAt: now, EndedAt: now},
		}},
	})
	fake.AddJobV3(fakes.JobV3{
		ID: fr2Job3ID, Name: "unit-tests", Type: "build", Phase: "ended", Outcome: "failed",
		StartedAt: now, EndedAt: now, WorkflowID: fr2WfID, ProjectID: runTestProjectID,
		Executions: [][]fakes.JobStep{{
			{Name: "run unit tests", Type: "run", Num: 101, Phase: "ended", Outcome: "failed", ExitCode: new(1), StartedAt: now, EndedAt: now},
		}},
	})
	fake.AddJobTests(fr2Job3ID,
		fakes.TestResult{Classname: "pkg/foo", Name: "TestBar", Result: "success"},
		fakes.TestResult{Classname: "pkg/foo", Name: "TestBaz", Result: "failure", Message: "expected true, got false"},
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fr2RunID, "--failure-report-2"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, cmp.Equal(result.ExitCode, 0), result.Stderr)
	assert.Check(t, cmp.Equal(result.Stderr, ""))

	var out struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Kind      string `json:"kind"`
				Message   string `json:"message"`
				Classname string `json:"classname"`
				Name      string `json:"name"`
				RawType   string `json:"debug_raw_type"`
			} `json:"attributes"`
			References struct {
				Workflow *struct {
					ID         string `json:"id"`
					Attributes struct {
						Name string `json:"name"`
					} `json:"attributes"`
				} `json:"workflow"`
				Job *struct {
					ID string `json:"id"`
				} `json:"job"`
			} `json:"references"`
		} `json:"data"`
	}
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))

	kinds := make(map[string]int)
	for _, item := range out.Data {
		kinds[item.Attributes.Kind]++
	}
	assert.Check(t, cmp.Equal(kinds["missing_config"], 1), "raw type %q should map to missing_config", "config-fetch-error")
	assert.Check(t, cmp.Equal(kinds["failed_job"], 2))
	assert.Check(t, cmp.Equal(kinds["unauthorized_job"], 1))
	assert.Check(t, cmp.Equal(kinds["failed_test"], 1))

	for _, item := range out.Data {
		if item.Attributes.Kind == "failed_test" {
			assert.Check(t, cmp.Equal(item.Attributes.Classname, "pkg/foo"))
			assert.Check(t, cmp.Equal(item.Attributes.Name, "TestBaz"))
			assert.Check(t, cmp.Equal(item.Attributes.Message, "expected true, got false"))
			assert.Assert(t, item.References.Job != nil)
			assert.Check(t, cmp.Equal(item.References.Job.ID, fr2Job3ID))
		}
		if item.Attributes.Kind == "missing_config" {
			assert.Check(t, cmp.Equal(item.Attributes.RawType, "config-fetch-error"))
		}
	}
}

// TestRunGet_FailureReport2_MutuallyExclusiveWithJSON verifies --failure-report-2
// is rejected when combined with --json.
func TestRunGet_FailureReport2_MutuallyExclusiveWithJSON(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = "https://circleci.com" // never reached

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fr2RunID, "--failure-report-2", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, cmp.Contains(result.Stderr, "run.failure_report_2_exclusive"))
}

// TestRunGet_FailureReport2_MutuallyExclusiveWithFailureReport verifies
// --failure-report-2 is rejected when combined with --failure-report.
func TestRunGet_FailureReport2_MutuallyExclusiveWithFailureReport(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = "https://circleci.com" // never reached

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fr2RunID, "--failure-report-2", "--failure-report"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, cmp.Contains(result.Stderr, "does not compose with the other output flags"))
}

// TestRunGet_FailureReport2_NoWorkflows exits 0 with only run-level items when
// the run's workflows have not materialised yet (404).
func TestRunGet_FailureReport2_NoWorkflows(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	run := fakeRunV3(fr2RunID, runTestProjectID, "created", "", "main", "abc1234def5678")
	run.Errors = []fakes.RunError{{Type: "config", Message: "bad config"}}
	fake.AddRunV3(fr2RunID, runTestProjectID, run)
	fake.SetRunWorkflowsV3NotFound(fr2RunID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", fr2RunID, "--failure-report-2"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, cmp.Equal(result.ExitCode, 0), result.Stderr)
	assert.Check(t, cmp.Equal(result.Stderr, ""))

	var out struct {
		Data []struct {
			Attributes struct {
				Kind string `json:"kind"`
			} `json:"attributes"`
		} `json:"data"`
	}
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Assert(t, cmp.Len(out.Data, 1))
	assert.Check(t, cmp.Equal(out.Data[0].Attributes.Kind, "invalid_config"))
}
