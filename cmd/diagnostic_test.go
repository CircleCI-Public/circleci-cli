package cmd_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Diagnostic", func() {
	var (
		config  *os.File
		command *exec.Cmd
	)

	BeforeEach(func() {
		var err error
		config, err = ioutil.TempFile("", "cmd_test")
		Expect(err).ToNot(HaveOccurred())

		command = exec.Command(pathCLI, "-c", config.Name(), "diagnostic")
	})

	AfterEach(func() {
		Expect(os.Remove(config.Name())).To(Succeed())
	})

	Context("token and endpoint set in config file", func() {
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

	Context("token not set in config file", func() {
		BeforeEach(func() {
			_, err := config.Write([]byte(`
endpoint: https://example.com
`))
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Close()).To(Succeed())

			var stdin bytes.Buffer
			stdin.Write([]byte(`mytoken`))
			command.Stdin = &stdin
		})

		It("prompt for token and print success", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out).Should(gbytes.Say("Please enter your CircleCI API token:"))
			Eventually(session.Out).Should(gbytes.Say("OK, got a token."))
			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("token set to empty string in config file", func() {
		BeforeEach(func() {
			_, err := config.Write([]byte(`
endpoint: https://example.com
token: 
`))
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Close()).To(Succeed())
		})

		It("print error", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("Please set a token!"))
			Eventually(session).Should(gexec.Exit(1))
		})
	})
})
