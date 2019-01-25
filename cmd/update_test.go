package cmd_test

import (
	"net/http"
	"os/exec"
	"path/filepath"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"gotest.tools/golden"
)

var _ = Describe("Update", func() {
	var (
		command      *exec.Cmd
		response     string
		tempSettings *clitest.TempSettings
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()

		response = `
[
  {
    "id": 1,
    "tag_name": "v1.0.0",
    "name": "v1.0.0",
    "published_at": "2013-02-27T19:35:32Z",
    "assets": [
      {
        "id": 1,
        "name": "linux_amd64.zip",
        "label": "short description",
        "content_type": "application/zip",
        "size": 1024
      }
    ]
  }
]
`

		tempSettings.TestServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/repos/CircleCI-Public/circleci-cli/releases"),
				ghttp.RespondWith(http.StatusOK, response),
			),
		)
	})

	AfterEach(func() {
		tempSettings.Cleanup()
	})

	Describe("update --check", func() {
		BeforeEach(func() {
			command = exec.Command(pathCLI,
				"update", "--check",
				"--github-api", tempSettings.TestServer.URL(),
			)
		})

		It("with flag should tell the user how to update and install", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Err.Contents()).Should(BeEmpty())

			Eventually(session.Out).Should(gbytes.Say("You are running 0.0.0-dev"))
			Eventually(session.Out).Should(gbytes.Say("A new release is available (.*)"))

			Eventually(session.Out).Should(gbytes.Say("You can visit the Github releases page for the CLI to manually download and install:"))
			Eventually(session.Out).Should(gbytes.Say("https://github.com/CircleCI-Public/circleci-cli/releases"))

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Describe("update check", func() {
		BeforeEach(func() {
			command = exec.Command(pathCLI,
				"update", "check",
				"--github-api", tempSettings.TestServer.URL(),
			)
		})

		It("without flag should tell the user how to update and install", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Out).Should(gbytes.Say("You are running 0.0.0-dev"))
			Eventually(session.Out).Should(gbytes.Say("A new release is available (.*)"))

			Eventually(session.Out).Should(gbytes.Say("You can visit the Github releases page for the CLI to manually download and install:"))
			Eventually(session.Out).Should(gbytes.Say("https://github.com/CircleCI-Public/circleci-cli/releases"))

			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Describe("update", func() {
		BeforeEach(func() {
			updateCLI, err := gexec.Build("github.com/CircleCI-Public/circleci-cli")
			Expect(err).ShouldNot(HaveOccurred())

			command = exec.Command(updateCLI,
				"update",
				"--github-api", tempSettings.TestServer.URL(),
			)

			assetBytes := golden.Get(GinkgoT(), filepath.FromSlash("update/foo.zip"))
			assetResponse := string(assetBytes)

			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/repos/CircleCI-Public/circleci-cli/releases"),
					ghttp.RespondWith(http.StatusOK, response),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/repos/CircleCI-Public/circleci-cli/releases/assets/1"),
					ghttp.RespondWith(http.StatusOK, assetResponse),
				),
			)
		})

		It("should update the program", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Out).Should(gbytes.Say("You are running 0.0.0-dev"))
			Eventually(session.Out).Should(gbytes.Say("A new release is available (.*)"))

			Eventually(session.Out).Should(gbytes.Say("Updated to 1.0.0"))

			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session).Should(gexec.Exit(0))
		})
	})

	FDescribe("When Github returns a 403 error", func() {
		BeforeEach(func() {
			command = exec.Command(pathCLI,
				"update", "check",
				"--github-api", tempSettings.TestServer.URL(),
			)

			tempSettings.TestServer.Reset()
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/repos/CircleCI-Public/circleci-cli/releases"),
					ghttp.RespondWith(http.StatusForbidden, []byte("Forbidden")),
				),
			)
		})

		It("should print a helpful error message & exit 255", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			// TODO: This should exit with error status 1, since 255 is a
			// special error status for: "exit status outside of range".
			Eventually(session.Err).Should(gbytes.Say("Failed to query the GitHub API for updates."))
			Eventually(session.Err).Should(gbytes.Say("This is most likely due to GitHub rate-limiting on unauthenticated requests."))
			Eventually(session.Err).Should(gbytes.Say("We call the github releases API to look for new releases. More information"))
			Eventually(session.Err).Should(gbytes.Say("about that API can be found here: https://developer.github.com/v3/repos/releases/"))
			Eventually(session.Err).Should(gbytes.Say("To have the circleci-cli make authenticated requests please:"))
			Eventually(session.Err).Should(gbytes.Say("  1. Generate a token at https://github.com/settings/tokens"))
			Eventually(session.Err).Should(gbytes.Say("  2. Set the token by either adding it to your ~/.gitconfig"))
			Eventually(session.Err).Should(gbytes.Say("     setting the GITHUB_TOKEN environment variable"))
			Eventually(session.Err).Should(gbytes.Say("Instructions for generating a token can be found at"))
			Eventually(session.Err).Should(gbytes.Say("https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/"))

			Eventually(session).Should(gexec.Exit(255))
		})
	})
})
