package cmd_test

import (
	"bytes"
	"net/http"
	"os/exec"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Query", func() {
	var (
		token        string
		tempSettings *clitest.TempSettings
		stdin        bytes.Buffer
		command      *exec.Cmd
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()

		token = "mytoken"
		command = commandWithHome(pathCLI, tempSettings.Home,
			"query", "-",
			"--skip-update-check",
			"--token", token,
			"--host", tempSettings.TestServer.URL(),
		)
		command.Stdin = &stdin
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Describe("query provided to STDIN", func() {
		var responseData string

		BeforeEach(func() {
			query := `query {
	hero {
		name
		friends {
			name
		}
	}
}
`
			responseData = `{
	"hero": {
		"name": "R2-D2",
		"friends": [
			{
				"name": "Luke Skywalker"
			},
			{
				"name": "Han Solo"
			},
			{
				"name": "Leia Organa"
			}
		]
	}
}
`

			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/graphql-unstable"),
					ghttp.VerifyHeader(http.Header{
						"Authorization": []string{token},
					}),
					ghttp.RespondWith(http.StatusOK, `{"data": `+responseData+`}`),
				),
			)

			_, err := stdin.WriteString(query)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should make request and return result as JSON", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			session.Wait()
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out.Contents()).Should(MatchJSON(responseData))
			Eventually(session).Should(gexec.Exit(0))
		})
	})
})
