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
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

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
		fakeJobV3("d0000000-0000-4000-8000-000000000201", "run-tests", testWorkflowDetailID, wfProjectID),
		fakeJobV3("d0000000-0000-4000-8000-000000000202", "deploy", testWorkflowDetailID, wfProjectID),
	)
	fake.SetRerunResponse(testWorkflowDetailID, http.StatusCreated) // the real endpoint answers 201
	fake.SetCancelResponse(testWorkflowDetailID, http.StatusAccepted)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- workflow get ---

func TestWorkflowGet(t *testing.T) {
	fake, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizeAppHost(result.Stdout, fake.URL()), t.Name()+".txt"))
}

func TestWorkflowGet_Color(t *testing.T) {
	fake, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "get", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(normalizeAppHost(result.Stdout, fake.URL()), t.Name()+".txt"))
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
		fakeWorkflowV3("b0000000-0000-4000-8000-0000000000aa", "build", testPipelineForWF, "a0000000-0000-4000-8000-000000000010", "ended", "succeeded"),
		fakeWorkflowV3("b0000000-0000-4000-8000-0000000000bb", "deploy", testPipelineForWF, "a0000000-0000-4000-8000-000000000010", "ended", "failed"),
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
		fakeWorkflowV3("b0000000-0000-4000-8000-0000000000aa", "build", testPipelineForWF, "a0000000-0000-4000-8000-000000000010", "ended", "succeeded"),
		fakeWorkflowV3("b0000000-0000-4000-8000-0000000000bb", "deploy", testPipelineForWF, "a0000000-0000-4000-8000-000000000010", "ended", "failed"),
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
		fakeWorkflowV3("b0000000-0000-4000-8000-0000000000aa", "build", testPipelineForWF, "a0000000-0000-4000-8000-000000000010", "ended", "succeeded"),
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
	assert.Check(t, cmp.Equal(out[0]["id"], "b0000000-0000-4000-8000-0000000000aa"))
	assert.Check(t, cmp.Equal(out[0]["name"], "build"))
	assert.Check(t, cmp.Equal(out[0]["phase"], "ended"))
	assert.Check(t, cmp.Equal(out[0]["outcome"], "succeeded"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestWorkflowList_JSON_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunWorkflowsV3(testPipelineForWF,
		fakeWorkflowV3("b0000000-0000-4000-8000-0000000000aa", "build", testPipelineForWF, "a0000000-0000-4000-8000-000000000010", "ended", "succeeded"),
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
const wfListProjectID = "a0000000-0000-4000-8000-00000000f100"

func setupRecentRuns(t *testing.T, fake *fakes.CircleCI) {
	t.Helper()
	addProjectBySlug(fake, testSlug, wfListProjectID)
	fake.AddRunV3(testRunRecent1, wfListProjectID,
		fakeRunV3(testRunRecent1, wfListProjectID, "ended", "failed", "main", "abc1234567890"))
	fake.AddRunV3(testRunRecent2, wfListProjectID,
		fakeRunV3(testRunRecent2, wfListProjectID, "started", "", "main", "abc1234567890"))
	fake.AddRunWorkflowsV3(testRunRecent1,
		fakeWorkflowV3("b0000000-0000-4000-8000-00000000fa01", "build", testRunRecent1, wfListProjectID, "ended", "succeeded"),
		fakeWorkflowV3("b0000000-0000-4000-8000-00000000fb01", "deploy", testRunRecent1, wfListProjectID, "ended", "failed"),
	)
	fake.AddRunWorkflowsV3(testRunRecent2,
		fakeWorkflowV3("b0000000-0000-4000-8000-00000000fc01", "build", testRunRecent2, wfListProjectID, "started", ""),
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
	addProjectBySlug(fake, testSlug, wfListProjectID)
	fake.AddRunV3(testRunRecent1, wfListProjectID,
		fakeRunV3(testRunRecent1, wfListProjectID, "ended", "succeeded", "main", "abc1234567890"))
	fake.AddRunWorkflowsV3(testRunRecent1,
		fakeWorkflowV3("b0000000-0000-4000-8000-00000000fa01", "build", testRunRecent1, wfListProjectID, "ended", "succeeded"),
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
	assert.Check(t, cmp.Equal(out[0]["id"], "b0000000-0000-4000-8000-00000000fa01"))
	assert.Check(t, cmp.Equal(out[0]["name"], "build"))
	assert.Check(t, cmp.Equal(out[0]["phase"], "ended"))
	assert.Check(t, cmp.Equal(out[0]["outcome"], "succeeded"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestWorkflowList_NoArg_JSON_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	addProjectBySlug(fake, testSlug, wfListProjectID)
	fake.AddRunV3(testRunRecent1, wfListProjectID,
		fakeRunV3(testRunRecent1, wfListProjectID, "ended", "succeeded", "main", "abc1234567890"))
	fake.AddRunWorkflowsV3(testRunRecent1,
		fakeWorkflowV3("b0000000-0000-4000-8000-00000000fa01", "build", testRunRecent1, wfListProjectID, "ended", "succeeded"),
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
	addProjectBySlug(fake, testSlug, wfListProjectID)

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

	// The rerun created a new workflow, and its id — not the spent one that was
	// passed in — is what the user needs next.
	assert.Check(t, cmp.Contains(result.Stdout, fakes.DefaultRerunWorkflowID))
	// Without --from-failed the endpoint must see is_from_failed false.
	assert.Check(t, !fake.RerunWasFromFailed(testWorkflowDetailID))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/workflows/" + testWorkflowDetailID + "/rerun"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"is_from_failed":false}`),
		}, ignoreCommonHeaders))
	})
}

// TestWorkflowRerun_JSON pins the machine-readable shape. Reporting the new
// workflow id is the point of --json here: it is the handle for whatever comes
// next, and previously the CLI decoded it from the response and threw it away.
func TestWorkflowRerun_JSON(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID, "--from-failed", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out struct {
		WorkflowID string `json:"workflow_id"`
		RerunFrom  string `json:"rerun_from"`
		FromFailed bool   `json:"from_failed"`
	}
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out.WorkflowID, fakes.DefaultRerunWorkflowID))
	assert.Check(t, cmp.Equal(out.RerunFrom, testWorkflowDetailID))
	assert.Check(t, cmp.Equal(out.FromFailed, true))
	// The two must differ, or the id is useless for following the new run.
	assert.Check(t, out.WorkflowID != out.RerunFrom)
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
			URL:    url.URL{Path: "/api/v3/workflows/" + testWorkflowDetailID + "/rerun"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"is_from_failed":true}`),
		}, ignoreCommonHeaders))
	})

	// The assertion with teeth: what the endpoint decoded, not merely what the
	// client and the fake agreed to exchange. Sending the v2 field name to this v3
	// endpoint left it false, and everything else still looked fine.
	assert.Check(t, fake.RerunWasFromFailed(testWorkflowDetailID),
		"the endpoint must have seen is_from_failed true")
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

// --- workflow rerun --job / --ssh ---

const (
	wfRunTestsJobID = "d0000000-0000-4000-8000-000000000201"
	wfDeployJobID   = "d0000000-0000-4000-8000-000000000202"
)

// wfSSHRerunJobID is the run-tests job in the workflow an SSH rerun creates.
const wfSSHRerunJobID = "d0000000-0000-4000-8000-000000000301"

// sshStepOutput is the Enable SSH step output of a real Docker job (CRLF line
// endings included) for the given job ID and execution.
func sshStepOutput(jobID string, execution int) []byte {
	return []byte(fmt.Sprintf("\r\nEnabling SSH access through ssh.circleci.com...\r\n\r\n"+
		"You can now SSH into this job if your SSH public key is added:\r\n"+
		"    $ ssh %s-%d@ssh.circleci.com\r\n\r\n"+
		"Use the same SSH public key that you've added in your User Settings or that you use for your VCS-provider (e.g., GitHub).\r\n\r\n"+
		"Job SSH keys can be added in your User Settings at https://app.circleci.com/settings/user/job-ssh-keys\r\n\r\n"+
		"SSH Session Activity Log (times in UTC):\r\n", jobID, execution))
}

// addSSHRerunJob registers run-tests in the rerun workflow as a started job
// whose executions each have a running Enable SSH step (step 101, as on a real
// job) that has printed its command. It returns the job, for a test that lists
// it alongside others.
func addSSHRerunJob(fake *fakes.CircleCI, executions int) fakes.JobV3 {
	job := fakes.JobV3{
		ID:         wfSSHRerunJobID,
		Name:       "run-tests",
		Type:       "build",
		Phase:      "started",
		StartedAt:  "2026-11-17T17:11:12.621Z",
		ProjectID:  wfProjectID,
		WorkflowID: fakes.DefaultRerunWorkflowID,
	}
	for i := range executions {
		job.Executions = append(job.Executions, []fakes.JobStep{
			{Name: "Spin up environment", Type: "spinup_environment", Num: 0, Phase: "ended", Outcome: "succeeded",
				StartedAt: "2026-11-17T17:11:10.895Z", EndedAt: "2026-11-17T17:11:15.625Z"},
			{Name: "Enable SSH", Type: "enable_ssh", Num: 101, Phase: "started", StartedAt: "2026-11-17T17:11:16.552Z"},
		})
		fake.AddJobStdout(wfSSHRerunJobID, i, 101, sshStepOutput(wfSSHRerunJobID, i))
	}
	fake.AddWorkflowJobsV3(fakes.DefaultRerunWorkflowID, job)
	fake.AddJobV3(job)
	return job
}

func TestWorkflowRerun_SSH(t *testing.T) {
	fake, env := setupWorkflowFake(t)
	addSSHRerunJob(fake, 1)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID, "--ssh", "--job", "run-tests"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	// What the endpoint decoded, not merely what was sent: the v2 names
	// (enable_ssh) would read as unset here, as they do on the real v3 handler.
	t.Run("endpoint saw ssh and the job", func(t *testing.T) {
		req, ok := fake.RerunRequestFor(testWorkflowDetailID)
		assert.Assert(t, ok, "no rerun was requested")
		assert.Check(t, cmp.DeepEqual(req, fakes.RerunRequest{
			IsSSHEnabled: true,
			Jobs:         []string{wfRunTestsJobID},
		}))
	})
}

// TestWorkflowRerun_SSH_JSON waits on a job with two parallel executions, each
// with its own command.
func TestWorkflowRerun_SSH_JSON(t *testing.T) {
	fake, env := setupWorkflowFake(t)
	addSSHRerunJob(fake, 2)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID, "--ssh", "--job", "run-tests", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(result.Stderr, ""), "--json prints no progress")

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"workflow_id": fakes.DefaultRerunWorkflowID,
		"rerun_from":  testWorkflowDetailID,
		"from_failed": false,
		"ssh":         true,
		"jobs": []any{
			map[string]any{
				"id":           wfRunTestsJobID,
				"name":         "run-tests",
				"rerun_job_id": wfSSHRerunJobID,
				"ssh_commands": []any{
					map[string]any{
						"execution": float64(0),
						"command":   "ssh " + wfSSHRerunJobID + "-0@ssh.circleci.com",
						"user":      wfSSHRerunJobID + "-0",
						"host":      "ssh.circleci.com",
					},
					map[string]any{
						"execution": float64(1),
						"command":   "ssh " + wfSSHRerunJobID + "-1@ssh.circleci.com",
						"user":      wfSSHRerunJobID + "-1",
						"host":      "ssh.circleci.com",
					},
				},
			},
		},
	}))
}

func TestWorkflowRerun_SSH_NoWait(t *testing.T) {
	fake, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID, "--ssh", "--job", "run-tests", "--no-wait"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	// The rerun is the last request: nothing polls the new workflow.
	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/workflows/" + testWorkflowDetailID + "/rerun"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"is_from_failed":false,"is_ssh_enabled":true,"jobs":["` + wfRunTestsJobID + `"]}`),
		}, ignoreCommonHeaders))
	})
}

