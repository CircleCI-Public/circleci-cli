package cmd_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Diagnostic", func() {
	var (
		tempHome   string
		command    *exec.Cmd
		testServer *ghttp.Server
	)

	BeforeEach(func() {
		var err error
		tempHome, err = ioutil.TempDir("", "circleci-cli-test-")
		Expect(err).ToNot(HaveOccurred())

		testServer = ghttp.NewServer()

		command = exec.Command(pathCLI,
			"diagnostic",
			"--endpoint", testServer.URL())

		command.Env = append(os.Environ(),
			fmt.Sprintf("HOME=%s", tempHome),
		)

		tmpBytes, err := ioutil.ReadFile(filepath.Join("testdata/diagnostic", "response.json"))
		Expect(err).ShouldNot(HaveOccurred())
		mockResponse := string(tmpBytes)

		testServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/"),
				ghttp.VerifyContentType("application/json; charset=utf-8"),
				ghttp.RespondWith(http.StatusOK, `{ "data": `+mockResponse+`}`),
			),
		)
		testServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/"),
				ghttp.VerifyContentType("application/json; charset=utf-8"),
				ghttp.RespondWith(http.StatusOK, `{ "data": { "me": { "name": "zzak" } } }`),
			),
		)
	})

	AfterEach(func() {
		testServer.Close()
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

		Describe("token set in config file", func() {
			BeforeEach(func() {
				_, err := config.Write([]byte(`token: mytoken`))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())
			})

			It("print success", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("GraphQL API address: %s", testServer.URL())))
				Eventually(session.Out).Should(gbytes.Say("OK, got a token."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("fully-qualified address from --endpoint preferred over host in config ", func() {
			BeforeEach(func() {
				_, err := config.Write([]byte(`
host: https://circleci.com/
token: mytoken
`))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())
			})

			It("print success", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("GraphQL API address: %s", testServer.URL())))
				Eventually(session.Out).Should(gbytes.Say("OK, got a token."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("empty token in config file", func() {
			BeforeEach(func() {
				_, err := config.Write([]byte(`token: `))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())
			})

			It("print error", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: please set a token"))
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("GraphQL API address: %s", testServer.URL())))
				Eventually(session).Should(gexec.Exit(255))
			})
		})

		Context("debug outputs introspection query results", func() {
			BeforeEach(func() {
				_, err := config.Write([]byte(`token: zomg`))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())

				command = exec.Command(pathCLI,
					"diagnostic",
					"--endpoint", testServer.URL(),
					"--verbose",
				)

				command.Env = append(os.Environ(),
					fmt.Sprintf("HOME=%s", tempHome),
				)
			})
			It("print success", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("Trying an introspection query on API..."))
				Eventually(session.Err).Should(gbytes.Say("Introspection query result with Schema.QueryType of QueryRoot"))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("whoami returns a user", func() {
			BeforeEach(func() {
				_, err := config.Write([]byte(`token: mytoken`))
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Close()).To(Succeed())
			})

			It("print success", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("GraphQL API address: %s", testServer.URL())))
				Eventually(session.Out).Should(gbytes.Say("OK, got a token."))
				Eventually(session.Out).Should(gbytes.Say("Hello, zzak."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})
	})
})
