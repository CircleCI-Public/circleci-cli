package project_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/api/project"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"gotest.tools/v3/assert"
)

func getProjectRestClient(server *httptest.Server) (project.ProjectClient, error) {
	client := &http.Client{}

	return project.NewProjectRestClient(settings.Config{
		RestEndpoint: "api/v2",
		Host:         server.URL,
		HTTPClient:   client,
		Token:        "token",
	})
}

func Test_projectRestClient_ListAllEnvironmentVariables(t *testing.T) {
	const (
		vcsType  = "github"
		orgName  = "test-org"
		projName = "test-proj"
	)
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    []*project.ProjectEnvironmentVariable
		wantErr bool
	}{
		{
			name: "Should handle a successful request with ListAllEnvironmentVariables",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/project/%s/%s/%s/envvar", vcsType, orgName, projName))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
				{
					"items": [{
						"name": "foo",
						"value": "xxxx1234"
					}],
					"next_page_token": ""
				}`))
				assert.NilError(t, err)
			},
			want: []*project.ProjectEnvironmentVariable{
				{
					Name:  "foo",
					Value: "xxxx1234",
				},
			},
		},
		{
			name: "Should handle a request containing next_page_token with ListAllEnvironmentVariables",
			handler: func(w http.ResponseWriter, r *http.Request) {
				u, err := url.ParseQuery(r.URL.RawQuery)
				assert.NilError(t, err)

				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusOK)
				if tk := u.Get("page-token"); tk == "" {
					_, err := w.Write([]byte(`
					{
						"items": [
							{
								"name":  "foo1",
								"value": "xxxx1234"
							},
							{
								"name":  "foo2",
								"value": "xxxx2345"
							}
						],
						"next_page_token": "pagetoken"
					}`))
					assert.NilError(t, err)
				} else {
					assert.Equal(t, tk, "pagetoken")
					_, err := w.Write([]byte(`
					{
						"items": [
							{
								"name":  "bar1",
								"value": "xxxxabcd"
							},
							{
								"name":  "bar2",
								"value": "xxxxbcde"
							}
						],
						"next_page_token": ""
					}`))
					assert.NilError(t, err)
				}
			},
			want: []*project.ProjectEnvironmentVariable{
				{
					Name:  "foo1",
					Value: "xxxx1234",
				},
				{
					Name:  "foo2",
					Value: "xxxx2345",
				},
				{
					Name:  "bar1",
					Value: "xxxxabcd",
				},
				{
					Name:  "bar2",
					Value: "xxxxbcde",
				},
			},
		},
		{
			name: "Should handle an error request with ListAllEnvironmentVariables",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"message": "error"}`))
				assert.NilError(t, err)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			p, err := getProjectRestClient(server)
			assert.NilError(t, err)

			got, err := p.ListAllEnvironmentVariables(vcsType, orgName, projName)
			if (err != nil) != tt.wantErr {
				t.Errorf("projectRestClient.ListAllEnvironmentVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("projectRestClient.ListAllEnvironmentVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}