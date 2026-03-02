package cmd_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
	"gotest.tools/v3/assert"
)

func meHandler(t *testing.T, name string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/me" {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body := map[string]any{
			"name":       name,
			"login":      "zomg",
			"id":         "97491110-fea3-49b1-83da-ffd38ac8840c",
			"avatar_url": "https://avatars.githubusercontent.com/u/980172390812730912?v=4",
		}
		_ = json.NewEncoder(w).Encode(body)
	}
}

func TestDiagnosticTelemetry(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	server := httptest.NewServer(meHandler(t, "zomg"))
	t.Cleanup(func() { server.Close() })

	ts.WriteConfig(t, "token: mytoken")

	result := testhelpers.RunCLI(t, binary,
		[]string{"diagnostic", "--skip-update-check", "--host", server.URL},
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	testhelpers.AssertTelemetrySubset(t, ts, []telemetry.Event{
		telemetry.CreateDiagnosticEvent(nil),
	})
}

func TestDiagnosticTokenSet(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	server := httptest.NewServer(meHandler(t, "zomg"))
	t.Cleanup(func() { server.Close() })

	ts.WriteConfig(t, "token: mytoken")

	result := testhelpers.RunCLI(t, binary,
		[]string{"diagnostic", "--skip-update-check", "--host", server.URL},
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stderr, "")
	assert.Assert(t, strings.Contains(result.Stdout, fmt.Sprintf("API host: %s", server.URL)),
		"stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "API endpoint: graphql-unstable"),
		"stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "OK, got a token."),
		"stdout: %s", result.Stdout)
}

func TestDiagnosticHostOverride(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	server := httptest.NewServer(meHandler(t, "zomg"))
	t.Cleanup(func() { server.Close() })

	ts.WriteConfig(t, fmt.Sprintf("host: https://circleci.com/\ntoken: mytoken\n"))

	result := testhelpers.RunCLI(t, binary,
		[]string{"diagnostic", "--skip-update-check", "--host", server.URL},
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stderr, "")
	assert.Assert(t, strings.Contains(result.Stdout, fmt.Sprintf("API host: %s", server.URL)),
		"stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "API endpoint: graphql-unstable"),
		"stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "OK, got a token."),
		"stdout: %s", result.Stdout)
}

func TestDiagnosticEmptyToken(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	server := httptest.NewServer(meHandler(t, "zomg"))
	t.Cleanup(func() { server.Close() })

	ts.WriteConfig(t, "token: ")

	result := testhelpers.RunCLI(t, binary,
		[]string{"diagnostic", "--skip-update-check", "--host", server.URL},
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
	)

	assert.Equal(t, result.ExitCode, testhelpers.ShouldFail(),
		"stdout: %s\nstderr: %s", result.Stdout, result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: please set a token with 'circleci setup'"),
		"stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, fmt.Sprintf("API host: %s", server.URL)),
		"stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "API endpoint: graphql-unstable"),
		"stdout: %s", result.Stdout)
}

func TestDiagnosticWhoamiReturnsUser(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	server := httptest.NewServer(meHandler(t, "zzak"))
	t.Cleanup(func() { server.Close() })

	ts.WriteConfig(t, "token: mytoken")

	cmd := exec.Command(binary,
		"diagnostic",
		"--skip-update-check",
		"--host", server.URL,
	)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
	)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	assert.NilError(t, err)
	assert.Equal(t, stderr.String(), "")
	assert.Assert(t, strings.Contains(stdout.String(), fmt.Sprintf("API host: %s", server.URL)),
		"stdout: %s", stdout.String())
	assert.Assert(t, strings.Contains(stdout.String(), "API endpoint: graphql-unstable"),
		"stdout: %s", stdout.String())
	assert.Assert(t, strings.Contains(stdout.String(), "OK, got a token."),
		"stdout: %s", stdout.String())
	assert.Assert(t, strings.Contains(stdout.String(), "Hello, zzak."),
		"stdout: %s", stdout.String())
}
