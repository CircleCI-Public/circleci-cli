package cmd_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"io"

	"gotest.tools/golden"

	"github.com/CircleCI-Public/circleci-cli/client"
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
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
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
				Eventually(session.Out).Should(gbytes.Say("Orb input is valid."))
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
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
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
				Eventually(session.Out).Should(gbytes.Say("Orb at `.*orb.yml` is valid."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("when validating orb", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "validate", orb.Path,
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
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
				Eventually(session.Out).Should(gbytes.Say("Orb at `.*myorb/orb.yml` is valid."))
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

		Describe("when processing orb", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "process",
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
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
				Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
				Eventually(session).ShouldNot(gexec.Exit(0))

			})
		})

		Describe("when releasing a semantic version", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "publish",
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
					orb.Path,
					"my/orb@0.0.1",
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
            "query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
            "variables": {
              "name": "my/orb",
              "namespace": "my"
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
				Eventually(session.Out).Should(gbytes.Say("Orb `my/orb@0.0.1` was published."))
				Eventually(session.Out).Should(gbytes.Say("Please note that this is an open orb and is world-readable."))
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
            "query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
            "variables": {
              "name": "my/orb",
              "namespace": "my"
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
				Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
				Eventually(session).ShouldNot(gexec.Exit(0))

			})
		})

		Describe("when releasing a development version", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "publish",
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
					orb.Path,
					"my/orb@dev:foo",
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
            "query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
            "variables": {
              "name": "my/orb",
              "namespace": "my"
            }
          }`

				gqlPublishResponse := `{
					"publishOrb": {
						"errors": [],
						"orb": {
							"version": "dev:foo"
						}
					}
				}`

				expectedPublishRequest := `{
					"query": "\n\t\tmutation($config: String!, $orbId: UUID!, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbId: $orbId,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
					"variables": {
						"config": "some orb",
						"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
						"version": "dev:foo"
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
				Eventually(session.Out).Should(gbytes.Say("Orb `my/orb@dev:foo` was published."))
				Eventually(session.Out).Should(gbytes.Say("Please note that this is an open orb and is world-readable."))
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
            "query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
            "variables": {
              "name": "my/orb",
              "namespace": "my"
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
						"version": "dev:foo"
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
				Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
				Eventually(session).ShouldNot(gexec.Exit(0))

			})
		})

		Describe("when incrementing a released version", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "publish", "increment",
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
					orb.Path,
					"my/orb", "minor",
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
            "query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
            "variables": {
              "name": "my/orb",
              "namespace": "my"
            }
          }`

				gqlVersionResponse := `{
					"orb": {
						"versions": [
                                                        {"version": "0.0.1"}
                                                 ]
					}
				}`

				expectedVersionRequest := `{
            "query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      versions(count: 1) {\n\t\t\t\t    version\n\t\t\t      }\n\t\t\t    }\n\t\t      }",
            "variables": {
              "name": "my/orb"
            }
				}`

				gqlPublishResponse := `{
					"publishOrb": {
						"errors": [],
						"orb": {
							"version": "0.1.0"
						}
					}
				}`

				expectedPublishRequest := `{
					"query": "\n\t\tmutation($config: String!, $orbId: UUID!, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbId: $orbId,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
					"variables": {
						"config": "some orb",
						"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
						"version": "0.1.0"
					}
				}`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrbIDRequest,
					Response: gqlOrbIDResponse})
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedVersionRequest,
					Response: gqlVersionResponse})
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedPublishRequest,
					Response: gqlPublishResponse})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("Orb `my/orb` has been incremented to `my/orb@0.1.0`."))
				Eventually(session.Out).Should(gbytes.Say("Please note that this is an open orb and is world-readable."))
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
            "query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
            "variables": {
              "name": "my/orb",
              "namespace": "my"
            }
          }`

				gqlVersionResponse := `{
					"orb": {
						"versions": [
                                                        {"version": "0.0.1"}
                                                 ]
					}
				}`

				expectedVersionRequest := `{
            "query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      versions(count: 1) {\n\t\t\t\t    version\n\t\t\t      }\n\t\t\t    }\n\t\t      }",
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
						"version": "0.1.0"
					}
				}`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrbIDRequest,
					Response: gqlOrbIDResponse})
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedVersionRequest,
					Response: gqlVersionResponse})
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedPublishRequest,
					Response: gqlPublishResponse})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
				Eventually(session).ShouldNot(gexec.Exit(0))

			})
		})

		Describe("when promoting a development version", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "publish", "promote",
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
					"my/orb@dev:foo",
					"minor",
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
            "query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
            "variables": {
              "name": "my/orb",
              "namespace": "my"
            }
          }`

				gqlVersionResponse := `{
					"orb": {
						"versions": [
                                                        {"version": "0.0.1"}
                                                 ]
					}
				}`

				expectedVersionRequest := `{
            "query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      versions(count: 1) {\n\t\t\t\t    version\n\t\t\t      }\n\t\t\t    }\n\t\t      }",
            "variables": {
              "name": "my/orb"
            }
				}`

				gqlPromoteResponse := `{
					"promoteOrb": {
						"errors": [],
						"orb": {
							"version": "0.1.0",
                                                        "source": "some orb"
						}
					}
				}`

				expectedPromoteRequest := `{
                                        "query": "\n\t\tmutation($orbId: UUID!, $devVersion: String!, $semanticVersion: String!) {\n\t\t\tpromoteOrb(\n\t\t\t\torbId: $orbId,\n\t\t\t\tdevVersion: $devVersion,\n\t\t\t\tsemanticVersion: $semanticVersion\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t\tsource\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
					"variables": {
						"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
						"devVersion": "dev:foo",
						"semanticVersion": "0.1.0"
					}
				}`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrbIDRequest,
					Response: gqlOrbIDResponse})
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedVersionRequest,
					Response: gqlVersionResponse})
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedPromoteRequest,
					Response: gqlPromoteResponse})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("Orb `my/orb@dev:foo` was promoted to `my/orb@0.1.0`."))
				Eventually(session.Out).Should(gbytes.Say("Please note that this is an open orb and is world-readable."))
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
            "query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
            "variables": {
              "name": "my/orb",
              "namespace": "my"
            }
          }`

				gqlVersionResponse := `{
					"orb": {
						"versions": [
                                                        {"version": "0.0.1"}
                                                 ]
					}
				}`

				expectedVersionRequest := `{
            "query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      versions(count: 1) {\n\t\t\t\t    version\n\t\t\t      }\n\t\t\t    }\n\t\t      }",
            "variables": {
              "name": "my/orb"
            }
				}`

				gqlPromoteResponse := `{
					"promoteOrb": {
								"errors": [
									{"message": "error1"},
									{"message": "error2"}
								],
								"orb": null
					}
				}`

				expectedPromoteRequest := `{
                                        "query": "\n\t\tmutation($orbId: UUID!, $devVersion: String!, $semanticVersion: String!) {\n\t\t\tpromoteOrb(\n\t\t\t\torbId: $orbId,\n\t\t\t\tdevVersion: $devVersion,\n\t\t\t\tsemanticVersion: $semanticVersion\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t\tsource\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
					"variables": {
						"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
						"devVersion": "dev:foo",
						"semanticVersion": "0.1.0"
					}
				}`

				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrbIDRequest,
					Response: gqlOrbIDResponse})
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedVersionRequest,
					Response: gqlVersionResponse})
				appendPostHandler(testServer, token, MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedPromoteRequest,
					Response: gqlPromoteResponse})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
				Eventually(session).ShouldNot(gexec.Exit(0))

			})
		})

		Describe("when creating / reserving an orb", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "create",
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
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
				Eventually(session.Out).Should(gbytes.Say("Orb `bar-ns/foo-orb` created."))
				Eventually(session.Out).Should(gbytes.Say("Please note that any versions you publish of this orb are world-readable."))
				Eventually(session.Out).Should(gbytes.Say("You can now register versions of `bar-ns/foo-orb` using `circleci orb publish`"))
				Eventually(session).Should(gexec.Exit(0))
			})

			It("prints all in-band errors returned by the GraphQL API", func() {
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

				gqlErrors := `[ { "message": "ignored error" } ]`

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
						Status:        http.StatusOK,
						Request:       expectedOrbRequest,
						Response:      gqlOrbResponse,
						ErrorResponse: gqlErrors,
					})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})

		Describe("when listing all orbs", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list",
					"--skip-update-check",
					"--host", testServer.URL(),
				)
			})

			It("sends multiple requests when there are more than 1 page of orbs", func() {
				By("setting up a mock server")

				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list/first_response.json"))
				firstGqlResponse := string(tmpBytes)

				tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list/second_response.json"))
				secondGqlResponse := string(tmpBytes)

				// Use Gomega's default matcher instead of our custom appendPostHandler
				// since this query doesn't pass in a token.
				// Skip checking the content type field to make this test simpler.
				testServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/graphql-unstable"),
						ghttp.RespondWith(http.StatusOK, firstGqlResponse),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/graphql-unstable"),
						ghttp.RespondWith(http.StatusOK, secondGqlResponse),
					),
				)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				Expect(testServer.ReceivedRequests()).Should(HaveLen(2))
			})

		})

		Describe("when listing all orbs with the --json flag", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list",
					"--skip-update-check",
					"--host", testServer.URL(),
					"--json",
				)
			})
			It("sends multiple requests and groups the results into a single json output", func() {
				By("setting up a mock server")

				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list/first_response.json"))
				firstGqlResponse := string(tmpBytes)

				tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list/second_response.json"))
				secondGqlResponse := string(tmpBytes)

				tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list/pretty_json_output.json"))
				expectedOutput := string(tmpBytes)

				// Use Gomega's default matcher instead of our custom appendPostHandler
				// since this query doesn't pass in a token.
				// Skip checking the content type field to make this test simpler.
				testServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/graphql-unstable"),
						ghttp.RespondWith(http.StatusOK, firstGqlResponse),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/graphql-unstable"),
						ghttp.RespondWith(http.StatusOK, secondGqlResponse),
					),
				)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				// Use the pattern from
				// https://onsi.github.io/gomega/#gexec-testing-external-processes
				// instead of Say() since we want to perform a substring match, not a regexp
				// match
				completeOutput := string(session.Wait().Out.Contents())
				Expect(completeOutput).Should(MatchJSON(expectedOutput))
				Expect(testServer.ReceivedRequests()).Should(HaveLen(2))
			})
		})

		Describe("when listing all orbs with --uncertified", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list",
					"--skip-update-check",
					"--uncertified",
					"--host", testServer.URL(),
				)
				By("setting up a mock server")

				query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    usageStats {
		last30DaysBuildCount,
		last30DaysProjectCount,
		last30DaysOrganizationCount
	    }
		  versions(count: 1) {
			version,
			source
		  }
		}
	}
    pageInfo {
      hasNextPage
    }
  }
}
`

				firstRequest := client.NewUnauthorizedRequest(query)
				firstRequest.Variables["after"] = ""
				firstRequest.Variables["certifiedOnly"] = false

				firstRequestEncoded, err := firstRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				secondRequest := client.NewUnauthorizedRequest(query)
				secondRequest.Variables["after"] = "test/here-we-go"
				secondRequest.Variables["certifiedOnly"] = false

				secondRequestEncoded, err := secondRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_uncertified/first_response.json"))
				firstResponse := string(tmpBytes)

				tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_uncertified/second_response.json"))
				secondResponse := string(tmpBytes)

				appendPostHandler(testServer, "",
					MockRequestResponse{
						Status:   http.StatusOK,
						Request:  firstRequestEncoded.String(),
						Response: firstResponse,
					})
				appendPostHandler(testServer, "",
					MockRequestResponse{
						Status:   http.StatusOK,
						Request:  secondRequestEncoded.String(),
						Response: secondResponse,
					})
			})

			It("sends a GraphQL request with 'uncertifiedOnly: false'", func() {
				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				Eventually(session.Out).Should(gbytes.Say("Orbs found: 11. Includes all certified and uncertified orbs."))
				// Include an orb with content from the first mocked response
				Eventually(session.Out).Should(gbytes.Say("circleci/codecov-clojure \\(0.0.4\\)"))
				// Include an orb with contents from the second mocked response
				Eventually(session.Out).Should(gbytes.Say("zzak/test4 \\(0.1.0\\)"))
				Expect(testServer.ReceivedRequests()).Should(HaveLen(2))
			})

			Context("with the --json flag", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "list",
						"--skip-update-check",
						"--uncertified",
						"--host", testServer.URL(),
						"--json",
					)
				})

				It("sends a GraphQL request with 'uncertifiedOnly: false' and prints out json", func() {
					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session).Should(gexec.Exit(0))

					tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_uncertified/pretty_json_output.json"))
					expectedOutput := string(tmpBytes)
					completeOutput := string(session.Wait().Out.Contents())
					Expect(completeOutput).Should(MatchJSON(expectedOutput))
					Expect(testServer.ReceivedRequests()).Should(HaveLen(2))
				})
			})
		})

		Describe("when listing all orbs with --details", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list",
					"--skip-update-check",
					"--host", testServer.URL(),
					"--details",
				)
				By("setting up a mock server")

				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_details/response.json"))
				gqlResponse := string(tmpBytes)

				// Use Gomega's default matcher instead of our custom appendPostHandler
				// since this query doesn't pass in a token.
				// Skip checking the content type field to make this test simpler.
				testServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/graphql-unstable"),
						ghttp.RespondWith(http.StatusOK, gqlResponse),
					),
				)
			})

			It("lists detailed orbs", func() {
				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`Orbs found: 1. Showing only certified orbs. Add -u for a list of all orbs.

