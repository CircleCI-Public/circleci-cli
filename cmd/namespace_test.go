package cmd_test

import (
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Namespace integration tests", func() {
	var (
		testServer *ghttp.Server
		token      string = "testtoken"
		command    *exec.Cmd
	)

	BeforeEach(func() {
		testServer = ghttp.NewServer()
	})

	AfterEach(func() {
		testServer.Close()
	})

	Describe("registering a namespace", func() {
		BeforeEach(func() {
			command = exec.Command(pathCLI,
				"namespace", "create",
				"--token", token,
				"--host", testServer.URL(),
				"foo-ns",
				"BITBUCKET",
				"test-org",
			)
		})

		It("works with organizationName and organizationVcs", func() {
			By("setting up a mock server")

			gqlOrganizationResponse := `{
    											"organization": {
      												"name": "test-org",
      												"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
    											}
  				}`

			expectedOrganizationRequest := `{
            "query": "\n\t\t\tquery($organizationName: String!, $organizationVcs: VCSType!) {\n\t\t\t\torganization(\n\t\t\t\t\tname: $organizationName\n\t\t\t\t\tvcsType: $organizationVcs\n\t\t\t\t) {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t}",
            "variables": {
              "organizationName": "test-org",
              "organizationVcs": "BITBUCKET"
            }
          }`

			gqlNsResponse := `{
									"createNamespace": {
										"errors": [],
										"namespace": {
														"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
										}
									}
				}`

			expectedNsRequest := `{
            "query": "\n\t\t\tmutation($name: String!, $organizationId: UUID!) {\n\t\t\t\tcreateNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t\torganizationId: $organizationId\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
            "variables": {
              "name": "foo-ns",
              "organizationId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            }
          }`

			appendPostHandler(testServer, token, MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedOrganizationRequest,
				Response: gqlOrganizationResponse})
			appendPostHandler(testServer, token, MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedNsRequest,
				Response: gqlNsResponse})

			By("running the command")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Out).Should(gbytes.Say("Namespace `foo-ns` created."))
			Eventually(session.Out).Should(gbytes.Say("Please note that any orbs you publish in this namespace are open orbs and are world-readable."))
			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Describe("when creating / reserving a namespace", func() {
		BeforeEach(func() {
			command = exec.Command(pathCLI,
				"namespace", "create",
				"--token", token,
				"--host", testServer.URL(),
				"foo-ns",
				"BITBUCKET",
				"test-org",
			)
		})

		It("works with organizationName and organizationVcs", func() {
			By("setting up a mock server")

			gqlOrganizationResponse := `{
    											"organization": {
      												"name": "test-org",
      												"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
    											}
  				}`

			expectedOrganizationRequest := `{
            "query": "\n\t\t\tquery($organizationName: String!, $organizationVcs: VCSType!) {\n\t\t\t\torganization(\n\t\t\t\t\tname: $organizationName\n\t\t\t\t\tvcsType: $organizationVcs\n\t\t\t\t) {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t}",
            "variables": {
              "organizationName": "test-org",
              "organizationVcs": "BITBUCKET"
            }
          }`

			gqlNsResponse := `{
									"createNamespace": {
										"errors": [],
										"namespace": {
														"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
										}
									}
				}`

			expectedNsRequest := `{
            "query": "\n\t\t\tmutation($name: String!, $organizationId: UUID!) {\n\t\t\t\tcreateNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t\torganizationId: $organizationId\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
            "variables": {
              "name": "foo-ns",
              "organizationId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            }
          }`

			appendPostHandler(testServer, token, MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedOrganizationRequest,
				Response: gqlOrganizationResponse})
			appendPostHandler(testServer, token, MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedNsRequest,
				Response: gqlNsResponse})

			By("running the command")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Out).Should(gbytes.Say("Namespace `foo-ns` created."))
			Eventually(session.Out).Should(gbytes.Say("Please note that any orbs you publish in this namespace are open orbs and are world-readable."))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("prints all in-band errors returned by the GraphQL API", func() {
			By("setting up a mock server")

			gqlOrganizationResponse := `{
    											"organization": {
      												"name": "test-org",
      												"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
    											}
  				}`

			expectedOrganizationRequest := `{
            "query": "\n\t\t\tquery($organizationName: String!, $organizationVcs: VCSType!) {\n\t\t\t\torganization(\n\t\t\t\t\tname: $organizationName\n\t\t\t\t\tvcsType: $organizationVcs\n\t\t\t\t) {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t}",
            "variables": {
              "organizationName": "test-org",
              "organizationVcs": "BITBUCKET"
            }
          }`

			gqlResponse := `{
									"createNamespace": {
										"errors": [
													{"message": "error1"},
													{"message": "error2"}
								  					],
										"namespace": null
									}
								}`

			gqlNativeErrors := `[ { "message": "ignored error" } ]`

			expectedRequestJSON := `{
            			"query": "\n\t\t\tmutation($name: String!, $organizationId: UUID!) {\n\t\t\t\tcreateNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t\torganizationId: $organizationId\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
            			"variables": {
              			"name": "foo-ns",
						"organizationId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            			}
          		}`

			appendPostHandler(testServer, token,
				MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrganizationRequest,
					Response: gqlOrganizationResponse,
				})
			appendPostHandler(testServer, token,
				MockRequestResponse{
					Status:        http.StatusOK,
					Request:       expectedRequestJSON,
					Response:      gqlResponse,
					ErrorResponse: gqlNativeErrors,
				})

			By("running the command")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("Error: error1: error2"))
			Eventually(session).ShouldNot(gexec.Exit(0))
		})
	})

})
