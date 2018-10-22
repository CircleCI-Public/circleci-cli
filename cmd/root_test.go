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
				"-ldflags", "-X github.com/CircleCI-Public/circleci-cli/cmd.AutoUpdate=false",
			)
			Expect(err).ShouldNot(HaveOccurred())

			command = exec.Command(buildCLI, "help")
		})

		It("doesn't include the update command in help text", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Err.Contents()).Should(BeEmpty())

			Consistently(session.Out).ShouldNot(gbytes.Say("update      Update the tool to the latest version"))

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