foo/test (0.7.0)
  Commands:
    - bar: 1 parameter(s)
       - hello: string (default: 'world')
    - myfoo: 0 parameter(s)
  Jobs:
    - hello-build: 0 parameter(s)
  Executors:
    - default: 1 parameter(s)
       - tag: string (default: 'curl-browsers')

`))
				Eventually(session).Should(gexec.Exit(0))
				Expect(testServer.ReceivedRequests()).Should(HaveLen(1))
			})

			Context("with the --json flag", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "list",
						"--skip-update-check",
						"--host", testServer.URL(),
						"--details",
						"--json",
					)
				})

				It("is overridden by the --json flag", func() {
					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session).Should(gexec.Exit(0))

					tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_details/pretty_json_output.json"))
					expectedOutput := string(tmpBytes)
					completeOutput := string(session.Wait().Out.Contents())
					Expect(completeOutput).Should(MatchJSON(expectedOutput))
					Expect(testServer.ReceivedRequests()).Should(HaveLen(1))
				})
			})
		})

		Describe("when listing orbs with a namespace argument", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list", "circleci",
					"--skip-update-check",
					"--host", testServer.URL(),
					"--details",
				)
				By("setting up a mock server")

				query := `
query namespaceOrbs ($namespace: String, $after: String!) {
	registryNamespace(name: $namespace) {
		name
		orbs(first: 20, after: $after) {
			edges {
				cursor
				node {
					versions {
						source
						version
					}
					name
	                                usageStats {
		                           last30DaysBuildCount,
		                           last30DaysProjectCount,
		                           last30DaysOrganizationCount
	                               }
				}
			}
			totalCount
			pageInfo {
				hasNextPage
			}
		}
	}
}
`
				firstRequest := client.NewUnauthorizedRequest(query)
				firstRequest.Variables["after"] = ""
				firstRequest.Variables["namespace"] = "circleci"

				firstRequestEncoded, err := firstRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				secondRequest := client.NewUnauthorizedRequest(query)
				secondRequest.Variables["after"] = "circleci/codecov-clojure"
				secondRequest.Variables["namespace"] = "circleci"

				secondRequestEncoded, err := secondRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				// These responses are generated from production data,
				// but using a 5-per-page limit instead of the 20 requested.
				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_with_namespace/first_response.json"))
				firstResponse := string(tmpBytes)

				tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_with_namespace/second_response.json"))
				secondResponse := string(tmpBytes)

				appendPostHandler(testServer, "",
					MockRequestResponse{
						Status:   http.StatusOK,
						Request:  firstRequestEncoded.String(),
						Response: firstResponse,
					})
				appendPostHandler(testServer, "",
					MockRequestResponse{
						Status:   http.StatusOK,
						Request:  secondRequestEncoded.String(),
						Response: secondResponse,
					})
			})

			It("makes a namespace query and requests all orbs on that namespace", func() {
				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("circleci/gradle"))
				Eventually(session.Out).Should(gbytes.Say("Jobs"))
				Eventually(session.Out).Should(gbytes.Say("- test"))
				Eventually(session.Out).Should(gbytes.Say("circleci/rollbar"))
				Eventually(session.Out).Should(gbytes.Say("Commands"))
				Eventually(session.Out).Should(gbytes.Say("- notify_deploy"))
				Eventually(session).Should(gexec.Exit(0))
				Expect(testServer.ReceivedRequests()).Should(HaveLen(2))
			})

			Context("with the --json flag", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "list", "circleci",
						"--skip-update-check",
						"--host", testServer.URL(),
						"--json",
					)
				})

				It("sends a GraphQL request with 'uncertifiedOnly: false' and prints out json", func() {

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_with_namespace/pretty_json_output.json"))
					expectedOutput := string(tmpBytes)
					completeOutput := string(session.Wait().Out.Contents())
					Expect(completeOutput).Should(MatchJSON(expectedOutput))
					Eventually(session).Should(gexec.Exit(0))
					Expect(testServer.ReceivedRequests()).Should(HaveLen(2))
				})
			})
		})

		Describe("when creating an orb without a token", func() {
			var tempHome string

			BeforeEach(func() {
				tempHome, _, _ = withTempSettings()

				command = exec.Command(pathCLI,
					"orb", "create", "bar-ns/foo-orb",
					"--skip-update-check",
					"--token", "",
				)
				command.Env = append(os.Environ(),
					fmt.Sprintf("HOME=%s", tempHome),
				)
			})

			AfterEach(func() {
				Expect(os.RemoveAll(tempHome)).To(Succeed())
			})

			It("instructs the user to run 'circleci setup' and create a new token", func() {
				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say(`Error: please set a token with 'circleci setup'
