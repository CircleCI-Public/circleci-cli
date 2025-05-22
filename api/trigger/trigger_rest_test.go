package trigger_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/api/trigger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"gotest.tools/v3/assert"
)

func getTriggerRestClient(server *httptest.Server) (trigger.TriggerClient, error) {
	client := &http.Client{}

	return trigger.NewTriggerRestClient(settings.Config{
		RestEndpoint: "api/v2",
		Host:         server.URL,
		HTTPClient:   client,
		Token:        "token",
	})
}

func Test_triggerRestClient_CreateTrigger(t *testing.T) {
	tests := []struct {
		name    string
		options trigger.CreateTriggerOptions
		handler http.HandlerFunc
		want    *trigger.CreateTriggerInfo
		wantErr bool
	}{
		{
			name: "Should handle a successful request with CreateTrigger",
			options: trigger.CreateTriggerOptions{
				ProjectID:            "testProjectID",
				PipelineDefinitionID: "pipelineDefinitionID",
				Name:                 "testName",
				Description:          "description",
				RepoID:               "repoID",
				EventPreset:          "eventPreset",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/projects/%s/pipeline-definitions/%s/triggers", "testProjectID", "pipelineDefinitionID"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				resp, err := json.Marshal(map[string]any{
					"id":   "123",
					"name": "testName",
				})
				assert.NilError(t, err)
				_, err = w.Write(resp)
				assert.NilError(t, err)
			},
			want: &trigger.CreateTriggerInfo{
				Id:   "123",
				Name: "testName",
			},
		},
		{
			name: "Should handle a successful request with CreateTrigger with checkoutRef and configRef",
			options: trigger.CreateTriggerOptions{
				ProjectID:            "projectID",
				PipelineDefinitionID: "pipelineDefinitionID",
				Name:                 "testName",
				Description:          "description",
				RepoID:               "repoID",
				EventPreset:          "eventPreset",
				ConfigRef:            "configRef",
				CheckoutRef:          "checkoutRef",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/projects/%s/pipeline-definitions/%s/triggers", "projectID", "pipelineDefinitionID"))

				expectedRequestBody := map[string]any{
					"name":         "testName",
					"description":  "description",
					"event_preset": "eventPreset",
					"event_source": map[string]any{
						"provider": "github_app",
						"repo": map[string]any{
							"external_id": "repoID",
						},
					},
					"config_ref":   "configRef",
					"checkout_ref": "checkoutRef",
				}

				b, err := io.ReadAll(r.Body)
				assert.NilError(t, err)

				var actualRequestBody any
				err = json.Unmarshal(b, &actualRequestBody)
				assert.NilError(t, err)

				assert.DeepEqual(t, actualRequestBody, expectedRequestBody)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				resp, err := json.Marshal(map[string]any{
					"id":   "123",
					"name": "testName",
				})
				assert.NilError(t, err)
				_, err = w.Write(resp)
				assert.NilError(t, err)
			},
			want: &trigger.CreateTriggerInfo{
				Id:   "123",
				Name: "testName",
			},
		},
		{
			name: "Should handle an error request with CreateTrigger",
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

			p, err := getTriggerRestClient(server)
			assert.NilError(t, err)

			got, err := p.CreateTrigger(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("triggerRestClient.CreateTrigger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("triggerRestClient.CreateTrigger() = %v, want %v", got, tt.want)
			}
		})
	}
}
