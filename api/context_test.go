package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	// we can't dot-import ginkgo because api.Context is a thing.
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type graphQLRequest struct {
	Query     string
	Variables map[string]interface{}
}

func createSingleUseGraphQLServer(result interface{}, requestAssertions func(requestCount uint64, req *graphQLRequest)) (*httptest.Server, *GraphQLContextClient) {
	response := graphql.Response{
		Data: result,
	}

	var requestCount uint64 = 0

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		atomic.AddUint64(&requestCount, 1)
		defer ginkgo.GinkgoRecover()
		var request graphQLRequest
		Expect(json.NewDecoder(req.Body).Decode(&request)).To(Succeed())
		requestAssertions(requestCount, &request)
		bytes, err := json.Marshal(response)
		Expect(err).ToNot(HaveOccurred())
		_, err = rw.Write(bytes)
		Expect(err).ToNot(HaveOccurred())
	}))
	client := NewContextGraphqlClient(server.URL, server.URL, "token", false)
	return server, client
}

var _ = ginkgo.Describe("API", func() {
	ginkgo.Describe("FooBar", func() {
		ginkgo.It("improveVcsTypeError", func() {

			unrelatedError := errors.New("foo")

			Expect(unrelatedError).Should(Equal(improveVcsTypeError(unrelatedError)))

			errors := []graphql.ResponseError{
				graphql.ResponseError{
					Message: "foo",
				},
			}

			errors[0].Extensions.EnumType = "VCSType"
			errors[0].Extensions.Value = "pear"
			errors[0].Extensions.AllowedValues = []string{"apple", "banana"}
			var vcsError graphql.ResponseErrorsCollection = errors
			Expect("Invalid vcs-type 'pear' provided, expected one of apple, banana").Should(Equal(improveVcsTypeError(vcsError).Error()))

		})
	})

	ginkgo.Describe("Create Context", func() {

		ginkgo.It("can handles failure creating contexts", func() {

			var result struct {
				CreateContext struct {
					Error struct {
						Type string
					}
				}
			}

			result.CreateContext.Error.Type = "force-this-error"

			server, client := createSingleUseGraphQLServer(result, func(count uint64, req *graphQLRequest) {
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
			err := client.CreateContext("test-vcs", "test-org", "foo-bar")
			Expect(err).To(MatchError("Error creating context: force-this-error"))

		})

	})

	ginkgo.It("can handles success creating contexts", func() {

		var result struct {
			CreateContext struct {
				Error struct {
					Type string
				}
			}
		}

		result.CreateContext.Error.Type = ""

		server, client := createSingleUseGraphQLServer(result, func(count uint64, req *graphQLRequest) {

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

		Expect(client.CreateContext( "test-vcs", "test-org", "foo-bar")).To(Succeed())

	})

	ginkgo.Describe("List Contexts", func() {

		ginkgo.It("can list contexts", func() {

			ctx := circleCIContext{
				CreatedAt: "2018-04-24T19:38:37.212Z",
				Name:      "Sheep",
			}

			list := contextsQueryResponse{}

			list.Organization.Id = "C3D79A95-6BD5-40B4-9958-AB6BDC4CAD50"
			list.Organization.Contexts.Edges = []struct{ Node circleCIContext }{
				struct{ Node circleCIContext }{
					Node: ctx,
				},
			}

			server, client := createSingleUseGraphQLServer(list, func(count uint64, req *graphQLRequest) {
				switch count {
				case 1:
					Expect(req.Variables["organizationName"]).To(Equal("test-org"))
					Expect(req.Variables["organizationVcs"]).To(Equal("TEST-VCS"))
				case 2:
					Expect(req.Variables["orgId"]).To(Equal("C3D79A95-6BD5-40B4-9958-AB6BDC4CAD50"))
				}
			})
			defer server.Close()

			result, err := client.Contexts("test-vcs", "test-org")
			Expect(err).NotTo(HaveOccurred())
			context := (*result)[0]
			Expect(context.Name).To(Equal("Sheep"))
		})
	})
})
