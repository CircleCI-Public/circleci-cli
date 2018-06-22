package cmd_test

import (
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

func appendPostHandler(server *ghttp.Server, authToken string, statusCode int, expectedRequestJson string, responseBody string) {
	server.AppendHandlers(
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/"),
			ghttp.VerifyHeader(http.Header{
				"Authorization": []string{authToken},
			}),
			ghttp.VerifyContentType("application/json; charset=utf-8"),
			// From Gomegas ghttp.VerifyJson to avoid the
			// VerifyContentType("application/json") check
			// that fails with "application/json; charset=utf-8"
			func(w http.ResponseWriter, req *http.Request) {
				body, err := ioutil.ReadAll(req.Body)
				req.Body.Close()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(body).Should(MatchJSON(expectedRequestJson), "JSON Mismatch")
			},
			ghttp.RespondWith(statusCode, `{ "data": `+responseBody+`}`),
		),
	)
}

type configYaml struct {
	TempHome string
	Path     string
	YamlFile *os.File
}

func openConfigYaml() (configYaml, error) {
	var (
		config configYaml = configYaml{}
		err    error
	)

	const (
		configDir  = ".circleci"
		configFile = "config.yaml"
	)

	tempHome, err := ioutil.TempDir("", "circleci-cli-test-")
	if err != nil {
		return config, err
	}

	err = os.Mkdir(filepath.Join(tempHome, configDir), 0700)
	if err != nil {
		return config, err
	}

	config.Path = filepath.Join(tempHome, configDir, configFile)

	var file *os.File
	file, err = os.OpenFile(
		config.Path,
		os.O_RDWR|os.O_CREATE,
		0600,
	)
	if err != nil {
		return config, err
	}

	config.YamlFile = file

	return config, nil
}

var _ = Describe("Config", func() {
	Describe("with an api and config.yml", func() {
		var (
			testServer *ghttp.Server
			config     configYaml
		)

		BeforeEach(func() {
			var err error
			config, err = openConfigYaml()
			Expect(err).ToNot(HaveOccurred())

			testServer = ghttp.NewServer()
		})

		AfterEach(func() {
			config.YamlFile.Close()
			os.RemoveAll(config.TempHome)

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
					"-p", config.Path,
				)
			})

			It("works", func() {
				_, err := config.YamlFile.Write([]byte(`some config`))
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
				_, err := config.YamlFile.Write([]byte(`some config`))
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
				Eventually(session.Err).Should(gbytes.Say("-- invalid_config"))
				Eventually(session).Should(gexec.Exit(255))

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
					"-p", config.Path,
				)
			})

			It("works", func() {
				_, err := config.YamlFile.Write([]byte(`some config`))
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
				_, err := config.YamlFile.Write([]byte(`some config`))
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
				Eventually(session.Err).Should(gbytes.Say("Error:"))
				Eventually(session.Err).Should(gbytes.Say("-- error1,"))
				Eventually(session.Err).Should(gbytes.Say("-- error2,"))
				Eventually(session).Should(gexec.Exit(255))

			})
		})
	})
})
