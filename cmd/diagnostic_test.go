package cmd_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/CircleCI-Public/circleci-cli/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Diagnostic", func() {
	var (
		tempHome        string
		command         *exec.Cmd
		testServer      *ghttp.Server
		defaultEndpoint = "graphql-unstable"
	)

	BeforeEach(func() {
		var err error
		tempHome, err = ioutil.TempDir("", "circleci-cli-test-")
		Expect(err).ToNot(HaveOccurred())

		testServer = ghttp.NewServer()

		command = exec.Command(pathCLI,
			"diagnostic",
			"--skip-update-check",
			"--host", testServer.URL())

		command.Env = append(os.Environ(),
			fmt.Sprintf("HOME=%s", tempHome),
		)

		query := `query IntrospectionQuery {
		    __schema {
		      queryType { name }
		      mutationType { name }
		      types {
		        ...FullType
		      }
		    }
		  }

		  fragment FullType on __Type {
		    kind
		    name
		    description
		    fields(includeDeprecated: true) {
		      name
		    }
		  }`

		request := client.NewUnauthorizedRequest(query)
		expected, err := request.Encode()
		Expect(err).ShouldNot(HaveOccurred())

		tmpBytes, err := ioutil.ReadFile(filepath.Join("testdata/diagnostic", "response.json"))
		Expect(err).ShouldNot(HaveOccurred())
		mockResponse := string(tmpBytes)

		appendPostHandler(testServer, "", MockRequestResponse{
			Status:   http.StatusOK,
			Request:  expected.String(),
			Response: mockResponse})

		// Stub any "me" queries regardless of token
		query = `query { me { name } }`
		request = client.NewUnauthorizedRequest(query)
		expected, err = request.Encode()
		Expect(err).ShouldNot(HaveOccurred())

		response := `{ "me": { "name": "zzak" } }`

		appendPostHandler(testServer, "", MockRequestResponse{
			Status:   http.StatusOK,
			Request:  expected.String(),
			Response: response})
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
					fmt.Sprintf("API host: %s", testServer.URL())))
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("API endpoint: %s", defaultEndpoint)))
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
					fmt.Sprintf("API host: %s", testServer.URL())))
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("API endpoint: %s", defaultEndpoint)))
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
				Eventually(session.Err).Should(gbytes.Say("Error: please set a token with 'circleci setup'"))
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("API host: %s", testServer.URL())))
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("API endpoint: %s", defaultEndpoint)))
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
					"--skip-update-check",
					"--host", testServer.URL(),
					"--debug",
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
	})

	Describe("whoami returns a user", func() {
		var (
			tempHome        string
			command         *exec.Cmd
			testServer      *ghttp.Server
			defaultEndpoint = "graphql-unstable"
			config          *os.File
		)

		BeforeEach(func() {
			var err error
			tempHome, err = ioutil.TempDir("", "circleci-cli-test-")
			Expect(err).ToNot(HaveOccurred())

			testServer = ghttp.NewServer()

			const (
				configDir  = ".circleci"
				configFile = "cli.yml"
			)

			Expect(os.Mkdir(filepath.Join(tempHome, configDir), 0700)).To(Succeed())

			config, err = os.OpenFile(
				filepath.Join(tempHome, configDir, configFile),
				os.O_RDWR|os.O_CREATE,
				0600,
			)
			Expect(err).ToNot(HaveOccurred())

			_, err = config.Write([]byte(`token: mytoken`))
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Close()).To(Succeed())

			command = exec.Command(pathCLI,
				"diagnostic",
				"--skip-update-check",
				"--host", testServer.URL())

			command.Env = append(os.Environ(),
				fmt.Sprintf("HOME=%s", tempHome),
			)

			query := `query IntrospectionQuery {
		    __schema {
		      queryType { name }
		      mutationType { name }
		      types {
		        ...FullType
		      }
		    }
		  }

		  fragment FullType on __Type {
		    kind
		    name
		    description
		    fields(includeDeprecated: true) {
		      name
		    }
		  }`

			request := client.NewUnauthorizedRequest(query)
			expected, err := request.Encode()
			Expect(err).ShouldNot(HaveOccurred())

			tmpBytes, err := ioutil.ReadFile(filepath.Join("testdata/diagnostic", "response.json"))
			Expect(err).ShouldNot(HaveOccurred())
			mockResponse := string(tmpBytes)

			appendPostHandler(testServer, "", MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expected.String(),
				Response: mockResponse})

			// Here we want to actually validate the token in our test too
			query = `query { me { name } }`
			request, err = client.NewAuthorizedRequest(query, "mytoken")
			Expect(err).ShouldNot(HaveOccurred())
			expected, err = request.Encode()
			Expect(err).ShouldNot(HaveOccurred())

			response := `{ "me": { "name": "zzak" } }`

			appendPostHandler(testServer, "mytoken", MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expected.String(),
				Response: response})
		})

		AfterEach(func() {
			testServer.Close()
			Expect(os.RemoveAll(tempHome)).To(Succeed())
		})

		It("print success", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out).Should(gbytes.Say(
				fmt.Sprintf("API host: %s", testServer.URL())))
			Eventually(session.Out).Should(gbytes.Say(
				fmt.Sprintf("API endpoint: %s", defaultEndpoint)))
			Eventually(session.Out).Should(gbytes.Say("OK, got a token."))
			Eventually(session.Out).Should(gbytes.Say("Hello, zzak."))
			Eventually(session).Should(gexec.Exit(0))
		})
	})
})
