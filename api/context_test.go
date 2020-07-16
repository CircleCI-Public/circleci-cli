package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"

	"github.com/CircleCI-Public/circleci-cli/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type graphQLRequst struct {
	Query     string
	Variables map[string]interface{}
}

func createSingleUseGraphQLServer(result interface{}, requestAssertions func(requestCount uint64, req *graphQLRequst)) (*httptest.Server, *client.Client) {
	response := client.Response{
		Data: result,
	}

	var requestCount uint64 = 0

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		atomic.AddUint64(&requestCount, 1)
		defer GinkgoRecover()
		var request graphQLRequst
		Expect(json.NewDecoder(req.Body).Decode(&request)).To(Succeed())
		requestAssertions(requestCount, &request)
		bytes, err := json.Marshal(response)
		Expect(err).ToNot(HaveOccurred())
		_, err = rw.Write(bytes)
		Expect(err).ToNot(HaveOccurred())
	}))
	client := client.NewClient(server.URL, server.URL, "token", false)
	return server, client
}

var _ = Describe("API", func() {
	Describe("FooBar", func() {
		It("improveVcsTypeError", func() {

			unrelatedError := errors.New("foo")

			Expect(unrelatedError).Should(Equal(improveVcsTypeError(unrelatedError)))

			errors := []client.ResponseError{
				client.ResponseError{
					Message: "foo",
				},
			}

			errors[0].Extensions.EnumType = "VCSType"
			errors[0].Extensions.Value = "pear"
			errors[0].Extensions.AllowedValues = []string{"apple", "banana"}
			var vcsError client.ResponseErrorsCollection = errors
			Expect("Invalid vcs-type 'pear' provided, expected one of apple, banana").Should(Equal(improveVcsTypeError(vcsError).Error()))

		})
	})

	Describe("Create Context", func() {

		It("can handles failure creating contexts", func() {

			var result struct {
				CreateContext struct {
					Error struct {
						Type string
					}
				}
			}

			result.CreateContext.Error.Type = "force-this-error"

			server, client := createSingleUseGraphQLServer(result, func(count uint64, req *graphQLRequst) {
				switch count {
				case 1:
					Expect(req.Variables["organizationName"]).To(Equal("test-org"))
					Expect(req.Variables["organizationVcs"]).To(Equal("TEST-VCS"))
				case 2:
					Expect(req.Variables["input"].(map[string]interface{})["ownerType"]).To(Equal("ORGANIZATION"))
					Expect(req.Variables["input"].(map[string]interface{})["contextName"]).To(Equal("foo-bar"))
				}
			})
			defer server.Close()
			err := CreateContext(client, "test-vcs", "test-org", "foo-bar")
			Expect(err).To(MatchError("Error creating context: force-this-error"))

		})

	})

	It("can handles success creating contexts", func() {

		var result struct {
			CreateContext struct {
				Error struct {
					Type string
				}
			}
		}

		result.CreateContext.Error.Type = ""

		server, client := createSingleUseGraphQLServer(result, func(count uint64, req *graphQLRequst) {

			switch count {
			case 1:
				Expect(req.Variables["organizationName"]).To(Equal("test-org"))
				Expect(req.Variables["organizationVcs"]).To(Equal("TEST-VCS"))
			case 2:
				Expect(req.Variables["input"].(map[string]interface{})["ownerType"]).To(Equal("ORGANIZATION"))
				Expect(req.Variables["input"].(map[string]interface{})["contextName"]).To(Equal("foo-bar"))
			}

		})
		defer server.Close()

		Expect(CreateContext(client, "test-vcs", "test-org", "foo-bar")).To(Succeed())

	})

	Describe("List Contexts", func() {

		It("can list contexts", func() {

			ctx := CircleCIContext{
				CreatedAt: "2018-04-24T19:38:37.212Z",
				Name:      "Sheep",
				Resources: []Resource{
					{
						CreatedAt:      "2018-04-24T19:38:37.212Z",
						Variable:       "CI",
						TruncatedValue: "1234",
					},
				},
			}

			list := ContextsQueryResponse{}

			list.Organization.Id = "C3D79A95-6BD5-40B4-9958-AB6BDC4CAD50"
			list.Organization.Contexts.Edges = []struct{ Node CircleCIContext }{
				struct{ Node CircleCIContext }{
					Node: ctx,
				},
			}

			server, client := createSingleUseGraphQLServer(list, func(count uint64, req *graphQLRequst) {
				switch count {
				case 1:
					Expect(req.Variables["organizationName"]).To(Equal("test-org"))
					Expect(req.Variables["organizationVcs"]).To(Equal("TEST-VCS"))
				case 2:
					Expect(req.Variables["orgId"]).To(Equal("C3D79A95-6BD5-40B4-9958-AB6BDC4CAD50"))
				}
			})
			defer server.Close()

			result, err := ListContexts(client, "test-org", "test-vcs")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Organization.Id).To(Equal("C3D79A95-6BD5-40B4-9958-AB6BDC4CAD50"))
			context := result.Organization.Contexts.Edges[0].Node
			Expect(context.Name).To(Equal("Sheep"))
			Expect(context.Resources).To(HaveLen(1))
			resource := context.Resources[0]
			Expect(resource.Variable).To(Equal("CI"))
			Expect(resource.TruncatedValue).To(Equal("1234"))

		})
	})
})
