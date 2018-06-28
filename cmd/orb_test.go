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

// func appendPostHandler(server *ghttp.Server, authToken string, statusCode int, expectedRequestJson string, responseBody string) {
// 	server.AppendHandlers(
// 		ghttp.CombineHandlers(
// 			ghttp.VerifyRequest("POST", "/"),
// 			ghttp.VerifyHeader(http.Header{
// 				"Authorization": []string{authToken},
// 			}),
// 			ghttp.VerifyContentType("application/json; charset=utf-8"),
// 			// From Gomegas ghttp.VerifyJson to avoid the
// 			// VerifyContentType("application/json") check
// 			// that fails with "application/json; charset=utf-8"
// 			func(w http.ResponseWriter, req *http.Request) {
// 				body, err := ioutil.ReadAll(req.Body)
// 				req.Body.Close()
// 				Expect(err).ShouldNot(HaveOccurred())
// 				Expect(body).Should(MatchJSON(expectedRequestJson), "JSON Mismatch")
// 			},
// 			ghttp.RespondWith(statusCode, `{ "data": `+responseBody+`}`),
// 		),
// 	)
// }

type orbYaml struct {
	TempHome string
	Path     string
	YamlFile *os.File
}

func openOrbYaml() (orbYaml, error) {
	var (
		orb orbYaml = orbYaml{}
		err error
	)

	const (
		orbDir  = "myorb"
		orbFile = "orb.yml"
	)

	tempHome, err := ioutil.TempDir("", "circleci-cli-test-")
	if err != nil {
		return orb, err
	}

	err = os.Mkdir(filepath.Join(tempHome, orbDir), 0700)
	if err != nil {
		return orb, err
	}

	orb.Path = filepath.Join(tempHome, orbDir, orbFile)

	var file *os.File
	file, err = os.OpenFile(
		orb.Path,
		os.O_RDWR|os.O_CREATE,
		0600,
	)
	if err != nil {
		return orb, err
	}

	orb.YamlFile = file

	return orb, nil
}

