package cmd_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
	"gotest.tools/v3/assert"
)

func githubReleasesResponse() string {
	return `
[
  {
    "id": 1,
    "tag_name": "v1.0.0",
    "name": "v1.0.0",
    "published_at": "2013-02-27T19:35:32Z",
    "assets": [
      {
        "id": 1,
        "name": "` + runtime.GOOS + "_" + runtime.GOARCH + `.zip",
        "label": "short description",
        "content_type": "application/zip",
        "size": 1024
      }
    ]
  }
]
`
}

func appendReleasesHandler(ts *testhelpers.TestServer, response string) {
	ts.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v3/repos/CircleCI-Public/circleci-cli/releases" {
			http.Error(w, fmt.Sprintf("unexpected request: %s %s", r.Method, r.URL.Path), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	})
}

// buildFreshCLI builds a fresh copy of the CLI binary that can be safely
// modified by the update command without corrupting the shared cached binary.
func buildFreshCLI(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binaryName := "circleci"
	if runtime.GOOS == "windows" {
		binaryName = "circleci.exe"
	}
	binaryPath := filepath.Join(tmpDir, binaryName)
	cmd := exec.Command("go", "build",
		"-o", binaryPath,
		"-ldflags=-X github.com/CircleCI-Public/circleci-cli/telemetry.SegmentEndpoint=https://api.segment.io",
		".",
	)
	cmd.Dir = filepath.Join("..")
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, "go build failed: %s", string(out))
	return binaryPath
}

func TestUpdateTelemetryParentCommand(t *testing.T) {
	// This test runs "update" (not just "check"), which replaces the binary.
	// Use a fresh copy to avoid corrupting the shared cached binary.
	binary := buildFreshCLI(t)
	ts := testhelpers.WithTempSettings(t)
	response := githubReleasesResponse()

	appendReleasesHandler(ts.Server, response)

	assetBytes, err := os.ReadFile(filepath.Join("testdata", "update", "foo.zip"))
	assert.NilError(t, err)

	appendReleasesHandler(ts.Server, response)
	ts.Server.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/repos/CircleCI-Public/circleci-cli/releases/assets/1" {
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(assetBytes)
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"update",
			"--github-api", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	testhelpers.AssertTelemetrySubset(t, ts, []telemetry.Event{
		telemetry.CreateUpdateEvent(telemetry.CommandInfo{
			Name:      "update",
			LocalArgs: map[string]string{},
		}),
	})
}

func TestUpdateTelemetryChildCommand(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	response := githubReleasesResponse()

	appendReleasesHandler(ts.Server, response)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"update", "check",
			"--github-api", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	testhelpers.AssertTelemetrySubset(t, ts, []telemetry.Event{
		telemetry.CreateUpdateEvent(telemetry.CommandInfo{
			Name:      "check",
			LocalArgs: map[string]string{},
		}),
	})
}

func TestUpdateCheckFlag(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	response := githubReleasesResponse()

	appendReleasesHandler(ts.Server, response)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"update", "--check",
			"--github-api", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Equal(t, result.Stderr, "")

	assert.Assert(t, strings.Contains(result.Stdout, "You are running 0.0.0-dev"))
	assert.Assert(t, strings.Contains(result.Stdout, "A new release is available"))
	assert.Assert(t, strings.Contains(result.Stdout, "You can visit the Github releases page for the CLI to manually download and install:"))
	assert.Assert(t, strings.Contains(result.Stdout, "https://github.com/CircleCI-Public/circleci-cli/releases"))
}

func TestUpdateCheckSubcommand(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	response := githubReleasesResponse()

	appendReleasesHandler(ts.Server, response)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"update", "check",
			"--github-api", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Equal(t, result.Stderr, "")

	assert.Assert(t, strings.Contains(result.Stdout, "You are running 0.0.0-dev"))
	assert.Assert(t, strings.Contains(result.Stdout, "A new release is available"))
	assert.Assert(t, strings.Contains(result.Stdout, "You can visit the Github releases page for the CLI to manually download and install:"))
	assert.Assert(t, strings.Contains(result.Stdout, "https://github.com/CircleCI-Public/circleci-cli/releases"))
}

func TestUpdateInstall(t *testing.T) {
	// This test runs "update" (not just "check"), which replaces the binary.
	// Use a fresh copy to avoid corrupting the shared cached binary.
	binary := buildFreshCLI(t)
	ts := testhelpers.WithTempSettings(t)
	response := githubReleasesResponse()

	appendReleasesHandler(ts.Server, response)

	assetBytes, err := os.ReadFile(filepath.Join("testdata", "update", "foo.zip"))
	assert.NilError(t, err)

	appendReleasesHandler(ts.Server, response)
	ts.Server.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/repos/CircleCI-Public/circleci-cli/releases/assets/1" {
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(assetBytes)
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"update",
			"--github-api", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Equal(t, result.Stderr, "")

	assert.Assert(t, strings.Contains(result.Stdout, "You are running 0.0.0-dev"))
	assert.Assert(t, strings.Contains(result.Stdout, "A new release is available"))
	assert.Assert(t, strings.Contains(result.Stdout, "Updated to 1.0.0"))
}

func TestUpdateGithub403(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	ts.Server.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Forbidden"))
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"update", "check",
			"--github-api", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0, "expected non-zero exit, stderr: %q, stdout: %q", result.Stderr, result.Stdout)

	assert.Assert(t, strings.Contains(result.Stderr, `Error: Failed to query the GitHub API for updates.`),
		"expected stderr to contain error message, got: %q", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, `This is most likely due to GitHub rate-limiting on unauthenticated requests.`),
		"expected stderr to contain rate-limiting message, got: %q", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, `https://developer.github.com/v3/repos/releases/`),
		"expected stderr to contain releases API URL, got: %q", result.Stderr)
}
