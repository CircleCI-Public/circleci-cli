package runner

import (
	"bytes"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"gotest.tools/v3/assert"
)

type testTelemetryClient struct {
	events []telemetry.Event
}

func (c *testTelemetryClient) Track(event telemetry.Event) error {
	c.events = append(c.events, event)
	return nil
}

func (c *testTelemetryClient) Close() error { return nil }

func Test_RunnerTelemetry(t *testing.T) {
	t.Run("resource-class", func(t *testing.T) {
		telemetryClient := &testTelemetryClient{make([]telemetry.Event, 0)}
		runner := runnerMock{}
		cmd := newResourceClassCommand(&runnerOpts{r: &runner, createTelemetry: func() telemetry.Client { return telemetryClient }}, nil)
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)

		defer runner.reset()
		defer stdout.Reset()
		defer stderr.Reset()

		cmd.SetArgs([]string{
			"create",
			"my-namespace/my-other-resource-class",
			"my-description",
			"--generate-token",
		})

		err := cmd.Execute()
		assert.NilError(t, err)

		assert.DeepEqual(t, telemetryClient.events, []telemetry.Event{
			telemetry.CreateRunnerResourceClassEvent(telemetry.CommandInfo{
				Name: "create",
				LocalArgs: map[string]string{
					"generate-token": "true",
					"help":           "false",
				},
			}),
		})
	})
}
