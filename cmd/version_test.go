package cmd_test

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
)

func TestVersionTelemetry(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{"version"},
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)

	assert.Equal(t, result.ExitCode, 0, "expected exit code 0, got %d\nstdout: %s\nstderr: %s", result.ExitCode, result.Stdout, result.Stderr)

	testhelpers.AssertTelemetrySubset(t, ts, []telemetry.Event{
		telemetry.CreateVersionEvent("0.0.0-dev+dirty-local-tree (source)"),
	})
}
