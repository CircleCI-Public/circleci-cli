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
	testWorkflowDetailID = "cccccccc-0000-0000-0000-000000000010"
	testPipelineForWF    = "aaaaaaaa-0000-0000-0000-000000000010"
)

const wfProjectID = "3936b1ba-3289-44a2-96d8-d0b4fe366795"

func setupWorkflowFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddWorkflowV3(testWorkflowDetailID,
		fakeWorkflowV3(testWorkflowDetailID, "build", testPipelineForWF, wfProjectID, "ended", "failed"))
	fake.AddWorkflowJobsV3(testWorkflowDetailID,
		fakeJobV3("job-uuid-201", "run-tests", testWorkflowDetailID, wfProjectID),
		fakeJobV3("job-uuid-202", "deploy", testWorkflowDetailID, wfProjectID),
	)
	fake.SetRerunResponse(testWorkflowDetailID, http.StatusAccepted)
	fake.SetCancelResponse(testWorkflowDetailID, http.StatusAccepted)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- workflow get ---

func TestWorkflowGet(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestWorkflowGet_Color(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestWorkflowGet_JSON(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get", "--json", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(out["id"], testWorkflowDetailID))
	assert.Check(t, cmp.Equal(out["name"], "build"))
	assert.Check(t, cmp.Equal(out["phase"], "ended"))
	assert.Check(t, cmp.Equal(out["outcome"], "failed"))

	jobs := out["jobs"].([]any)
	assert.Check(t, cmp.Len(jobs, 2))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["name"], "run-tests"))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["phase"], "ended"))
	assert.Check(t, cmp.Equal(jobs[0].(map[string]any)["outcome"], "succeeded"))
}

func TestWorkflowGet_JQ(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get", "--json", "--jq", ".name", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(strings.TrimSpace(result.Stdout), "build"))
}

func TestWorkflowGet_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestWorkflowGet_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestWorkflowGet_NotFound_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get", "--quiet", "--json", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

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

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get", "--json", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

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