// TestWorkflowRerun_SSH_Timeout reruns a job that never leaves the queue.
func TestWorkflowRerun_SSH_Timeout(t *testing.T) {
	fake, env := setupWorkflowFake(t)
	queued := fakes.JobV3{ID: wfSSHRerunJobID, Name: "run-tests", Type: "build", Phase: "queued"}
	fake.AddWorkflowJobsV3(fakes.DefaultRerunWorkflowID, queued)
	fake.AddJobV3(queued)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID, "--ssh", "--job", "run-tests", "--timeout", "1s"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 8), "stderr: %s", result.Stderr) // ExitTimeout
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestWorkflowRerun_SSH_Timeout_JSON reruns two jobs, one of which connects
// and one of which never leaves the queue. The command found is still printed.
func TestWorkflowRerun_SSH_Timeout_JSON(t *testing.T) {
	fake, env := setupWorkflowFake(t)
	runTests := addSSHRerunJob(fake, 1)
	deploy := fakes.JobV3{ID: "d0000000-0000-4000-8000-000000000302", Name: "deploy", Type: "build", Phase: "queued"}
	fake.AddWorkflowJobsV3(fakes.DefaultRerunWorkflowID, runTests, deploy)
	fake.AddJobV3(deploy)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"workflow", "rerun", testWorkflowDetailID,
			"--ssh", "--job", "run-tests", "--job", "deploy", "--timeout", "1s", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 8), "stderr: %s", result.Stderr) // ExitTimeout
	assert.Check(t, cmp.Contains(result.Stderr, "No ssh command for deploy within 1s"))

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"workflow_id": fakes.DefaultRerunWorkflowID,
		"rerun_from":  testWorkflowDetailID,
		"from_failed": false,
		"ssh":         true,
		"jobs": []any{
			map[string]any{
				"id":           wfRunTestsJobID,
				"name":         "run-tests",
				"rerun_job_id": wfSSHRerunJobID,
				"ssh_commands": []any{
					map[string]any{
						"execution": float64(0),
						"command":   "ssh " + wfSSHRerunJobID + "-0@ssh.circleci.com",
						"user":      wfSSHRerunJobID + "-0",
						"host":      "ssh.circleci.com",
					},
				},
			},
			map[string]any{"id": wfDeployJobID, "name": "deploy"},
		},
	}))
}

