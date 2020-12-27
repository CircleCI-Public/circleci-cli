package cmd_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"gotest.tools/v3/golden"
)

var _ = Describe("Diagnostic", func() {
	var (
		tempSettings    *clitest.TempSettings
		command         *exec.Cmd
		defaultEndpoint = "graphql-unstable"
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()

		command = commandWithHome(pathCLI, tempSettings.Home,
			"diagnostic",
			"--skip-update-check",
			"--host", tempSettings.TestServer.URL())

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

		request := graphql.NewRequest(query)
		expected, err := request.Encode()
		Expect(err).ShouldNot(HaveOccurred())

		tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("diagnostic/response.json"))
		mockResponse := string(tmpBytes)

		tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
			Status:   http.StatusOK,
			Request:  expected.String(),
			Response: mockResponse})

		// Stub any "me" queries regardless of token
		query = `query { me { name } }`
		request = graphql.NewRequest(query)
		expected, err = request.Encode()
		Expect(err).ShouldNot(HaveOccurred())

		response := `{ "me": { "name": "zzak" } }`

		tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
			Status:   http.StatusOK,
			Request:  expected.String(),
			Response: response})
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Describe("existing config file", func() {
		Describe("token set in config file", func() {
			BeforeEach(func() {
				tempSettings.Config.Write([]byte(`token: mytoken`))
			})

			It("print success", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("API host: %s", tempSettings.TestServer.URL())))
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("API endpoint: %s", defaultEndpoint)))
				Eventually(session.Out).Should(gbytes.Say("OK, got a token."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("fully-qualified address from --endpoint preferred over host in config ", func() {
			BeforeEach(func() {
				tempSettings.Config.Write([]byte(`
host: https://circleci.com/
token: mytoken
`))
			})

			It("print success", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("API host: %s", tempSettings.TestServer.URL())))
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("API endpoint: %s", defaultEndpoint)))
				Eventually(session.Out).Should(gbytes.Say("OK, got a token."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("empty token in config file", func() {
			BeforeEach(func() {
				tempSettings.Config.Write([]byte(`token: `))
			})

			It("print error", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: please set a token with 'circleci setup'"))
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("API host: %s", tempSettings.TestServer.URL())))
				Eventually(session.Out).Should(gbytes.Say(
					fmt.Sprintf("API endpoint: %s", defaultEndpoint)))
				Eventually(session).Should(clitest.ShouldFail())
			})
		})

		Context("debug outputs introspection query results", func() {
			BeforeEach(func() {
				tempSettings.Config.Write([]byte(`token: zomg`))

				command = commandWithHome(pathCLI, tempSettings.Home,
					"diagnostic",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
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
			defaultEndpoint = "graphql-unstable"
		)

		BeforeEach(func() {
			tempSettings = clitest.WithTempSettings()
			tempSettings.Config.Write([]byte(`token: mytoken`))

			command = commandWithHome(pathCLI, tempSettings.Home,
				"diagnostic",
				"--skip-update-check",
				"--host", tempSettings.TestServer.URL())

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

			request := graphql.NewRequest(query)
			expected, err := request.Encode()
			Expect(err).ShouldNot(HaveOccurred())

			tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("diagnostic/response.json"))
			mockResponse := string(tmpBytes)

			tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expected.String(),
				Response: mockResponse})

			// Here we want to actually validate the token in our test too
			query = `query { me { name } }`
			request = graphql.NewRequest(query)
			request.SetToken("mytoken")
			Expect(err).ShouldNot(HaveOccurred())
			expected, err = request.Encode()
			Expect(err).ShouldNot(HaveOccurred())

			response := `{ "me": { "name": "zzak" } }`

			tempSettings.AppendPostHandler("mytoken", clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expected.String(),
				Response: response})
		})

		AfterEach(func() {
			tempSettings.Close()
		})

		It("print success", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out).Should(gbytes.Say(
				fmt.Sprintf("API host: %s", tempSettings.TestServer.URL())))
			Eventually(session.Out).Should(gbytes.Say(
				fmt.Sprintf("API endpoint: %s", defaultEndpoint)))
			Eventually(session.Out).Should(gbytes.Say("OK, got a token."))
			Eventually(session.Out).Should(gbytes.Say("Hello, zzak."))
			Eventually(session).Should(gexec.Exit(0))
		})
	})
})
