package cmd_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/cmd"
)

var _ = Describe("Root", func() {
	Describe("subcommands", func() {
		It("can create commands", func() {
			commands := cmd.MakeCommands()
			Expect(len(commands.Commands())).To(Equal(23))
		})
	})

	Describe("build without auto update", func() {
		var (
			command      *exec.Cmd
			err          error
			noUpdateCLI  string
			tempSettings *clitest.TempSettings
		)

		BeforeEach(func() {
			tempSettings = clitest.WithTempSettings()

			noUpdateCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli",
				"-ldflags",
				"-X github.com/CircleCI-Public/circleci-cli/version.packageManager=homebrew",
			)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			tempSettings.Close()
		})

		It("reports update command as unavailable", func() {
			command = commandWithHome(noUpdateCLI, tempSettings.Home,
				"help", "--skip-update-check",
			)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Err.Contents()).Should(BeEmpty())

			Eventually(session.Out).Should(gbytes.Say("update      This command is unavailable on your platform"))

			Eventually(session).Should(gexec.Exit(0))
		})

		It("tells the user to update using their package manager", func() {
			command = exec.Command(noUpdateCLI, "update")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Err.Contents()).Should(BeEmpty())

			Eventually(session.Out).Should(gbytes.Say("`update` is not available because this tool was installed using `homebrew`."))
			Eventually(session.Out).Should(gbytes.Say("Please consult the package manager's documentation on how to update the CLI."))
			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Describe("build with auto update", func() {
		var (
			command   *exec.Cmd
			err       error
			updateCLI string
		)

		BeforeEach(func() {
			updateCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli")
			Expect(err).ShouldNot(HaveOccurred())

			command = exec.Command(updateCLI, "help",
				"--skip-update-check",
			)
		})

		It("does include the update command in help text", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Err.Contents()).Should(BeEmpty())

			Eventually(session.Out).Should(gbytes.Say("update      Update the tool to the latest version"))

			Eventually(session).Should(gexec.Exit(0))
		})
	})

})
