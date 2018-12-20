package cmd_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Config", func() {
	Describe("with an api and config.yml", func() {
		var (
			testServer *ghttp.Server
			config     tmpFile
			tmpDir     string
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = openTmpDir("")
			Expect(err).ToNot(HaveOccurred())

			config, err = openTmpFile(tmpDir, filepath.Join(".circleci", "config.yaml"))
			Expect(err).ToNot(HaveOccurred())

			testServer = ghttp.NewServer()
		})

		AfterEach(func() {
			config.close()
			os.RemoveAll(tmpDir)
			testServer.Close()
		})

		Describe("when validating config", func() {
			var (
				token   string
				command *exec.Cmd
			)

			BeforeEach(func() {
				token = "testtoken"
				command = exec.Command(pathCLI,
					"config", "validate",
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
					config.Path,
				)
			})

			It("works", func() {
				err := config.write(`some config`)
				Expect(err).ToNot(HaveOccurred())

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

				appendPostHandler(testServer, token, MockRequestResponse{
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
				err := config.write(`some config`)
				Expect(err).ToNot(HaveOccurred())

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

				appendPostHandler(testServer, token, MockRequestResponse{
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
				command *exec.Cmd
			)

			BeforeEach(func() {
				token = "testtoken"
				command = exec.Command(pathCLI,
					"config", "process",
					"--skip-update-check",
					"--token", token,
					"--host", testServer.URL(),
					config.Path,
				)
			})

			It("works", func() {
				err := config.write(`some config`)
				Expect(err).ToNot(HaveOccurred())

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

				appendPostHandler(testServer, token, MockRequestResponse{
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
				err := config.write(`some config`)
				Expect(err).ToNot(HaveOccurred())

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

				appendPostHandler(testServer, token, MockRequestResponse{
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
			command *exec.Cmd
			results []byte
		)

		Describe("a .circleci folder with config.yml and local orbs folder containing the hugo orb", func() {
			BeforeEach(func() {
				var err error
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					"testdata/hugo-pack/.circleci")
				results, err = ioutil.ReadFile("testdata/hugo-pack/result.yml")
				Expect(err).ShouldNot(HaveOccurred())
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
				var err error
				var path string = "nested-orbs-and-local-commands-etc"
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					filepath.Join("testdata", path, "test"))
				results, err = ioutil.ReadFile(filepath.Join("testdata", path, "result.yml"))
				Expect(err).ShouldNot(HaveOccurred())
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
				var err error
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					"testdata/myorb/test")
				results, err = ioutil.ReadFile("testdata/myorb/result.yml")
				Expect(err).ShouldNot(HaveOccurred())
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
				var err error
				var path string = "test-with-large-nested-rails-orb"
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					filepath.Join("testdata", path, "test"))
				results, err = ioutil.ReadFile(filepath.Join("testdata", path, "result.yml"))
				Expect(err).ShouldNot(HaveOccurred())
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
			var (
				config tmpFile
				tmpDir string
			)

			BeforeEach(func() {
				var err error
				tmpDir, err = openTmpDir("")
				Expect(err).ToNot(HaveOccurred())

				config, err = openTmpFile(tmpDir, filepath.Join(".circleci", "config.yaml"))
				Expect(err).ToNot(HaveOccurred())

				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					filepath.Join(tmpDir, ".circleci"),
				)
			})

			It("prints an error about invalid YAML", func() {
				err := config.write(`[]`)
				Expect(err).ShouldNot(HaveOccurred())

				fullPath := filepath.Join(tmpDir, ".circleci", "config.yaml")
				expected := fmt.Sprintf("Error: Failed trying to marshal the tree to YAML : expected a map, got an array for %s\n", fullPath)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())

				stderr := session.Wait().Err.Contents()
				Expect(string(stderr)).To(Equal(expected))
				Eventually(session).Should(gexec.Exit(255))
			})
		})
	})
})
