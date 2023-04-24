package api

import (
	"fmt"
	"io"
	"net/http"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

type MockRequestResponse struct {
	Request       string
	Status        int
	Response      string
	ErrorResponse string
}

// Uses Ginkgo http handler to mock out http requests and make assertions off the results.
// If ErrorResponse is defined in the passed handler it will override the Response.
func appendRESTPostHandler(server *ghttp.Server, combineHandlers ...MockRequestResponse) {
	for _, handler := range combineHandlers {
		responseBody := handler.Response
		if handler.ErrorResponse != "" {
			responseBody = handler.ErrorResponse
		}

		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v2/context"),
				ghttp.VerifyContentType("application/json"),
				func(w http.ResponseWriter, req *http.Request) {
					body, err := io.ReadAll(req.Body)
					Expect(err).ShouldNot(HaveOccurred())
					err = req.Body.Close()
					Expect(err).ShouldNot(HaveOccurred())
					Expect(handler.Request).Should(MatchJSON(body), "JSON Mismatch")
				},
				ghttp.RespondWith(handler.Status, responseBody),
			),
		)
	}
}

func getContextRestClient(server *ghttp.Server) (*ContextRestClient, error) {
	client := &http.Client{}

	return NewContextRestClient(settings.Config{
		RestEndpoint: "api/v2",
		Host:         server.URL(),
		HTTPClient:   client,
		Token:        "token",
	})
}

var _ = ginkgo.Describe("Context Rest Tests", func() {
	ginkgo.It("Should handle a successful request with createContextWithOrgID", func() {
		server := ghttp.NewServer()

		defer server.Close()

		name := "name"
		orgID := "497f6eca-6276-4993-bfeb-53cbbbba6f08"
		client, err := getContextRestClient(server)
		Expect(err).To(BeNil())

		appendRESTPostHandler(server, MockRequestResponse{
			Status:   http.StatusOK,
			Request:  fmt.Sprintf(`{"name": "%s","owner":{"id":"%s"}}`, name, orgID),
			Response: fmt.Sprintf(`{"id": "%s", "name": "%s", "created_at": "2015-09-21T17:29:21.042Z" }`, orgID, name),
		})

		err = client.CreateContextWithOrgID(&orgID, name)
		Expect(err).To(BeNil())
	})

	ginkgo.It("Should handle an error request with createContextWithOrgID", func() {
		server := ghttp.NewServer()

		defer server.Close()

		name := "name"
		orgID := "497f6eca-6276-4993-bfeb-53cbbbba6f08"
		client, err := getContextRestClient(server)
		Expect(err).To(BeNil())

		appendRESTPostHandler(server, MockRequestResponse{
			Status:        http.StatusInternalServerError,
			Request:       fmt.Sprintf(`{"name": "%s","owner":{"id":"%s"}}`, name, orgID),
			ErrorResponse: `{"message": "🍎"}`,
		})

		err = client.CreateContextWithOrgID(&orgID, name)
		Expect(err).ToNot(BeNil())
	})
})
