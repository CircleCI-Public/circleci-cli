package cmd_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/context"
	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var (
	contentTypeHeader http.Header = map[string][]string{"Content-Type": {"application/json"}}
)

func mockServerForREST(tempSettings *clitest.TempSettings) {
	tempSettings.TestServer.AppendHandlers(
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/api/v2/openapi.json"),
			ghttp.RespondWith(
				http.StatusOK,
				`{"paths":{"/context":{}}}`,
				contentTypeHeader,
			),
		),
	)
}

var _ = Describe("Context integration tests", func() {
	var (
		tempSettings *clitest.TempSettings
		token        string = "testtoken"
		command      *exec.Cmd
		contextName  string = "foo-context"
		orgID        string = "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		vcsType      string = "bitbucket"
		orgName      string = "test-org"
		orgSlug      string = fmt.Sprintf("%s/%s", vcsType, orgName)
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Describe("any command", func() {
		It("should inform about invalid token", func() {
			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "list", "github", "foo",
				"--skip-update-check",
				"--token", "",
			)

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say(`Error: please set a token with 'circleci setup'
You can create a new personal API token here:
https://circleci.com/account/api`))
			Eventually(session).Should(clitest.ShouldFail())
		})

		It("should handle errors", func() {
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-id=%s", orgID)),
					ghttp.RespondWith(
						http.StatusBadRequest,
						`{"message":"no context found"}`,
						contentTypeHeader,
					),
				),
			)

			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "list", "--org-id", orgID,
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit())
			// Exit codes are different between Unix and Windows so we're only checking that it does not equal 0
			Expect(session.ExitCode()).ToNot(Equal(0))
			Expect(string(session.Err.Contents())).To(Equal("Error: no context found\n"))
		})
	})

	Describe("list", func() {
		It("should list context with VCS / org name", func() {
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-slug=%s", orgSlug)),
					ghttp.RespondWith(
						http.StatusOK,
						`{"items":[]}`,
						contentTypeHeader,
					),
				),
			)

			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "list", vcsType, orgName,
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(Equal(`+----+------+------------+
| ID | NAME | CREATED AT |
+----+------+------------+
+----+------+------------+
`))
		})

		It("should list context with VCS / org name", func() {
			contexts := []context.Context{
				{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
				{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
			}
			body, err := json.Marshal(struct{ Items []context.Context }{contexts})
			Expect(err).ShouldNot(HaveOccurred())
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-slug=%s", orgSlug)),
					ghttp.RespondWith(
						http.StatusOK,
						body,
						contentTypeHeader,
					),
				),
			)

			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "list", vcsType, orgName,
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			lines := strings.Split(string(session.Out.Contents()), "\n")
			Expect(lines[1]).To(MatchRegexp("|\\w+ID\\w+|\\w+NAME\\w+|\\w+CREATED AT\\w+|"))
			Expect(lines).To(HaveLen(7))
		})

		It("should list context with org id", func() {
			contexts := []context.Context{
				{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
				{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
			}
			body, err := json.Marshal(struct{ Items []context.Context }{contexts})
			Expect(err).ShouldNot(HaveOccurred())
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-id=%s", orgID)),
					ghttp.RespondWith(
						http.StatusOK,
						body,
						contentTypeHeader,
					),
				),
			)

			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "list", "--org-id", orgID,
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			lines := strings.Split(string(session.Out.Contents()), "\n")
			Expect(lines[1]).To(MatchRegexp("|\\w+ID\\w+|\\w+NAME\\w+|\\w+CREATED AT\\w+|"))
			Expect(lines).To(HaveLen(7))
		})
	})

	Describe("show", func() {
		var (
			contexts = []context.Context{
				{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
				{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
			}
			ctxBody, _ = json.Marshal(struct{ Items []context.Context }{contexts})
			envVars    = []context.EnvironmentVariable{
				{Variable: "var-name", ContextID: contexts[1].ID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{Variable: "any-name", ContextID: contexts[1].ID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			}
			envBody, _ = json.Marshal(struct{ Items []context.EnvironmentVariable }{envVars})
		)

		It("should show context with vcs type / org name", func() {
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-slug=%s", orgSlug)),
					ghttp.RespondWith(
						http.StatusOK,
						ctxBody,
						contentTypeHeader,
					),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v2/context/%s/environment-variable", contexts[1].ID)),
					ghttp.RespondWith(
						http.StatusOK,
						envBody,
						contentTypeHeader,
					),
				),
			)

			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "show", vcsType, orgName, "context-name",
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			lines := strings.Split(string(session.Out.Contents()), "\n")
			Expect(lines[0]).To(Equal("Context: context-name"))
			Expect(lines[2]).To(MatchRegexp("|\\w+ENVIRONMENT VARIABLE\\w+|\\w+VALUE\\w+|"))
			Expect(lines).To(HaveLen(8))
		})

		It("should show context with org id", func() {
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-id=%s", orgID)),
					ghttp.RespondWith(
						http.StatusOK,
						ctxBody,
						contentTypeHeader,
					),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v2/context/%s/environment-variable", contexts[1].ID)),
					ghttp.RespondWith(
						http.StatusOK,
						envBody,
						contentTypeHeader,
					),
				),
			)

			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "show", "context-name", "--org-id", orgID,
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			lines := strings.Split(string(session.Out.Contents()), "\n")
			Expect(lines[0]).To(Equal("Context: context-name"))
			Expect(lines[2]).To(MatchRegexp("|\\w+ENVIRONMENT VARIABLE\\w+|\\w+VALUE\\w+|"))
			Expect(lines).To(HaveLen(8))
		})
	})

	Describe("store", func() {
		var (
			contexts = []context.Context{
				{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
				{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
			}
			ctxBody, _ = json.Marshal(struct{ Items []context.Context }{contexts})
			envVar     = context.EnvironmentVariable{
				Variable:  "env var name",
				ContextID: uuid.NewString(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			varBody, _ = json.Marshal(envVar)
		)

		It("should store value when giving vcs type / org name", func() {
			By("setting up a mock server")
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-slug=%s", orgSlug)),
					ghttp.RespondWith(
						http.StatusOK,
						ctxBody,
						contentTypeHeader,
					),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", fmt.Sprintf("/api/v2/context/%s/environment-variable/%s", contexts[1].ID, envVar.Variable)),
					ghttp.VerifyJSON(`{"value":"value"}`),
					ghttp.RespondWith(
						http.StatusOK,
						varBody,
						contentTypeHeader,
					),
				),
			)

			By("running the command")
			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "store-secret", vcsType, orgName, contexts[1].Name, envVar.Variable,
				"--skip-update-check",
				"--integration-testing",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
		})

		It("should store value when giving vcs type / org name", func() {
			By("setting up a mock server")
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-id=%s", orgID)),
					ghttp.RespondWith(
						http.StatusOK,
						ctxBody,
						contentTypeHeader,
					),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", fmt.Sprintf("/api/v2/context/%s/environment-variable/%s", contexts[1].ID, envVar.Variable)),
					ghttp.VerifyJSON(`{"value":"value"}`),
					ghttp.RespondWith(
						http.StatusOK,
						varBody,
						contentTypeHeader,
					),
				),
			)

			By("running the command")
			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "store-secret", "--org-id", orgID, contexts[1].Name, envVar.Variable,
				"--skip-update-check",
				"--integration-testing",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Describe("remove", func() {
		var (
			contexts = []context.Context{
				{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
				{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
			}
			ctxBody, _ = json.Marshal(struct{ Items []context.Context }{contexts})
			varName    = "env var name"
		)

		It("should remove environment variable with vcs type / org name", func() {
			By("setting up a mock server")
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-slug=%s", orgSlug)),
					ghttp.RespondWith(
						http.StatusOK,
						ctxBody,
						contentTypeHeader,
					),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("DELETE", fmt.Sprintf("/api/v2/context/%s/environment-variable/%s", contexts[1].ID, varName)),
					ghttp.RespondWith(
						http.StatusOK,
						`{"message":"Deleted env var"}`,
						contentTypeHeader,
					),
				),
			)

			By("running the command")
			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "remove-secret", vcsType, orgName, contexts[1].Name, varName,
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(Equal("Removed secret env var name from context context-name.\n"))
		})

		It("should remove environment variable with org id", func() {
			By("setting up a mock server")
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-id=%s", orgID)),
					ghttp.RespondWith(
						http.StatusOK,
						ctxBody,
						contentTypeHeader,
					),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("DELETE", fmt.Sprintf("/api/v2/context/%s/environment-variable/%s", contexts[1].ID, varName)),
					ghttp.RespondWith(
						http.StatusOK,
						`{"message":"Deleted env var"}`,
						contentTypeHeader,
					),
				),
			)

			By("running the command")
			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "remove-secret", "--org-id", orgID, contexts[1].Name, varName,
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(Equal("Removed secret env var name from context context-name.\n"))
		})
	})

	Describe("delete", func() {
		var (
			contexts = []context.Context{
				{ID: uuid.NewString(), Name: "another-name", CreatedAt: time.Now()},
				{ID: uuid.NewString(), Name: "context-name", CreatedAt: time.Now()},
			}
			ctxBody, _ = json.Marshal(struct{ Items []context.Context }{contexts})
		)

		It("should delete context with vcs type / org name", func() {
			By("setting up a mock server")
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-slug=%s", orgSlug)),
					ghttp.RespondWith(
						http.StatusOK,
						ctxBody,
						contentTypeHeader,
					),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("DELETE", fmt.Sprintf("/api/v2/context/%s", contexts[1].ID)),
					ghttp.RespondWith(
						http.StatusOK,
						`{"message":"Deleted context"}`,
						contentTypeHeader,
					),
				),
			)

			By("running the command")
			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "delete", "-f", vcsType, orgName, contexts[1].Name,
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(Equal("Deleted context context-name.\n"))
		})

		It("should delete context with org id", func() {
			By("setting up a mock server")
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v2/context", fmt.Sprintf("owner-id=%s", orgID)),
					ghttp.RespondWith(
						http.StatusOK,
						ctxBody,
						contentTypeHeader,
					),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("DELETE", fmt.Sprintf("/api/v2/context/%s", contexts[1].ID)),
					ghttp.RespondWith(
						http.StatusOK,
						`{"message":"Deleted context"}`,
						contentTypeHeader,
					),
				),
			)

			By("running the command")
			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "delete", "-f", "--org-id", orgID, contexts[1].Name,
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(Equal("Deleted context context-name.\n"))
		})
	})

	Describe("create", func() {
		var (
			context = context.Context{
				ID:        uuid.NewString(),
				CreatedAt: time.Now(),
				Name:      contextName,
			}
			ctxResp, _ = json.Marshal(&context)
		)

		It("should create new context using an org id", func() {
			By("setting up a mock server")
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v2/context"),
					ghttp.VerifyContentType("application/json"),
					ghttp.VerifyJSON(fmt.Sprintf(`{"name":"%s","owner":{"type":"organization","id":"%s"}}`, contextName, orgID)),
					ghttp.RespondWith(http.StatusOK, ctxResp, contentTypeHeader),
				),
			)

			By("running the command")
			command = commandWithHome(pathCLI, tempSettings.Home,
				"context", "create", "--org-id", orgID, contextName,
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
				"--integration-testing",
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
		})

		It("should create new context using vcs type / org name", func() {
			By("setting up a mock server")
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v2/context"),
					ghttp.VerifyContentType("application/json"),
					ghttp.VerifyJSON(fmt.Sprintf(`{"name":"%s","owner":{"type":"organization","slug":"%s/%s"}}`, contextName, vcsType, orgName)),
					ghttp.RespondWith(http.StatusOK, ctxResp, contentTypeHeader),
				),
			)

			By("running the command")
			command := exec.Command(pathCLI,
				"context", "create",
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
				"--integration-testing",
				vcsType,
				orgName,
				contextName,
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
		})

		It("handles errors", func() {
			By("setting up a mock server")
			mockServerForREST(tempSettings)
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v2/context"),
					ghttp.VerifyContentType("application/json"),
					ghttp.VerifyJSON(fmt.Sprintf(`{"name":"%s","owner":{"type":"organization","slug":"%s/%s"}}`, contextName, vcsType, orgName)),
					ghttp.RespondWith(http.StatusInternalServerError, []byte(`{"message":"ignored error"}`), contentTypeHeader),
				),
			)
			By("running the command")
			command := exec.Command(pathCLI,
				"context", "create",
				"--skip-update-check",
				"--token", token,
				"--host", tempSettings.TestServer.URL(),
				"--integration-testing",
				vcsType,
				orgName,
				contextName,
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say(`Error: ignored error`))
			Eventually(session).ShouldNot(gexec.Exit(0))
		})
	})
})
