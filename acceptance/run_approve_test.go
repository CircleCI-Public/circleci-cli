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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	approveRunID     = "5034460f-c7c4-4c43-9457-de07e2029e7b"
	approveDeployWf  = "b0000000-0000-4000-8000-0000000a0001"
	approveReleaseWf = "b0000000-0000-4000-8000-0000000a0002"
	approveEndedWf   = "b0000000-0000-4000-8000-0000000a0003"
	approveDeployJob = "d0000000-0000-4000-8000-0000000a0001"
	approveGateJob   = "d0000000-0000-4000-8000-0000000a0002"
	approveProjectID = "a0000000-0000-4000-8000-0000000a0001"
)

// approvalJob is an approval job in the given phase. A pending one is
// "started" with a start time, as the real jobs API reports it.
func approvalJob(id, name, workflowID, phase string) fakes.JobV3 {
	j := fakes.JobV3{ID: id, Name: name, Type: "approval", Phase: phase, WorkflowID: workflowID, ProjectID: approveProjectID}
	if phase == "started" || phase == "ended" {
		j.StartedAt = "2020-01-01T12:00:00Z"
	}
	return j
}

// approveWorkflow is a running workflow and the jobs it lists.
type approveWorkflow struct {
	workflow fakes.WorkflowV3
	jobs     []fakes.JobV3
}

// setupApproveFake serves run 42 (approveRunID) in watchSlug, with an ended
// "nightly" workflow followed by the given running workflows, in order.
func setupApproveFake(t *testing.T, workflows ...approveWorkflow) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	run := fakeRun(approveRunID, 42, "created", watchSlug, "main")
	fake.AddRun(approveRunID, run)
	fake.AddProjectRuns(watchSlug, run)

	// An ended workflow is never searched, even if it held an approval.
	ended := fakeWorkflowV3(approveEndedWf, "nightly", approveRunID, approveProjectID, "ended", "success")
	fake.AddWorkflowJobsV3(approveEndedWf, approvalJob("d0000000-0000-4000-8000-0000000a00ff", "old-hold", approveEndedWf, "started"))
	wfs := make([]fakes.WorkflowV3, 0, 1+len(workflows))
	wfs = append(wfs, ended)
	for _, w := range workflows {
		wfs = append(wfs, w.workflow)
		fake.AddWorkflowJobsV3(w.workflow.ID, w.jobs...)
	}
	fake.AddRunWorkflowsV3(approveRunID, wfs...)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

func deployWorkflow() fakes.WorkflowV3 {
	return fakeWorkflowV3(approveDeployWf, "deploy", approveRunID, approveProjectID, "started", "")
}

func releaseWorkflow() fakes.WorkflowV3 {
	return fakeWorkflowV3(approveReleaseWf, "release", approveRunID, approveProjectID, "started", "")
}

// oneApprovalOnHold is a deploy workflow with one approval on hold, beside a
// build job and an approval still waiting on the jobs it requires.
func oneApprovalOnHold() approveWorkflow {
	return approveWorkflow{deployWorkflow(), []fakes.JobV3{
		{ID: "d0000000-0000-4000-8000-0000000a0010", Name: "build", Type: "build", Phase: "ended", Outcome: "success", StartedAt: "2020-01-01T12:00:00Z"},
		approvalJob(approveDeployJob, "hold-deploy", approveDeployWf, "started"),
		approvalJob("d0000000-0000-4000-8000-0000000a0011", "hold-prod", approveDeployWf, "created"),
	}}
}

func approveRequests(fake *fakes.CircleCI) []string {
	var paths []string
	for _, r := range fake.AllRequests() {
		if r.Method == http.MethodPost {
			paths = append(paths, r.URL.Path)
		}
	}
	return paths
}

func TestRunApprove(t *testing.T) {
	fake, env := setupApproveFake(t, oneApprovalOnHold())
	fake.SetApproveResponse(approveDeployWf, approveDeployJob, http.StatusAccepted)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "approve", "--force", approveRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("approves only the job on hold", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(approveRequests(fake), []string{
			"/api/v2/workflow/" + approveDeployWf + "/approve/" + approveDeployJob,
		}))
	})
}

func TestRunApprove_ByNumber(t *testing.T) {
	fake, env := setupApproveFake(t, oneApprovalOnHold())
	fake.SetApproveResponse(approveDeployWf, approveDeployJob, http.StatusAccepted)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "approve", "42", "--project", watchSlug, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestRunApprove_JSON(t *testing.T) {
	fake, env := setupApproveFake(t, oneApprovalOnHold())
	fake.SetApproveResponse(approveDeployWf, approveDeployJob, http.StatusAccepted)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "approve", "--force", "--json", approveRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"run_id":        approveRunID,
		"workflow_id":   approveDeployWf,
		"workflow_name": "deploy",
		"job_id":        approveDeployJob,
		"job_name":      "hold-deploy",
	}))
}

