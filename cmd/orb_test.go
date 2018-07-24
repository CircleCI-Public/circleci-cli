package cmd_test

import (
	"net/http"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Orb integration tests", func() {
	Describe("CLI behavior with a stubbed api and an orb.yml provided", func() {
		var (
			testServer *ghttp.Server
			orb        tmpFile
			token      string = "testtoken"
			command    *exec.Cmd
		)

		BeforeEach(func() {
			var err error
			orb, err = openTmpFile(filepath.Join("myorb", "orb.yml"))
			Expect(err).ToNot(HaveOccurred())

			testServer = ghttp.NewServer()
		})

		AfterEach(func() {
			orb.close()
			testServer.Close()
		})

		Describe("when validating orb", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "validate",
					"-t", token,
					"-e", testServer.URL(),
					"-p", orb.Path,
				)
			})

			It("works", func() {
				By("setting up a mock server")
				err := orb.write(`{}`)
				Expect(err).ToNot(HaveOccurred())

				gqlResponse := `{
							"orbConfig": {
								"sourceYaml": "{}",
								"valid": true,
								"errors": []
							}
						}`

				expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateOrb ($config: String!) {\n\t\t\torbConfig(orbYaml: $config) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
						"config": "{}"
					}
				}`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedRequestJson,
					Response: gqlResponse,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				// the .* is because the full path with temp dir is printed
				Eventually(session.Out).Should(gbytes.Say("Orb at .*myorb/orb.yml is valid"))
				Eventually(session).Should(gexec.Exit(0))
			})

			It("prints errors if invalid", func() {
				By("setting up a mock server")
				err := orb.write(`some orb`)
				Expect(err).ToNot(HaveOccurred())

				gqlResponse := `{
							"orbConfig": {
								"sourceYaml": "hello world",
								"valid": false,
								"errors": [
									{"message": "invalid_orb"}
								]
							}
						}`

				expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateOrb ($config: String!) {\n\t\t\torbConfig(orbYaml: $config) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
					  "config": "some orb"
					}
				  }`
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedRequestJson,
					Response: gqlResponse,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: invalid_orb"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})

		Describe("when expanding orb", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "expand",
					"-t", token,
					"-e", testServer.URL(),
					"-p", orb.Path,
				)
			})

			It("works", func() {
				By("setting up a mock server")
				err := orb.write(`some orb`)
				Expect(err).ToNot(HaveOccurred())

				gqlResponse := `{
							"orbConfig": {
								"outputYaml": "hello world",
								"valid": true,
								"errors": []
							}
						}`

				expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateOrb ($config: String!) {\n\t\t\torbConfig(orbYaml: $config) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
					  "config": "some orb"
					}
				  }`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedRequestJson,
					Response: gqlResponse,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("hello world"))
				Eventually(session).Should(gexec.Exit(0))
			})

			It("prints errors if invalid", func() {
				By("setting up a mock server")
				err := orb.write(`some orb`)
				Expect(err).ToNot(HaveOccurred())

				gqlResponse := `{
							"orbConfig": {
								"outputYaml": "hello world",
								"valid": false,
								"errors": [
									{"message": "error1"},
									{"message": "error2"}
								]
							}
						}`

				expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateOrb ($config: String!) {\n\t\t\torbConfig(orbYaml: $config) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
					  "config": "some orb"
					}
				  }`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedRequestJson,
					Response: gqlResponse,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1: error2"))
				Eventually(session).ShouldNot(gexec.Exit(0))

			})
		})

		Describe("when publishing an orb version", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "publish",
					"-t", token,
					"-e", testServer.URL(),
					"-p", orb.Path,
					"--orb-version", "0.0.1",
					"--orb-id", "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				)
			})

			It("works", func() {

				// TODO: factor out common test setup into a top-level JustBeforeEach. Rely
				// on BeforeEach in each block to specify server mocking.
				By("setting up a mock server")
				// write to test file
				err := orb.write(`some orb`)
				// assert write to test file successful
				Expect(err).ToNot(HaveOccurred())

				gqlResponse := `{
					"publishOrb": {
						"errors": [],
						"orb": {
							"createdAt": "2018-07-16T18:03:18.961Z",
							"version": "0.0.1"
						}
					}
				}`

				expectedRequestJson := `{
					"query": "\n\t\tmutation($config: String!, $orbId: UUID!, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbId: $orbId,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t\tcreatedAt\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
					"variables": {
						"config": "some orb",
						"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
						"version": "0.0.1"
					}
				}`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedRequestJson,
					Response: gqlResponse,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("Orb published"))
				Eventually(session).Should(gexec.Exit(0))
			})

			It("prints all errors returned by the GraphQL API", func() {
				By("setting up a mock server")
				err := orb.write(`some orb`)
				Expect(err).ToNot(HaveOccurred())

				gqlResponse := `{
							"publishOrb": {
								"errors": [
									{"message": "error1"},
									{"message": "error2"}
								],
								"orb": null
							}
						}`

				expectedRequestJson := `{
					"query": "\n\t\tmutation($config: String!, $orbId: UUID!, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbId: $orbId,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t\tcreatedAt\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
					"variables": {
						"config": "some orb",
						"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
						"version": "0.0.1"
					}
				}`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedRequestJson,
					Response: gqlResponse,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1: error2"))
				Eventually(session).ShouldNot(gexec.Exit(0))

			})
		})

		Describe("when creating / reserving a namespace", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "ns", "create",
					"-t", token,
					"-e", testServer.URL(),
					"foo-ns",
					"--org-name", "test-org",
					"--vcs", "BITBUCKET",
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
														"createdAt": "2018-07-16T18:03:18.961Z",
														"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
										}
									}
				}`

				expectedNsRequest := `{
            "query": "\n\t\t\tmutation($name: String!, $organizationId: UUID!) {\n\t\t\t\tcreateNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t\torganizationId: $organizationId\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tcreatedAt\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
            "variables": {
              "name": "foo-ns",
              "organizationId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            }
          }`

				appendPostHandler(testServer, token,
					MockRequestResponse{Status: http.StatusOK,
						Request:  expectedOrganizationRequest,
						Response: gqlOrganizationResponse})

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedNsRequest,
					Response: gqlNsResponse})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("Namespace created"))
				Eventually(session).Should(gexec.Exit(0))
			})

			It("prints all errors returned by the GraphQL API", func() {
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

				expectedRequestJson := `{
            			"query": "\n\t\t\tmutation($name: String!, $organizationId: UUID!) {\n\t\t\t\tcreateNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t\torganizationId: $organizationId\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tcreatedAt\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
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
						Status:   http.StatusOK,
						Request:  expectedRequestJson,
						Response: gqlResponse,
					})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1: error2"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})

		Describe("when creating / reserving an orb", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "create",
					"-t", token,
					"-e", testServer.URL(),
					"foo-orb",
					"bar-ns",
				)
			})

			It("works", func() {
				By("setting up a mock server")

				gqlNamespaceResponse := `{
											"namespace": {
      											"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
    										}
  										 }`

				expectedNamespaceRequest := ``

				gqlOrbResponse := `{
									 "createOrb": {
										 "errors": [],
										 "orb": {
											"createdAt": "2018-07-16T18:03:18.961Z",
											"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
										 }
									 }
								   }`

				expectedOrbRequest := `{

          		}`

				appendPostHandler(testServer, token, MockRequestResponse{
						Status: http.StatusOK,
						Request:  expectedNamespaceRequest,
						Response: gqlNamespaceResponse})

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrbRequest,
					Response: gqlOrbResponse})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("Orb created"))
				Eventually(session).Should(gexec.Exit(0))
			})

			//It("prints all errors returned by the GraphQL API", func() {
			//	By("setting up a mock server")
			//
			//	gqlNamespaceResponse := `{
			//								"namespace": {
      		//									"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
    			//							}
  			//	}`
			//
			//	expectedNamespaceRequest := `{
			//
		     //   }`
			//
			//	gqlOrbResponse := `{
			//						 "createOrb": {
			//							 "errors": [
			//										{"message": "error1"},
			//										{"message": "error2"}
			//									   ],
			//							 "orb": null
			//						}
			//	}`
			//
			//	expectedOrbRequest := `{
			//
          	//	}`
			//
			//	appendPostHandler(testServer, token,
			//		MockRequestResponse{
			//			Status:   http.StatusOK,
			//			Request:  expectedNamespaceRequest,
			//			Response: gqlNamespaceResponse,
			//		})
			//	appendPostHandler(testServer, token,
			//		MockRequestResponse{
			//			Status:   http.StatusOK,
			//			Request:  expectedOrbRequest,
			//			Response: gqlOrbResponse,
			//		})
			//
			//	By("running the command")
			//	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			//
			//	Expect(err).ShouldNot(HaveOccurred())
			//	Eventually(session.Err).Should(gbytes.Say("Error: error1: error2"))
			//	Eventually(session).ShouldNot(gexec.Exit(0))
			//})
		})

	})
})
