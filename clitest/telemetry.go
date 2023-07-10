package clitest

import (
	"encoding/json"
	"os"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/onsi/gomega"
)

func CompareTelemetryEvent(filePath string, expected []telemetry.Event) {
	content, err := os.ReadFile(filePath)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	result := []telemetry.Event{}
	err = json.Unmarshal(content, &result)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(result).To(gomega.Equal(expected))
}
