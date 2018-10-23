package cmd_test

import (
	"os/exec"

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
			Expect(len(commands.Commands())).To(Equal(14))
		})
	})

	Describe("build without auto update", func() {
		var (
			command  *exec.Cmd
			err      error
			buildCLI string
		)

		BeforeEach(func() {
			buildCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli",
				"-ldflags",
				"-X github.com/CircleCI-Public/circleci-cli/cmd.AutoUpdate=false -X github.com/CircleCI-Public/circleci-cli/cmd.PackageManager=homebrew",
			)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("reports update command as unavailable", func() {
			command = exec.Command(buildCLI, "help")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Err.Contents()).Should(BeEmpty())

			Eventually(session.Out).Should(gbytes.Say("update      This command is unavailable on your platform"))

			Eventually(session).Should(gexec.Exit(0))
		})

		It("tells the user to update using their package manager", func() {
			command = exec.Command(buildCLI, "update")
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
			command  *exec.Cmd
			err      error
			buildCLI string
		)

		BeforeEach(func() {
			buildCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli",
				"-ldflags", "-X github.com/CircleCI-Public/circleci-cli/cmd.AutoUpdate=true",
			)
			Expect(err).ShouldNot(HaveOccurred())

			command = exec.Command(buildCLI, "help")
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