// TestWorkflowRerun_SSH_EndedWhileQueued reruns a job that ends before it
// starts (here, canceled while queued). It has no executions, and must fail
// at once rather than read as queued until the timeout.
func TestWorkflowRerun_SSH_EndedWhileQueued(t *testing.T) {
	fake, env := setupWorkflowFake(t)
	canceled := fakes.JobV3{ID: wfSSHRerunJobID, Name: "run-tests", Type: "build", Phase: "ended", Outcome: "canceled"}
	fake.AddWorkflowJobsV3(fakes.DefaultRerunWorkflowID, canceled)
	fake.AddJobV3(canceled)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID, "--ssh", "--job", "run-tests"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 1), "stderr: %s", result.Stderr) // ExitGeneralError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestWorkflowRerun_SSH_JobEndedWithoutSSH reruns a job that ends without ever
// running an SSH step.
func TestWorkflowRerun_SSH_JobEndedWithoutSSH(t *testing.T) {
	fake, env := setupWorkflowFake(t)
	ended := fakes.JobV3{
		ID: wfSSHRerunJobID, Name: "run-tests", Type: "build",
		Phase: "ended", Outcome: "failed",
		StartedAt: "2026-11-17T17:11:12.621Z", EndedAt: "2026-11-17T17:11:20.000Z",
		Executions: [][]fakes.JobStep{{
			{Name: "Spin up environment", Type: "spinup_environment", Num: 0, Phase: "ended", Outcome: "failed",
				StartedAt: "2026-11-17T17:11:10.895Z", EndedAt: "2026-11-17T17:11:20.000Z"},
		}},
	}
	fake.AddWorkflowJobsV3(fakes.DefaultRerunWorkflowID, ended)
	fake.AddJobV3(ended)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID, "--ssh", "--job", "run-tests"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 1), "stderr: %s", result.Stderr) // ExitGeneralError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestWorkflowRerun_JobByNameAndID_JSON reruns jobs named both ways at once,
