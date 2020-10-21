package cmd_test

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Namespace integration tests", func() {
	var (
		tempSettings *clitest.TempSettings
		token        string = "testtoken"
		command      *exec.Cmd
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Context("create, skipping prompts", func() {
		Describe("registering a namespace", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"namespace", "create",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--no-prompt",
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
            "query": "query($organizationName: String!, $organizationVcs: VCSType!) {\n\t\t\t\torganization(\n\t\t\t\t\tname: $organizationName\n\t\t\t\t\tvcsType: $organizationVcs\n\t\t\t\t) {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t}","variables":{"organizationName":"test-org","organizationVcs":"BITBUCKET"}}`

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

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrganizationRequest,
					Response: gqlOrganizationResponse})
				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
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
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--no-prompt",
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
            "query": "query($organizationName: String!, $organizationVcs: VCSType!) {\n\t\t\t\torganization(\n\t\t\t\t\tname: $organizationName\n\t\t\t\t\tvcsType: $organizationVcs\n\t\t\t\t) {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t}",
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

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrganizationRequest,
					Response: gqlOrganizationResponse})
				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
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
            "query": "query($organizationName: String!, $organizationVcs: VCSType!) {\n\t\t\t\torganization(\n\t\t\t\t\tname: $organizationName\n\t\t\t\t\tvcsType: $organizationVcs\n\t\t\t\t) {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t}",
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

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrganizationRequest,
					Response: gqlOrganizationResponse,
				})
				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:        http.StatusOK,
					Request:       expectedRequestJSON,
					Response:      gqlResponse,
					ErrorResponse: gqlNativeErrors,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})
	})

	Context("create, with interactive prompts", func() {
		Describe("registering a namespace", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"namespace", "create",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--integration-testing",
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
            "query": "query($organizationName: String!, $organizationVcs: VCSType!) {\n\t\t\t\torganization(\n\t\t\t\t\tname: $organizationName\n\t\t\t\t\tvcsType: $organizationVcs\n\t\t\t\t) {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t}","variables":{"organizationName":"test-org","organizationVcs":"BITBUCKET"}}`

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

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrganizationRequest,
					Response: gqlOrganizationResponse})
				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedNsRequest,
					Response: gqlNsResponse})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				stdout := session.Wait().Out.Contents()

				Expect(string(stdout)).To(ContainSubstring(fmt.Sprintf(`You are creating a namespace called "%s".

This is the only namespace permitted for your bitbucket organization, test-org.

To change the namespace, you will have to contact CircleCI customer support.

Are you sure you wish to create the namespace: %s
Namespace %s created.
Please note that any orbs you publish in this namespace are open orbs and are world-readable.`, "foo-ns", "`foo-ns`", "`foo-ns`")))
			})
		})

		Describe("when creating / reserving a namespace", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"namespace", "create",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--integration-testing",
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
            "query": "query($organizationName: String!, $organizationVcs: VCSType!) {\n\t\t\t\torganization(\n\t\t\t\t\tname: $organizationName\n\t\t\t\t\tvcsType: $organizationVcs\n\t\t\t\t) {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t}",
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

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrganizationRequest,
					Response: gqlOrganizationResponse})
				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedNsRequest,
					Response: gqlNsResponse})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				stdout := session.Wait().Out.Contents()

				Expect(string(stdout)).To(ContainSubstring(fmt.Sprintf(`You are creating a namespace called "%s".

This is the only namespace permitted for your bitbucket organization, test-org.

To change the namespace, you will have to contact CircleCI customer support.

Are you sure you wish to create the namespace: %s
Namespace %s created.
Please note that any orbs you publish in this namespace are open orbs and are world-readable.`, "foo-ns", "`foo-ns`", "`foo-ns`")))
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
            "query": "query($organizationName: String!, $organizationVcs: VCSType!) {\n\t\t\t\torganization(\n\t\t\t\t\tname: $organizationName\n\t\t\t\t\tvcsType: $organizationVcs\n\t\t\t\t) {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t}",
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

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrganizationRequest,
					Response: gqlOrganizationResponse,
				})
				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:        http.StatusOK,
					Request:       expectedRequestJSON,
					Response:      gqlResponse,
					ErrorResponse: gqlNativeErrors,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})
	})

	Describe("renaming a namespace", func() {
		var (
			gqlGetNsResponse string
			expectedGetNsRequest string
			expectedRenameRequest string
		)
		BeforeEach(func () {
			command = exec.Command(pathCLI,
				"namespace", "rename",
				"ns-0", "ns-1",
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
				"--no-prompt",
			)
			gqlGetNsResponse = `{
					"errors": [],
					"registryNamespace": {
						"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
					}
				}`
			expectedGetNsRequest = `{
				"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
				"variables": {
					"name": "ns-0"
				}
			}`
			expectedRenameRequest = `{
		"query": "\n\t\tmutation($namespaceId: UUID!, $newName: String!){\n\t\t\trenameNamespace(\n\t\t\t\tnamespaceId: $namespaceId,\n\t\t\t\tnewName: $newName\n\t\t\t){\n\t\t\t\tnamespace {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t\terrors {\n\t\t\t\t\tmessage\n\t\t\t\t\ttype\n\t\t\t\t}\n\t\t\t}\n\t\t}",
		"variables": {"newName": "ns-1", "namespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"}
}`
		})

		It("works in the basic case", func () {
			By("setting up a mock server")
			gqlRenameResponse := `{"data":{"renameNamespace":{"namespace":{"id":"4e377fe3-330d-4e4c-af62-821850fe9595"},"errors":[]}}}`		
			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:http.StatusOK,
				Request: expectedGetNsRequest,
				Response: gqlGetNsResponse})
			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:http.StatusOK,
				Request: expectedRenameRequest,
				Response: gqlRenameResponse})

			By("running the command")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Out, "5s").Should(gbytes.Say("`ns-0` renamed to `ns-1`"))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("returns an error when renaming a namespace fails", func() {
			By("setting up a mock server")
			gqlRenameResponse := `{
			        "renameNamespace": {
					"errors": [
						{"message": "error1"},
						{"message": "error2"}
					],
					"namespace": null
				}
			}`
			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:http.StatusOK,
				Request: expectedGetNsRequest,
				Response: gqlGetNsResponse})
			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:http.StatusOK,
				Request: expectedRenameRequest,
				Response: gqlRenameResponse})
			By("running the command")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
			Eventually(session).ShouldNot(gexec.Exit(0))
		})
	})
})
