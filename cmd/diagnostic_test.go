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

var _ = Describe("Diagnostic", func() {
	var (
		tempHome string
		command  *exec.Cmd
	)

	BeforeEach(func() {
		var err error
		tempHome, err = ioutil.TempDir("", "circleci-cli-test-")
		Expect(err).ToNot(HaveOccurred())

		command = exec.Command(pathCLI, "diagnostic")
		command.Env = append(os.Environ(),
			fmt.Sprintf("HOME=%s", tempHome),
		)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempHome)).To(Succeed())
	})

	Describe("existing config file", func() {
		var config *os.File

		BeforeEach(func() {
			const (
				configDir  = ".circleci"
				configFile = "cli.yml"
			)

			Expect(os.Mkdir(filepath.Join(tempHome, configDir), 0700)).To(Succeed())

			var err error
			config, err = os.OpenFile(
				filepath.Join(tempHome, configDir, configFile),
				os.O_RDWR|os.O_CREATE,
				0600,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("token and endpoint set in config file", func() {
			BeforeEach(func() {
				_, err := config.Write([]byte(`
endpoint: https://example.com/graphql
token: mytoken
`))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())
			})

			It("print success", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out).Should(gbytes.Say("GraphQL API endpoint: https://example.com/graphql"))
				Eventually(session.Out).Should(gbytes.Say("OK, got a token."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("token set to empty string in config file", func() {
			BeforeEach(func() {
				_, err := config.Write([]byte(`
endpoint: https://example.com/graphql
token: 
`))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())
			})

			It("print error", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: please set a token"))
				Eventually(session.Out).Should(gbytes.Say("GraphQL API endpoint: https://example.com/graphql"))
				Eventually(session).Should(gexec.Exit(1))
			})
		})
	})
})
