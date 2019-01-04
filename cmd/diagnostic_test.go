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
		tempSettings    *temporarySettings
		command         *exec.Cmd
		testServer      *ghttp.Server
		defaultEndpoint = "graphql-unstable"
	)

	BeforeEach(func() {
		tempSettings = withTempSettings()
		testServer = ghttp.NewServer()

		command = commandWithHome(pathCLI, tempSettings.home,
			"diagnostic",
			"--skip-update-check",
			"--host", testServer.URL())

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

		request := client.NewRequest(query)
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
		request = client.NewRequest(query)
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
		Expect(os.RemoveAll(tempSettings.home)).To(Succeed())
	})

	Describe("existing config file", func() {
		Describe("token set in config file", func() {
			BeforeEach(func() {
				tempSettings.writeToConfigAndClose([]byte(`token: mytoken`))
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
				tempSettings.writeToConfigAndClose([]byte(`
host: https://circleci.com/
token: mytoken
`))
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
				tempSettings.writeToConfigAndClose([]byte(`token: `))
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
				tempSettings.writeToConfigAndClose([]byte(`token: zomg`))

				command = commandWithHome(pathCLI, tempSettings.home,
					"diagnostic",
					"--skip-update-check",
					"--host", testServer.URL(),
					"--debug",
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
			command         *exec.Cmd
			testServer      *ghttp.Server
			defaultEndpoint = "graphql-unstable"
		)

		BeforeEach(func() {
			testServer = ghttp.NewServer()
			tempSettings = withTempSettings()
			tempSettings.writeToConfigAndClose([]byte(`token: mytoken`))

			command = commandWithHome(pathCLI, tempSettings.home,
				"diagnostic",
				"--skip-update-check",
				"--host", testServer.URL())

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

			request := client.NewRequest(query)
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
			request = client.NewRequest(query)
			request.SetToken("mytoken")
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
			Expect(os.RemoveAll(tempSettings.home)).To(Succeed())
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
