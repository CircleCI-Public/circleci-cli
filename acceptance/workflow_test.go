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
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

const (
	testWorkflowDetailID = "cccccccc-0000-0000-0000-000000000010"
	testPipelineForWF    = "aaaaaaaa-0000-0000-0000-000000000010"
)

func fakeWorkflowDetail(id, name, status, pipelineID, slug string) map[string]any {
	now := time.Now().UTC().Format(time.RFC3339)
	return map[string]any{
		"id":              id,
		"name":            name,
		"status":          status,
		"pipeline_id":     pipelineID,
		"pipeline_number": 42,
		"project_slug":    slug,
		"started_by":      "testuser-uuid",
		"created_at":      now,
		"stopped_at":      now,
	}
}

func setupWorkflowFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddWorkflowDetail(testWorkflowDetailID,
		fakeWorkflowDetail(testWorkflowDetailID, "build", "failed", testPipelineForWF, testSlug))
	fake.AddWorkflowJobs(testWorkflowDetailID,
		fakeJob("job-uuid-201", "run-tests", 201, testSlug),
		fakeJob("job-uuid-202", "deploy", 202, testSlug),
	)
	fake.SetRerunResponse(testWorkflowDetailID, http.StatusAccepted)
	fake.SetCancelResponse(testWorkflowDetailID, http.StatusAccepted)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- workflow get ---

func TestWorkflowGet(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "get", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, testWorkflowDetailID), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "build"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "failed"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "run-tests"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "#201"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "#202"), "stdout: %s", result.Stdout)
}

func TestWorkflowGet_JSON(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "get", "--json", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], testWorkflowDetailID))
	assert.Check(t, cmp.Equal(out["name"], "build"))
	assert.Check(t, cmp.Equal(out["status"], "failed"))

	jobs := out["jobs"].([]any)
	assert.Check(t, cmp.Len(jobs, 2))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["name"], "run-tests"))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["number"], float64(201)))
}

func TestWorkflowGet_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"workflow", "get", "00000000-0000-0000-0000-000000000000"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
	assert.Check(t, cmp.Contains(result.Stderr, "No workflow found"), "stderr: %s", result.Stderr)
}

func TestWorkflowGet_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "get", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, cmp.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}

func TestWorkflowGet_NotFound_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"workflow", "get", "--quiet", "--json", "00000000-0000-0000-0000-000000000000"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)

	var errOut map[string]any
	err := json.Unmarshal([]byte(result.Stderr), &errOut)
	assert.NilError(t, err, "stderr should be JSON: %s", result.Stderr)
	assert.Check(t, cmp.Equal(errOut["error"], true))
	assert.Check(t, cmp.Equal(errOut["exit_code"], float64(5)))
	assert.Check(t, errOut["code"] != nil, "code field missing")
	assert.Check(t, errOut["message"] != nil, "message field missing")
	// stdout must be empty — no partial data output
	stdout := strings.TrimSpace(result.Stdout)
	assert.Check(t, cmp.Equal(stdout, ""))
}

func TestWorkflowGet_NoToken_JSON(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "get", "--json", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)

	var errOut map[string]any
	err := json.Unmarshal([]byte(result.Stderr), &errOut)
	assert.NilError(t, err, "stderr should be JSON: %s", result.Stderr)
	assert.Check(t, cmp.Equal(errOut["error"], true))
	assert.Check(t, cmp.Equal(errOut["exit_code"], float64(3)))
	stdout := strings.TrimSpace(result.Stdout)
	assert.Check(t, cmp.Equal(stdout, ""))
}

// --- workflow list ---

// minimalPipeline returns a minimal pipeline payload sufficient for the fake
// server's pipeline-existence check in handleGetPipelineWorkflows.
func minimalPipeline(id string) map[string]any {
	return map[string]any{"id": id, "number": 1, "state": "created",
		"project_slug": testSlug, "created_at": "2026-01-01T00:00:00Z"}
}

func TestWorkflowList(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(testPipelineForWF, minimalPipeline(testPipelineForWF))
	fake.AddPipelineWorkflows(testPipelineForWF,
		map[string]any{"id": "wf-uuid-aaa", "name": "build", "status": "success"},
		map[string]any{"id": "wf-uuid-bbb", "name": "deploy", "status": "failed"},
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, []string{"workflow", "list", testPipelineForWF}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "wf-uuid-aaa"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "build"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "wf-uuid-bbb"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "deploy"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "failed"), "stdout: %s", result.Stdout)
}

func TestWorkflowList_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(testPipelineForWF, minimalPipeline(testPipelineForWF))
	fake.AddPipelineWorkflows(testPipelineForWF,
		map[string]any{"id": "wf-uuid-aaa", "name": "build", "status": "success"},
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, []string{"workflow", "list", "--json", testPipelineForWF}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 1))
	assert.Check(t, cmp.Equal(out[0]["id"], "wf-uuid-aaa"))
	assert.Check(t, cmp.Equal(out[0]["name"], "build"))
	assert.Check(t, cmp.Equal(out[0]["status"], "success"))
}

