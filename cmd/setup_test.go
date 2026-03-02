package cmd_test

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
	"gotest.tools/v3/assert"
)

func TestSetupTelemetry(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"setup",
			"--integration-testing",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"--token", "testtoken",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	testhelpers.AssertTelemetrySubset(t, ts, []telemetry.Event{
		telemetry.CreateSetupEvent(true),
	})
}

func TestSetupNewConfigFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions test not applicable on Windows")
	}

	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"setup",
			"--integration-testing",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"--token", "testtoken",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	assert.Assert(t, strings.Contains(result.Stdout, "CircleCI API Token"))
	assert.Assert(t, strings.Contains(result.Stdout, "API token has been set."))
	assert.Assert(t, strings.Contains(result.Stdout, "CircleCI Host"))
	assert.Assert(t, strings.Contains(result.Stdout, "CircleCI host has been set."))
	assert.Assert(t, strings.Contains(result.Stdout, "Setup complete."))
	assert.Assert(t, strings.Contains(result.Stdout, ts.Config))
	assert.Equal(t, result.Stderr, "")

	fileInfo, err := os.Stat(ts.Config)
	assert.NilError(t, err)
	assert.Equal(t, fileInfo.Mode().Perm().String(), "-rw-------")
}

func TestSetupExistingConfigFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions test not applicable on Windows")
	}

	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"setup",
			"--integration-testing",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"--token", "testtoken",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	fileInfo, err := os.Stat(ts.Config)
	assert.NilError(t, err)
	assert.Equal(t, fileInfo.Mode().Perm().String(), "-rw-------")
}

func TestSetupExistingTokenAndHost(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	ts.WriteConfig(t, `
host: https://example.com/graphql
token: fooBarBaz
`)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"setup",
			"--integration-testing",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"--token", "testtoken",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	assert.Assert(t, strings.Contains(result.Stdout, "A CircleCI token is already set. Do you want to change it"))
	assert.Assert(t, strings.Contains(result.Stdout, "CircleCI API Token"))
	assert.Assert(t, strings.Contains(result.Stdout, "API token has been set."))
	assert.Assert(t, strings.Contains(result.Stdout, "CircleCI Host"))
	assert.Assert(t, strings.Contains(result.Stdout, "CircleCI host has been set."))
	assert.Assert(t, strings.Contains(result.Stdout, fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.\n", ts.Config)))
}

func TestSetupNoPromptWithValidConfig(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	ts.WriteConfig(t, `
host: https://example.com
token: fooBarBaz
`)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"setup",
			"--no-prompt",
			"--skip-update-check",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, fmt.Sprintf("Setup has kept your existing configuration at %s.\n", ts.Config)))

	reread, err := os.ReadFile(ts.Config)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(reread), "host: https://example.com"))
	assert.Assert(t, strings.Contains(string(reread), "token: fooBarBaz"))
}

func TestSetupNoPromptChangeHost(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	ts.WriteConfig(t, `
host: https://example.com
token: fooBarBaz
`)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"setup",
			"--host", "https://asdf.example.com",
			"--no-prompt",
			"--skip-update-check",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	expected := fmt.Sprintf("Token unchanged from existing config. Use --token with --no-prompt to overwrite it.\nSetup complete.\nYour configuration has been saved to %s.\n", ts.Config)
	assert.Equal(t, result.Stdout, expected)

	reread, err := os.ReadFile(ts.Config)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(reread), "host: https://asdf.example.com"))
	assert.Assert(t, strings.Contains(string(reread), "token: fooBarBaz"))
}

func TestSetupNoPromptChangeToken(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	ts.WriteConfig(t, `
host: https://example.com
token: fooBarBaz
`)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"setup",
			"--token", "asdf",
			"--no-prompt",
			"--skip-update-check",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	expected := fmt.Sprintf("Host unchanged from existing config. Use --host with --no-prompt to overwrite it.\nSetup complete.\nYour configuration has been saved to %s.\n", ts.Config)
	assert.Equal(t, result.Stdout, expected)

	reread, err := os.ReadFile(ts.Config)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(reread), "host: https://example.com"))
	assert.Assert(t, strings.Contains(string(reread), "token: asdf"))
}

func TestSetupNoPromptInvalidHost(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	ts.WriteConfig(t, `
host: https://example.com
token: fooBarBaz
`)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"setup",
			"--host", "not-a-valid-url",
			"--no-prompt",
			"--skip-update-check",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Equal(t, result.Stderr, "Error: invalid CircleCI URL\n")
}

func TestSetupNoPromptMissingHostAndToken(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"setup",
			"--no-prompt",
			"--skip-update-check",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Equal(t, result.Stderr, "Error: No existing host or token saved.\nThe proper format is `circleci setup --host HOST --token TOKEN --no-prompt\n")
}

func TestSetupNoPromptWithBothHostAndToken(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"setup",
			"--host", "https://zomg.com",
			"--token", "mytoken",
			"--no-prompt",
			"--skip-update-check",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	expected := fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.\n", ts.Config)
	assert.Equal(t, result.Stdout, expected)

	reread, err := os.ReadFile(ts.Config)
	assert.NilError(t, err)
	content := string(reread)
	assert.Assert(t, strings.Contains(content, "host: https://zomg.com"))
	assert.Assert(t, strings.Contains(content, "token: mytoken"))
}
