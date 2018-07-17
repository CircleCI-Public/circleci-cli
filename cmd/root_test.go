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
			Expect(len(commands.Commands())).To(Equal(8))
		})

	})

	Describe("when run with --help", func() {
		It("return exit code 0 with help message", func() {
			command := exec.Command(pathCLI, "--help")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out).Should(gbytes.Say("Usage:"))
			Eventually(session).Should(gexec.Exit(0))
		})
	})
})
