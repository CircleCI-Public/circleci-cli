package cmd_test

import (
	"fmt"
	"os/exec"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Version telemetry", func() {
	var (
		command      *exec.Cmd
		tempSettings *clitest.TempSettings
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()
		command = commandWithHome(pathCLI, tempSettings.Home, "version")
		command.Env = append(command.Env, fmt.Sprintf("MOCK_TELEMETRY=%s", tempSettings.TelemetryDestPath))
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	It("should send a telemetry event", func() {
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))
		clitest.CompareTelemetryEvent(tempSettings, []telemetry.Event{
			telemetry.CreateVersionEvent("0.0.0-dev+dirty-local-tree (source)"),
		})
	})
})
