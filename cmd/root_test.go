package cmd_test

import (
	"os/exec"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/cmd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Root", func() {
	Describe("subcommands", func() {
		It("can create commands", func() {
			commands := cmd.MakeCommands()
			Expect(len(commands.Commands())).To(Equal(15))
		})
	})

	Describe("Help text", func() {
		It("shows a link to the docs", func() {
			command := exec.Command(pathCLI, "--help")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Out).Should(gbytes.Say(`Use CircleCI from the command line.

This project is the seed for CircleCI's new command-line application.

For more help, see the documentation here: https://circleci.com/docs/2.0/local-cli/
`))
			Eventually(session).Should(gexec.Exit(0))
		})

		Context("if user changes host settings through configuration", func() {
			var (
				tempSettings *clitest.TempSettings
				command      *exec.Cmd
			)

			BeforeEach(func() {
				tempSettings = clitest.WithTempSettings()

				command = commandWithHome(pathCLI, tempSettings.Home, "--help")

				tempSettings.Config.Write([]byte(`host: foo.bar`))
			})

			AfterEach(func() {
				tempSettings.Cleanup()
			})

			It("doesn't link to docs if user changes --host", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())

				Consistently(session.Out).ShouldNot(gbytes.Say("For more help, see the documentation here: https://circleci.com/docs/2.0/local-cli/"))
				Eventually(session).Should(gexec.Exit(0))
			})
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
				"-X github.com/CircleCI-Public/circleci-cli/cmd.PackageManager=homebrew",
			)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			tempSettings.Cleanup()
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

	Describe("token in help text", func() {
		var (
			command      *exec.Cmd
			tempSettings *clitest.TempSettings
		)

		BeforeEach(func() {
			tempSettings = clitest.WithTempSettings()

			command = commandWithHome(pathCLI, tempSettings.Home,
				"help", "--skip-update-check",
			)
		})

		AfterEach(func() {
			tempSettings.Cleanup()
		})

		Describe("existing config file", func() {
			BeforeEach(func() {
				tempSettings.Config.Write([]byte(`token: secret`))
			})

			It("does not include the users token in help text", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())

				Ω(session.Wait().Out.Contents()).ShouldNot(ContainSubstring("your token for using CircleCI (default \"secret\")"))
			})
		})
	})
})
