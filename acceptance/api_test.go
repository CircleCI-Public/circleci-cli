package acceptance_test

import (
	"encoding/json"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

func TestAPI_Get(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(testPipelineID, fakePipeline(testPipelineID, 42, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"api", "/pipeline/" + testPipelineID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, testPipelineID), "stdout: %s", result.Stdout)
}

func TestAPI_Get_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(testPipelineID, fakePipeline(testPipelineID, 42, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"api", "--json", "/pipeline/" + testPipelineID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	// --json must produce valid, indented JSON.
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out), "stdout: %s", result.Stdout)
	assert.Equal(t, out["id"], testPipelineID)
}

func TestAPI_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"api", "/pipeline/does-not-exist"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 4, "stderr: %s", result.Stderr) // ExitAPIError
}

func TestAPI_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"api", "/me"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
}

func TestAPI_PathDefaultsToV2(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(testPipelineID, fakePipeline(testPipelineID, 7, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	// Path without /api/ prefix should be routed to /api/v2.
	result := binary.RunCLI(t,
		[]string{"api", "/pipeline/" + testPipelineID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, testPipelineID))
}
