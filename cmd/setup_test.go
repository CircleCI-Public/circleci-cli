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

var _ = Describe("Setup", func() {
	var (
		tempHome string
		command  *exec.Cmd
	)

	const (
		configDir  = ".circleci"
		configFile = "cli.yml"
	)

	BeforeEach(func() {
		var err error
		tempHome, err = ioutil.TempDir("", "circleci-cli-test-")
		Expect(err).ToNot(HaveOccurred())

		command = exec.Command(pathCLI, "setup", "--testing")
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
			Eventually(session.Out).Should(gbytes.Say("Setup complete. Your configuration has been saved."))

			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session).Should(gexec.Exit(0))

			fileInfo, err := os.Stat(filepath.Join(tempHome, configDir, configFile))
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
				filepath.Join(tempHome, configDir, configFile),
				os.O_RDWR|os.O_CREATE,
				0600,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should set file permissions to 0600", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			fileInfo, err := os.Stat(filepath.Join(tempHome, configDir, configFile))
			Expect(err).ToNot(HaveOccurred())
			Expect(fileInfo.Mode().Perm().String()).To(Equal("-rw-------"))
		})

		Describe("token and endpoint set in config file", func() {

			It("print success", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())

				Eventually(session.Out).Should(gbytes.Say("CircleCI API Token"))
				Eventually(session.Out).Should(gbytes.Say("API token has been set."))
				Eventually(session.Out).Should(gbytes.Say("CircleCI Host"))
				Eventually(session.Out).Should(gbytes.Say("CircleCI host has been set."))
				Eventually(session.Out).Should(gbytes.Say("Setup complete. Your configuration has been saved."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("token set to some string in config file", func() {
			BeforeEach(func() {
				_, err := config.Write([]byte(`
endpoint: https://example.com/graphql
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
				Eventually(session.Out).Should(gbytes.Say("Setup complete. Your configuration has been saved."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})
	})
})
