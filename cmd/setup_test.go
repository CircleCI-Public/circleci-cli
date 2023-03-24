package cmd_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Setup with prompts", func() {
	var (
		command      *exec.Cmd
		tempSettings *clitest.TempSettings
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()

		command = commandWithHome(pathCLI, tempSettings.Home,
			"setup",
			"--integration-testing",
			"--skip-update-check",
		)
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Describe("new config file", func() {
		It("should set file permissions to 0600", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Out).Should(gbytes.Say("CircleCI API Token"))
			Eventually(session.Out).Should(gbytes.Say("API token has been set."))
			Eventually(session.Out).Should(gbytes.Say("CircleCI Host"))
			Eventually(session.Out).Should(gbytes.Say("CircleCI host has been set."))

			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.\n", regexp.QuoteMeta(tempSettings.Config.Path))))
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session).Should(gexec.Exit(0))

			fileInfo, err := os.Stat(tempSettings.Config.Path)
			Expect(err).ToNot(HaveOccurred())
			if runtime.GOOS != "windows" {
				Expect(fileInfo.Mode().Perm().String()).To(Equal("-rw-------"))
			}
		})
	})

	Describe("existing config file", func() {
		It("should set file permissions to 0600", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			fileInfo, err := os.Stat(tempSettings.Config.Path)
			Expect(err).ToNot(HaveOccurred())
			if runtime.GOOS != "windows" {
				Expect(fileInfo.Mode().Perm().String()).To(Equal("-rw-------"))
			}
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
				Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.\n", regexp.QuoteMeta(tempSettings.Config.Path))))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("token set to some string in config file", func() {
			BeforeEach(func() {
				tempSettings.Config.Write([]byte(`
host: https://example.com/graphql
token: fooBarBaz
`))
			})

			It("print error", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("A CircleCI token is already set. Do you want to change it"))
				Eventually(session.Out).Should(gbytes.Say("CircleCI API Token"))
				Eventually(session.Out).Should(gbytes.Say("API token has been set."))
				Eventually(session.Out).Should(gbytes.Say("CircleCI Host"))
				Eventually(session.Out).Should(gbytes.Say("CircleCI host has been set."))
				Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.\n", regexp.QuoteMeta(tempSettings.Config.Path))))
				Eventually(session).Should(gexec.Exit(0))
			})
		})
	})
})

var _ = Describe("Setup without prompts", func() {
	var (
		tempSettings *clitest.TempSettings
		command      *exec.Cmd
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()
		command = commandWithHome(pathCLI, tempSettings.Home,
			"setup",
			"--no-prompt",
			"--skip-update-check",
		)
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Context("with an existing config", func() {
		Describe("of valid settings", func() {
			BeforeEach(func() {
				tempSettings.Config.Write([]byte(`
host: https://example.com
token: fooBarBaz
`))
			})

			It("should keep the existing configuration", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("Setup has kept your existing configuration at %s.\n", regexp.QuoteMeta(tempSettings.Config.Path))))

				Context("re-open the config to check the contents", func() {
					tempSettings.AssertConfigRereadMatches(`
host: https://example.com
token: fooBarBaz
`)
				})
			})

			It("should change if provided one of flags", func() {
				command = commandWithHome(pathCLI, tempSettings.Home,
					"setup",
					"--host", "asdf",
					"--no-prompt",
					"--skip-update-check",
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())

				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(fmt.Sprintf(`Token unchanged from existing config. Use --token with --no-prompt to overwrite it.
Setup complete.
Your configuration has been saved to %s.
`, tempSettings.Config.Path)))
				Eventually(session).Should(gexec.Exit(0))

				Context("re-open the config to check the contents", func() {
					tempSettings.AssertConfigRereadMatches(`host: asdf
endpoint: graphql-unstable
token: fooBarBaz
`)
				})
			})

			It("should change only the provided token", func() {
				command = commandWithHome(pathCLI, tempSettings.Home,
					"setup",
					"--token", "asdf",
					"--no-prompt",
					"--skip-update-check",
				)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())

				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(fmt.Sprintf(`Host unchanged from existing config. Use --host with --no-prompt to overwrite it.
Setup complete.
Your configuration has been saved to %s.
`, tempSettings.Config.Path)))
				Eventually(session).Should(gexec.Exit(0))

				Context("re-open the config to check the contents", func() {
					tempSettings.AssertConfigRereadMatches(`host: https://example.com
endpoint: graphql-unstable
token: asdf
`)
				})
			})
		})

	})

	Context("with no existing config", func() {
		Context("with no host or token flags", func() {
			BeforeEach(func() {
				command = commandWithHome(pathCLI, tempSettings.Home,
					"setup",
					"--no-prompt",
					"--skip-update-check",
				)
			})

			It("Should raise an error about missing host and token flags", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(clitest.ShouldFail())

				stderr := session.Wait().Err.Contents()
				Expect(string(stderr)).To(Equal("Error: No existing host or token saved.\nThe proper format is `circleci setup --host HOST --token TOKEN --no-prompt\n"))
			})
		})

		Context("with both host and token flags", func() {
			BeforeEach(func() {
				command = commandWithHome(pathCLI, tempSettings.Home,
					"setup",
					"--host", "https://zomg.com",
					"--token", "mytoken",
					"--no-prompt",
					"--skip-update-check",
				)
			})

			It("write the configuration to a file", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(fmt.Sprintf(`Setup complete.
Your configuration has been saved to %s.
`, tempSettings.Config.Path)))

				Context("re-open the config to check the contents", func() {
					file, err := os.Open(tempSettings.Config.Path)
					Expect(err).ShouldNot(HaveOccurred())

					reread, err := io.ReadAll(file)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(string(reread)).To(Equal(`host: https://zomg.com
endpoint: graphql-unstable
token: mytoken
rest_endpoint: api/v2
tls_cert: ""
tls_insecure: false
orb_publishing:
    default_namespace: ""
    default_vcs_provider: ""
    default_owner: ""
`))
				})
			})
		})
	})
})
