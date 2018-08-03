package cmd_test

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"io"

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
			tmpDir     string
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = openTmpDir("")
			Expect(err).ToNot(HaveOccurred())

			orb, err = openTmpFile(tmpDir, filepath.Join("myorb", "orb.yml"))
			Expect(err).ToNot(HaveOccurred())

			testServer = ghttp.NewServer()
		})

		AfterEach(func() {
			orb.close()
			os.RemoveAll(tmpDir)
			testServer.Close()
		})

		Describe("when using STDIN", func() {
			BeforeEach(func() {
				token = "testtoken"
				command = exec.Command(pathCLI,
					"orb", "validate",
					"-t", token,
					"-e", testServer.URL(),
					"-",
				)
				stdin, err := command.StdinPipe()
				Expect(err).ToNot(HaveOccurred())
				go func() {
					defer stdin.Close()
					io.WriteString(stdin, "{}")
				}()
			})

			It("works", func() {
				By("setting up a mock server")

				gqlResponse := `{
							"orbConfig": {
								"sourceYaml": "{}",
								"valid": true,
								"errors": []
							}
						}`

				response := struct {
					Query     string `json:"query"`
					Variables struct {
						Config string `json:"config"`
					} `json:"variables"`
				}{
					Query: `
		query ValidateOrb ($config: String!) {
			orbConfig(orbYaml: $config) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`,
					Variables: struct {
						Config string `json:"config"`
					}{
						Config: "{}",
					},
				}
				expected, err := json.Marshal(response)
				Expect(err).ShouldNot(HaveOccurred())

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  string(expected),
					Response: gqlResponse})
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				// the .* is because the full path with temp dir is printed
				Eventually(session.Out).Should(gbytes.Say("Orb at - is valid"))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("when using default path", func() {
			BeforeEach(func() {
				var err error
				orb, err = openTmpFile(command.Dir, "orb.yml")
				Expect(err).ToNot(HaveOccurred())

				token = "testtoken"
				command = exec.Command(pathCLI,
					"orb", "validate", orb.Path,
					"-t", token,
					"-e", testServer.URL(),
				)
			})

			AfterEach(func() {
				orb.close()
				os.Remove(orb.Path)
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
					Response: gqlResponse})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				// the .* is because the full path with temp dir is printed
				Eventually(session.Out).Should(gbytes.Say("Orb at .*orb.yml is valid"))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("when validating orb", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "validate", orb.Path,
					"-t", token,
					"-e", testServer.URL(),
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
					orb.Path,
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

		Describe("when releasing an semantic version", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "publish", "release",
					"-t", token,
					"-e", testServer.URL(),
					orb.Path,
					"my",
					"orb",
					"0.0.1",
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

				gqlOrbIDResponse := `{
    											"orb": {
      												"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
    											}
  				}`

				expectedOrbIDRequest := `{
            "query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      id\n\t\t\t    }\n\t\t      }",
            "variables": {
              "name": "my/orb"
            }
          }`

				gqlPublishResponse := `{
					"publishOrb": {
						"errors": [],
						"orb": {
							"version": "0.0.1"
						}
					}
				}`

				expectedPublishRequest := `{
					"query": "\n\t\tmutation($config: String!, $orbId: UUID!, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbId: $orbId,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
					"variables": {
						"config": "some orb",
						"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
						"version": "0.0.1"
					}
				}`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrbIDRequest,
					Response: gqlOrbIDResponse})
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedPublishRequest,
					Response: gqlPublishResponse})

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

				gqlOrbIDResponse := `{
    											"orb": {
      												"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
    											}
  				}`

				expectedOrbIDRequest := `{
            "query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      id\n\t\t\t    }\n\t\t      }",
            "variables": {
              "name": "my/orb"
            }
          }`

				gqlPublishResponse := `{
					"publishOrb": {
								"errors": [
									{"message": "error1"},
									{"message": "error2"}
								],
								"orb": null
					}
				}`

				expectedPublishRequest := `{
					"query": "\n\t\tmutation($config: String!, $orbId: UUID!, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbId: $orbId,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
					"variables": {
						"config": "some orb",
						"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
						"version": "0.0.1"
					}
				}`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrbIDRequest,
					Response: gqlOrbIDResponse})
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedPublishRequest,
					Response: gqlPublishResponse})

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
					"bar-ns/foo-orb",
				)
			})

			It("works", func() {
				By("setting up a mock server")

				gqlNamespaceResponse := `{
    											"registryNamespace": {
      												"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
    											}
  				}`

				expectedNamespaceRequest := `{
            "query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
            "variables": {
              "name": "bar-ns"
            }
          }`

				gqlOrbResponse := `{
									 "createOrb": {
										 "errors": [],
										 "orb": {
											"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
										 }
									 }
								   }`

				expectedOrbRequest := `{
            "query": "mutation($name: String!, $registryNamespaceId: UUID!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
            "variables": {
              "name": "foo-orb",
              "registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            }
          }`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
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

			It("prints all errors returned by the GraphQL API", func() {
				By("setting up a mock server")

				gqlNamespaceResponse := `{
											"registryNamespace": {
												"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
										}
				}`

				expectedNamespaceRequest := `{
            "query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
            "variables": {
              "name": "bar-ns"
            }
          }`

				gqlOrbResponse := `{
									 "createOrb": {
										 "errors": [
													{"message": "error1"},
													{"message": "error2"}
												   ],
										 "orb": null
									}
				}`

				expectedOrbRequest := `{
            "query": "mutation($name: String!, $registryNamespaceId: UUID!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
            "variables": {
              "name": "foo-orb",
              "registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            }
          }`

				appendPostHandler(testServer, token,
					MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedNamespaceRequest,
						Response: gqlNamespaceResponse,
					})
				appendPostHandler(testServer, token,
					MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbRequest,
						Response: gqlOrbResponse,
					})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1: error2"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})

	})
})
