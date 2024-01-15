package cmd_test

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Namespace integration tests", func() {
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

	Describe("telemetry", func() {
		It("sends expected event", func() {
			command = exec.Command(pathCLI,
				"namespace", "create",
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
				"--integration-testing",
				"foo-ns",
				"--org-id", `"bb604b45-b6b0-4b81-ad80-796f15eddf87"`,
			)
			command.Env = append(command.Env, fmt.Sprintf("MOCK_TELEMETRY=%s", tempSettings.TelemetryDestPath))

			tempSettings.TestServer.AppendHandlers(func(res http.ResponseWriter, req *http.Request) {
				res.WriteHeader(http.StatusOK)
				_, _ = res.Write([]byte(`{"data":{"organization":{"name":"test-org","id":"bb604b45-b6b0-4b81-ad80-796f15eddf87"}}}`))
			})

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			clitest.CompareTelemetryEvent(tempSettings, []telemetry.Event{
				telemetry.CreateNamespaceEvent(telemetry.CommandInfo{
					Name: "create",
					LocalArgs: map[string]string{
						"help":                "false",
						"integration-testing": "true",
						"no-prompt":           "false",
						"org-id":              "\"bb604b45-b6b0-4b81-ad80-796f15eddf87\"",
					},
				}),
			})
		})
	})

	Context("create, with interactive prompts", func() {
		Describe("registering a namespace with orgID", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"namespace", "create",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--integration-testing",
					"foo-ns",
					"--org-id", `"bb604b45-b6b0-4b81-ad80-796f15eddf87"`,
				)
			})

			It("works with organizationID", func() {
				By("setting up a mock server")

				gqlOrganizationResponse := `{
    											"organization": {
      												"name": "test-org",
      												"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
    											}
  				}`

				expectedOrganizationRequest := `{
            "query": "\n\t\t\tmutation($name: String!, $organizationId: UUID!) {\n\t\t\t\tcreateNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t\torganizationId: $organizationId\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
            "variables": {
              "name": "foo-ns",
              "organizationId": "\"bb604b45-b6b0-4b81-ad80-796f15eddf87\""
            }
          }`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expectedOrganizationRequest,
					Response: gqlOrganizationResponse})

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))

				stdout := session.Wait().Out.Contents()

				Expect(string(stdout)).To(ContainSubstring(fmt.Sprintf(`You are creating a namespace called "%s".

This is the only namespace permitted for your organization with id "%s".

To change the namespace, you will have to contact CircleCI customer support.

Are you sure you wish to create the namespace: %s
Namespace %s created.
Please note that any orbs you publish in this namespace are open orbs and are world-readable.`, "foo-ns", "bb604b45-b6b0-4b81-ad80-796f15eddf87", "`foo-ns`", "`foo-ns`")))
			})
		})
	})
})
