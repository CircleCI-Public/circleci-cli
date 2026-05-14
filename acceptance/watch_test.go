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
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const watchSlug = "gh/testorg/testrepo"

// setupWatchFake builds a fake with one run (number 75) whose single
// workflow has the given status.
func setupWatchFake(t *testing.T, runID, wfID, wfStatus string) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	r := fakeRun(runID, 75, "created", watchSlug, "main")
	r["vcs"].(map[string]any)["revision"] = "abc1234def5678abcdef"

	fake := fakes.NewCircleCI(t)
	fake.AddRun(runID, r)
	fake.AddProjectRuns(watchSlug, r)
	fake.AddRunWorkflows(runID, map[string]any{"id": wfID, "name": "build", "status": wfStatus})
	fake.AddWorkflowJobs(wfID,
		fakeJob("job-1", "lint", 100, watchSlug),
		fakeJob("job-2", "test", 101, watchSlug),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- watch by number ---

func TestRunWatch_ByNumber(t *testing.T) {
	_, env := setupWatchFake(t, "watch-pid-001", "watch-wf-001", "success")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "watch", "75", "--project", watchSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "#75"), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "succeeded"), "stderr: %s", result.Stderr)
}

// --- watch by UUID (no --project or --branch needed) ---

func TestRunWatch_ByUUID(t *testing.T) {
	runID := "0b0e6eca-4e9a-43d7-b74e-a7ed4b7d11cd"
	_, env := setupWatchFake(t, runID, "watch-wf-uuid-001", "success")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "watch", runID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "#75"), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "succeeded"), "stderr: %s", result.Stderr)
}

// --- watch latest (no number arg) ---

func TestRunWatch_Latest(t *testing.T) {
	runID := "watch-pid-002"
	wfID := "watch-wf-002"
	r := fakeRun(runID, 76, "created", watchSlug, "main")

	fake := fakes.NewCircleCI(t)
	fake.AddRun(runID, r)
	fake.AddProjectRuns(watchSlug, r)
	fake.AddRunWorkflows(runID, map[string]any{"id": wfID, "name": "build", "status": "success"})
	fake.AddWorkflowJobs(wfID, fakeJob("job-1", "test", 100, watchSlug))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "watch", "--project", watchSlug, "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "succeeded"), "stderr: %s", result.Stderr)
}

// --- failed run → exit 1 ---

func TestRunWatch_Failed(t *testing.T) {
	_, env := setupWatchFake(t, "watch-pid-003", "watch-wf-003", "failed")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "watch", "75", "--project", watchSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "failed"), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "circleci logs --last-failed"), "stderr: %s", result.Stderr)
}

// --- failed pipeline with a failed job → suggests `circleci logs <num>` ---

func TestRunWatch_Failed_SuggestsJobLogs(t *testing.T) {
	pipelineID := "watch-pid-failedjob"
	wfID := "watch-wf-failedjob"
	p := fakeRun(pipelineID, 75, "created", watchSlug, "main")

	failedJob := fakeJob("job-1", "integration-test", 156057, watchSlug)
	failedJob["status"] = "failed"

	fake := fakes.NewCircleCI(t)
	fake.AddRun(pipelineID, p)
	fake.AddProjectRuns(watchSlug, p)
	fake.AddRunWorkflows(pipelineID, map[string]any{"id": wfID, "name": "build", "status": "failed"})
	fake.AddWorkflowJobs(wfID,
		failedJob,
		fakeJob("job-2", "lint", 156058, watchSlug),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "watch", "75", "--project", watchSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, `"integration-test"`), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "circleci logs 156057"), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "circleci logs --last-failed"), "stderr: %s", result.Stderr)
}

// --- cancelled run → exit 6 ---

func TestRunWatch_Cancelled(t *testing.T) {
	_, env := setupWatchFake(t, "watch-pid-004", "watch-wf-004", "canceled")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "watch", "75", "--project", watchSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "cancelled"), "stderr: %s", result.Stderr)
}

// --- --sha: run already present ---

func TestRunWatch_SHA(t *testing.T) {
	_, env := setupWatchFake(t, "watch-pid-005", "watch-wf-005", "success")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"run", "watch", "--sha", "abc1234",
			"--project", watchSlug, "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "succeeded"), "stderr: %s", result.Stderr)
}

// --- --sha: not found within wait window → exit 5 ---