// with one named twice, and checks each resolves to a single job.
func TestWorkflowRerun_JobByNameAndID_JSON(t *testing.T) {
	fake, env := setupWorkflowFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{"workflow", "rerun", testWorkflowDetailID,
			"--job", "run-tests", "--job", wfDeployJobID, "--job", wfRunTestsJobID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"workflow_id": fakes.DefaultRerunWorkflowID,
		"rerun_from":  testWorkflowDetailID,
		"from_failed": false,
		"ssh":         false,
		"jobs": []any{
			map[string]any{"id": wfRunTestsJobID, "name": "run-tests"},
			map[string]any{"id": wfDeployJobID, "name": "deploy"},
		},
	}))

	req, ok := fake.RerunRequestFor(testWorkflowDetailID)
	assert.Assert(t, ok, "no rerun was requested")
	assert.Check(t, cmp.DeepEqual(req, fakes.RerunRequest{
		Jobs: []string{wfRunTestsJobID, wfDeployJobID},
	}))
}

func TestWorkflowRerun_InvalidFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "ssh without job",
			args: []string{"--ssh"},
			want: "--ssh needs at least one --job",
		},
		{
			name: "ssh with from-failed",
			args: []string{"--ssh", "--job", "run-tests", "--from-failed"},
			want: "--ssh and --from-failed can't be used together",
		},
		{
			name: "job with from-failed",
			args: []string{"--job", "run-tests", "--from-failed"},
			want: "--job and --from-failed can't be used together",
		},
		{
			name: "empty job",
			args: []string{"--job", ""},
			want: "--job needs a job name or job ID",
		},
		{
			name: "zero timeout",
			args: []string{"--ssh", "--job", "run-tests", "--timeout", "0s"},
			want: "--timeout must be a positive duration",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake, env := setupWorkflowFake(t)

			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    append([]string{"workflow", "rerun", testWorkflowDetailID}, tc.args...),
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr) // ExitBadArguments
			assert.Check(t, cmp.Contains(result.Stderr, tc.want))
			_, rerun := fake.RerunRequestFor(testWorkflowDetailID)
			assert.Check(t, !rerun, "no rerun may be requested for invalid flags")
		})
	}
}

