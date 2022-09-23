package cmd_test

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Context integration tests", func() {
	var (
		tempSettings *clitest.TempSettings
		token        string = "testtoken"
		command      *exec.Cmd
		contextName  string = "foo-context"
		orgID        string = "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		vcsType      string = "BITBUCKET"
		orgName      string = "test-org"
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Context("create, with interactive prompts", func() {

		Describe("when listing contexts without a token", func() {
			BeforeEach(func() {
				command = commandWithHome(pathCLI, tempSettings.Home,
					"context", "list", "github", "foo",
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
		})
	})

	Context("create, with interactive prompts", func() {
		//tests context creation via orgid
		Describe("using an org id to create a context", func() {

			BeforeEach(func() {
				command = commandWithHome(pathCLI, tempSettings.Home,
					"context", "create",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--integration-testing",
					contextName,
					"--org-id", fmt.Sprintf(`"%s"`, orgID),
				)
			})

			It("should create new context using an org id", func() {
				By("setting up a mock server")
				tempSettings.AppendRESTPostHandler(clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  fmt.Sprintf(`{"name": "%s","owner":{"id":"\"%s\""}}`, contextName, orgID),
					Response: fmt.Sprintf(`{"id": "497f6eca-6276-4993-bfeb-53cbbbba6f08", "name": "%s", "created_at": "2015-09-21T17:29:21.042Z" }`, contextName),
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
			})
		})
		//tests context creation via orgname and vcs type
		Describe("using an vcs and org name to create a context", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"context", "create",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--integration-testing",
					vcsType,
					orgName,
					contextName,
				)
			})

			It("user creating new context", func() {
				By("setting up a mock server")

				tempSettings.AppendRESTPostHandler(clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  fmt.Sprintf(`{"name": "%s","owner":{"slug":"%s"}}`, contextName, vcsType+"/"+orgName),
					Response: fmt.Sprintf(`{"id": "497f6eca-6276-4993-bfeb-53cbbbba6f08", "name": "%s", "created_at": "2015-09-21T17:29:21.042Z" }`, contextName),
				})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
			})

			It("prints all in-band errors returned by the API", func() {
				By("setting up a mock server")
				tempSettings.AppendRESTPostHandler(clitest.MockRequestResponse{
					Status:        http.StatusInternalServerError,
					Request:       fmt.Sprintf(`{"name": "%s","owner":{"slug":"%s"}}`, contextName, vcsType+"/"+orgName),
					Response:      fmt.Sprintf(`{"id": "497f6eca-6276-4993-bfeb-53cbbbba6f08", "name": "%s", "created_at": "2015-09-21T17:29:21.042Z" }`, contextName),
					ErrorResponse: `{ "message": "ignored error" }`,
				})
				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say(`Error: ignored error`))
				Eventually(session).ShouldNot(gexec.Exit(0))
			})
		})
	})
})
