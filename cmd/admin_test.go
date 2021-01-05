package cmd_test

import (
	"net/http"
	"os/exec"
	"time"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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

	Describe("deleting namespace aliases", func() {
		BeforeEach(func() {
			command = exec.Command(pathCLI,
				"admin",
				"delete-namespace-alias",
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
				"--integration-testing",
				"foo-ns",
			)
		})

		It("returns message for when deletion unexpectedly failed", func() {
			gqlDeleteNsAliasResponse := `{
				"deleteNamespaceAlias": {
					"errors": [],
					"deleted": false
				}
			}`
			expectedDeleteNsAliasRequest := `{
				"query": "\nmutation($name: String!) {\n  deleteNamespaceAlias(name: $name) {\n    deleted\n    errors {\n      type\n      message\n    }\n  }\n}\n",
				"variables": {
					"name": "foo-ns"
				}
			}`

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedDeleteNsAliasRequest,
				Response: gqlDeleteNsAliasResponse})

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("Namespace alias deletion failed for unknown reasons."))
			Eventually(session).ShouldNot(gexec.Exit(0))
		})

		It("returns all errors returned by the GraphQL API", func() {
			gqlDeleteNsAliasResponse := `{
				"deleteNamespaceAlias": {
					"errors": [{"message": "error1"}],
					"deleted": false
				}
			}`
			expectedDeleteNsAliasRequest := `{
				"query": "\nmutation($name: String!) {\n  deleteNamespaceAlias(name: $name) {\n    deleted\n    errors {\n      type\n      message\n    }\n  }\n}\n",
				"variables": {
					"name": "foo-ns"
				}
			}`

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedDeleteNsAliasRequest,
				Response: gqlDeleteNsAliasResponse})

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("Error: error1"))
			Eventually(session).ShouldNot(gexec.Exit(0))
		})

		It("works given an alias name", func() {
			By("setting up a mock server")
			gqlDeleteNsAliasResponse := `{
				"deleteNamespaceAlias": {
					"errors": [],
					"deleted": true
				}
			}`
			expectedDeleteNsAliasRequest := `{
				"query": "\nmutation($name: String!) {\n  deleteNamespaceAlias(name: $name) {\n    deleted\n    errors {\n      type\n      message\n    }\n  }\n}\n",
				"variables": {
					"name": "foo-ns"
				}
			}`

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedDeleteNsAliasRequest,
				Response: gqlDeleteNsAliasResponse})

			By("running the command")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session, time.Second*5).Should(gexec.Exit(0))
		})
	})

	Describe("renaming a namespace", func() {
		var (
			gqlGetNsResponse      string
			expectedGetNsRequest  string
			expectedRenameRequest string
		)
		BeforeEach(func() {
			command = exec.Command(pathCLI,
				"admin", "rename-namespace",
				"ns-0", "ns-1",
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
				"--no-prompt",
			)
			gqlGetNsResponse = `{
					"errors": [],
					"registryNamespace": {
						"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
					}
				}`
			expectedGetNsRequest = `{
				"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
				"variables": {
					"name": "ns-0"
				}
			}`
			expectedRenameRequest = `{
		"query": "\n\t\tmutation($namespaceId: UUID!, $newName: String!){\n\t\t\trenameNamespace(\n\t\t\t\tnamespaceId: $namespaceId,\n\t\t\t\tnewName: $newName\n\t\t\t){\n\t\t\t\tnamespace {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t\terrors {\n\t\t\t\t\tmessage\n\t\t\t\t\ttype\n\t\t\t\t}\n\t\t\t}\n\t\t}",
		"variables": {"newName": "ns-1", "namespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"}
}`
		})

		It("works in the basic case", func() {
			By("setting up a mock server")
			gqlRenameResponse := `{"data":{"renameNamespace":{"namespace":{"id":"4e377fe3-330d-4e4c-af62-821850fe9595"},"errors":[]}}}`
			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedGetNsRequest,
				Response: gqlGetNsResponse})
			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedRenameRequest,
				Response: gqlRenameResponse})

			By("running the command")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Out, "5s").Should(gbytes.Say("`ns-0` renamed to `ns-1`"))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("returns an error when renaming a namespace fails", func() {
			By("setting up a mock server")
			gqlRenameResponse := `{
			        "renameNamespace": {
					"errors": [
						{"message": "error1"},
						{"message": "error2"}
					],
					"namespace": null
				}
			}`
			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedGetNsRequest,
				Response: gqlGetNsResponse})
			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedRenameRequest,
				Response: gqlRenameResponse})
			By("running the command")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("Error: error1\nerror2"))
			Eventually(session).ShouldNot(gexec.Exit(0))
		})
	})
})
