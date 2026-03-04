package pipeline

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pipelineapi "github.com/CircleCI-Public/circleci-cli/api/pipeline"
	projectapi "github.com/CircleCI-Public/circleci-cli/api/project"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestPipelineDefinitionsListCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/project/gh/test-org/test-repo":
			assert.Equal(t, http.MethodGet, r.Method)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "test-project-id"})
		case "/projects/test-project-id/pipeline-definitions":
			assert.Equal(t, http.MethodGet, r.Method)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []pipelineapi.PipelineDefinitionInfo{
					{
						ID:          "123",
						Name:        "test-pipeline",
						Description: "test-description",
						ConfigSource: pipelineapi.ConfigSourceResponse{
							Provider: "github_app",
							Repo: pipelineapi.RepoResponse{
								FullName: "",
							},
							FilePath: "",
						},
						CheckoutSource: pipelineapi.CheckoutSourceResponse{
							Provider: "github_app",
							Repo: pipelineapi.RepoResponse{
								FullName: "",
							},
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := settings.Config{
		Token:      "testtoken",
		Host:       server.URL,
		HTTPClient: http.DefaultClient,
	}

	pipelineClient, err := pipelineapi.NewPipelineRestClient(cfg)
	assert.NoError(t, err)

	projectClient, err := projectapi.NewProjectRestClient(cfg)
	assert.NoError(t, err)

	opts := &pipelineOpts{
		pipelineClient: pipelineClient,
		projectClient:  projectClient,
	}

	noValidator := func(_ *cobra.Command, _ []string) error { return nil }

	cmd := newDefinitionsListCommand(opts, noValidator)
	cmd.SetArgs([]string{"gh/test-org/test-repo"})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	assert.NoError(t, err)
	assert.Equal(t, `Pipeline Definitions:

ID: 123
Name: test-pipeline
Description: test-description
Config Source:  ()
Checkout Source: 
`, outBuf.String()+errBuf.String())
}
