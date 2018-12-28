package cmd_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

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
				err        error
				tempHome   string
				command    *exec.Cmd
				config     *os.File
				configDir  = ".circleci"
				configFile = "cli.yml"
			)

			BeforeEach(func() {
				command = exec.Command(pathCLI, "--help")

				tempHome, err = ioutil.TempDir("", "circleci-cli-test-")
				Expect(err).ToNot(HaveOccurred())

				command.Env = append(os.Environ(),
					fmt.Sprintf("HOME=%s", tempHome),
					fmt.Sprintf("USERPROFILE=%s", tempHome), // windows
				)

				Expect(os.Mkdir(filepath.Join(tempHome, configDir), 0700)).To(Succeed())

				config, err = os.OpenFile(
					filepath.Join(tempHome, configDir, configFile),
					os.O_RDWR|os.O_CREATE,
					0600,
				)
				Expect(err).ToNot(HaveOccurred())
				_, err = config.Write([]byte(`host: foo.bar`))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(tempHome)).To(Succeed())
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
			command     *exec.Cmd
			err         error
			noUpdateCLI string
			tempHome    string
		)

		BeforeEach(func() {
			tempHome, _, _ = withTempSettings()

			noUpdateCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli",
				"-ldflags",
				"-X github.com/CircleCI-Public/circleci-cli/cmd.PackageManager=homebrew",
			)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tempHome)).To(Succeed())
		})

		It("reports update command as unavailable", func() {
			command = exec.Command(noUpdateCLI, "help",
				"--skip-update-check",
			)
			command.Env = append(os.Environ(),
				fmt.Sprintf("HOME=%s", tempHome),
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
			command  *exec.Cmd
			tempHome string
		)

		const (
			configDir  = ".circleci"
			configFile = "cli.yml"
		)

		BeforeEach(func() {
			var err error
			tempHome, err = ioutil.TempDir("", "circleci-cli-test-")
			Expect(err).ToNot(HaveOccurred())

			command = exec.Command(pathCLI, "help",
				"--skip-update-check",
			)
			command.Env = append(os.Environ(),
				fmt.Sprintf("HOME=%s", tempHome),
				fmt.Sprintf("USERPROFILE=%s", tempHome), // windows
			)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tempHome)).To(Succeed())
		})

		Describe("existing config file", func() {
			var config *os.File

			BeforeEach(func() {
				Expect(os.Mkdir(filepath.Join(tempHome, configDir), 0700)).To(Succeed())

				var err error
				config, err = os.OpenFile(
					filepath.Join(tempHome, configDir, configFile),
					os.O_RDWR|os.O_CREATE,
					0600,
				)
				Expect(err).ToNot(HaveOccurred())

				_, err = config.Write([]byte(`token: secret`))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())
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
