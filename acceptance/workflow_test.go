package acceptance_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"

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
	assert.Assert(t, strings.Contains(result.Stdout, testWorkflowDetailID), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "build"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "failed"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "run-tests"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "#201"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "#202"), "stdout: %s", result.Stdout)
}

func TestWorkflowGet_JSON(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "get", "--json", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Equal(t, out["id"], testWorkflowDetailID)
	assert.Equal(t, out["name"], "build")
	assert.Equal(t, out["status"], "failed")

	jobs := out["jobs"].([]any)
	assert.Equal(t, len(jobs), 2)
	assert.Equal(t, jobs[0].(map[string]any)["name"], "run-tests")
	assert.Equal(t, jobs[0].(map[string]any)["number"], float64(201))
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
	assert.Assert(t, strings.Contains(result.Stderr, "No workflow found"), "stderr: %s", result.Stderr)
}

func TestWorkflowGet_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "get", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Assert(t, strings.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}

func TestWorkflowGet_NotFound_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"workflow", "get", "--json", "00000000-0000-0000-0000-000000000000"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)

	var errOut map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stderr), &errOut), "stderr should be JSON: %s", result.Stderr)
	assert.Equal(t, errOut["error"], true)
	assert.Equal(t, errOut["exit_code"], float64(5))
	assert.Assert(t, errOut["code"] != nil, "code field missing")
	assert.Assert(t, errOut["message"] != nil, "message field missing")
	// stdout must be empty — no partial data output
	assert.Equal(t, strings.TrimSpace(result.Stdout), "")
}

func TestWorkflowGet_NoToken_JSON(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "get", "--json", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)

	var errOut map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stderr), &errOut), "stderr should be JSON: %s", result.Stderr)
	assert.Equal(t, errOut["error"], true)
	assert.Equal(t, errOut["exit_code"], float64(3))
	assert.Equal(t, strings.TrimSpace(result.Stdout), "")
}

// --- workflow rerun ---

func TestWorkflowRerun(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "rerun", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "from scratch"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, testWorkflowDetailID), "stdout: %s", result.Stdout)
}

func TestWorkflowRerun_FromFailed(t *testing.T) {
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "rerun", "--from-failed", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "failed jobs"), "stdout: %s", result.Stdout)
}

func TestWorkflowRerun_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "rerun", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Assert(t, strings.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
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
	assert.Assert(t, strings.Contains(result.Stdout, "Cancelled"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, testWorkflowDetailID), "stdout: %s", result.Stdout)
}

func TestWorkflowCancel_RequiresForce(t *testing.T) {
	// In non-interactive mode (no TTY), --force is required.
	_, env := setupWorkflowFake(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "cancel", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr) // ExitCancelled
	assert.Assert(t, strings.Contains(result.Stderr, "--force"), "stderr: %s", result.Stderr)
}

func TestWorkflowCancel_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"workflow", "cancel", "--force", testWorkflowDetailID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Assert(t, strings.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
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
