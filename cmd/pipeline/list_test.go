package pipeline

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/api/pipeline"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestListPipelineDefinitions(t *testing.T) {
	tests := []struct {
		name           string
		response       []*pipeline.PipelineDefinitionInfo
		expectedOutput string
		expectedError  string
	}{
		{
			name: "List pipeline definitions successfully",
			response: []*pipeline.PipelineDefinitionInfo{
				{
					ID:          "123",
					Name:        "test-pipeline",
					Description: "test-description",
					ConfigSource: pipeline.ConfigSourceResponse{
						Provider: "github_app",
						Repo: pipeline.RepoResponse{
							FullName: "",
						},
						FilePath: "",
					},
					CheckoutSource: pipeline.CheckoutSourceResponse{
						Provider: "github_app",
						Repo: pipeline.RepoResponse{
							FullName: "",
						},
					},
				},
			},
			expectedOutput: `Pipeline Definitions:

ID: 123
Name: test-pipeline
Description: test-description
Config Source:  ()
Checkout Source: 
`,
		},
		{
			name:           "Handle empty pipeline definitions list",
			response:       []*pipeline.PipelineDefinitionInfo{},
			expectedOutput: "No pipeline definitions found for this project.\n",
		},
		{
			name:           "Handle API error",
			response:       nil,
			expectedError:  "error",
			expectedOutput: "Error: error\nUsage:\n  list <project-id> [flags]\n\nFlags:\n  -h, --help   help for list\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := "testtoken"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/projects/test-project-id/pipeline-definitions", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				if tt.response == nil {
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]string{"message": "error"})
					return
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"items": tt.response,
				})
			}))
			defer server.Close()

			cfg := settings.Config{
				Token:      token,
				Host:       server.URL,
				HTTPClient: http.DefaultClient,
			}

			client, err := pipeline.NewPipelineRestClient(cfg)
			assert.NoError(t, err)

			opts := &pipelineOpts{
				pipelineClient: client,
			}

			noValidator := func(_ *cobra.Command, _ []string) error {
				return nil
			}

			cmd := newListCommand(opts, noValidator)
			cmd.SetArgs([]string{"test-project-id"})

			var outBuf, errBuf bytes.Buffer
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)

			err = cmd.Execute()
			got := outBuf.String() + errBuf.String()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, got, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOutput, got)
			}
		})
	}
}
