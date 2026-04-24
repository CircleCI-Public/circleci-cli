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

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

const watchSlug = "gh/testorg/testrepo"

// setupWatchFake builds a fake with one pipeline (number 75) whose single
// workflow has the given status.
func setupWatchFake(t *testing.T, pipelineID, wfID, wfStatus string) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	p := fakePipeline(pipelineID, 75, "created", watchSlug, "main")
	p["vcs"].(map[string]any)["revision"] = "abc1234def5678abcdef"

	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(pipelineID, p)
	fake.AddProjectPipelines(watchSlug, p)
	fake.AddPipelineWorkflows(pipelineID, map[string]any{"id": wfID, "name": "build", "status": wfStatus})
	fake.AddWorkflowJobs(wfID,
		fakeJob("job-1", "lint", 100, watchSlug),
		fakeJob("job-2", "test", 101, watchSlug),
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- watch by number ---

func TestPipelineWatch_ByNumber(t *testing.T) {
	_, env := setupWatchFake(t, "watch-pid-001", "watch-wf-001", "success")

	result := binary.RunCLI(t,
		[]string{"pipeline", "watch", "75", "--project", watchSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "#75"), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "succeeded"), "stderr: %s", result.Stderr)
}

// --- watch by UUID (no --project or --branch needed) ---

func TestPipelineWatch_ByUUID(t *testing.T) {
	pipelineID := "0b0e6eca-4e9a-43d7-b74e-a7ed4b7d11cd"
	_, env := setupWatchFake(t, pipelineID, "watch-wf-uuid-001", "success")

	result := binary.RunCLI(t,
		[]string{"pipeline", "watch", pipelineID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "#75"), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "succeeded"), "stderr: %s", result.Stderr)
}

// --- watch latest (no number arg) ---

func TestPipelineWatch_Latest(t *testing.T) {
	pipelineID := "watch-pid-002"
	wfID := "watch-wf-002"
	p := fakePipeline(pipelineID, 76, "created", watchSlug, "main")

	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(pipelineID, p)
	fake.AddProjectPipelines(watchSlug, p)
	fake.AddPipelineWorkflows(pipelineID, map[string]any{"id": wfID, "name": "build", "status": "success"})
	fake.AddWorkflowJobs(wfID, fakeJob("job-1", "test", 100, watchSlug))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"pipeline", "watch", "--project", watchSlug, "--branch", "main"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "succeeded"), "stderr: %s", result.Stderr)
}

// --- failed pipeline → exit 1 ---

func TestPipelineWatch_Failed(t *testing.T) {
	_, env := setupWatchFake(t, "watch-pid-003", "watch-wf-003", "failed")

	result := binary.RunCLI(t,
		[]string{"pipeline", "watch", "75", "--project", watchSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "failed"), "stderr: %s", result.Stderr)
}

// --- cancelled pipeline → exit 6 ---

func TestPipelineWatch_Cancelled(t *testing.T) {
	_, env := setupWatchFake(t, "watch-pid-004", "watch-wf-004", "canceled")

	result := binary.RunCLI(t,
		[]string{"pipeline", "watch", "75", "--project", watchSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "cancelled"), "stderr: %s", result.Stderr)
}

// --- --sha: pipeline already present ---

func TestPipelineWatch_SHA(t *testing.T) {
	_, env := setupWatchFake(t, "watch-pid-005", "watch-wf-005", "success")

	result := binary.RunCLI(t,
		[]string{"pipeline", "watch", "--sha", "abc1234",
			"--project", watchSlug, "--branch", "main"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "succeeded"), "stderr: %s", result.Stderr)
}

// --- --sha: not found within wait window → exit 5 ---

func TestPipelineWatch_SHA_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectPipelines(watchSlug) // empty — no matching pipeline

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	// Shorten the 2-minute SHA wait window so the test is fast.
	env.Extra["CIRCLECI_SHA_WAIT_MS"] = "50"

	result := binary.RunCLI(t,
		[]string{"pipeline", "watch", "--sha", "deadbeef",
			"--project", watchSlug, "--branch", "main"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
	assert.Check(t, cmp.Contains(result.Stderr, "No pipeline found"), "stderr: %s", result.Stderr)
}

// --- watch timeout while pipeline still running → exit 8 ---

func TestPipelineWatch_Timeout(t *testing.T) {
	pipelineID := "watch-pid-006"
	wfID := "watch-wf-006"
	p := fakePipeline(pipelineID, 77, "created", watchSlug, "main")

	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(pipelineID, p)
	fake.AddProjectPipelines(watchSlug, p)
	// Workflow is permanently "running" — will never complete.
	fake.AddPipelineWorkflows(pipelineID, map[string]any{"id": wfID, "name": "build", "status": "running"})
	fake.AddWorkflowJobs(wfID, fakeJob("job-1", "test", 100, watchSlug))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"pipeline", "watch", "77", "--project", watchSlug, "--timeout", "1s"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 8, "stderr: %s", result.Stderr) // ExitTimeout
}

// --- no token → exit 3 ---

func TestPipelineWatch_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"pipeline", "watch", "75", "--project", watchSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
}
