package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Orb integration tests", func() {
	Describe("Orb help text", func() {
		It("shows a link to the docs", func() {
			command := exec.Command(pathCLI, "orb", "--help")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Out).Should(gbytes.Say(`Operate on orbs

See a full explanation and documentation on orbs here: https://circleci.com/docs/2.0/orb-intro/
`))
			Eventually(session).Should(gexec.Exit(0))
		})

		Context("if user changes host settings through configuration", func() {
			var (
				tempSettings *clitest.TempSettings
				command      *exec.Cmd
			)

			BeforeEach(func() {
				tempSettings = clitest.WithTempSettings()

				command = commandWithHome(pathCLI, tempSettings.Home,
					"orb", "--help",
				)

				tempSettings.Config.Write([]byte(`host: foo.bar`))
			})

			AfterEach(func() {
				tempSettings.Close()
			})

			It("doesn't link to docs if user changes --host", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())

				Consistently(session.Out).ShouldNot(gbytes.Say("See a full explanation and documentation on orbs here: https://circleci.com/docs/2.0/orb-intro/"))
				Eventually(session).Should(gexec.Exit(0))
			})
		})
	})

	Describe("CLI behavior with a stubbed api and an orb.yml provided", func() {
		var (
			tempSettings *clitest.TempSettings
			orb          *clitest.TmpFile
			token        string = "testtoken"
			command      *exec.Cmd
		)

		BeforeEach(func() {
			tempSettings = clitest.WithTempSettings()
			orb = clitest.OpenTmpFile(tempSettings.Home, filepath.Join("myorb", "orb.yml"))
		})

		AfterEach(func() {
			tempSettings.Close()
			orb.Close()
		})

		Describe("when using STDIN", func() {
			BeforeEach(func() {
				token = "testtoken"
				command = exec.Command(pathCLI,
					"orb", "validate",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"-",
				)
				stdin, err := command.StdinPipe()
				Expect(err).ToNot(HaveOccurred())
				go func() {
					defer stdin.Close()
					_, err := io.WriteString(stdin, "{}")
					if err != nil {
						panic(err)
					}
				}()
			})

			It("works", func() {
				By("setting up a mock server")

				mockOrbIntrospection(true, "", tempSettings)

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
		query ValidateOrb ($config: String!, $owner: UUID) {
			orbConfig(orbYaml: $config, ownerId: $owner) {
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

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
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
				orb = clitest.OpenTmpFile(tempSettings.Home, "orb.yml")

				token = "testtoken"
				command = exec.Command(pathCLI,
					"orb", "validate", orb.Path,
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
				)
			})

			AfterEach(func() {
				tempSettings.Close()
				orb.Close()
			})

			It("works", func() {
				By("setting up a mock server")
				orb.Write([]byte(`{}`))

				mockOrbIntrospection(true, "", tempSettings)

				gqlResponse := `{
							"orbConfig": {
								"sourceYaml": "{}",
								"valid": true,
								"errors": []
							}
						}`

				expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateOrb ($config: String!, $owner: UUID) {\n\t\t\torbConfig(orbYaml: $config, ownerId: $owner) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
						"config": "{}"
					}
				}`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
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

		Describe("with old server version", func() {
			BeforeEach(func() {
				token = "testtoken"
				command = exec.Command(pathCLI,
					"orb", "validate",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"-",
				)
				stdin, err := command.StdinPipe()
				Expect(err).ToNot(HaveOccurred())
				go func() {
					defer stdin.Close()
					_, err := io.WriteString(stdin, "{}")
					if err != nil {
						panic(err)
					}
				}()
			})

			It("should use the old GraphQL resolver", func() {
				By("setting up a mock server")

				mockOrbIntrospection(false, "", tempSettings)

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

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  string(expected),
					Response: gqlResponse})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("Orb input is valid."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("with 'some orb'", func() {
			BeforeEach(func() {
				orb.Write([]byte(`some orb`))
			})

			Describe("when validating orb", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "validate", orb.Path,
						"--skip-update-check",
						"--token", token,
						"--host", tempSettings.TestServer.URL(),
					)
				})

				It("works", func() {
					By("setting up a mock server")

					mockOrbIntrospection(true, "", tempSettings)

					gqlResponse := `{
							"orbConfig": {
								"sourceYaml": "{}",
								"valid": true,
								"errors": []
							}
						}`

					expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateOrb ($config: String!, $owner: UUID) {\n\t\t\torbConfig(orbYaml: $config, ownerId: $owner) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
						"config": "some orb"
					}
				}`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedRequestJson,
						Response: gqlResponse,
					})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					// the .* is because the full path with temp dir is printed
					Eventually(session.Out).Should(gbytes.Say("Orb at `.*orb.yml` is valid."))
					Eventually(session).Should(gexec.Exit(0))
				})

				It("prints errors if invalid", func() {
					By("setting up a mock server")

					mockOrbIntrospection(true, "", tempSettings)

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
					"query": "\n\t\tquery ValidateOrb ($config: String!, $owner: UUID) {\n\t\t\torbConfig(orbYaml: $config, ownerId: $owner) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
					  "config": "some orb"
					}
				  }`
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
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
						"--host", tempSettings.TestServer.URL(),
						orb.Path,
					)
				})

				It("works", func() {
					By("setting up a mock server")

					mockOrbIntrospection(true, "", tempSettings)

					gqlResponse := `{
							"orbConfig": {
								"outputYaml": "hello world",
								"valid": true,
								"errors": []
							}
						}`

					expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateOrb ($config: String!, $owner: UUID) {\n\t\t\torbConfig(orbYaml: $config, ownerId: $owner) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
					  "config": "some orb"
					}
				  }`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
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

					mockOrbIntrospection(true, "", tempSettings)

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
					"query": "\n\t\tquery ValidateOrb ($config: String!, $owner: UUID) {\n\t\t\torbConfig(orbYaml: $config, ownerId: $owner) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
					  "config": "some orb"
					}
				  }`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
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
						"--host", tempSettings.TestServer.URL(),
						orb.Path,
						"my/orb@0.0.1",
					)
				})

				It("works", func() {
					By("setting up a mock server")

					gqlPublishResponse := `{
					"publishOrb": {
						"errors": [],
						"orb": {
							"version": "0.0.1"
						}
					}
				}`

					expectedPublishRequest := `{
						"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "config": "some orb",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "version": "0.0.1"
						}
					  }`

					gqlOrbIDResponse := `{
						"orb": {"id": "orbid1", "isPrivate": false},
						"registryNamespace": {"id": "nsid1"}
					}`

					expectedOrbIDRequest := `{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
						"variables": {
							"name": "my/orb",
							"namespace": "my"
						}
					}`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPublishRequest,
						Response: gqlPublishResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbIDRequest,
						Response: gqlOrbIDResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Out).Should(gbytes.Say("Orb `my/orb@0.0.1` was published."))
					Eventually(session).Should(gexec.Exit(0))
				})

				It("prints all errors returned by the GraphQL API", func() {
					By("setting up a mock server")

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
						"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "config": "some orb",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "version": "0.0.1"
						}
					  }`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPublishRequest,
						Response: gqlPublishResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
					Eventually(session).ShouldNot(gexec.Exit(0))

				})

				It("returns no error message if no orb is found from orbIsPrivateOrNotExists", func() {
					By("setting up a mock server")

					gqlPublishResponse := `{
						"publishOrb": {
							"errors": [],
							"orb": {
								"version": "0.0.1"
							}
						}
					}`

					expectedPublishRequest := `{
						"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "config": "some orb",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "version": "0.0.1"
						}
					  }`

					gqlOrbIDResponse := `{
						"orb": null,
						"registryNamespace": {"id": "nsid1"}
					}`

					expectedOrbIDRequest := `{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
						"variables": {
							"name": "my/orb",
							"namespace": "my"
						}
					}`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPublishRequest,
						Response: gqlPublishResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbIDRequest,
						Response: gqlOrbIDResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Out).Should(gbytes.Say("Orb `my/orb@0.0.1` was published."))
					Eventually(session.Out).ShouldNot(gbytes.Say("Please note that this is an open orb and is world-readable."))
					Eventually(session).Should(gexec.Exit(0))
				})
			})

			Describe("when releasing a development version", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "publish",
						"--skip-update-check",
						"--token", token,
						"--host", tempSettings.TestServer.URL(),
						orb.Path,
						"my/orb@dev:foo",
					)
				})

				It("works", func() {
					By("setting up a mock server")

					gqlPublishResponse := `{
					"publishOrb": {
						"errors": [],
						"orb": {
							"version": "dev:foo"
						}
					}
				}`

					expectedPublishRequest := `{
						"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "config": "some orb",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "version": "dev:foo"
						}
					  }`

					gqlOrbIDResponse := `{
						"orb": {"id": "orbid1", "isPrivate": false},
						"registryNamespace": {"id": "nsid1"}
					}`

					expectedOrbIDRequest := `{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
						"variables": {
							"name": "my/orb",
							"namespace": "my"
						}
					}`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPublishRequest,
						Response: gqlPublishResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbIDRequest,
						Response: gqlOrbIDResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Out).Should(gbytes.Say("Orb `my/orb@dev:foo` was published."))
					Eventually(session).Should(gexec.Exit(0))
				})

				It("prints all errors returned by the GraphQL API", func() {
					By("setting up a mock server")

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
						"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "config": "some orb",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "version": "dev:foo"
						}
					  }`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPublishRequest,
						Response: gqlPublishResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
					Eventually(session).ShouldNot(gexec.Exit(0))

				})

				It("returns no error message if no orb is found from orbIsPrivateOrNotExists", func() {
					By("setting up a mock server")

					gqlPublishResponse := `{
						"publishOrb": {
							"errors": [],
							"orb": {
								"version": "dev:foo"
							}
						}
					}`

					expectedPublishRequest := `{
						"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "config": "some orb",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "version": "dev:foo"
						}
					  }`

					gqlOrbIDResponse := `{
						"orb": null,
						"registryNamespace": {"id": "nsid1"}
					}`

					expectedOrbIDRequest := `{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
						"variables": {
							"name": "my/orb",
							"namespace": "my"
						}
					}`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPublishRequest,
						Response: gqlPublishResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbIDRequest,
						Response: gqlOrbIDResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Out).Should(gbytes.Say("Orb `my/orb@dev:foo` was published."))
					Eventually(session.Out).ShouldNot(gbytes.Say("Please note that this is an open orb and is world-readable."))
					Eventually(session).Should(gexec.Exit(0))
				})
			})

			Describe("when incrementing a released version", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "publish", "increment",
						"--skip-update-check",
						"--token", token,
						"--host", tempSettings.TestServer.URL(),
						orb.Path,
						"my/orb", "minor",
					)
				})

				It("works", func() {
					By("setting up a mock server")

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
						"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "config": "some orb",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "version": "0.1.0"
						}
					  }`

					gqlOrbIDResponse := `{
						"orb": {"id": "orbid1", "isPrivate": false},
						"registryNamespace": {"id": "nsid1"}
					}`

					expectedOrbIDRequest := `{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
						"variables": {
							"name": "my/orb",
							"namespace": "my"
						}
					}`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedVersionRequest,
						Response: gqlVersionResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPublishRequest,
						Response: gqlPublishResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbIDRequest,
						Response: gqlOrbIDResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Out).Should(gbytes.Say("Orb `my/orb` has been incremented to `my/orb@0.1.0`."))
					Eventually(session).Should(gexec.Exit(0))
				})

				It("prints all errors returned by the GraphQL API", func() {
					By("setting up a mock server")

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
						"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "config": "some orb",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "version": "0.1.0"
						}
					  }`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedVersionRequest,
						Response: gqlVersionResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPublishRequest,
						Response: gqlPublishResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
					Eventually(session).ShouldNot(gexec.Exit(0))

				})

				It("returns no error message if no orb is found from orbIsPrivateOrNotExists", func() {
					By("setting up a mock server")

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
						"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "config": "some orb",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "version": "0.1.0"
						}
					  }`

					gqlOrbIDResponse := `{
						"orb": null,
						"registryNamespace": {"id": "nsid1"}
					}`

					expectedOrbIDRequest := `{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
						"variables": {
							"name": "my/orb",
							"namespace": "my"
						}
					}`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedVersionRequest,
						Response: gqlVersionResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPublishRequest,
						Response: gqlPublishResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbIDRequest,
						Response: gqlOrbIDResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Out).Should(gbytes.Say("Orb `my/orb` has been incremented to `my/orb@0.1.0`."))
					Eventually(session.Out).ShouldNot(gbytes.Say("Please note that this is an open orb and is world-readable."))
					Eventually(session).Should(gexec.Exit(0))
				})
			})

			Describe("when promoting a development version", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "publish", "promote",
						"--skip-update-check",
						"--token", token,
						"--host", tempSettings.TestServer.URL(),
						"my/orb@dev:foo",
						"minor",
					)
				})

				It("works", func() {
					By("setting up a mock server")

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
						"query": "\n\t\tmutation($orbName: String, $namespaceName: String, $devVersion: String!, $semanticVersion: String!) {\n\t\t\tpromoteOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\tdevVersion: $devVersion,\n\t\t\t\tsemanticVersion: $semanticVersion\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t\tsource\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "devVersion": "dev:foo",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "semanticVersion": "0.1.0"
						}
					  }`

					gqlOrbIDResponse := `{
						"orb": {"id": "orbid1", "isPrivate": false},
						"registryNamespace": {"id": "nsid1"}
					}`

					expectedOrbIDRequest := `{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
						"variables": {
							"name": "my/orb",
							"namespace": "my"
						}
					}`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedVersionRequest,
						Response: gqlVersionResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPromoteRequest,
						Response: gqlPromoteResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbIDRequest,
						Response: gqlOrbIDResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Out).Should(gbytes.Say("Orb `my/orb@dev:foo` was promoted to `my/orb@0.1.0`."))
					Eventually(session).Should(gexec.Exit(0))
				})

				It("prints all errors returned by the GraphQL API", func() {
					By("setting up a mock server")

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
						"query": "\n\t\tmutation($orbName: String, $namespaceName: String, $devVersion: String!, $semanticVersion: String!) {\n\t\t\tpromoteOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\tdevVersion: $devVersion,\n\t\t\t\tsemanticVersion: $semanticVersion\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t\tsource\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "devVersion": "dev:foo",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "semanticVersion": "0.1.0"
						}
					  }`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedVersionRequest,
						Response: gqlVersionResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPromoteRequest,
						Response: gqlPromoteResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
					Eventually(session).ShouldNot(gexec.Exit(0))

				})

				It("returns no error message if no orb is found from orbIsPrivateOrNotExists", func() {
					By("setting up a mock server")

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
						"query": "\n\t\tmutation($orbName: String, $namespaceName: String, $devVersion: String!, $semanticVersion: String!) {\n\t\t\tpromoteOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\tdevVersion: $devVersion,\n\t\t\t\tsemanticVersion: $semanticVersion\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t\tsource\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
						"variables": {
						  "devVersion": "dev:foo",
						  "namespaceName": "my",
						  "orbName": "orb",
						  "semanticVersion": "0.1.0"
						}
					  }`

					gqlOrbIDResponse := `{
						"orb": null,
						"registryNamespace": {"id": "nsid1"}
					}`

					expectedOrbIDRequest := `{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
						"variables": {
							"name": "my/orb",
							"namespace": "my"
						}
					}`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedVersionRequest,
						Response: gqlVersionResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedPromoteRequest,
						Response: gqlPromoteResponse})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbIDRequest,
						Response: gqlOrbIDResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session.Out).Should(gbytes.Say("Orb `my/orb@dev:foo` was promoted to `my/orb@0.1.0`."))
					Eventually(session.Out).ShouldNot(gbytes.Say("Please note that this is an open orb and is world-readable."))
					Eventually(session).Should(gexec.Exit(0))
				})
			})
		})

		Describe("when creating / reserving an orb", func() {
			Context("skipping prompts", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "create",
						"--skip-update-check",
						"--token", token,
						"--host", tempSettings.TestServer.URL(),
						"--no-prompt",
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
            "query": "mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId,\n\t\t\t\t\tisPrivate: $isPrivate\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
            "variables": {
              "isPrivate": false,
              "name": "foo-orb",
              "registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            }
          }`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedNamespaceRequest,
						Response: gqlNamespaceResponse})

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbRequest,
						Response: gqlOrbResponse})

					By("running the command")
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session).Should(gexec.Exit(0))

					stdout := session.Wait().Out.Contents()
					Expect(string(stdout)).To(ContainSubstring(fmt.Sprintf(`Please note that any versions you publish of this orb will be world readable unless you create it with the '--private' flag

Orb %s created.
You can now register versions of %s using %s`, "`bar-ns/foo-orb`", "`bar-ns/foo-orb`", "`circleci orb publish`")))
				})

				It("works for private orbs", func() {
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
            "query": "mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId,\n\t\t\t\t\tisPrivate: $isPrivate\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
            "variables": {
              "isPrivate": true,
              "name": "foo-orb",
              "registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            }
          }`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedNamespaceRequest,
						Response: gqlNamespaceResponse})

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedOrbRequest,
						Response: gqlOrbResponse})

					By("running the command")
					command = exec.Command(pathCLI,
						"orb", "create",
						"--private",
						"--skip-update-check",
						"--token", token,
						"--host", tempSettings.TestServer.URL(),
						"--no-prompt",
						"bar-ns/foo-orb",
					)
					session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					Expect(err).ShouldNot(HaveOccurred())
					Eventually(session).Should(gexec.Exit(0))

					stdout := session.Wait().Out.Contents()
					Expect(string(stdout)).To(ContainSubstring(fmt.Sprintf(`This orb will not be listed on the registry and is usable only by org users.

Orb %s created.
You can now register versions of %s using %s`, "`bar-ns/foo-orb`", "`bar-ns/foo-orb`", "`circleci orb publish`")))
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
            "query": "mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId,\n\t\t\t\t\tisPrivate: $isPrivate\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
            "variables": {
			  "isPrivate": false,
              "name": "foo-orb",
              "registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            }
          }`

					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
						Status:   http.StatusOK,
						Request:  expectedNamespaceRequest,
						Response: gqlNamespaceResponse,
					})
					tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
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

			Context("with interactive prompts", func() {
				Describe("when creating / reserving an orb", func() {
					BeforeEach(func() {
						command = exec.Command(pathCLI,
							"orb", "create",
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL(),
							"--integration-testing",
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
            "query": "mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId,\n\t\t\t\t\tisPrivate: $isPrivate\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
            "variables": {
			  "isPrivate": false,
              "name": "foo-orb",
              "registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            }
          }`

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedNamespaceRequest,
							Response: gqlNamespaceResponse})

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedOrbRequest,
							Response: gqlOrbResponse})

						By("running the command")
						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session).Should(gexec.Exit(0))

						stdout := session.Wait().Out.Contents()

						Expect(string(stdout)).To(ContainSubstring(fmt.Sprintf(`You are creating an orb called "%s".

You will not be able to change the name of this orb.

If you change your mind about the name, you will have to create a new orb with the new name.

Please note that any versions you publish of this orb will be world readable unless you create it with the '--private' flag

Are you sure you wish to create the orb: %s
Orb %s created.
You can now register versions of %s using %s.`,
							"bar-ns/foo-orb", "`bar-ns/foo-orb`", "`bar-ns/foo-orb`", "`bar-ns/foo-orb`", "`circleci orb publish`")))
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
            "query": "mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId,\n\t\t\t\t\tisPrivate: $isPrivate\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
            "variables": {
			  "isPrivate": false,
              "name": "foo-orb",
              "registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
            }
          }`

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedNamespaceRequest,
							Response: gqlNamespaceResponse,
						})
						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
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
			})
		})

		Describe("when setting the listed status of an orb", func() {
			Context("with an authorized user's token", func() {
				DescribeTable("when setting the listed status of an orb",
					func(list bool, expectedDisplayedStatus string) {
						command = exec.Command(pathCLI,
							"orb", "unlist",
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL(),
							"bar-ns/foo-orb",
							strconv.FormatBool(!list),
						)

						gqlOrbIDResponse := `{
							"orb": {
								  "id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
							}
						}`

						expectedOrbIDRequest := `{
							"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
							"variables": {
								"name": "bar-ns/foo-orb",
								"namespace": "bar-ns"
							}
						}`

						gqlOrbResponse := fmt.Sprintf(`{
							"setOrbListStatus": {
								"listed": %t,
								"errors": []
							}
						}`, list)

						orbRequest := map[string]interface{}{
							"query": `
mutation($orbId: UUID!, $list: Boolean!) {
	setOrbListStatus(
		orbId: $orbId,
		list: $list
	) {
		listed
		errors {
			message
			type
		}
	}
}
	`,
							"variables": map[string]interface{}{
								"list":  list,
								"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
							},
						}

						expectedOrbRequest, err := json.Marshal(orbRequest)
						Expect(err).ToNot(HaveOccurred())

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedOrbIDRequest,
							Response: gqlOrbIDResponse})

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  string(expectedOrbRequest),
							Response: gqlOrbResponse})

						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session).Should(gexec.Exit(0))

						stdout := session.Wait().Out.Contents()
						Expect(string(stdout)).To(ContainSubstring(fmt.Sprintf(`The listing of orb %s is now %s.`, "`bar-ns/foo-orb`", expectedDisplayedStatus)))
					},
					Entry("listing an orb", true, "enabled"),
					Entry("unlisting an orb", false, "disabled"),
				)
			})
			Context("with an unauthorized user's token", func() {
				DescribeTable("when setting the listed status of an orb",
					func(list bool) {
						command = exec.Command(pathCLI,
							"orb", "unlist",
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL(),
							"bar-ns/foo-orb",
							strconv.FormatBool(!list),
						)

						gqlOrbIDResponse := `{
							"orb": {
								  "id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
							}
						}`

						expectedOrbIDRequest := `{
							"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
							"variables": {
								"name": "bar-ns/foo-orb",
								"namespace": "bar-ns"
							}
						}`

						gqlOrbResponse := `{
							"setOrbListStatus": {
								"listed": null,
								"errors": [
								  {
									"message": "AUTHORIZATION_FAILURE",
									"type": "AUTHORIZATION_FAILURE"
								  }
								]
							}
						}`

						orbRequest := map[string]interface{}{
							"query": `
mutation($orbId: UUID!, $list: Boolean!) {
	setOrbListStatus(
		orbId: $orbId,
		list: $list
	) {
		listed
		errors {
			message
			type
		}
	}
}
	`,
							"variables": map[string]interface{}{
								"list":  list,
								"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
							},
						}

						expectedOrbRequest, err := json.Marshal(orbRequest)
						Expect(err).ToNot(HaveOccurred())

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedOrbIDRequest,
							Response: gqlOrbIDResponse})

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  string(expectedOrbRequest),
							Response: gqlOrbResponse})

						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session.Err).Should(gbytes.Say("AUTHORIZATION_FAILURE"))
						Eventually(session).ShouldNot(gexec.Exit(0))
					},
					Entry("listing an orb", true),
					Entry("unlisting an orb", false),
				)
			})
			Context("specified namespace does not exist", func() {
				DescribeTable("when setting the listed status of an orb",
					func(list bool) {
						command = exec.Command(pathCLI,
							"orb", "unlist",
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL(),
							"bar-ns/foo-orb",
							strconv.FormatBool(!list),
						)

						gqlOrbIDResponse := `{
							"orb": null,
							"registryNamespace": null
						}`

						expectedOrbIDRequest := `{
							"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
							"variables": {
								"name": "bar-ns/foo-orb",
								"namespace": "bar-ns"
							}
						}`

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedOrbIDRequest,
							Response: gqlOrbIDResponse})

						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session.Err).Should(gbytes.Say("Error: the namespace 'bar-ns' does not exist."))
						Eventually(session).ShouldNot(gexec.Exit(0))
					},
					Entry("listing an orb", true),
					Entry("unlisting an orb", false),
				)
			})
			Context("specified orb does not exist in the namespace", func() {
				DescribeTable("when setting the listed status of an orb",
					func(list bool) {
						command = exec.Command(pathCLI,
							"orb", "unlist",
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL(),
							"bar-ns/foo-orb",
							strconv.FormatBool(!list),
						)

						gqlOrbIDResponse := `{
							"orb": null,
							"registryNamespace": {
								"id": "eac63dee-9960-48c2-b763-612e1683194e"
							}
						}`

						expectedOrbIDRequest := `{
							"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
							"variables": {
								"name": "bar-ns/foo-orb",
								"namespace": "bar-ns"
							}
						}`

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedOrbIDRequest,
							Response: gqlOrbIDResponse})

						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session.Err).Should(gbytes.Say("Error: the 'foo-orb' orb does not exist in the 'bar-ns' namespace."))
						Eventually(session).ShouldNot(gexec.Exit(0))
					},
					Entry("listing an orb", true),
					Entry("unlisting an orb", false),
				)
			})
			Context("orb unexpectedly cannot be found from the looked-up orb id", func() {
				DescribeTable("when setting the listed status of an orb",
					func(list bool) {
						command = exec.Command(pathCLI,
							"orb", "unlist",
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL(),
							"bar-ns/foo-orb",
							strconv.FormatBool(!list),
						)

						gqlOrbIDResponse := `{
							"orb": {
								  "id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
							}
						}`

						expectedOrbIDRequest := `{
							"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
							"variables": {
								"name": "bar-ns/foo-orb",
								"namespace": "bar-ns"
							}
						}`

						// This is to test the case of the orb unexpectedly not being able to be looked up with the orb id
						// returned in the response to the OrbID request
						gqlOrbResponse := `{
							"setOrbListStatus": {
								"listed": null,
								"errors": [
								  {
									"message": "Namespace not found for provided orb-id bb604b45-b6b0-4b81-ad80-796f15eddf87."
								  }
								]
							}
						}`

						orbRequest := map[string]interface{}{
							"query": `
mutation($orbId: UUID!, $list: Boolean!) {
	setOrbListStatus(
		orbId: $orbId,
		list: $list
	) {
		listed
		errors {
			message
			type
		}
	}
}
	`,
							"variables": map[string]interface{}{
								"list":  list,
								"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
							},
						}

						expectedOrbRequest, err := json.Marshal(orbRequest)
						Expect(err).ToNot(HaveOccurred())

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedOrbIDRequest,
							Response: gqlOrbIDResponse})

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  string(expectedOrbRequest),
							Response: gqlOrbResponse})

						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session.Err).Should(gbytes.Say("Namespace not found for provided orb-id bb604b45-b6b0-4b81-ad80-796f15eddf87."))
						Eventually(session).ShouldNot(gexec.Exit(0))
					},
					Entry("listing an orb", true),
					Entry("unlisting an orb", false),
				)
			})
			Context("incorrect number of arguments supplied", func() {
				DescribeTable("when setting the listed status of an orb",
					func(args ...string) {
						argList := []string{"orb", "unlist",
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL()}
						newArgList := append(argList, args...)
						command = exec.Command(pathCLI,
							newArgList...,
						)
						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session.Err).Should(gbytes.Say("Error: accepts 2 arg\\(s\\), received %d", len(args)))
						Eventually(session).ShouldNot(gexec.Exit(0))
					},
					Entry("0 args"),
					Entry("1 arg", "bar-ns/foo-orb"),
					Entry("3 args", "bar-ns/foo-orb", "true", "true"),
				)
			})
			Context("invalid arguments supplied", func() {
				DescribeTable("when setting the listed status of an orb",
					func(expectedError string, args ...string) {
						argList := []string{"orb", "unlist",
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL()}
						newArgList := append(argList, args...)
						command = exec.Command(pathCLI,
							newArgList...,
						)
						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session.Err).Should(gbytes.Say(expectedError))
						Eventually(session).ShouldNot(gexec.Exit(0))
					},
					Entry("invalid orb name", "Error: Invalid orb foo-orb. Expected a namespace and orb in the form 'namespace/orb'", "foo-orb", "true"),
					Entry("non-boolean value", "Error: expected \"true\" or \"false\", got \"falsey\"", "bar-ns/foo-orb", "falsey"),
				)
			})
		})

		Describe("when listing all orbs", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
				)
			})

			It("sends multiple requests when there are more than 1 page of orbs", func() {
				By("setting up a mock server")

				query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
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

				firstRequest := graphql.NewRequest(query)
				firstRequest.Variables["after"] = ""
				firstRequest.Variables["certifiedOnly"] = true

				firstRequestEncoded, err := firstRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				secondRequest := graphql.NewRequest(query)
				secondRequest.Variables["after"] = "test/test"
				secondRequest.Variables["certifiedOnly"] = true

				secondRequestEncoded, err := secondRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list/first_response.json"))
				firstResponse := string(tmpBytes)

				tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list/second_response.json"))
				secondResponse := string(tmpBytes)

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  firstRequestEncoded.String(),
					Response: firstResponse,
				})
				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  secondRequestEncoded.String(),
					Response: secondResponse,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
				Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(2))
			})

		})

		Describe("when sorting orbs by builds with --sort", func() {
			BeforeEach(func() {
				By("setting up a mock server")

				query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
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

				request := graphql.NewRequest(query)
				request.Variables["after"] = ""
				request.Variables["certifiedOnly"] = true

				encoded, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_sort/response.json"))
				response := string(tmpBytes)

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  encoded.String(),
					Response: response,
				})

			})

			It("should sort by builds", func() {
				By("running the command")
				command = exec.Command(pathCLI,
					"orb", "list",
					"--sort", "builds",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
				)
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				// the orb named "second" actually has more builds
				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`Orbs found: 3. Showing only certified orbs.
Add --uncertified for a list of all orbs.

second (0.8.0)
third (0.9.0)
first (0.7.0)

In order to see more details about each orb, type: ` + "`circleci orb info orb-namespace/orb-name`" + `

Search, filter, and view sources for all Orbs online at https://circleci.com/developer/orbs/
`))
			})

			It("should sort by projects", func() {
				By("running the command")
				command = exec.Command(pathCLI,
					"orb", "list",
					"--sort", "projects",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
				)
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				// the orb named "third" actually has the most projects
				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`Orbs found: 3. Showing only certified orbs.
Add --uncertified for a list of all orbs.

third (0.9.0)
first (0.7.0)
second (0.8.0)

In order to see more details about each orb, type: ` + "`circleci orb info orb-namespace/orb-name`" + `

Search, filter, and view sources for all Orbs online at https://circleci.com/developer/orbs/
`))
			})

			It("should sort by orgs", func() {
				By("running the command")
				command = exec.Command(pathCLI,
					"orb", "list",
					"--sort", "orgs",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
				)
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				// the orb named "second" actually has the most orgs
				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`Orbs found: 3. Showing only certified orbs.
Add --uncertified for a list of all orbs.

second (0.8.0)
first (0.7.0)
third (0.9.0)

In order to see more details about each orb, type: ` + "`circleci orb info orb-namespace/orb-name`" + `

Search, filter, and view sources for all Orbs online at https://circleci.com/developer/orbs/
`))
			})

		})

		Describe("when using --sort with invalid option", func() {
			It("should throw an error", func() {
				By("running the command")
				command = exec.Command(pathCLI,
					"orb", "list",
					"--sort", "idontknow",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
				)
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(clitest.ShouldFail())

				stderr := session.Wait().Err.Contents()
				Expect(string(stderr)).To(Equal("Error: expected `idontknow` to be one of: builds, orgs, projects\n"))
			})
		})

		Describe("when listing all orbs with the --json flag", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
					"--json",
				)
			})
			It("sends multiple requests and groups the results into a single json output", func() {
				By("setting up a mock server")

				query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
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

				firstRequest := graphql.NewRequest(query)
				firstRequest.Variables["after"] = ""
				firstRequest.Variables["certifiedOnly"] = true

				firstRequestEncoded, err := firstRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				secondRequest := graphql.NewRequest(query)
				secondRequest.Variables["after"] = "test/test"
				secondRequest.Variables["certifiedOnly"] = true

				secondRequestEncoded, err := secondRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list/first_response.json"))
				firstResponse := string(tmpBytes)

				tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list/second_response.json"))
				secondResponse := string(tmpBytes)

				tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list/pretty_json_output.json"))
				expectedOutput := string(tmpBytes)

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  firstRequestEncoded.String(),
					Response: firstResponse,
				})
				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  secondRequestEncoded.String(),
					Response: secondResponse,
				})

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
				Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(2))
			})
		})

		Describe("when listing all orbs with --uncertified", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list",
					"--skip-update-check",
					"--uncertified",
					"--host", tempSettings.TestServer.URL(),
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
	    statistics {
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

				firstRequest := graphql.NewRequest(query)
				firstRequest.Variables["after"] = ""
				firstRequest.Variables["certifiedOnly"] = false

				firstRequestEncoded, err := firstRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				secondRequest := graphql.NewRequest(query)
				secondRequest.Variables["after"] = "test/here-we-go"
				secondRequest.Variables["certifiedOnly"] = false

				secondRequestEncoded, err := secondRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_uncertified/first_response.json"))
				firstResponse := string(tmpBytes)

				tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_uncertified/second_response.json"))
				secondResponse := string(tmpBytes)

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  firstRequestEncoded.String(),
					Response: firstResponse,
				})
				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
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

				Eventually(session.Out).Should(gbytes.Say("In order to see more details about each orb, type: `circleci orb info orb-namespace/orb-name`"))
				Eventually(session.Out).Should(gbytes.Say("Search, filter, and view sources for all Orbs online at https://circleci.com/developer/orbs/"))
				Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(2))
			})

			Context("with the --json flag", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "list",
						"--skip-update-check",
						"--uncertified",
						"--host", tempSettings.TestServer.URL(),
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
					Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(2))
				})
			})
		})

		Describe("when listing all orbs with --details", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
					"--details",
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
	    statistics {
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

				request := graphql.NewRequest(query)
				request.Variables["after"] = ""
				request.Variables["certifiedOnly"] = true

				encoded, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_details/response.json"))

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  encoded.String(),
					Response: string(tmpBytes),
				})
			})

			It("lists detailed orbs", func() {
				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`Orbs found: 1. Showing only certified orbs.
Add --uncertified for a list of all orbs.

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
  Statistics:
    - last30DaysBuildCount: 0
    - last30DaysOrganizationCount: 0
    - last30DaysProjectCount: 0

In order to see more details about each orb, type: ` + "`circleci orb info orb-namespace/orb-name`" + `

Search, filter, and view sources for all Orbs online at https://circleci.com/developer/orbs/
`))
				Eventually(session).Should(gexec.Exit(0))
				Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(1))
			})

			Context("with the --json flag", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "list",
						"--skip-update-check",
						"--host", tempSettings.TestServer.URL(),
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
					Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(1))
				})
			})
		})

		Describe("when listing orbs with a namespace argument", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list", "circleci",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
					"--details",
				)
				By("setting up a mock server")

				query := `
query namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {
	registryNamespace(name: $namespace) {
		name
                id
		orbs(first: 20, after: $after, view: $view) {
			edges {
				cursor
				node {
					versions (count: 1){ source, version
					}
					name
	                                statistics {
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
				firstRequest := graphql.NewRequest(query)
				firstRequest.Variables["after"] = ""
				firstRequest.Variables["namespace"] = "circleci"
				firstRequest.Variables["view"] = "PUBLIC_ONLY"

				firstRequestEncoded, err := firstRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				secondRequest := graphql.NewRequest(query)
				secondRequest.Variables["after"] = "circleci/codecov-clojure"
				secondRequest.Variables["namespace"] = "circleci"
				secondRequest.Variables["view"] = "PUBLIC_ONLY"

				secondRequestEncoded, err := secondRequest.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				// These responses are generated from production data,
				// but using a 5-per-page limit instead of the 20 requested.
				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_with_namespace/first_response.json"))
				firstResponse := string(tmpBytes)

				tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_with_namespace/second_response.json"))
				secondResponse := string(tmpBytes)

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  firstRequestEncoded.String(),
					Response: firstResponse,
				})
				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
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
				Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(2))
			})

			Context("with the --json flag", func() {
				BeforeEach(func() {
					command = exec.Command(pathCLI,
						"orb", "list", "circleci",
						"--skip-update-check",
						"--host", tempSettings.TestServer.URL(),
						"--details",
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
					Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(2))
				})
			})
		})

		Describe("when listing orb that doesn't exist", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "list", "nonexist",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
				)

				By("setting up a mock server")

				query := `
query namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {
	registryNamespace(name: $namespace) {
		name
                id
		orbs(first: 20, after: $after, view: $view) {
			edges {
				cursor
				node {
					versions { version
					}
					name
	                                statistics {
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

				request := graphql.NewRequest(query)
				request.Variables["after"] = ""
				request.Variables["namespace"] = "nonexist"
				request.Variables["view"] = "PUBLIC_ONLY"

				encodedRequest, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				mockResponse := `{"data": {}}`

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  encodedRequest.String(),
					Response: mockResponse,
				})
			})

			It("returns an error", func() {
				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("No namespace found"))
				Eventually(session).Should(clitest.ShouldFail())
				Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Describe("when listing private orbs", func() {
			BeforeEach(func() {
				By("setting up a mock server")

				query := `
query namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {
	registryNamespace(name: $namespace) {
		name
                id
		orbs(first: 20, after: $after, view: $view) {
			edges {
				cursor
				node {
					versions { version
					}
					name
	                                statistics {
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

				request := graphql.NewRequest(query)
				request.Variables["after"] = ""
				request.Variables["namespace"] = "circleci"
				request.Variables["view"] = "PRIVATE_ONLY"

				encodedRequest, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_list_with_namespace/second_response.json"))
				mockResponse := string(tmpBytes)

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  encodedRequest.String(),
					Response: mockResponse,
				})
			})

			It("returns an error when private is provided without a namespace", func() {
				command = exec.Command(pathCLI,
					"orb", "list",
					"--private",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
				)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Namespace must be provided when listing private orbs"))
				Eventually(session).Should(clitest.ShouldFail())
				Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(0))
			})
			It("successfully returns private orbs within a given namespace", func() {
				command = exec.Command(pathCLI,
					"orb", "list", "circleci",
					"--private",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
				)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`Orbs found: 5. Showing only private orbs.

circleci/delete-me (Not published)
circleci/delete-me-too (Not published)
circleci/gradle (0.0.1)
circleci/heroku (Not published)
circleci/rollbar (0.0.1)

`))
				Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Describe("when creating an orb without a token", func() {
			BeforeEach(func() {
				command = commandWithHome(pathCLI, tempSettings.Home,
					"orb", "create", "bar-ns/foo-orb",
					"--skip-update-check",
					"--token", "",
				)
			})

			It("instructs the user to run 'circleci setup' and create a new token", func() {
				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say(`Error: please set a token with 'circleci setup'
You can create a new personal API token here:
https://circleci.com/account/api`))
				Eventually(session).Should(clitest.ShouldFail())
			})

			It("uses the host setting from config in the url", func() {
				command = commandWithHome(pathCLI, tempSettings.Home,
					"orb", "create", "bar-ns/foo-orb",
					"--skip-update-check",
					"--token", "",
					"--host", "foo.bar",
				)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say(`Error: please set a token with 'circleci setup'
You can create a new personal API token here:
foo.bar/account/api`))
				Eventually(session).Should(clitest.ShouldFail())
			})
		})

		Describe("when fetching an orb's source", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "source",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
					"my/orb@dev:foo",
				)
			})

			It("works", func() {
				// TODO: factor out common test setup into a top-level JustBeforeEach. Rely
				// on BeforeEach in each block to specify server mocking.
				By("setting up a mock server")

				request := graphql.NewRequest(`query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`)
				request.Variables["orbVersionRef"] = "my/orb@dev:foo"
				encoded, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				response := `{
							"orbVersion": {
								"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
								"version": "dev:foo",
								"orb": {
								        "id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
								},
								"source": "some orb"
							}
						}`

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  encoded.String(),
					Response: response})

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

				query := `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`
				request := graphql.NewRequest(query)
				request.Variables["orbVersionRef"] = "my/orb@dev:foo"
				expected, err := request.Encode()
				Expect(err).ShouldNot(HaveOccurred())

				response := `{"data": { "orbVersion": {} }}`

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expected.String(),
					Response: response,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("no Orb 'my/orb@dev:foo' was found; please check that the Orb reference is correct"))

				Eventually(session).Should(clitest.ShouldFail())
			})
		})

		Describe("when fetching an orb's meta-data", func() {
			var (
				request  *graphql.Request
				query    string
				expected bytes.Buffer
				err      error
			)

			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"orb", "info",
					"--skip-update-check",
					"--host", tempSettings.TestServer.URL(),
					"my/orb@dev:foo",
				)

				query = `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb {
                                    id
                                    createdAt
									name
									namespace {
									  name
									}
                                    categories {
                                      id
                                      name
                                    }
	                            statistics {
		                        last30DaysBuildCount,
		                        last30DaysProjectCount,
		                        last30DaysOrganizationCount
	                            }
                                    versions(count: 200) {
                                        createdAt
                                        version
                                    }
                                }
                                source
                                createdAt
			    }
		      }`

				request = graphql.NewRequest(query)
				request.Variables["orbVersionRef"] = "my/orb@dev:foo"
				expected, err = request.Encode()
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("works", func() {
				response := `{
							"orbVersion": {
								"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
								"version": "dev:foo",
								"orb": {
								        "id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
								        "createdAt": "2018-09-24T08:53:37.086Z",
																																				"name": "my/orb",
																																				"categories": [
																																					{
																																						"id": "cc604b45-b6b0-4b81-ad80-796f15eddf87",
																																						"name": "Infra Automation"
																																					},
																																					{
																																						"id": "dd604b45-b6b0-4b81-ad80-796f15eddf87",
																																						"name": "Testing"
																																					}
																																				],
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
						}`

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expected.String(),
					Response: response,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())

				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`
Latest: my/orb@0.0.1
Last-updated: 2018-10-11T22:12:19.477Z
Created: 2018-09-24T08:53:37.086Z
Total-revisions: 1

Total-commands: 1
Total-executors: 0
Total-jobs: 0

## Statistics (30 days):
Builds: 0
Projects: 0
Orgs: 0

## Categories:
Infra Automation
Testing

Learn more about this orb online in the CircleCI Orb Registry:
https://circleci.com/developer/orbs/orb/my/orb
`))

				Eventually(session).Should(gexec.Exit(0))
			})

			It("reports usage statistics", func() {
				response := `{
							"orbVersion": {
								"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
								"version": "dev:foo",
								"orb": {
								        "id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
								        "createdAt": "2018-09-24T08:53:37.086Z",
                                                                        "name": "my/orb",
                                                                        "statistics": {
                                                                                "last30DaysBuildCount": 555,
                                                                                "last30DaysProjectCount": 777,
                                                                                "last30DaysOrganizationCount": 999
                                                                        },
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
						}`

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expected.String(),
					Response: response,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())

				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`
Latest: my/orb@0.0.1
Last-updated: 2018-10-11T22:12:19.477Z
Created: 2018-09-24T08:53:37.086Z
Total-revisions: 1

Total-commands: 1
Total-executors: 0
Total-jobs: 0

## Statistics (30 days):
Builds: 555
Projects: 777
Orgs: 999

Learn more about this orb online in the CircleCI Orb Registry:
https://circleci.com/developer/orbs/orb/my/orb
`))

				Eventually(session).Should(gexec.Exit(0))
			})

			It("reports when an dev orb hasn't released any semantic versions", func() {
				response := `{
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

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expected.String(),
					Response: response,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())

				stdout := session.Wait().Out.Contents()
				Expect(string(stdout)).To(Equal(`
This orb hasn't published any versions yet.

Total-commands: 1
Total-executors: 0
Total-jobs: 0

## Statistics (30 days):
Builds: 0
Projects: 0
Orgs: 0

Learn more about this orb online in the CircleCI Orb Registry:
https://circleci.com/developer/orbs/orb/my/orb
`))

				Eventually(session).Should(gexec.Exit(0))
			})

			It("reports when an dev orb hasn't released any semantic versions", func() {
				response := `{ "orbVersion": {} }`

				tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expected.String(),
					Response: response,
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())

				Eventually(session.Err).Should(gbytes.Say("no Orb 'my/orb@dev:foo' was found; please check that the Orb reference is correct"))

				Eventually(session).Should(clitest.ShouldFail())
			})
		})

		Describe("list orb categories", func() {
			Context("with mock server", func() {
				DescribeTable("sends multiple requests when there are more than 1 page of orb categories",
					func(json bool) {
						argList := []string{"orb", "list-categories",
							"--skip-update-check",
							"--host", tempSettings.TestServer.URL()}
						if json {
							argList = append(argList, "--json")
						}

						command = exec.Command(pathCLI,
							argList...,
						)

						query := `
	query ListOrbCategories($after: String!) {
		orbCategories(first: 20, after: $after) {
			totalCount
			edges {
				cursor
				node {
					id
					name
				}
			}
			pageInfo {
				hasNextPage
			}
		}
	}
`

						firstRequest := graphql.NewRequest(query)
						firstRequest.Variables["after"] = ""

						firstRequestEncoded, err := firstRequest.Encode()
						Expect(err).ShouldNot(HaveOccurred())

						secondRequest := graphql.NewRequest(query)
						secondRequest.Variables["after"] = "Testing"

						secondRequestEncoded, err := secondRequest.Encode()
						Expect(err).ShouldNot(HaveOccurred())

						tmpBytes := golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_category_list/first_response.json"))
						firstResponse := string(tmpBytes)

						tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_category_list/second_response.json"))
						secondResponse := string(tmpBytes)

						tmpBytes = golden.Get(GinkgoT(), filepath.FromSlash("gql_orb_category_list/pretty_json_output.json"))
						expectedOutput := string(tmpBytes)

						tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  firstRequestEncoded.String(),
							Response: firstResponse,
						})
						tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  secondRequestEncoded.String(),
							Response: secondResponse,
						})

						By("running the command")
						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session).Should(gexec.Exit(0))
						Expect(tempSettings.TestServer.ReceivedRequests()).Should(HaveLen(2))

						displayedOutput := string(session.Wait().Out.Contents())
						if json {
							Expect(displayedOutput).Should(MatchJSON(expectedOutput))
						} else {
							Expect(displayedOutput).To(Equal(`Artifacts/Registry
Build
Cloud Platform
Code Analysis
Collaboration
Containers
Deployment
Infra Automation
Kubernetes
Language/Framework
Monitoring
Notifications
Reporting
Security
Testing
Windows Server 2003
Windows Server 2010
`))
						}
					},
					Entry("with --json", true),
					Entry("without --json", false),
				)
			})
		})

		Describe("Add/remove orb categorization", func() {
			var (
				orbId            string
				orbNamespaceName string
				orbFullName      string
				orbName          string
				categoryId       string
				categoryName     string
			)

			BeforeEach(func() {
				orbId = "bb604b45-b6b0-4b81-ad80-796f15eddf87"
				orbNamespaceName = "bar-ns"
				orbName = "foo-orb"
				orbFullName = orbNamespaceName + "/" + orbName
				categoryId = "cc604b45-b6b0-4b81-ad80-796f15eddf87"
				categoryName = "Cloud Platform"
			})

			Context("with mock server", func() {
				DescribeTable("add/remove a valid orb to/from a valid category",
					func(mockErrorResponse bool, updateType api.UpdateOrbCategorizationRequestType) {
						commandName := "add-to-category"
						operationName := "addCategorizationToOrb"
						expectedOutputSegment := "added to"
						if updateType == api.Remove {
							commandName = "remove-from-category"
							operationName = "removeCategorizationFromOrb"
							expectedOutputSegment = "removed from"
						}

						command = exec.Command(pathCLI,
							"orb", commandName,
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL(),
							orbFullName, categoryName)

						gqlOrbIDResponse := fmt.Sprintf(`{
						"orb": {
								"id": "%s"
						}
					}`, orbId)

						expectedOrbIDRequest := fmt.Sprintf(`{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
						"variables": {
							"name": "%s",
							"namespace": "%s"
						}
					}`, orbFullName, orbNamespaceName)

						expectedCategoryIDRequest := fmt.Sprintf(`{
							"query": "\n\tquery ($name: String!) {\n\t\torbCategoryByName(name: $name) {\n\t\t  id\n\t\t}\n\t}",
							"variables": {
								"name": "%s"
							}
						}`, categoryName)

						gqlCategoryIDResponse := fmt.Sprintf(`{
							"orbCategoryByName": {
									"id": "%s"
							}
						}`, categoryId)

						expectedOrbCategorizationRequest := fmt.Sprintf(`{
							"query": "\n\t\tmutation($orbId: UUID!, $categoryId: UUID!) {\n\t\t\t%s(\n\t\t\t\torbId: $orbId,\n\t\t\t\tcategoryId: $categoryId\n\t\t\t) {\n\t\t\t\torbId\n\t\t\t\tcategoryId\n\t\t\t\terrors {\n\t\t\t\t\tmessage\n\t\t\t\t\ttype\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t",
							"variables": {
								"categoryId": "%s",
								"orbId": "%s"
							}
						}`, operationName, categoryId, orbId)

						gqlCategorizationResponse := fmt.Sprintf(`{
							"%s": {
								"orbId": "%s",
								"categoryId": "%s",
								"errors": []
							}
						}`, operationName, orbId, categoryId)

						if mockErrorResponse {
							gqlCategorizationResponse = fmt.Sprintf(`{
								"%s": {
									"orbId": "",
									"categoryId": "",
									"errors": [{
										"message": "Mock error message",
										"type": "Mock error from server"
									}]
								}
							}`, operationName)
						}

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedOrbIDRequest,
							Response: gqlOrbIDResponse})

						tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedCategoryIDRequest,
							Response: gqlCategoryIDResponse})

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedOrbCategorizationRequest,
							Response: gqlCategorizationResponse})

						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
						Expect(err).ShouldNot(HaveOccurred())

						stdout := session.Wait().Out.Contents()
						if mockErrorResponse {
							Eventually(session).Should(clitest.ShouldFail())
							errorMsg := fmt.Sprintf(`Error: Failed to add orb %s to category %s: Mock error message`, orbFullName, categoryName)
							if updateType == api.Remove {
								errorMsg = fmt.Sprintf(`Error: Failed to remove orb %s from category %s: Mock error message`, orbFullName, categoryName)
							}
							Eventually(session.Err).Should(gbytes.Say(errorMsg))
						} else {
							Eventually(session).Should(gexec.Exit(0))
							Expect(string(stdout)).To(ContainSubstring(fmt.Sprintf(`%s is successfully %s the "%s" category.`, orbFullName, expectedOutputSegment, categoryName)))
						}
					},
					Entry("add categorization success", false, api.Add),
					Entry("remove categorization success", false, api.Remove),
					Entry("server error on adding categorization", true, api.Add),
					Entry("server error on removing categorization", true, api.Remove),
				)
			})
			Context("with mock server", func() {
				DescribeTable("orb does not exist",
					func(updateType api.UpdateOrbCategorizationRequestType) {
						commandName := "add-to-category"
						if updateType == api.Remove {
							commandName = "remove-from-category"
						}

						command = exec.Command(pathCLI,
							"orb", commandName,
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL(),
							orbFullName, categoryName)

						expectedOrbIDRequest := fmt.Sprintf(`{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
						"variables": {
							"name": "%s",
							"namespace": "%s"
						}
					}`, orbFullName, orbNamespaceName)

						gqlOrbIDResponse := `{
						"orb": null,
						"registryNamespace": {
							"id": "eac63dee-9960-48c2-b763-612e1683194e"
						}
					}`

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedOrbIDRequest,
							Response: gqlOrbIDResponse})

						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session).Should(clitest.ShouldFail())
						errorMsg := fmt.Sprintf(`Error: Failed to add orb %s to category %s: the '%s' orb does not exist in the '%s' namespace. Did you misspell the namespace or the orb name?`, orbFullName, categoryName, orbName, orbNamespaceName)
						if updateType == api.Remove {
							errorMsg = fmt.Sprintf(`Error: Failed to remove orb %s from category %s: the '%s' orb does not exist in the '%s' namespace. Did you misspell the namespace or the orb name?`, orbFullName, categoryName, orbName, orbNamespaceName)
						}
						Eventually(session.Err).Should(gbytes.Say(errorMsg))
					},
					Entry("add categorization to non-existent orb", api.Add),
					Entry("remove categorization to non-existent orb", api.Remove),
				)
			})
			Context("with mock server", func() {
				DescribeTable("category does not exist",
					func(updateType api.UpdateOrbCategorizationRequestType) {
						commandName := "add-to-category"
						if updateType == api.Remove {
							commandName = "remove-from-category"
						}

						command = exec.Command(pathCLI,
							"orb", commandName,
							"--skip-update-check",
							"--token", token,
							"--host", tempSettings.TestServer.URL(),
							orbFullName, categoryName)

						expectedOrbIDRequest := fmt.Sprintf(`{
						"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
						"variables": {
							"name": "%s",
							"namespace": "%s"
						}
					}`, orbFullName, orbNamespaceName)

						gqlOrbIDResponse := fmt.Sprintf(`{
							"orb": {
									"id": "%s"
							}
						}`, orbId)

						expectedCategoryIDRequest := fmt.Sprintf(`{
							"query": "\n\tquery ($name: String!) {\n\t\torbCategoryByName(name: $name) {\n\t\t  id\n\t\t}\n\t}",
							"variables": {
								"name": "%s"
							}
						}`, categoryName)

						gqlCategoryIDResponse := `{
							"orbCategoryByName": null
						}`

						tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedOrbIDRequest,
							Response: gqlOrbIDResponse})

						tempSettings.AppendPostHandler("", clitest.MockRequestResponse{
							Status:   http.StatusOK,
							Request:  expectedCategoryIDRequest,
							Response: gqlCategoryIDResponse})

						session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
						Expect(err).ShouldNot(HaveOccurred())
						Eventually(session).Should(clitest.ShouldFail())
						errorCause := fmt.Sprintf(`the '%s' category does not exist. Did you misspell the category name? To see the list of category names, please run 'circleci orb list-categories'.`, categoryName)
						errorMsg := fmt.Sprintf(`Error: Failed to add orb %s to category %s: %s`, orbFullName, categoryName, errorCause)
						if updateType == api.Remove {
							errorMsg = fmt.Sprintf(`Error: Failed to remove orb %s from category %s: %s`, orbFullName, categoryName, errorCause)
						}
						stderr := session.Wait().Err.Contents()
						Expect(string(stderr)).To(ContainSubstring(errorMsg))
					},
					Entry("add orb to non-existent category", api.Add),
					Entry("remove orb to non-existent category", api.Remove),
				)
			})
		})
	})

	Describe("Orb pack", func() {
		var (
			tempSettings *clitest.TempSettings
			orb          *clitest.TmpFile
			script       *clitest.TmpFile
			command      *exec.Cmd
		)
		BeforeEach(func() {
			tempSettings = clitest.WithTempSettings()
			orb = clitest.OpenTmpFile(tempSettings.Home, filepath.Join("commands", "orb.yml"))
			clitest.OpenTmpFile(tempSettings.Home, "@orb.yml")
			orb.Write([]byte(`steps:
    - run:
        name: Say hello
        command: <<include(scripts/script.sh)>>

examples:
    example:
        description: |
            An example of how to use the orb.
        usage:
            version: 2.1
            orbs:
                orb-name: company/orb-name@1.2.3
            setup: true
            workflows:
                create-pipeline:
                    jobs:
                        orb-name: create-pipeline-x
`))
			script = clitest.OpenTmpFile(tempSettings.Home, filepath.Join("scripts", "script.sh"))
			script.Write([]byte(`echo Hello, world!`))
			command = exec.Command(pathCLI,
				"orb", "pack",
				"--skip-update-check",
				tempSettings.Home,
			)
		})

		AfterEach(func() {
			tempSettings.Close()
			orb.Close()
			script.Close()
		})

		It("Includes a script in the packed Orb file", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Out).Should(gbytes.Say(`steps:
            - run:
                command: echo Hello, world!
                name: Say hello
`))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("Includes the setup key when an orb example uses a dynamic pipeline", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Out).Should(gbytes.Say(`orbs:
                        orb-name: company/orb-name@1.2.3
                    setup: true
                    version: 2.1
                    workflows:
`))
			Eventually(session).Should(gexec.Exit(0))
		})

	})

	Describe("Orb diff", func() {
		var (
			token        string
			tempSettings *clitest.TempSettings
			command      *exec.Cmd
		)

		BeforeEach(func() {
			token = "testtoken"
			tempSettings = clitest.WithTempSettings()
		})

		AfterEach(func() {
			tempSettings.Close()
		})

		DescribeTable("Shows the expected diff", func(source1, source2, expected, color string) {
			orbName := "somenamespace/someorb"
			version1 := "1.0.0"
			orb1 := fmt.Sprintf("%s@%s", orbName, version1)
			version2 := "2.0.0"
			orb2 := fmt.Sprintf("%s@%s", orbName, version2)
			command = exec.Command(pathCLI, "orb", "diff", orbName, version1, version2,
				"--token", token,
				"--host", tempSettings.TestServer.URL())

			mockOrbSource(source1, orb1, token, tempSettings)
			mockOrbSource(source2, orb2, token, tempSettings)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Out).WithTimeout(5 * time.Second).Should(gbytes.Say(expected))
			Eventually(session).Should(gexec.Exit(0))
		},
			Entry("Detect identical sources", "orb-source", "orb-source", "No diff found", "auto"),
			Entry(
				"Detect difference",
				"line1\\nline3\\n",
				"line1\\nline2\\n",
				`--- somenamespace/someorb@1.0.0
\+\+\+ somenamespace/someorb@2.0.0
@@ -1,2 \+1,2 @@
 line1
-line3
\+line2`,
				"auto",
			),
		)
	})
})

func mockOrbSource(source, orbVersion, token string, tempSettings *clitest.TempSettings) {
	requestStruct := struct {
		Query     string `json:"query"`
		Variables struct {
			OrbVersionRef string `json:"orbVersionRef"`
		} `json:"variables"`
	}{
		Query: `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`,
		Variables: struct {
			OrbVersionRef string `json:"orbVersionRef"`
		}{OrbVersionRef: orbVersion},
	}
	request, err := json.Marshal(requestStruct)
	Expect(err).ToNot(HaveOccurred())
	response := fmt.Sprintf(`{
	"orbVersion": {
			"id": "some-id",
			"version": "some-version",
			"orb": { "id": "some-id" },
			"source": "%s"
	}
}`, source)
	tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
		Status:   http.StatusOK,
		Request:  string(request),
		Response: response,
	})
}

func mockOrbIntrospection(isValid bool, token string, tempSettings *clitest.TempSettings) {
	args := []map[string]interface{}{
		{
			"name": "orbYaml",
		},
	}
	if isValid {
		args = append(args, map[string]interface{}{
			"name": "ownerId",
		})
	}

	responseStruct := map[string]interface{}{
		"__schema": map[string]interface{}{
			"queryType": map[string]interface{}{
				"fields": []map[string]interface{}{
					{
						"name": "orbConfig",
						"args": args,
					},
				},
			},
		},
	}
	response, err := json.Marshal(responseStruct)
	Expect(err).ToNot(HaveOccurred())

	requestStruct := map[string]interface{}{
		"query": `
query ValidateOrb {
  __schema {
    queryType {
      fields(includeDeprecated: true) {
        name
        args {
          name
          __typename
          type {
            name
          }
        }
      }
    }
  }
}`,
		"variables": map[string]interface{}{},
	}
	request, err := json.Marshal(requestStruct)
	Expect(err).ToNot(HaveOccurred())

	tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
		Status:   http.StatusOK,
		Request:  string(request),
		Response: string(response),
	})
}
