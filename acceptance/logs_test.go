package acceptance_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

const (
	testLogsJobNumber = 99

	// Separate pipeline/workflow IDs to avoid collision with artifact tests.
	testLogsPipelineID = "aaaaaaaa-0000-0000-0000-000000000002"
	testLogsWorkflowID = "bbbbbbbb-0000-0000-0000-000000000002"
)

func logLine(msg string) map[string]any {
	return map[string]any{
		"type":    "out",
		"time":    time.Now().UTC().Format(time.RFC3339Nano),
		"message": msg,
	}
}

func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// setupLogsFake builds a fake server with one failed pipeline → workflow → job
// with two steps. Step 0 succeeds; step 1 fails.
func setupLogsFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	step0Path := "/output/job/99/step/0"
	step1Path := "/output/job/99/step/1"

	fake.AddStepOutput(step0Path, mustMarshal([]map[string]any{logLine("Setting up...\n")}))
	fake.AddStepOutput(step1Path, mustMarshal([]map[string]any{logLine("FAIL: TestFoo\n")}))

	now := time.Now().UTC().Format(time.RFC3339)
	fake.AddJob(testSlug, testLogsJobNumber, map[string]any{
		"job_number": testLogsJobNumber,
		"name":       "build-and-test",
		"status":     "failed",
		"started_at": now,
		"stopped_at": now,
		"steps": []map[string]any{
			{
				"name": "Spin up environment",
				"actions": []map[string]any{
					{
						"index":      0,
						"name":       "Spin up environment",
						"status":     "success",
						"exit_code":  0,
						"start_time": now,
						"end_time":   now,
						"output_url": fake.URL() + step0Path,
					},
				},
			},
			{
				"name": "Run tests",
				"actions": []map[string]any{
					{
						"index":      1,
						"name":       "Run tests",
						"status":     "failed",
						"exit_code":  1,
						"start_time": now,
						"end_time":   now,
						"output_url": fake.URL() + step1Path,
					},
				},
			},
		},
	})

	// Wire up pipeline → workflow → job for --last-failed / --last-job inference.
	fake.AddPipeline(testLogsPipelineID,
		fakePipeline(testLogsPipelineID, 8, "created", testSlug, "main"))
	fake.AddProjectPipelines(testSlug,
		fakePipeline(testLogsPipelineID, 8, "created", testSlug, "main"))
	fake.AddPipelineWorkflows(testLogsPipelineID,
		map[string]any{"id": testLogsWorkflowID, "name": "build", "status": "failed"})
	fake.AddWorkflowJobs(testLogsWorkflowID, map[string]any{
		"id":           "job-uuid-99",
		"name":         "build-and-test",
		"job_number":   testLogsJobNumber,
		"status":       "failed",
		"type":         "build",
		"project_slug": testSlug,
		"started_at":   now,
		"stopped_at":   now,
	})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return fake, env
}

func TestJobLogs_ByNumber(t *testing.T) {
	_, env := setupLogsFake(t)

	result := binary.RunCLI(t,
		[]string{"job", "logs", fmt.Sprintf("%d", testLogsJobNumber), "--project", testSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "Spin up environment"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Setting up..."), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Run tests (failed)"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "FAIL: TestFoo"), "stdout: %s", result.Stdout)
}

func TestJobLogs_FilterStep(t *testing.T) {
	_, env := setupLogsFake(t)

	result := binary.RunCLI(t,
		[]string{"job", "logs", fmt.Sprintf("%d", testLogsJobNumber), "--project", testSlug, "--step", "Run tests"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "FAIL: TestFoo"), "stdout: %s", result.Stdout)
	assert.Assert(t, !strings.Contains(result.Stdout, "Setting up..."), "stdout should not contain step 0: %s", result.Stdout)
}