func TestWorkflowList(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunWorkflowsV3(testPipelineForWF,
		fakeWorkflowV3("wf-uuid-aaa", "build", testPipelineForWF, "proj-1", "ended", "succeeded"),
		fakeWorkflowV3("wf-uuid-bbb", "deploy", testPipelineForWF, "proj-1", "ended", "failed"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", testPipelineForWF},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestWorkflowList_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunWorkflowsV3(testPipelineForWF,
		fakeWorkflowV3("wf-uuid-aaa", "build", testPipelineForWF, "proj-1", "ended", "succeeded"),
		fakeWorkflowV3("wf-uuid-bbb", "deploy", testPipelineForWF, "proj-1", "ended", "failed"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", testPipelineForWF},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestWorkflowList_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunWorkflowsV3(testPipelineForWF,
		fakeWorkflowV3("wf-uuid-aaa", "build", testPipelineForWF, "proj-1", "ended", "succeeded"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", "--json", testPipelineForWF},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 1))
	assert.Check(t, cmp.Equal(out[0]["id"], "wf-uuid-aaa"))
	assert.Check(t, cmp.Equal(out[0]["name"], "build"))
	assert.Check(t, cmp.Equal(out[0]["phase"], "ended"))
	assert.Check(t, cmp.Equal(out[0]["outcome"], "succeeded"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestWorkflowList_JSON_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunWorkflowsV3(testPipelineForWF,
		fakeWorkflowV3("wf-uuid-aaa", "build", testPipelineForWF, "proj-1", "ended", "succeeded"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", "--json", testPipelineForWF},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestWorkflowList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", testPipelineForWF},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestWorkflowList_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// V3 returns empty list for unknown runs instead of 404.
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "No workflows found"))
}

func TestWorkflowList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", testPipelineForWF},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- workflow list (no-arg / recent-runs mode) ---

const testRunRecent1 = "bbbbbbbb-0000-0000-0000-000000000001"
const testRunRecent2 = "bbbbbbbb-0000-0000-0000-000000000002"
const wfListProjectID = "proj-uuid-wflist"

func setupRecentRuns(t *testing.T, fake *fakes.CircleCI) {
	t.Helper()
	addProjectInfo(fake, testSlug, wfListProjectID)
	fake.AddRunV3(testRunRecent1, wfListProjectID,
		fakeRunV3(testRunRecent1, wfListProjectID, "ended", "failed", "main", "abc1234567890"))
	fake.AddRunV3(testRunRecent2, wfListProjectID,
		fakeRunV3(testRunRecent2, wfListProjectID, "started", "", "main", "abc1234567890"))
	fake.AddRunWorkflowsV3(testRunRecent1,
		fakeWorkflowV3("wf-recent-aaa", "build", testRunRecent1, wfListProjectID, "ended", "succeeded"),
		fakeWorkflowV3("wf-recent-bbb", "deploy", testRunRecent1, wfListProjectID, "ended", "failed"),
	)
	fake.AddRunWorkflowsV3(testRunRecent2,
		fakeWorkflowV3("wf-recent-ccc", "build", testRunRecent2, wfListProjectID, "started", ""),
	)
}

func TestWorkflowList_NoArg(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	setupRecentRuns(t, fake)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", "--project", testSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestWorkflowList_NoArg_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	setupRecentRuns(t, fake)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", "--project", testSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestWorkflowList_NoArg_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	addProjectInfo(fake, testSlug, wfListProjectID)
	fake.AddRunV3(testRunRecent1, wfListProjectID,
		fakeRunV3(testRunRecent1, wfListProjectID, "ended", "succeeded", "main", "abc1234567890"))
	fake.AddRunWorkflowsV3(testRunRecent1,
		fakeWorkflowV3("wf-recent-aaa", "build", testRunRecent1, wfListProjectID, "ended", "succeeded"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", "--json", "--project", testSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 1))
	assert.Check(t, cmp.Equal(out[0]["run_id"], testRunRecent1))
	assert.Check(t, cmp.Equal(out[0]["id"], "wf-recent-aaa"))
	assert.Check(t, cmp.Equal(out[0]["name"], "build"))
	assert.Check(t, cmp.Equal(out[0]["phase"], "ended"))
	assert.Check(t, cmp.Equal(out[0]["outcome"], "succeeded"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestWorkflowList_NoArg_JSON_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	addProjectInfo(fake, testSlug, wfListProjectID)
	fake.AddRunV3(testRunRecent1, wfListProjectID,
		fakeRunV3(testRunRecent1, wfListProjectID, "ended", "succeeded", "main", "abc1234567890"))
	fake.AddRunWorkflowsV3(testRunRecent1,
		fakeWorkflowV3("wf-recent-aaa", "build", testRunRecent1, wfListProjectID, "ended", "succeeded"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", "--json", "--project", testSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestWorkflowList_NoArg_NoRuns(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	addProjectInfo(fake, testSlug, wfListProjectID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "list", "--project", testSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- workflow rerun ---

func TestWorkflowRerun(t *testing.T) {
	fake, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/workflow/" + testWorkflowDetailID + "/rerun"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"from_failed":false}`),
		}, ignoreCommonHeaders))
	})
}

func TestWorkflowRerun_Color(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestWorkflowRerun_FromFailed(t *testing.T) {
	fake, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", "--from-failed", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/workflow/" + testWorkflowDetailID + "/rerun"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"from_failed":true}`),
		}, ignoreCommonHeaders))
	})
}

func TestWorkflowRerun_FromFailed_Color(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", "--from-failed", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestWorkflowRerun_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestWorkflowRerun_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
}

// --- workflow cancel ---

func TestWorkflowCancel(t *testing.T) {
	fake, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "cancel", "--force", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/workflow/" + testWorkflowDetailID + "/cancel"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{}`),
		}, ignoreCommonHeaders))
	})
}

func TestWorkflowCancel_Color(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "cancel", "--force", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestWorkflowCancel_RequiresForce(t *testing.T) {
	// In non-interactive mode (no TTY), --force is required.
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "cancel", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr) // ExitCancelled
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestWorkflowCancel_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "cancel", "--force", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestWorkflowCancel_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "cancel", "--force", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
}
