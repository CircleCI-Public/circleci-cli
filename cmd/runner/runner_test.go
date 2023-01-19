package runner

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func TestConfigForServer(t *testing.T) {
	var tests = []struct {
		name         string
		expectedUrls []string
		restEndpoint string
	}{
		{
			name: "Test config updated for server",
			expectedUrls: []string{
				"/api/v2/runner/resource",
				"/api/v2/runner/token",
			},
			restEndpoint: "api/v2",
		},
		{
			name: "Test config updated for cloud",
			expectedUrls: []string{
				"/api/v3/runner/resource",
				"/api/v3/runner/token",
			},
			restEndpoint: "api/v3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestCount int
			response := func(w http.ResponseWriter) {
				_, _ = io.WriteString(w, `
				{
					"id": "2bc0df8e-d258-4ae8-9c2b-3793f004725f",
					"resource_class": "the-namespace/the-resource-class",
					"description": "the-description"
				}`)
			}

			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.URL.String(), tt.expectedUrls[requestCount])
				w.Header().Add("Content-Type", "application/json")
				response(w)
				requestCount++
			}))
			defer svr.Close()

			config := &settings.Config{Host: svr.URL, RestEndpoint: tt.restEndpoint, HTTPClient: http.DefaultClient}
			cmd := NewCommand(config, nil)

			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)

			cmd.SetArgs([]string{
				"resource-class", "create", "testing/test-runner", "testing",
			})

			err := cmd.Execute()

			assert.NilError(t, err)
		})
	}
}
