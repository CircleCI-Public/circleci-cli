package trigger_test

import (
	"fmt"
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
	const (
		vcsType              = "circleci"
		orgName              = "test-org"
		projectID            = "test-project-id"
		pipelineDefinitionID = "test-pipeline-definition-id"
		repoID               = "test-repo-id"
		testName             = "test-trigger"
		description          = "test-description"
		eventPreset          = "all-pushes"
	)

	// Use variables for optional fields
	// checkoutRef := "test-checkout-ref"
	// configRef := "test-config-ref"

	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    *trigger.CreateTriggerInfo
		wantErr bool
	}{
		{
			name: "Should handle a successful request with CreateTrigger",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/projects/%s/pipeline-definitions/%s/triggers", projectID, pipelineDefinitionID))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
				{
					"name": "test-trigger",
					"description": "test-description",
					"event_source": {
						"provider": "github_app",
						"repo": {
							"external_id": "test-repo-id",
						}
					},
					"event_preset": "all-pushes",
				}`))
				assert.NilError(t, err)
			},
			want: &trigger.CreateTriggerInfo{
				Id:   "123",
				Name: testName,
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
		{
			name: "Should handle a successful request with CreateTrigger with checkoutRef and configRef",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/projects/%s/pipeline-definitions/%s/triggers", projectID, pipelineDefinitionID))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
				{
					"name": "test-trigger",
					"description": "test-description",
					"event_source": {
						"provider": "github_app",
						"repo": {
							"external_id": "test-repo-id",
						}
					},
					"event_preset": "all-pushes",
					"config_ref": "test-config-ref",
					"checkout_ref": "test-checkout-ref"
				}`))
				assert.NilError(t, err)
			},
			want: &trigger.CreateTriggerInfo{
				Id:   "123",
				Name: testName,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			p, err := getTriggerRestClient(server)
			assert.NilError(t, err)

			options := trigger.CreateTriggerOptions{
				ProjectID:            projectID,
				PipelineDefinitionID: pipelineDefinitionID,
				Name:                 testName,
				Description:          description,
				RepoID:               repoID,
				EventPreset:          eventPreset,
			}

			got, err := p.CreateTrigger(options)
			if (err != nil) != tt.wantErr {
				t.Errorf("triggerRestClient.CreateTrigger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("pipelineRestClient.CreatePipeline() = %v, want %v", got, tt.want)
			}
		})
	}
}
