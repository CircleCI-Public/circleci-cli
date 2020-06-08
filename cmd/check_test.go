package cmd_test

import (
	"net/http"
	"os/exec"
	"time"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/settings"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Check", func() {
	var (
		command      *exec.Cmd
		err          error
		checkCLI     string
		tempSettings *clitest.TempSettings
		updateCheck  *settings.UpdateCheck
	)

	BeforeEach(func() {
		checkCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli")
		Expect(err).ShouldNot(HaveOccurred())

		tempSettings = clitest.WithTempSettings()

		updateCheck = &settings.UpdateCheck{
			LastUpdateCheck: time.Time{},
		}

		updateCheck.FileUsed = tempSettings.Update.File.Name()
		err = updateCheck.WriteToDisk()
		Expect(err).ShouldNot(HaveOccurred())

		command = commandWithHome(checkCLI, tempSettings.Home,
			"help", "--github-api", tempSettings.TestServer.URL(),
		)
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Describe("update auto checks with a new release", func() {
		var response string

		BeforeEach(func() {
			checkCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli",
				"-ldflags",
				"-X github.com/CircleCI-Public/circleci-cli/cmd.AutoUpdate=false -X github.com/CircleCI-Public/circleci-cli/version.packageManager=release",
			)
			Expect(err).ShouldNot(HaveOccurred())

			command = commandWithHome(checkCLI, tempSettings.Home,
				"help", "--skip-update-check=false", "--github-api", tempSettings.TestServer.URL(),
			)

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

		Context("using a binary release", func() {
			It("with flag should tell the user how to update and install", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())

				Eventually(session.Err).Should(gbytes.Say("You are running 0.0.0-dev"))
				Eventually(session.Err).Should(gbytes.Say("A new release is available (.*)"))
				Eventually(session.Err).Should(gbytes.Say("You can update with `circleci update install`"))

				Eventually(session).Should(gexec.Exit(0))
			})
		})
	})
})
