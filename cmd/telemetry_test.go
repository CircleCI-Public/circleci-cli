package cmd_test

import (
	"fmt"
	"os/exec"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Telemetry events on telemetry commands", func() {
	var (
		tempSettings *clitest.TempSettings
		command      *exec.Cmd
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Describe("telemetry enable", func() {
		It("should send an event", func() {
			command = exec.Command(pathCLI, "telemetry", "enable")
			command.Env = append(command.Env, fmt.Sprintf("MOCK_TELEMETRY=%s", tempSettings.TelemetryDestPath))

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			clitest.CompareTelemetryEvent(tempSettings, []telemetry.Event{
				telemetry.CreateChangeTelemetryStatusEvent("enabled", "telemetry-command", nil),
			})
		})
	})

	Describe("telemetry disable", func() {
		It("should send an event", func() {
			command = exec.Command(pathCLI, "telemetry", "disable")
			command.Env = append(command.Env, fmt.Sprintf("MOCK_TELEMETRY=%s", tempSettings.TelemetryDestPath))

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			clitest.CompareTelemetryEvent(tempSettings, []telemetry.Event{
				telemetry.CreateChangeTelemetryStatusEvent("disabled", "telemetry-command", nil),
			})
		})
	})
})
