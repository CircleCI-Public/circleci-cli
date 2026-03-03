package cmd_test

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
)

func TestTelemetryEnableEvent(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{"telemetry", "enable"},
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	testhelpers.AssertTelemetrySubset(t, ts, []telemetry.Event{
		telemetry.CreateChangeTelemetryStatusEvent("enabled", "telemetry-command", nil),
	})
}

func TestTelemetryDisableEvent(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{"telemetry", "disable"},
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	testhelpers.AssertTelemetrySubset(t, ts, []telemetry.Event{
		telemetry.CreateChangeTelemetryStatusEvent("disabled", "telemetry-command", nil),
	})
}
