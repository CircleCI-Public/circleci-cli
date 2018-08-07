package cmd_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Query", func() {
	var (
		server   *ghttp.Server
		token    string
		tempHome string
		stdin    bytes.Buffer
		command  *exec.Cmd
	)

	BeforeEach(func() {
		server = ghttp.NewServer()

		var err error
		tempHome, err = ioutil.TempDir("", "circleci-cli-test-")
		Expect(err).ToNot(HaveOccurred())

		token = "mytoken"
		command = exec.Command(pathCLI, "query", "-",
			"-t", token,
			"-e", server.URL(),
		)
		command.Stdin = &stdin
		command.Env = append(os.Environ(),
			fmt.Sprintf("HOME=%s", tempHome),
			fmt.Sprintf("USERPROFILE=%s", tempHome), // windows
		)
	})

	AfterEach(func() {
		server.Close()
		Expect(os.RemoveAll(tempHome)).To(Succeed())
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

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/"),
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
