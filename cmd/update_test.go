package cmd_test

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

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
		tempSettings *temporarySettings
	)

	BeforeEach(func() {
		tempSettings = withTempSettings()

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

		tempSettings.testServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/repos/CircleCI-Public/circleci-cli/releases"),
				ghttp.RespondWith(http.StatusOK, response),
			),
		)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempSettings.home)).To(Succeed())
		tempSettings.testServer.Close()
	})

	Describe("update --check", func() {
		BeforeEach(func() {
			command = exec.Command(pathCLI,
				"update", "--check",
				"--github-api", tempSettings.testServer.URL(),
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
				"--github-api", tempSettings.testServer.URL(),
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
				"--github-api", tempSettings.testServer.URL(),
			)

			assetBytes := golden.Get(GinkgoT(), filepath.FromSlash("update/foo.zip"))
			assetResponse := string(assetBytes)

			tempSettings.testServer.AppendHandlers(
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
})
