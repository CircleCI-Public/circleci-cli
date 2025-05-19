package pipeline_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/cmd/pipeline"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

type testCreatePipelineArgs struct {
	projectID    string
	statusCode   int
	pipelineName string
	description  string
	repoID       string
	filePath     string
}

type pipelineTestReader struct {
	inputs map[string]string
}

func (r pipelineTestReader) ReadSecretString(msg string) (string, error) {
	if val, ok := r.inputs[msg]; ok {
		return val, nil
	}
	return "", nil
}

func (r pipelineTestReader) AskConfirm(msg string) bool {
	return true
}

func scaffoldCMD(
	baseURL string,
	validator validator.Validator,
	opts ...pipeline.PipelineOption,
) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	config := &settings.Config{
		Token:      "testtoken",
		HTTPClient: http.DefaultClient,
		Host:       baseURL,
		DlHost:     baseURL,
	}
	cmd := pipeline.NewPipelineCommand(config, validator, opts...)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd, stdout, stderr
}

func TestCreatePipeline(t *testing.T) {
	const (
		projectID    = "test-project-id"
		pipelineName = "Test Pipeline"
		description  = "Test pipeline description"
		repoID       = "123456"
		filePath     = ".circleci/config.yml"
	)

	tests := []struct {
		name    string
		args    testCreatePipelineArgs
		want    string
		wantErr bool
	}{
		{
			name: "Create pipeline successfully",
			args: testCreatePipelineArgs{
				projectID:    projectID,
				statusCode:   http.StatusOK,
				pipelineName: pipelineName,
				description:  description,
				repoID:       repoID,
				filePath:     filePath,
			},
			want: fmt.Sprintf("Pipeline '%s' successfully created in repository 'test-org/test-repo'\nYou may view your new pipeline at: https://app.circleci.com/projects/%s/pipelines\n", pipelineName, projectID),
		},
		{
			name: "Handle API error when creating pipeline",
			args: testCreatePipelineArgs{
				projectID:    projectID,
				statusCode:   http.StatusInternalServerError,
				pipelineName: pipelineName,
				description:  description,
				repoID:       repoID,
				filePath:     filePath,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.String(), fmt.Sprintf("/projects/%s/pipeline-definitions", tt.args.projectID))

				// Validate request body
				body, err := io.ReadAll(r.Body)
				assert.NilError(t, err)

				var requestBody map[string]interface{}
				err = json.Unmarshal(body, &requestBody)
				assert.NilError(t, err)

				// Check the main fields
				assert.Equal(t, requestBody["name"].(string), tt.args.pipelineName)
				assert.Equal(t, requestBody["description"].(string), tt.args.description)

				// Check nested fields
				configSource, ok := requestBody["config_source"].(map[string]interface{})
				assert.Assert(t, ok, "config_source should be a map")
				assert.Equal(t, configSource["provider"].(string), "github_app")
				assert.Equal(t, configSource["file_path"].(string), tt.args.filePath)

				repo, ok := configSource["repo"].(map[string]interface{})
				assert.Assert(t, ok, "repo should be a map")
				assert.Equal(t, repo["external_id"].(string), tt.args.repoID)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.args.statusCode)

				if tt.args.statusCode == http.StatusOK {
					responseBody := map[string]interface{}{
						"id":          "pipeline-123",
						"name":        tt.args.pipelineName,
						"description": tt.args.description,
						"config_source": map[string]interface{}{
							"provider": "github_app",
							"repo": map[string]interface{}{
								"external_id": tt.args.repoID,
								"full_name":   "test-org/test-repo",
							},
							"file_path": tt.args.filePath,
						},
						"checkout_source": map[string]interface{}{
							"provider": "github_app",
							"repo": map[string]interface{}{
								"external_id": tt.args.repoID,
								"full_name":   "test-org/test-repo",
							},
						},
					}
					responseJSON, err := json.Marshal(responseBody)
					assert.NilError(t, err)
					_, err = w.Write(responseJSON)
					assert.NilError(t, err)
				} else {
					errorBody := map[string]string{
						"message": "Internal server error",
					}
					errorJSON, err := json.Marshal(errorBody)
					assert.NilError(t, err)
					_, err = w.Write(errorJSON)
					assert.NilError(t, err)
				}
			}

			server := httptest.NewServer(handler)
			defer server.Close()

			inputs := map[string]string{
				"Enter a name for the pipeline":                   tt.args.pipelineName,
				"Enter a description for the pipeline (optional)": tt.args.description,
				"Enter the ID of your github repository":          tt.args.repoID,
				"Enter the path to your circleci config file":     tt.args.filePath,
			}

			opts := []pipeline.PipelineOption{
				pipeline.CustomReader(pipelineTestReader{inputs: inputs}),
			}

			noValidator := func(_ *cobra.Command, _ []string) error {
				return nil
			}

			cmd, stdout, _ := scaffoldCMD(server.URL, noValidator, opts...)
			cmd.SetArgs([]string{"create", tt.args.projectID})

			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Create pipeline command test failed: error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				got := stdout.String()
				if got != tt.want {
					t.Errorf("Create pipeline command output = %q, want %q", got, tt.want)
				}
			}
		})
	}
}
