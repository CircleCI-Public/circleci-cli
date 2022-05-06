package cmd_test

import (
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

		Describe("using an org id to create a context", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"context", "create",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--integration-testing",
					"foo-context",
					"--org-id", `"bb604b45-b6b0-4b81-ad80-796f15eddf87"`,
				)
			})

			It("user creating new context", func() {
				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say(`Error: please use either an orgid or vcs and org name to create context`))
				Eventually(session).Should(clitest.ShouldFail())
			})
		})

		Describe("using an vcs and org name to create a context", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"context", "create",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--integration-testing",
					"BITBUCKET",
					"test-org",
					"foo-context",
				)
			})

			It("user creating new context", func() {
				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err).Should(gbytes.Say(`Error: please use either an orgid or vcs and org name to create context`))
				Eventually(session).Should(clitest.ShouldFail())
			})
		})
	})
})