You can create a new personal API token here:
https://circleci.com/account/api`))
				Eventually(session).Should(gexec.Exit(255))
			})
		})

		Describe("when fetching an orb's source", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "source",
					"--skip-update-check",
					"--host", testServer.URL(),
					"my/orb@dev:foo",
				)
			})

			It("works", func() {
				// TODO: factor out common test setup into a top-level JustBeforeEach. Rely
				// on BeforeEach in each block to specify server mocking.
				By("setting up a mock server")

				request := client.NewUnauthorizedRequest(`query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`)
				request.Variables["orbVersionRef"] = "my/orb@dev:foo"
				expected, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				gqlResponse := `{"data": {
							"orbVersion": {
								"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
								"version": "dev:foo",
								"orb": {
								        "id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
								},
								"source": "some orb"
							}
						}}`

				// Use Gomega's default matcher instead of our custom appendPostHandler
				// since this query doesn't pass in a token.
				// Skip checking the content type field to make this test simpler.
				testServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/graphql-unstable"),

						// TODO: Extract this into a verifyJSONUtf8 helper function
						ghttp.VerifyContentType("application/json; charset=utf-8"),
						// From Gomegas ghttp.VerifyJson to avoid the
						// VerifyContentType("application/json") check
						// that fails with "application/json; charset=utf-8"
						func(w http.ResponseWriter, req *http.Request) {
							body, error := ioutil.ReadAll(req.Body)
							req.Body.Close()
							Expect(error).ShouldNot(HaveOccurred())
							Expect(body).Should(MatchJSON(expected.String()), "JSON Mismatch")
						},
						ghttp.RespondWith(http.StatusOK, gqlResponse),
					),
				)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("some orb"))

				Eventually(session).Should(gexec.Exit(0))
			})

			It("reports when an orb hasn't published a version", func() {
				// TODO: factor out common test setup into a top-level JustBeforeEach. Rely
				// on BeforeEach in each block to specify server mocking.
				By("setting up a mock server")

				request := client.NewUnauthorizedRequest(`query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`)
				request.Variables["orbVersionRef"] = "my/orb@dev:foo"
				expected, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				gqlResponse := `{"data": { "orbVersion": {} }}`

				// Use Gomega's default matcher instead of our custom appendPostHandler
				// since this query doesn't pass in a token.
				// Skip checking the content type field to make this test simpler.
				testServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/graphql-unstable"),

						// TODO: Extract this into a verifyJSONUtf8 helper function
						ghttp.VerifyContentType("application/json; charset=utf-8"),
						// From Gomegas ghttp.VerifyJson to avoid the
						// VerifyContentType("application/json") check
						// that fails with "application/json; charset=utf-8"
						func(w http.ResponseWriter, req *http.Request) {
							body, error := ioutil.ReadAll(req.Body)
							req.Body.Close()
							Expect(error).ShouldNot(HaveOccurred())
							Expect(body).Should(MatchJSON(expected.String()), "JSON Mismatch")
						},
						ghttp.RespondWith(http.StatusOK, gqlResponse),
					),
				)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("no Orb 'my/orb@dev:foo' was found; please check that the Orb reference is correct"))

				Eventually(session).Should(gexec.Exit(255))
			})
		})

		Describe("when fetching an orb's meta-data", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "info",
					"--skip-update-check",
					"--host", testServer.URL(),
					"my/orb@dev:foo",
				)
			})

			It("works", func() {
				// TODO: factor out common test setup into a top-level JustBeforeEach. Rely
				// on BeforeEach in each block to specify server mocking.
				By("setting up a mock server")

				request := client.NewUnauthorizedRequest(`query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb {
                                    id
                                    createdAt
                                    name
                                    versions {
                                        createdAt
                                        version
                                    }
                                }
                                source
                                createdAt
			    }
		      }`)

				request.Variables["orbVersionRef"] = "my/orb@dev:foo"
				expected, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				gqlResponse := `{"data": {
							"orbVersion": {
								"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
								"version": "dev:foo",
								"orb": {
								        "id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
								        "createdAt": "2018-09-24T08:53:37.086Z",
                                                                        "name": "my/orb",
                                                                        "versions": [
                                                                            {
                                                                                "version": "0.0.1",
                                                                                "createdAt": "2018-10-11T22:12:19.477Z"
                                                                            }
                                                                        ]
								},
								"source": "description: zomg\ncommands: {foo: {parameters: {baz: {type: string}}}}",
                                                                "createdAt": "2018-09-24T08:53:37.086Z"
							}
						}}`

				// Use Gomega's default matcher instead of our custom appendPostHandler
				// since this query doesn't pass in a token.
				// Skip checking the content type field to make this test simpler.
				testServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/graphql-unstable"),

						// TODO: Extract this into a verifyJSONUtf8 helper function
						ghttp.VerifyContentType("application/json; charset=utf-8"),
						// From Gomegas ghttp.VerifyJson to avoid the
						// VerifyContentType("application/json") check
						// that fails with "application/json; charset=utf-8"
						func(w http.ResponseWriter, req *http.Request) {
							body, error := ioutil.ReadAll(req.Body)
							req.Body.Close()
							Expect(error).ShouldNot(HaveOccurred())
							Expect(body).Should(MatchJSON(expected.String()), "JSON Mismatch")
						},
						ghttp.RespondWith(http.StatusOK, gqlResponse),
					),
				)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())

				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`
Latest: my/orb@0.0.1
Last-updated: 2018-10-11T22:12:19.477Z
Created: 2018-09-24T08:53:37.086Z
First-release: 0.0.1 @ 2018-10-11T22:12:19.477Z
Total-revisions: 1

Total-commands: 1
Total-executors: 0
Total-jobs: 0
`))

				Eventually(session).Should(gexec.Exit(0))
			})

			It("reports when an dev orb hasn't released any semantic versions", func() {
				// TODO: factor out common test setup into a top-level JustBeforeEach. Rely
				// on BeforeEach in each block to specify server mocking.
				By("setting up a mock server")

				request := client.NewUnauthorizedRequest(`query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb {
                                    id
                                    createdAt
                                    name
                                    versions {
                                        createdAt
                                        version
                                    }
                                }
                                source
                                createdAt
			    }
		      }`)

				request.Variables["orbVersionRef"] = "my/orb@dev:foo"
				expected, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				gqlResponse := `{"data": {
							"orbVersion": {
								"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
								"version": "dev:foo",
								"orb": {
								        "id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
								        "createdAt": "2018-09-24T08:53:37.086Z",
                                                                        "name": "my/orb",
                                                                        "versions": []
								},
								"source": "description: zomg\ncommands: {foo: {parameters: {baz: {type: string}}}}",
                                                                "createdAt": "2018-09-24T08:53:37.086Z"
							}
						}}`

				// Use Gomega's default matcher instead of our custom appendPostHandler
				// since this query doesn't pass in a token.
				// Skip checking the content type field to make this test simpler.
				testServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/graphql-unstable"),

						// TODO: Extract this into a verifyJSONUtf8 helper function
						ghttp.VerifyContentType("application/json; charset=utf-8"),
						// From Gomegas ghttp.VerifyJson to avoid the
						// VerifyContentType("application/json") check
						// that fails with "application/json; charset=utf-8"
						func(w http.ResponseWriter, req *http.Request) {
							body, error := ioutil.ReadAll(req.Body)
							req.Body.Close()
							Expect(error).ShouldNot(HaveOccurred())
							Expect(body).Should(MatchJSON(expected.String()), "JSON Mismatch")
						},
						ghttp.RespondWith(http.StatusOK, gqlResponse),
					),
				)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())

				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`
This orb hasn't published any versions yet.

Total-commands: 1
Total-executors: 0
Total-jobs: 0
`))

				Eventually(session).Should(gexec.Exit(0))
			})

			It("reports when an dev orb hasn't released any semantic versions", func() {
				// TODO: factor out common test setup into a top-level JustBeforeEach. Rely
				// on BeforeEach in each block to specify server mocking.
				By("setting up a mock server")

				request := client.NewUnauthorizedRequest(`query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb {
                                    id
                                    createdAt
                                    name
                                    versions {
                                        createdAt
                                        version
                                    }
                                }
                                source
                                createdAt
			    }
		      }`)

				request.Variables["orbVersionRef"] = "my/orb@dev:foo"
				expected, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				gqlResponse := `{"data": { "orbVersion": {} }}`

				// Use Gomega's default matcher instead of our custom appendPostHandler
				// since this query doesn't pass in a token.
				// Skip checking the content type field to make this test simpler.
				testServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/graphql-unstable"),

						// TODO: Extract this into a verifyJSONUtf8 helper function
						ghttp.VerifyContentType("application/json; charset=utf-8"),
						// From Gomegas ghttp.VerifyJson to avoid the
						// VerifyContentType("application/json") check
						// that fails with "application/json; charset=utf-8"
						func(w http.ResponseWriter, req *http.Request) {
							body, error := ioutil.ReadAll(req.Body)
							req.Body.Close()
							Expect(error).ShouldNot(HaveOccurred())
							Expect(body).Should(MatchJSON(expected.String()), "JSON Mismatch")
						},
						ghttp.RespondWith(http.StatusOK, gqlResponse),
					),
				)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())

				Eventually(session.Err).Should(gbytes.Say("no Orb 'my/orb@dev:foo' was found; please check that the Orb reference is correct"))

				Eventually(session).Should(gexec.Exit(255))
			})
		})
	})
})