func TestRunApprove_RequiresForce(t *testing.T) {
	fake, env := setupApproveFake(t, oneApprovalOnHold())

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "approve", approveRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 6), "stderr: %s", result.Stderr) // ExitCancelled
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Len(approveRequests(fake), 0), "nothing is approved without --force")
}

func TestRunApprove_NothingOnHold(t *testing.T) {
	fake, env := setupApproveFake(t, approveWorkflow{deployWorkflow(), []fakes.JobV3{
		approvalJob(approveDeployJob, "hold-deploy", approveDeployWf, "created"),
	}})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "approve", "--force", approveRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Len(approveRequests(fake), 0))
}

// twoApprovalsOnHold has an approval on hold in each of two workflows.
func twoApprovalsOnHold() []approveWorkflow {
	return []approveWorkflow{
		{deployWorkflow(), []fakes.JobV3{approvalJob(approveDeployJob, "hold-deploy", approveDeployWf, "started")}},
		{releaseWorkflow(), []fakes.JobV3{approvalJob(approveGateJob, "release-gate", approveReleaseWf, "started")}},
	}
}

func TestRunApprove_SeveralOnHold(t *testing.T) {
	t.Run("without --job, outside a terminal", func(t *testing.T) {
		fake, env := setupApproveFake(t, twoApprovalsOnHold()...)

		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"run", "approve", "--force", approveRunID},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})

		assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr) // ExitBadArguments
		assert.Check(t, cmp.Contains(result.Stderr, "has 2 matching approval jobs on hold"))
		assert.Check(t, cmp.Contains(result.Stderr, "hold-deploy (workflow deploy)"))
		assert.Check(t, cmp.Contains(result.Stderr, "release-gate (workflow release)"))
		assert.Check(t, cmp.Len(approveRequests(fake), 0))
	})

	t.Run("--job picks one by name", func(t *testing.T) {
		fake, env := setupApproveFake(t, twoApprovalsOnHold()...)
		fake.SetApproveResponse(approveReleaseWf, approveGateJob, http.StatusAccepted)

		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"run", "approve", "--force", "--job", "release-gate", approveRunID},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})

		assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
		assert.Check(t, cmp.DeepEqual(approveRequests(fake), []string{
			"/api/v2/workflow/" + approveReleaseWf + "/approve/" + approveGateJob,
		}))
	})

	t.Run("--job picks one by UUID", func(t *testing.T) {
		fake, env := setupApproveFake(t, twoApprovalsOnHold()...)
		fake.SetApproveResponse(approveDeployWf, approveDeployJob, http.StatusAccepted)

		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"run", "approve", "--force", "--job", approveDeployJob, approveRunID},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})

		assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
		assert.Check(t, cmp.DeepEqual(approveRequests(fake), []string{
			"/api/v2/workflow/" + approveDeployWf + "/approve/" + approveDeployJob,
		}))
	})

	t.Run("picked from a list in a terminal", func(t *testing.T) {
		fake, env := setupApproveFake(t, twoApprovalsOnHold()...)
		fake.SetApproveResponse(approveReleaseWf, approveGateJob, http.StatusAccepted)

		console := binary.RunCLIInteractive(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"run", "approve", approveRunID},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})

		_, err := console.ExpectString("Which approval job should be approved?")
		assert.NilError(t, err)
		_, err = console.Send(keyDown + "\r")
		assert.NilError(t, err)
		_, err = console.ExpectString("Approved release-gate (workflow release) in run")
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(approveRequests(fake), []string{
			"/api/v2/workflow/" + approveReleaseWf + "/approve/" + approveGateJob,
		}))
	})
}

func TestRunApprove_JobNotFound(t *testing.T) {
	fake, env := setupApproveFake(t, oneApprovalOnHold())

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "approve", "--force", "--job", "nope", approveRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5), "stderr: %s", result.Stderr) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Len(approveRequests(fake), 0))
}

func TestRunApprove_AlreadyApproved(t *testing.T) {
	// The job was on hold when listed, but the approve call 404s: someone else
	// approved it in between.
	_, env := setupApproveFake(t, oneApprovalOnHold())

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "approve", "--force", approveRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5), "stderr: %s", result.Stderr) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunApprove_MissingArg(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "approve"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr)
}

func TestRunApprove_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "approve", "--force", approveRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 3), "stderr: %s", result.Stderr) // ExitAuthError
}