var _ = Describe("Orb", func() {
	Describe("with an api and orb.yml", func() {
		var (
			testServer *ghttp.Server
			orb        orbYaml
		)

		BeforeEach(func() {
			var err error
			orb, err = openOrbYaml()
			Expect(err).ToNot(HaveOccurred())

			testServer = ghttp.NewServer()
		})

		AfterEach(func() {
			orb.YamlFile.Close()
			os.RemoveAll(orb.TempHome)

			testServer.Close()
		})

		Describe("when validating orb", func() {
			var (
				token   string
				command *exec.Cmd
			)

			BeforeEach(func() {
				token = "testtoken"
				fmt.Fprintln(os.Stderr, "******************** Path CLI")
				fmt.Fprintln(os.Stderr, pathCLI)
				fmt.Fprintln(os.Stderr, token)
				fmt.Fprintln(os.Stderr, testServer.URL())
				fmt.Fprintln(os.Stderr, orb.Path)
				command = exec.Command(pathCLI,
					"orb", "validate",
					"-t", token,
					"-e", testServer.URL(),
					"-p", orb.Path,
				)
			})

			FIt("works", func() {
				_, err := orb.YamlFile.Write([]byte(`{}`))
				Expect(err).ToNot(HaveOccurred())

				gqlResponse := `{
							"orbConfig": {
								"sourceYaml": "{}",
								"valid": true,
								"errors": []
							}
						}`

				expectedRequestJson := ` {
					"query": "\n\t\tquery ValidateOrb ($orb: String!) {\n\t\t\torbConfig(orbYaml: $orb) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
					"variables": {
						"orb": "{}"
					}
				}`

				appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
                                // the .* is because the full path with temp dir is printed
				Eventually(session.Out).Should(gbytes.Say("Orb at .*myorb/orb.yml is valid"))
				Eventually(session).Should(gexec.Exit(0))
			})

			// It("prints errors if invalid", func() {
			// 	_, err := orb.YamlFile.Write([]byte(`some orb`))
			// 	Expect(err).ToNot(HaveOccurred())

			// 	gqlResponse := `{
			// 				"buildConfig": {
			// 					"sourceYaml": "hello world",
			// 					"valid": false,
			// 					"errors": [
			// 						{"message": "invalid_orb"}
			// 					]
			// 				}
			// 			}`

			// 	expectedRequestJson := ` {
			// 		"query": "\n\t\tquery ValidateConfig ($orb: String!) {\n\t\t\tbuildConfig(orbYaml: $orb) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
			// 		"variables": {
			// 		  "orb": "some orb"
			// 		}
			// 	  }`
			// 	appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

			// 	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			// 	Expect(err).ShouldNot(HaveOccurred())
			// 	Eventually(session.Err).Should(gbytes.Say("Error:"))
			// 	Eventually(session.Err).Should(gbytes.Say("-- invalid_orb"))
			// 	Eventually(session).Should(gexec.Exit(255))

			// })
		})

		// Describe("when expanding orb", func() {
		// 	var (
		// 		token   string
		// 		command *exec.Cmd
		// 	)

		// 	BeforeEach(func() {
		// 		token = "testtoken"
		// 		command = exec.Command(pathCLI,
		// 			"orb", "expand",
		// 			"-t", token,
		// 			"-e", testServer.URL(),
		// 			"-p", orb.Path,
		// 		)
		// 	})

		// 	It("works", func() {
		// 		_, err := orb.YamlFile.Write([]byte(`some orb`))
		// 		Expect(err).ToNot(HaveOccurred())

		// 		gqlResponse := `{
		// 					"buildConfig": {
		// 						"outputYaml": "hello world",
		// 						"valid": true,
		// 						"errors": []
		// 					}
		// 				}`

		// 		expectedRequestJson := ` {
		// 			"query": "\n\t\tquery ValidateConfig ($orb: String!) {\n\t\t\tbuildConfig(orbYaml: $orb) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
		// 			"variables": {
		// 			  "orb": "some orb"
		// 			}
		// 		  }`

		// 		appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

		// 		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

		// 		Expect(err).ShouldNot(HaveOccurred())
		// 		Eventually(session.Out).Should(gbytes.Say("hello world"))
		// 		Eventually(session).Should(gexec.Exit(0))
		// 	})

		// 	It("prints errors if invalid", func() {
		// 		_, err := orb.YamlFile.Write([]byte(`some orb`))
		// 		Expect(err).ToNot(HaveOccurred())

		// 		gqlResponse := `{
		// 					"buildConfig": {
		// 						"outputYaml": "hello world",
		// 						"valid": false,
		// 						"errors": [
		// 							{"message": "error1"},
		// 							{"message": "error2"}
		// 						]
		// 					}
		// 				}`

		// 		expectedRequestJson := ` {
		// 			"query": "\n\t\tquery ValidateConfig ($orb: String!) {\n\t\t\tbuildConfig(orbYaml: $orb) {\n\t\t\t\tvalid,\n\t\t\t\terrors { message },\n\t\t\t\tsourceYaml,\n\t\t\t\toutputYaml\n\t\t\t}\n\t\t}",
		// 			"variables": {
		// 			  "orb": "some orb"
		// 			}
		// 		  }`

		// 		appendPostHandler(testServer, token, http.StatusOK, expectedRequestJson, gqlResponse)

		// 		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

		// 		Expect(err).ShouldNot(HaveOccurred())
		// 		Eventually(session.Err).Should(gbytes.Say("Error:"))
		// 		Eventually(session.Err).Should(gbytes.Say("-- error1,"))
		// 		Eventually(session.Err).Should(gbytes.Say("-- error2,"))
		// 		Eventually(session).Should(gexec.Exit(255))

		// 	})
		// })
	})
})