func TestWorkflowList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(testPipelineForWF, minimalPipeline(testPipelineForWF))
	fake.AddPipelineWorkflows(testPipelineForWF) // no workflows

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, []string{"workflow", "list", testPipelineForWF}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "No workflows"), "stdout: %s", result.Stdout)
}

func TestWorkflowList_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"workflow", "list", "00000000-0000-0000-0000-000000000000"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
}

func TestWorkflowList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, []string{"workflow", "list", testPipelineForWF}, env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, cmp.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}

// --- workflow list (no-arg / recent-pipelines mode) ---

const testPipelineRecent1 = "bbbbbbbb-0000-0000-0000-000000000001"
const testPipelineRecent2 = "bbbbbbbb-0000-0000-0000-000000000002"

func recentPipeline(id string, number int64, branch string) map[string]any {
	return map[string]any{
		"id": id, "number": number, "state": "created",
		"project_slug": testSlug, "created_at": "2026-01-01T00:00:00Z",
		"vcs": map[string]any{"branch": branch, "revision": "abc1234567890"},
	}
}

func TestWorkflowList_NoArg(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	fake.AddPipeline(testPipelineRecent1, recentPipeline(testPipelineRecent1, 10, "main"))
	fake.AddPipeline(testPipelineRecent2, recentPipeline(testPipelineRecent2, 9, "main"))
	fake.AddProjectPipelines(testSlug,
		recentPipeline(testPipelineRecent1, 10, "main"),
		recentPipeline(testPipelineRecent2, 9, "main"),
	)
	fake.AddPipelineWorkflows(testPipelineRecent1,
		map[string]any{"id": "wf-recent-aaa", "name": "build", "status": "success"},
		map[string]any{"id": "wf-recent-bbb", "name": "deploy", "status": "failed"},
	)
	fake.AddPipelineWorkflows(testPipelineRecent2,
		map[string]any{"id": "wf-recent-ccc", "name": "build", "status": "running"},
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"workflow", "list", "--project", testSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "Pipeline #10"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "wf-recent-aaa"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "build"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "Pipeline #9"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, "wf-recent-ccc"), "stdout: %s", result.Stdout)
}

func TestWorkflowList_NoArg_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	fake.AddPipeline(testPipelineRecent1, recentPipeline(testPipelineRecent1, 10, "main"))
	fake.AddProjectPipelines(testSlug,
		recentPipeline(testPipelineRecent1, 10, "main"),
	)
	fake.AddPipelineWorkflows(testPipelineRecent1,
		map[string]any{"id": "wf-recent-aaa", "name": "build", "status": "success"},
	)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"workflow", "list", "--json", "--project", testSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 1))
	assert.Check(t, cmp.Equal(out[0]["pipeline_id"], testPipelineRecent1))
	assert.Check(t, cmp.Equal(out[0]["pipeline_number"], float64(10)))
	assert.Check(t, cmp.Equal(out[0]["id"], "wf-recent-aaa"))
	assert.Check(t, cmp.Equal(out[0]["name"], "build"))
	assert.Check(t, cmp.Equal(out[0]["status"], "success"))
}

func TestWorkflowList_NoArg_NoPipelines(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// no pipelines registered for project

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"workflow", "list", "--project", testSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "No pipelines found"), "stdout: %s", result.Stdout)
}

// --- workflow rerun ---

func TestWorkflowRerun(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "rerun", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "from scratch"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, testWorkflowDetailID), "stdout: %s", result.Stdout)
}

func TestWorkflowRerun_FromFailed(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "rerun", "--from-failed", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "failed jobs"), "stdout: %s", result.Stdout)
}

func TestWorkflowRerun_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "rerun", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, cmp.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}

func TestWorkflowRerun_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"workflow", "rerun", "00000000-0000-0000-0000-000000000000"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
}

// --- workflow cancel ---

func TestWorkflowCancel(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "cancel", "--force", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "Cancelled"), "stdout: %s", result.Stdout)
	assert.Check(t, cmp.Contains(result.Stdout, testWorkflowDetailID), "stdout: %s", result.Stdout)
}

func TestWorkflowCancel_RequiresForce(t *testing.T) {
	// In non-interactive mode (no TTY), --force is required.
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "cancel", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr) // ExitCancelled
	assert.Check(t, cmp.Contains(result.Stderr, "--force"), "stderr: %s", result.Stderr)
}

func TestWorkflowCancel_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "cancel", "--force", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, cmp.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}

func TestWorkflowCancel_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"workflow", "cancel", "--force", "00000000-0000-0000-0000-000000000000"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
}