func TestRunWatch_SHA_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddProjectRuns(watchSlug) // empty — no matching run

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	// Shorten the 2-minute SHA wait window so the test is fast.
	env.Extra["CIRCLECI_SHA_WAIT_MS"] = "50"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"run", "watch", "--sha", "deadbeef",
			"--project", watchSlug, "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
	assert.Check(t, cmp.Contains(result.Stderr, "No run found"), "stderr: %s", result.Stderr)
}

// --- --failfast: exit immediately when a job fails, without waiting for the rest of the run ---

func TestRunWatch_FailFast(t *testing.T) {
	runID := "watch-pid-failfast"
	wfID := "watch-wf-failfast"
	r := fakeRun(runID, 79, "created", watchSlug, "main")

	failedJob := fakeJob("job-1", "integration-test", 200, watchSlug)
	failedJob["status"] = "failed"

	fake := fakes.NewCircleCI(t)
	fake.AddRun(runID, r)
	fake.AddProjectRuns(watchSlug, r)
	// Workflow is still "running" — but it has a failed job. Without --failfast
	// the watch would block until the workflow finishes (or until the test
	// timeout); with --failfast it must exit on the first poll that sees the
	// failure.
	fake.AddRunWorkflows(runID, map[string]any{"id": wfID, "name": "build", "status": "running"})
	fake.AddWorkflowJobs(wfID,
		failedJob,
		fakeJob("job-2", "lint", 201, watchSlug),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "watch", "79", "--project", watchSlug, "--failfast"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 1, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "failing job"), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "integration-test"), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "circleci logs 200"), "stderr: %s", result.Stderr)
}

// --- watch timeout while run still running → exit 8 ---

func TestRunWatch_Timeout(t *testing.T) {
	runID := "watch-pid-006"
	wfID := "watch-wf-006"
	r := fakeRun(runID, 77, "created", watchSlug, "main")

	fake := fakes.NewCircleCI(t)
	fake.AddRun(runID, r)
	fake.AddProjectRuns(watchSlug, r)
	// Workflow is permanently "running" — will never complete.
	fake.AddRunWorkflows(runID, map[string]any{"id": wfID, "name": "build", "status": "running"})
	fake.AddWorkflowJobs(wfID, fakeJob("job-1", "test", 100, watchSlug))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "watch", "77", "--project", watchSlug, "--timeout", "1s"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 8, "stderr: %s", result.Stderr) // ExitTimeout
}

// --- Ctrl-C during polling exits within a poll interval (regression: was stuck for 5–30s) ---

func TestRunWatch_InterruptDuringPolling(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "os.Interrupt is not supported on Windows")

	runID := "watch-pid-interrupt"
	wfID := "watch-wf-interrupt"
	r := fakeRun(runID, 78, "created", watchSlug, "main")

	fake := fakes.NewCircleCI(t)
	fake.AddRun(runID, r)
	fake.AddProjectRuns(watchSlug, r)
	// Workflow stays "running" forever, so the watch loop is in its
	// time-based poll wait when the signal arrives.
	fake.AddRunWorkflows(runID, map[string]any{"id": wfID, "name": "build", "status": "running"})
	fake.AddWorkflowJobs(wfID, fakeJob("job-1", "test", 100, watchSlug))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	// Start the CLI directly (not via RunCLI) so we can deliver SIGINT.
	args := []string{
		"--insecure-storage", "--theme=dark",
		"run", "watch", "78", "--project", watchSlug,
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = t.TempDir()
	cmd.Env = env.Environ()
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	assert.NilError(t, cmd.Start())

	// Let the watch loop enter its first sleep.
	time.Sleep(1 * time.Second)
	assert.NilError(t, cmd.Process.Signal(os.Interrupt))

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	// With ctx-aware sleep, the process should exit within a fraction of a
	// second. Allow a generous 3s budget — but well below the 5s minimum
	// poll interval that would prove the bug is back.
	select {
	case err := <-done:
		var exitCode int
		var exitErr *exec.ExitError
		switch {
		case err == nil:
			exitCode = 0
		case errors.As(err, &exitErr):
			exitCode = exitErr.ExitCode()
		default:
			t.Fatalf("unexpected wait error: %v", err)
		}
		assert.Equal(t, exitCode, 6, "expected ExitCancelled (6); stderr: %s", stderr.String())
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("watch did not exit within 3s of SIGINT — stuck in time.Sleep?")
	}
}

// --- no token → exit 3 ---

func TestRunWatch_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "watch", "75", "--project", watchSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
}