func TestWorkflowRerun_JobNotFound(t *testing.T) {
	for _, tc := range []struct {
		name string
		job  string
	}{
		{name: "by name", job: "lint"},
		// A real job ID, but from some other workflow.
		{name: "by id", job: "d0000000-0000-4000-8000-000000000999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake, env := setupWorkflowFake(t)

			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    []string{"workflow", "rerun", testWorkflowDetailID, "--ssh", "--job", tc.job},
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Check(t, cmp.Equal(result.ExitCode, 5), "stderr: %s", result.Stderr) // ExitNotFound
			assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
			_, rerun := fake.RerunRequestFor(testWorkflowDetailID)
			assert.Check(t, !rerun, "no rerun may be requested for an unknown job")
		})
	}
}

func TestWorkflowRerun_SSHDisabled(t *testing.T) {
	fake, env := setupWorkflowFake(t)
	fake.SetProjectSettings(wfProjectID, map[string]any{"is_ssh_disabled": true})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID, "--ssh", "--job", "run-tests"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 1), "stderr: %s", result.Stderr) // ExitGeneralError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	_, rerun := fake.RerunRequestFor(testWorkflowDetailID)
	assert.Check(t, !rerun, "no rerun may be requested when SSH is disabled")
}

// TestWorkflowRerun_Disabled pins what the user sees when an organization has
// reruns switched off: the API's reason (the v3 detail), not a bare "Bad
// Request", plus where to turn them back on.
func TestWorkflowRerun_Disabled(t *testing.T) {
	fake, env := setupWorkflowFake(t)
	fake.SetRerunError(testWorkflowDetailID, http.StatusBadRequest, "Rerun requests are disabled")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4), "stderr: %s", result.Stderr) // ExitAPIError
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, cmp.Contains(result.Stderr, "Rerun requests are disabled"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestWorkflowRerun_NotFinished pins the refusal agents hit most with
// --from-failed: the workflow is still running because approval jobs are on
// hold. The user sees the API's reason and what to do about it.
func TestWorkflowRerun_NotFinished(t *testing.T) {
	fake, env := setupWorkflowFake(t)
	fake.SetRerunError(testWorkflowDetailID, http.StatusBadRequest, "Workflow must be in a terminal state to be rerun")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", "--from-failed", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4), "stderr: %s", result.Stderr) // ExitAPIError
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, cmp.Contains(result.Stderr, "Workflow must be in a terminal state to be rerun"))
	assert.Check(t, cmp.Contains(result.Stderr, "workflow cancel "+testWorkflowDetailID))
	assert.Check(t, !strings.Contains(result.Stderr, "Organization Settings"), "stderr: %s", result.Stderr)
}

// TestWorkflowRerun_LimitReached checks the other refusal the API makes with a
// reason, a 403, reaches the user as that reason.
func TestWorkflowRerun_LimitReached(t *testing.T) {
	fake, env := setupWorkflowFake(t)
	fake.SetRerunError(testWorkflowDetailID, http.StatusForbidden, "Rerun limit reached")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"workflow", "rerun", testWorkflowDetailID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4), "stderr: %s", result.Stderr) // ExitAPIError
	assert.Check(t, cmp.Contains(result.Stderr, "Rerun limit reached"))
	assert.Check(t, !strings.Contains(result.Stderr, "Organization Settings"), "stderr: %s", result.Stderr)
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
			URL:    url.URL{Path: "/api/v3/workflows/" + testWorkflowDetailID + "/cancel"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(``),
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
