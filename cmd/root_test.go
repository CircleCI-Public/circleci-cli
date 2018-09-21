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

		Context("when run with insufficient arguments", func() {
			It("returns exit code 255 and prints the subcommand's help body", func() {
				command := exec.Command(pathCLI, "orb", "publish", "dev")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("Usage:"))
				Eventually(session.Out).Should(gbytes.Say("\\s*circleci orb publish dev PATH NAMESPACE ORB LABEL \\[flags\\]"))
				Eventually(session.Err).Should(gbytes.Say("Error: accepts 4 arg\\(s\\), received 0"))
				Eventually(session).Should(gexec.Exit(255))
			})
		})

		Context("when a subcommand is run with invalid flags", func() {
			It("returns exit code 255 and prints the subcommand's help body", func() {
				command := exec.Command(pathCLI, "orb", "publish", "dev", "--foo")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("Usage:"))
				Eventually(session.Out).Should(gbytes.Say("\\s*circleci orb publish dev PATH NAMESPACE ORB LABEL \\[flags\\]"))
				Eventually(session.Err).Should(gbytes.Say("unknown flag: --foo"))
				Eventually(session).Should(gexec.Exit(255))
			})
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