func TestJobLogs_JSON(t *testing.T) {
	_, env := setupLogsFake(t)

	result := binary.RunCLI(t,
		[]string{"job", "logs", fmt.Sprintf("%d", testLogsJobNumber), "--project", testSlug, "--json"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Equal(t, len(out), 2)
	assert.Equal(t, out[0]["step"], "Spin up environment")
	assert.Equal(t, out[0]["status"], "success")
	assert.Equal(t, out[1]["step"], "Run tests")
	assert.Equal(t, out[1]["status"], "failed")
}

func TestLogs_ByNumber(t *testing.T) {
	_, env := setupLogsFake(t)

	result := binary.RunCLI(t,
		[]string{"logs", fmt.Sprintf("%d", testLogsJobNumber), "--project", testSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "Spin up environment"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "FAIL: TestFoo"), "stdout: %s", result.Stdout)
}

func TestLogs_LastFailed(t *testing.T) {
	_, env := setupLogsFake(t)

	result := binary.RunCLI(t,
		[]string{"logs", "--last-failed", "--project", testSlug, "--branch", "main"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "FAIL: TestFoo"), "stdout: %s", result.Stdout)
}

func TestLogs_LastJob(t *testing.T) {
	_, env := setupLogsFake(t)

	result := binary.RunCLI(t,
		[]string{"logs", "--last-job", "--project", testSlug, "--branch", "main"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "FAIL: TestFoo"), "stdout: %s", result.Stdout)
}

func TestLogs_LastFailed_AllPassed(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	passedPipelineID := "aaaaaaaa-0000-0000-0000-000000000099"
	passedWorkflowID := "bbbbbbbb-0000-0000-0000-000000000099"

	fake.AddPipeline(passedPipelineID,
		fakePipeline(passedPipelineID, 9, "created", testSlug, "main"))
	fake.AddProjectPipelines(testSlug,
		fakePipeline(passedPipelineID, 9, "created", testSlug, "main"))
	fake.AddPipelineWorkflows(passedPipelineID,
		map[string]any{"id": passedWorkflowID, "name": "build", "status": "success"})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"logs", "--last-failed", "--project", testSlug, "--branch", "main"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
	assert.Assert(t, strings.Contains(result.Stderr, "all workflows in this pipeline passed"), "stderr: %s", result.Stderr)
}

func TestLogs_NoArgs(t *testing.T) {
	env := testenv.New(t)
	env.Token = "testtoken"

	result := binary.RunCLI(t,
		[]string{"logs"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Assert(t, strings.Contains(result.Stderr, "--last-failed or --last-job"), "stderr: %s", result.Stderr)
}

func TestLogs_ConflictingArgs(t *testing.T) {
	env := testenv.New(t)
	env.Token = "testtoken"

	result := binary.RunCLI(t,
		[]string{"logs", "99", "--last-failed"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Assert(t, strings.Contains(result.Stderr, "Provide exactly one of"), "stderr: %s", result.Stderr)
}

// TestJobLogs_V1Fallback verifies that when the v2 job endpoint returns no steps,
// the CLI retries against v1.1 and successfully retrieves output.
func TestJobLogs_V1Fallback(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	step0Path := "/output/v1fallback/step/0"
	fake.AddStepOutput(step0Path, mustMarshal([]map[string]any{logLine("v1 output\n")}))

	now := time.Now().UTC().Format(time.RFC3339)

	// v2 job has no steps — simulates the real CircleCI v2 API behaviour.
	fake.AddJob(testSlug, testLogsJobNumber, map[string]any{
		"job_number": testLogsJobNumber,
		"name":       "build-and-test",
		"status":     "failed",
		"started_at": now,
		"stopped_at": now,
		"steps":      []any{},
	})

	// v1.1 job has steps with output URLs.
	// The fake key uses the full provider name (github not gh).
	const v1Slug = "github/testorg/testrepo"
	fake.AddJobV1(v1Slug, testLogsJobNumber, map[string]any{
		"build_num": testLogsJobNumber,
		"status":    "failed",
		"steps": []map[string]any{
			{
				"name": "Run tests",
				"actions": []map[string]any{
					{
						"index":      0,
						"name":       "Run tests",
						"status":     "failed",
						"exit_code":  1,
						"start_time": now,
						"end_time":   now,
						"output_url": fake.URL() + step0Path,
					},
				},
			},
		},
	})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"job", "logs", fmt.Sprintf("%d", testLogsJobNumber), "--project", testSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "v1 output"), "stdout: %s", result.Stdout)
}

func TestLogs_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"logs", "99", "--project", testSlug},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
	assert.Assert(t, strings.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}
