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
	Describe("when listing contexts without a token", func() {
		var (
			command      *exec.Cmd
			tempSettings *clitest.TempSettings
		)

		BeforeEach(func() {
			tempSettings = clitest.WithTempSettings()
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

	// TODO: add integration tests for happy path cases
})
