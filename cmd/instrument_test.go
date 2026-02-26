package cmd_test

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Command instrumentation middleware", func() {
	var (
		tempSettings *clitest.TempSettings
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	It("emits cli_command_started and cli_command_finished with matching invocation_id", func() {
		command := commandWithHome(pathCLI, tempSettings.Home, "version")
		command.Env = append(command.Env, fmt.Sprintf("MOCK_TELEMETRY=%s", tempSettings.TelemetryDestPath))

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(session).Should(gexec.Exit(0))

		events := clitest.ReadTelemetryEvents(tempSettings)

		var started, finished map[string]interface{}
		for _, e := range events {
			if e.Object == "cli_command_started" {
				started = e.Properties
			}
			if e.Object == "cli_command_finished" {
				finished = e.Properties
			}
		}

		Expect(started).NotTo(BeNil(), "expected cli_command_started event")
		Expect(finished).NotTo(BeNil(), "expected cli_command_finished event")

		Expect(started["command_path"]).To(Equal("circleci version"))
		Expect(finished["command_path"]).To(Equal("circleci version"))

		Expect(started["invocation_id"]).NotTo(BeEmpty())
		Expect(started["invocation_id"]).To(Equal(finished["invocation_id"]))

		Expect(finished["outcome"]).To(Equal("success"))
		Expect(finished["duration_ms"]).NotTo(BeNil())
	})

	It("emits flags_used as a map with flag values, redacting sensitive flags", func() {
		command := commandWithHome(pathCLI, tempSettings.Home, "version", "--skip-update-check", "--token=fake-secret-token")
		command.Env = append(command.Env, fmt.Sprintf("MOCK_TELEMETRY=%s", tempSettings.TelemetryDestPath))

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(session).Should(gexec.Exit(0))

		events := clitest.ReadTelemetryEvents(tempSettings)

		for _, e := range events {
			if e.Object == "cli_command_started" || e.Object == "cli_command_finished" {
				flags, ok := e.Properties["flags_used"].(map[string]interface{})
				if ok {
					Expect(flags).To(HaveKey("skip-update-check"))
					Expect(flags["skip-update-check"]).To(Equal("true"))

					Expect(flags).To(HaveKey("token"))
					Expect(flags["token"]).To(Equal("[REDACTED]"))
				}
			}
		}
	})

	It("emits args_error outcome for too many arguments", func() {
		command := commandWithHome(pathCLI, tempSettings.Home,
			"telemetry", "enable", "extra-arg")
		command.Env = append(command.Env, fmt.Sprintf("MOCK_TELEMETRY=%s", tempSettings.TelemetryDestPath))

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(session).Should(clitest.ShouldFail())

		events := clitest.ReadTelemetryEvents(tempSettings)

		var finished map[string]interface{}
		for _, e := range events {
			if e.Object == "cli_command_finished" {
				finished = e.Properties
			}
		}

		Expect(finished).NotTo(BeNil(), "expected cli_command_finished event")
		Expect(finished["outcome"]).To(Equal("args_error"))
	})
})
