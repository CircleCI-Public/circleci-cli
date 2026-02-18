package clitest

import (
	"encoding/json"
	"os"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/onsi/gomega"
)

// CompareTelemetryEvent asserts that the recorded telemetry events exactly
// match `expected`. Use this when you need strict ordering and field equality.
func CompareTelemetryEvent(settings *TempSettings, expected []telemetry.Event) {
	content, err := os.ReadFile(settings.TelemetryDestPath)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	result := []telemetry.Event{}
	err = json.Unmarshal(content, &result)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(result).To(gomega.Equal(expected))
}

// ReadTelemetryEvents reads and parses all telemetry events from the mock file.
func ReadTelemetryEvents(settings *TempSettings) []telemetry.Event {
	content, err := os.ReadFile(settings.TelemetryDestPath)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	var result []telemetry.Event
	err = json.Unmarshal(content, &result)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return result
}

// CompareTelemetryEventSubset asserts that every event in `expected` appears
// somewhere in the recorded events. Use this when the middleware also emits
// cli_command_started / cli_command_finished events alongside legacy events
// and you only want to verify the legacy event payload.
func CompareTelemetryEventSubset(settings *TempSettings, expected []telemetry.Event) {
	content, err := os.ReadFile(settings.TelemetryDestPath)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	result := []telemetry.Event{}
	err = json.Unmarshal(content, &result)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(result).To(gomega.ContainElements(expected))
}
