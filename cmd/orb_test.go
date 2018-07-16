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

var _ = Describe("Orb", func() {
	Describe("with an api and orb.yml", func() {
		var (
			testServer *ghttp.Server
			orb        tmpFile
			token      string
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
				token = "testtoken"
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

				appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

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
				appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: invalid_orb"))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})

		Describe("when expanding orb", func() {
			var (
				token   string
				command *exec.Cmd
			)

			BeforeEach(func() {
				token = "testtoken"
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

				appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

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

				appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say("Error: error1: error2"))
				Eventually(session).ShouldNot(gexec.Exit(0))

			})
		})

		Describe("when publishing an orb version", func() {
			JustBeforeEach(func() {
				token = "testtoken"
				command = exec.Command(pathCLI,
					"orb", "publish",
					"-t", token,
					"-e", testServer.URL(),
					"-p", orb.Path,
					"--verbose", "",
				)
			})

			FIt("works", func() {

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
						"valid": true,
						"orb": {
							"createdAt": "2018-07-16T18:03:18.961Z",
							"version": "0.0.1"
						}
					}
				}`

				// expectedRequestJson := `{
				// 	"query": '{}", version: "0.0.1") { orb { version createdAt } errors { message } } } '
				// 	}`

				expectedRequestJson := `{
					"query": "\n\t\tmutation($config: String!, $orbId: $String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbId: $orbId,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: \"\"\n\t\t\t)\n\t\t}\n\t",
					"variables": {
						"config": "some orb",
						"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
					}
				}`

				appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out).Should(gbytes.Say("response invalid"))
				Eventually(session).Should(gexec.Exit(0))
			})

			It("prints errors if invalid", func() {
			})
		})
	})
})
