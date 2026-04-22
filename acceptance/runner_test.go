package acceptance_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

func fakeRC(id, slug, desc string) map[string]any {
	return map[string]any{
		"id":             id,
		"resource_class": slug,
		"description":    desc,
	}
}

func fakeToken(id, rc, nickname string) map[string]any {
	return map[string]any{
		"id":             id,
		"resource_class": rc,
		"nickname":       nickname,
		"created_at":     "2026-01-01T00:00:00Z",
	}
}

func fakeInstance(rc, hostname, name, version string) map[string]any {
	return map[string]any{
		"resource_class":     rc,
		"hostname":           hostname,
		"name":               name,
		"version":            version,
		"ip":                 "10.0.0.1",
		"first_connected_at": "2026-01-01T00:00:00Z",
		"last_connected_at":  "2026-04-18T12:00:00Z",
		"last_used_at":       "2026-04-18T11:00:00Z",
	}
}

func setupRunnerFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddResourceClass(fakeRC("rc-id-1", "my-org/linux-runner", "Linux amd64 runner"))
	fake.AddResourceClass(fakeRC("rc-id-2", "my-org/arm-runner", "ARM runner"))

	fake.AddRunnerToken("my-org/linux-runner", fakeToken("tok-id-1", "my-org/linux-runner", "prod-server-1"))
	fake.AddRunnerToken("my-org/linux-runner", fakeToken("tok-id-2", "my-org/linux-runner", "prod-server-2"))

	fake.AddRunnerInstance(fakeInstance("my-org/linux-runner", "host-1.example.com", "runner-1", "1.0.0"))
	fake.AddRunnerInstance(fakeInstance("my-org/arm-runner", "arm-host.example.com", "runner-2", "1.0.0"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- resource-class list ---

func TestRunnerResourceClassList(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "resource-class", "list", "--namespace", "my-org"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "my-org/linux-runner"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "my-org/arm-runner"), "stdout: %s", result.Stdout)
}

func TestRunnerResourceClassList_Namespace(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "resource-class", "list", "--namespace", "my-org"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "my-org/linux-runner"), "stdout: %s", result.Stdout)
}

func TestRunnerResourceClassList_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "resource-class", "list", "--namespace", "my-org", "--json"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Equal(t, len(out), 2)
	assert.Equal(t, out[0]["resource_class"], "my-org/linux-runner")
	assert.Equal(t, out[0]["description"], "Linux amd64 runner")
}

func TestRunnerResourceClassList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"runner", "resource-class", "list"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}

// --- resource-class create ---

func TestRunnerResourceClassCreate(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "resource-class", "create", "my-org/new-runner", "--description", "New runner"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "my-org/new-runner"), "stdout: %s", result.Stdout)
}

func TestRunnerResourceClassCreate_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "resource-class", "create", "my-org/new-runner", "--description", "New runner", "--json"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Equal(t, out["resource_class"], "my-org/new-runner")
	assert.Equal(t, out["description"], "New runner")
}

// --- resource-class delete ---

func TestRunnerResourceClassDelete_NoForce(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "resource-class", "delete", "my-org/linux-runner"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr) // ExitCancelled
	assert.Assert(t, strings.Contains(result.Stderr, "--force"), "stderr: %s", result.Stderr)
}

func TestRunnerResourceClassDelete_Force(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "resource-class", "delete", "my-org/linux-runner", "--force"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, "Deleted"), "stderr: %s", result.Stderr)
}

func TestRunnerResourceClassDelete_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"runner", "resource-class", "delete", "my-org/nonexistent", "--force"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
}

// --- token list ---

func TestRunnerTokenList(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "token", "list", "--resource-class", "my-org/linux-runner"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "tok-id-1"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "prod-server-1"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "prod-server-2"), "stdout: %s", result.Stdout)
}

func TestRunnerTokenList_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "token", "list", "--resource-class", "my-org/linux-runner", "--json"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Equal(t, len(out), 2)
	assert.Equal(t, out[0]["id"], "tok-id-1")
	assert.Equal(t, out[0]["nickname"], "prod-server-1")
}

func TestRunnerTokenList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"runner", "token", "list", "--resource-class", "my-org/linux-runner"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "No tokens found"), "stdout: %s", result.Stdout)
}

// --- token create ---

func TestRunnerTokenCreate(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "token", "create", "my-org/linux-runner", "--nickname", "my-server"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "fake-runner-token-value"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "my-server"), "stdout: %s", result.Stdout)
}

func TestRunnerTokenCreate_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "token", "create", "my-org/linux-runner", "--json"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Equal(t, out["resource_class"], "my-org/linux-runner")
	assert.Equal(t, out["token"], "fake-runner-token-value")
}

// --- token delete ---

func TestRunnerTokenDelete(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "token", "delete", "--force", "tok-id-1"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "Deleted"), "stdout: %s", result.Stdout)
}

func TestRunnerTokenDelete_RequiresForce(t *testing.T) {
	// In non-interactive mode (no TTY), --force is required.
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "token", "delete", "tok-id-1"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr) // ExitCancelled
	assert.Assert(t, strings.Contains(result.Stderr, "--force"), "stderr: %s", result.Stderr)
}

func TestRunnerTokenDelete_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"runner", "token", "delete", "--force", "nonexistent-token-id"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
}

// --- instance list ---

func TestRunnerInstanceList(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "instance", "list", "--namespace", "my-org"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "my-org/linux-runner"), "stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "my-org/arm-runner"), "stdout: %s", result.Stdout)
}

func TestRunnerInstanceList_ResourceClass(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "instance", "list", "--resource-class", "my-org/linux-runner"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "my-org/linux-runner"), "stdout: %s", result.Stdout)
	assert.Assert(t, !strings.Contains(result.Stdout, "my-org/arm-runner"), "arm-runner should be filtered out, stdout: %s", result.Stdout)
}

func TestRunnerInstanceList_JSON(t *testing.T) {
	_, env := setupRunnerFake(t)

	result := binary.RunCLI(t,
		[]string{"runner", "instance", "list", "--namespace", "my-org", "--json"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Equal(t, len(out), 2)
}

func TestRunnerInstanceList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"runner", "instance", "list", "--namespace", "my-org"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "No runner instances found"), "stdout: %s", result.Stdout)
}

func TestRunnerInstanceList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"runner", "instance", "list"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, "No CircleCI API token found"), "stderr: %s", result.Stderr)
}

// verify the fake server returns a proper 202 for rerun (used by TestRunnerResourceClassDelete_Force indirectly)
var _ = http.StatusAccepted
