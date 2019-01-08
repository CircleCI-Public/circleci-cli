package cmd_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"gotest.tools/golden"
)

var _ = Describe("Config", func() {
	Describe("with an api and config.yml", func() {
		var tempSettings *clitest.TempSettings

		BeforeEach(func() {
			tempSettings = clitest.WithTempSettings()
		})

		AfterEach(func() {
			tempSettings.Cleanup()
		})

		Describe("when validating config", func() {
			var (
				token   string
				config  *clitest.TmpFile
				command *exec.Cmd
			)

			BeforeEach(func() {
				config = clitest.OpenTmpFile(tempSettings.Home, ".circleci/config.yaml")

				token = "testtoken"
				command = exec.Command(pathCLI,
					"config", "validate",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					config.Path,
				)
			})

			It("works", func() {
				config.Write([]byte(`some config`))

				gqlResponse := `{
							"buildConfig": {
								"sourceYaml": "hello world",
								"valid": true,
								"errors": []
							}
						}`

				expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateConfig ($config: String!) {\n\t\t\tbuildConfig(configYaml: $config) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
					  "config": "some config"
					}
				  }`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedRequestJson,
					Response: gqlResponse,
				})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("Config file at .*circleci/config.yaml is valid"))
				Eventually(session).Should(gexec.Exit(0))
			})

			It("prints errors if invalid", func() {
				config.Write([]byte(`some config`))

				gqlResponse := `{
							"buildConfig": {
								"sourceYaml": "hello world",
								"valid": false,
								"errors": [
									{"message": "invalid_config"}
								]
							}
						}`

				expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateConfig ($config: String!) {\n\t\t\tbuildConfig(configYaml: $config) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
					  "config": "some config"
					}
				  }`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedRequestJson,
					Response: gqlResponse,
				})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error:"))
				Eventually(session.Err).Should(gbytes.Say("invalid_config"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})

		Describe("when processing config", func() {
			var (
				token   string
				config  *clitest.TmpFile
				command *exec.Cmd
			)

			BeforeEach(func() {
				config = clitest.OpenTmpFile(tempSettings.Home, ".circleci/config.yaml")

				token = "testtoken"
				command = exec.Command(pathCLI,
					"config", "process",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					config.Path,
				)
			})

			It("works", func() {
				config.Write([]byte(`some config`))

				gqlResponse := `{
							"buildConfig": {
								"outputYaml": "hello world",
								"valid": true,
								"errors": []
							}
						}`

				expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateConfig ($config: String!) {\n\t\t\tbuildConfig(configYaml: $config) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
					  "config": "some config"
					}
				  }`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedRequestJson,
					Response: gqlResponse,
				})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("hello world"))
				Eventually(session).Should(gexec.Exit(0))
			})

			It("prints errors if invalid", func() {
				config.Write([]byte(`some config`))

				gqlResponse := `{
							"buildConfig": {
								"outputYaml": "hello world",
								"valid": false,
								"errors": [
									{"message": "error1"},
									{"message": "error2"}
								]
							}
						}`

				expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateConfig ($config: String!) {\n\t\t\tbuildConfig(configYaml: $config) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
					  "config": "some config"
					}
				  }`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedRequestJson,
					Response: gqlResponse,
				})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})
	})

	Describe("pack", func() {
		var (
			command      *exec.Cmd
			results      []byte
			tempSettings *clitest.TempSettings
		)

		BeforeEach(func() {
			tempSettings = clitest.WithTempSettings()
		})

		AfterEach(func() {
			tempSettings.Cleanup()
		})

		Describe("a .circleci folder with config.yml and local orbs folder containing the hugo orb", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					"testdata/hugo-pack/.circleci")
				results = golden.Get(GinkgoT(), filepath.FromSlash("hugo-pack/result.yml"))
			})

			It("pack all YAML contents as expected", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				session.Wait()
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out.Contents()).Should(MatchYAML(results))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("local orbs folder with mixed inline and local commands, jobs, etc", func() {
			BeforeEach(func() {
				var path string = "nested-orbs-and-local-commands-etc"
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					filepath.Join("testdata", path, "test"))
				results = golden.Get(GinkgoT(), filepath.FromSlash(fmt.Sprintf("%s/result.yml", path)))
			})

			It("pack all YAML contents as expected", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				session.Wait()
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out.Contents()).Should(MatchYAML(results))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("an orb containing local executors and commands in folder", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					"testdata/myorb/test")

				results = golden.Get(GinkgoT(), filepath.FromSlash("myorb/result.yml"))
			})

			It("pack all YAML contents as expected", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				session.Wait()
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out.Contents()).Should(MatchYAML(results))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("with a large nested config including rails orb", func() {
			BeforeEach(func() {
				var path string = "test-with-large-nested-rails-orb"
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					filepath.Join("testdata", path, "test"))
				results = golden.Get(GinkgoT(), filepath.FromSlash(fmt.Sprintf("%s/result.yml", path)))
			})

			It("pack all YAML contents as expected", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				session.Wait()
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out.Contents()).Should(MatchYAML(results))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("config is a list and not a map", func() {
			var config *clitest.TmpFile

			BeforeEach(func() {
				config = clitest.OpenTmpFile(filepath.Join(tempSettings.Home, "myorb"), "config.yaml")

				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					config.RootDir,
				)
			})

			It("prints an error about invalid YAML", func() {
				config.Write([]byte(`[]`))

				expected := fmt.Sprintf("Error: Failed trying to marshal the tree to YAML : expected a map, got a `[]interface {}` which is not supported at this time for \"%s\"\n", config.Path)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())

				stderr := session.Wait().Err.Contents()
				Expect(string(stderr)).To(Equal(expected))
				Eventually(session).Should(gexec.Exit(255))
			})
		})
	})
})
