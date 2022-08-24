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

	Describe("deleting namespace", func() {
		BeforeEach(func() {
			command = exec.Command(pathCLI,
				"admin",
				"delete-namespace",
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
				"--integration-testing",
				"foo-ns",
			)
		})

		It("fails when provided namespace does not exist", func() {
			gqlRegistryNsResponse := `{
				"registryNamespace": {
					"id": ""
				}
			}`

			expectedRegistryNsRequest := `{
				"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
				"variables": {
				  "name": "foo-ns"
				}
			  }`

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedRegistryNsRequest,
				Response: gqlRegistryNsResponse})

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("Error: namespace check failed: the namespace 'foo-ns' does not exist. Did you misspell the namespace, or maybe you meant to create the namespace first?"))
			Eventually(session).ShouldNot(gexec.Exit(0))
		})

		It("fails when list orbs returns an error", func() {
			gqlRegistryNsResponse := `{
				"registryNamespace": {
					"id": "f13a9e13-538c-435c-8f61-78596661acd6"
				}
			}`

			expectedRegistryNsRequest := `{
				"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
				"variables": {
				  "name": "foo-ns"
				}
			  }`

			gqlListNamespaceOrbsResponse := `{
				"orbs": [
					{"name": "test-orb-1"},
					{"name": "test-orb-2"}
				]
			}`

			expectedListOrbsRequest := `{
				"query": "\nquery namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {\n\tregistryNamespace(name: $namespace) {\n\t\tname\n                id\n\t\torbs(first: 20, after: $after, view: $view) {\n\t\t\tedges {\n\t\t\t\tcursor\n\t\t\t\tnode {\n\t\t\t\t\tversions {\n\t\t\t\t\t\tsource\n\t\t\t\t\t\tversion\n\t\t\t\t\t}\n\t\t\t\t\tname\n\t                                statistics {\n\t\t                           last30DaysBuildCount,\n\t\t                           last30DaysProjectCount,\n\t\t                           last30DaysOrganizationCount\n\t                               }\n\t\t\t\t}\n\t\t\t}\n\t\t\ttotalCount\n\t\t\tpageInfo {\n\t\t\t\thasNextPage\n\t\t\t}\n\t\t}\n\t}\n}\n",
				"variables": {
				  "after": "",
				  "namespace": "foo-ns",
				  "view": "PUBLIC_ONLY"
				}
			  }`

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedRegistryNsRequest,
				Response: gqlRegistryNsResponse})

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedListOrbsRequest,
				Response: gqlListNamespaceOrbsResponse})

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("unable to list orbs: No namespace found"))
			Eventually(session).ShouldNot(gexec.Exit(0))
		})

		It("fails when delete namespace returns an error", func() {
			gqlRegistryNsResponse := `{
				"registryNamespace": {
					"id": "f13a9e13-538c-435c-8f61-78596661acd6"
				}
			}`

			expectedRegistryNsRequest := `{
				"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
				"variables": {
				  "name": "foo-ns"
				}
			  }`

			gqlListNamespaceOrbsResponse := `{
				"registryNamespace": {
					"id": "f13a9e13-538c-435c-8f61-78596661acd6",
					"orbs": {
						"edges": [
							{
								"node": {
									"name": "test-orb-1"
								}
							}
						]
					}
				}
			}`

			expectedListOrbsRequest := `{
				"query": "\nquery namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {\n\tregistryNamespace(name: $namespace) {\n\t\tname\n                id\n\t\torbs(first: 20, after: $after, view: $view) {\n\t\t\tedges {\n\t\t\t\tcursor\n\t\t\t\tnode {\n\t\t\t\t\tversions {\n\t\t\t\t\t\tsource\n\t\t\t\t\t\tversion\n\t\t\t\t\t}\n\t\t\t\t\tname\n\t                                statistics {\n\t\t                           last30DaysBuildCount,\n\t\t                           last30DaysProjectCount,\n\t\t                           last30DaysOrganizationCount\n\t                               }\n\t\t\t\t}\n\t\t\t}\n\t\t\ttotalCount\n\t\t\tpageInfo {\n\t\t\t\thasNextPage\n\t\t\t}\n\t\t}\n\t}\n}\n",
				"variables": {
				  "after": "",
				  "namespace": "foo-ns",
				  "view": "PUBLIC_ONLY"
				}
			  }`

			gqlDeleteNamespaceResponse := `{
				"deleteNamespaceAndRelatedOrbs": {
					"deleted": false,
					"errors": [{"message": "test"}]
				}
			}`

			expectedDeleteNamespacerequest := `{
				"query": "\nmutation($id: UUID!) {\n  deleteNamespaceAndRelatedOrbs(namespaceId: $id) {\n    deleted\n    errors {\n      type\n      message\n    }\n  }\n}\n",
				"variables": {
				  "id": "f13a9e13-538c-435c-8f61-78596661acd6"
				}
			  }`

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedRegistryNsRequest,
				Response: gqlRegistryNsResponse})

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedListOrbsRequest,
				Response: gqlListNamespaceOrbsResponse})

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedDeleteNamespacerequest,
				Response: gqlDeleteNamespaceResponse})

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("Error: test"))
			Eventually(session).ShouldNot(gexec.Exit(0))
		})

		It("deletes namespace successfully", func() {
			gqlRegistryNsResponse := `{
				"registryNamespace": {
					"id": "f13a9e13-538c-435c-8f61-78596661acd6"
				}
			}`

			expectedRegistryNsRequest := `{
				"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
				"variables": {
				  "name": "foo-ns"
				}
			  }`

			gqlListNamespaceOrbsResponse := `{
				"registryNamespace": {
					"id": "f13a9e13-538c-435c-8f61-78596661acd6",
					"orbs": {
						"edges": [
							{
								"node": {
									"name": "test-orb-1"
								}
							}
						]
					}
				}
			}`

			expectedListOrbsRequest := `{
				"query": "\nquery namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {\n\tregistryNamespace(name: $namespace) {\n\t\tname\n                id\n\t\torbs(first: 20, after: $after, view: $view) {\n\t\t\tedges {\n\t\t\t\tcursor\n\t\t\t\tnode {\n\t\t\t\t\tversions {\n\t\t\t\t\t\tsource\n\t\t\t\t\t\tversion\n\t\t\t\t\t}\n\t\t\t\t\tname\n\t                                statistics {\n\t\t                           last30DaysBuildCount,\n\t\t                           last30DaysProjectCount,\n\t\t                           last30DaysOrganizationCount\n\t                               }\n\t\t\t\t}\n\t\t\t}\n\t\t\ttotalCount\n\t\t\tpageInfo {\n\t\t\t\thasNextPage\n\t\t\t}\n\t\t}\n\t}\n}\n",
				"variables": {
				  "after": "",
				  "namespace": "foo-ns",
				  "view": "PUBLIC_ONLY"
				}
			  }`

			gqlDeleteNamespaceResponse := `{
				"deleteNamespaceAndRelatedOrbs": {
					"deleted": true
				}
			}`

			expectedDeleteNamespacerequest := `{
				"query": "\nmutation($id: UUID!) {\n  deleteNamespaceAndRelatedOrbs(namespaceId: $id) {\n    deleted\n    errors {\n      type\n      message\n    }\n  }\n}\n",
				"variables": {
				  "id": "f13a9e13-538c-435c-8f61-78596661acd6"
				}
			  }`

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedRegistryNsRequest,
				Response: gqlRegistryNsResponse})

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedListOrbsRequest,
				Response: gqlListNamespaceOrbsResponse})

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expectedDeleteNamespacerequest,
				Response: gqlDeleteNamespaceResponse})

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
			Eventually(session).Should(gexec.Exit(0))

			stdout := session.Wait().Out.Contents()
			Expect(string(stdout)).To(ContainSubstring("Namespace `ns-0` renamed to `ns-1`. `ns-0` is an alias for `ns-1` so existing usages will continue to work, unless you delete the `ns-0` alias with `delete-namespace-alias ns-0`"))
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
