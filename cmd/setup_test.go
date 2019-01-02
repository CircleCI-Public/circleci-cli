package cmd_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Setup with prompts", func() {
	var (
		tempHome   string
		command    *exec.Cmd
		configDir  = ".circleci"
		configFile = "cli.yml"
		configPath string
	)

	BeforeEach(func() {
		var err error
		tempHome, err = ioutil.TempDir("", "circleci-cli-test-")
		Expect(err).ToNot(HaveOccurred())

		configPath = filepath.Join(tempHome, configDir, configFile)

		command = exec.Command(pathCLI,
			"setup",
			"--testing",
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

	Describe("new config file", func() {
		It("should set file permissions to 0600", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Out).Should(gbytes.Say("CircleCI API Token"))
			Eventually(session.Out).Should(gbytes.Say("API token has been set."))
			Eventually(session.Out).Should(gbytes.Say("CircleCI Host"))
			Eventually(session.Out).Should(gbytes.Say("CircleCI host has been set."))

			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.\n", configPath)))
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session).Should(gexec.Exit(0))

			fileInfo, err := os.Stat(configPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileInfo.Mode().Perm().String()).To(Equal("-rw-------"))
		})
	})

	Describe("existing config file", func() {
		var config *os.File

		BeforeEach(func() {
			Expect(os.Mkdir(filepath.Join(tempHome, configDir), 0700)).To(Succeed())

			var err error

			config, err = os.OpenFile(
				configPath,
				os.O_RDWR|os.O_CREATE,
				0600,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should set file permissions to 0600", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			fileInfo, err := os.Stat(configPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileInfo.Mode().Perm().String()).To(Equal("-rw-------"))
		})

		Describe("token and host set in config file", func() {

			It("print success", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())

				Eventually(session.Out).Should(gbytes.Say("CircleCI API Token"))
				Eventually(session.Out).Should(gbytes.Say("API token has been set."))
				Eventually(session.Out).Should(gbytes.Say("CircleCI Host"))
				Eventually(session.Out).Should(gbytes.Say("CircleCI host has been set."))
				Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.\n", configPath)))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("token set to some string in config file", func() {
			BeforeEach(func() {
				_, err := config.Write([]byte(`
host: https://example.com/graphql
token: fooBarBaz
`))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())
			})

			It("print error", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("A CircleCI token is already set. Do you want to change it"))
				Eventually(session.Out).Should(gbytes.Say("CircleCI API Token"))
				Eventually(session.Out).Should(gbytes.Say("API token has been set."))
				Eventually(session.Out).Should(gbytes.Say("CircleCI Host"))
				Eventually(session.Out).Should(gbytes.Say("CircleCI host has been set."))
				Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.\n", configPath)))
				Eventually(session).Should(gexec.Exit(0))
			})
		})
	})
})

var _ = Describe("Setup without prompts", func() {
	Context("with an existing config", func() {
		var (
			tempHome   string
			command    *exec.Cmd
			config     *os.File
			configDir  = ".circleci"
			configFile = "cli.yml"
			configPath string
		)

		BeforeEach(func() {
			var err error
			tempHome, err = ioutil.TempDir("", "circleci-cli-test-")
			Expect(err).ToNot(HaveOccurred())

			command = exec.Command(pathCLI,
				"setup",
				"--no-prompt",
				"--skip-update-check",
			)
			command.Env = append(os.Environ(),
				fmt.Sprintf("HOME=%s", tempHome),
				fmt.Sprintf("USERPROFILE=%s", tempHome), // windows
			)

			Expect(os.Mkdir(filepath.Join(tempHome, configDir), 0700)).To(Succeed())

			configPath = filepath.Join(tempHome, configDir, configFile)
			config, err = os.OpenFile(
				configPath,
				os.O_RDWR|os.O_CREATE,
				0600,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tempHome)).To(Succeed())
		})

		Describe("of valid settings", func() {
			BeforeEach(func() {
				_, err := config.Write([]byte(`
host: https://example.com
token: fooBarBaz
`))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())
			})

			It("should keep the existing configuration", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Setup has kept your existing configuration at %s.\n", configPath)))

				Context("re-open the config to check the contents", func() {
					file, err := os.Open(configPath)
					Expect(err).ShouldNot(HaveOccurred())

					reread, err := ioutil.ReadAll(file)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(string(reread)).To(Equal(`
host: https://example.com
token: fooBarBaz
`))
				})
			})

			It("should change if provided one of flags", func() {
				command = exec.Command(pathCLI,
					"setup",
					"--host", "asdf",
					"--no-prompt",
					"--skip-update-check",
				)
				command.Env = append(os.Environ(),
					fmt.Sprintf("HOME=%s", tempHome),
					fmt.Sprintf("USERPROFILE=%s", tempHome), // windows
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())

				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(fmt.Sprintf(`No token saved. You didn't specify a --token to use with --no-prompt.
Setup complete.
Your configuration has been saved to %s.
`, configPath)))
				Eventually(session).Should(gexec.Exit(0))

				Context("re-open the config to check the contents", func() {
					file, err := os.Open(configPath)
					Expect(err).ShouldNot(HaveOccurred())

					reread, err := ioutil.ReadAll(file)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(string(reread)).To(Equal(`host: asdf
endpoint: graphql-unstable
token: fooBarBaz
`))
				})
			})
		})

	})
})
