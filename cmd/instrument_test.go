package cmd_test

import (
	"fmt"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/testhelpers"
	"gotest.tools/v3/assert"
)

func TestInstrumentEmitsStartedAndFinished(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{"version"},
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	events := testhelpers.ReadTelemetryEventsFromFile(t, ts.TelemetryDestPath)

	var started, finished map[string]interface{}
	for _, e := range events {
		if e.Object == "cli_command_started" {
			started = e.Properties
		}
		if e.Object == "cli_command_finished" {
			finished = e.Properties
		}
	}

	assert.Assert(t, started != nil, "expected cli_command_started event")
	assert.Assert(t, finished != nil, "expected cli_command_finished event")

	assert.Equal(t, started["command_path"], "circleci version")
	assert.Equal(t, finished["command_path"], "circleci version")

	assert.Assert(t, started["invocation_id"] != nil && started["invocation_id"] != "",
		"expected non-empty invocation_id")
	assert.Equal(t, started["invocation_id"], finished["invocation_id"])

	assert.Equal(t, finished["outcome"], "success")
	assert.Assert(t, finished["duration_ms"] != nil, "expected duration_ms to be set")
}

func TestInstrumentFlagsUsed(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{"version", "--skip-update-check", "--token=fake-secret-token"},
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	events := testhelpers.ReadTelemetryEventsFromFile(t, ts.TelemetryDestPath)

	for _, e := range events {
		if e.Object == "cli_command_started" || e.Object == "cli_command_finished" {
			flags, ok := e.Properties["flags_used"].(map[string]interface{})
			if ok {
				assert.Assert(t, flags["skip-update-check"] != nil, "expected skip-update-check flag")
				assert.Equal(t, flags["skip-update-check"], "true")

				assert.Assert(t, flags["token"] != nil, "expected token flag")
				assert.Equal(t, flags["token"], "[REDACTED]")
			}
		}
	}
}

func TestInstrumentArgsError(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{"telemetry", "enable", "extra-arg"},
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)
	assert.Equal(t, result.ExitCode, testhelpers.ShouldFail(),
		"stdout: %s\nstderr: %s", result.Stdout, result.Stderr)

	events := testhelpers.ReadTelemetryEventsFromFile(t, ts.TelemetryDestPath)

	var finished map[string]interface{}
	for _, e := range events {
		if e.Object == "cli_command_finished" {
			finished = e.Properties
		}
	}

	assert.Assert(t, finished != nil, "expected cli_command_finished event")
	assert.Equal(t, finished["outcome"], "args_error")
}
