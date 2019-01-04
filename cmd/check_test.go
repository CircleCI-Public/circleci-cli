package cmd_test

import (
	"net/http"
	"os"
	"os/exec"
	"time"

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
		tempSettings *temporarySettings
		testServer   *ghttp.Server
		updateCheck  *settings.UpdateCheck
	)

	BeforeEach(func() {
		checkCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli")
		Expect(err).ShouldNot(HaveOccurred())

		tempSettings = withTempSettings()

		updateCheck = &settings.UpdateCheck{
			LastUpdateCheck: time.Time{},
		}

		updateCheck.FileUsed = tempSettings.updateFile.Name()
		err = updateCheck.WriteToDisk()
		Expect(err).ShouldNot(HaveOccurred())

		testServer = ghttp.NewServer()

		command = commandWithHome(checkCLI, tempSettings.home,
			"help", "--github-api", testServer.URL(),
		)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempSettings.home)).To(Succeed())
	})

	Describe("update auto checks with a new release", func() {
		var response string

		BeforeEach(func() {
			checkCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli",
				"-ldflags",
				"-X github.com/CircleCI-Public/circleci-cli/cmd.AutoUpdate=false -X github.com/CircleCI-Public/circleci-cli/cmd.PackageManager=release",
			)
			Expect(err).ShouldNot(HaveOccurred())

			tempSettings = withTempSettings()

			command = commandWithHome(checkCLI, tempSettings.home,
				"help", "--github-api", testServer.URL(),
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
      }
    ]
  }
]
`

			testServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/repos/CircleCI-Public/circleci-cli/releases"),
					ghttp.RespondWith(http.StatusOK, response),
				),
			)

		})

		AfterEach(func() {
			testServer.Close()
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
