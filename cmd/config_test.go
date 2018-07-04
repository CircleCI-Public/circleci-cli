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

var _ = Describe("Config", func() {
	Describe("with an api and config.yml", func() {
		var (
			testServer *ghttp.Server
			config     tmpFile
		)

		BeforeEach(func() {
			var err error
			config, err = openTmpFile(filepath.Join(".circleci", "config.yaml"))
			Expect(err).ToNot(HaveOccurred())

			testServer = ghttp.NewServer()
		})

		AfterEach(func() {
			config.close()
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
					"-t", token,
					"-e", testServer.URL(),
					"-c", config.Path,
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

				appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

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

				appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error:"))
				Eventually(session.Err).Should(gbytes.Say("invalid_config"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})

		Describe("when expanding config", func() {
			var (
				token   string
				command *exec.Cmd
			)

			BeforeEach(func() {
				token = "testtoken"
				command = exec.Command(pathCLI,
					"config", "expand",
					"-t", token,
					"-e", testServer.URL(),
					"-c", config.Path,
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

				appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

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

				appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1: error2"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})
	})
})
