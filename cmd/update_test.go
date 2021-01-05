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
	"gotest.tools/v3/golden"
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
      },
	  {
        "id": 1,
        "name": "darwin_amd64.tar.gz",
		"label": "short description",
        "content_type": "application/zip",
		"size": 1024
      },
      {
        "id": 1,
        "name": "windows_amd64.tar.gz",
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
		tempSettings.Close()
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

	Describe("When Github returns a 403 error", func() {
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
			Eventually(session).Should(clitest.ShouldFail())

			// TODO: This should exit with error status 1, since 255 is a
			// special error status for: "exit status outside of range".
			//
			// However this may be difficult to change, since all commands that return
			// an error after executing cause the program to exit with a non-zero code:
			// https://github.com/CircleCI-Public/circleci-cli/blob/5896baa95dad1b66f9c4a5b0a14571717c92aa55/cmd/root.go#L38
			stderr := session.Wait().Err.Contents()
			Expect(string(stderr)).To(ContainSubstring(`Error: Failed to query the GitHub API for updates.

This is most likely due to GitHub rate-limiting on unauthenticated requests.

To have the circleci-cli make authenticated requests please:

  1. Generate a token at https://github.com/settings/tokens
  2. Set the token by either adding it to your ~/.gitconfig or
     setting the GITHUB_TOKEN environment variable.

Instructions for generating a token can be found at:
https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/

We call the GitHub releases API to look for new releases.
More information about that API can be found here: https://developer.github.com/v3/repos/releases/`))
		})
	})
})
