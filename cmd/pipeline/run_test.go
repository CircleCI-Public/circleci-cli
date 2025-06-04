package pipeline

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/api/pipeline"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

func Test_newRunCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedError  string
		expectedConfig pipeline.TriggerConfigTestRunOptions
		reader         *mockReader
	}{
		{
			name:          "missing required arguments",
			args:          []string{},
			expectedError: "accepts 2 arg(s), received 0",
		},
		{
			name:          "missing project argument",
			args:          []string{"my-org"},
			expectedError: "accepts 2 arg(s), received 1",
		},
		{
			name: "missing pipeline definition id",
			args: []string{"my-org", "my-project"},
			reader: &mockReader{
				responses: map[string]string{
					"Enter the pipeline definition ID for your pipeline":                                                "abc123",
					"You must specify either a config branch or tag. Enter a branch (or leave blank to enter a tag):":   "main",
					"You must specify either a checkout branch or tag. Enter a branch (or leave blank to enter a tag):": "feature",
				},
				confirmPrompts: map[string]bool{
					"Do you want to test with a local config file? This will bypass the config file in the repository.": false,
				},
			},
			expectedConfig: pipeline.TriggerConfigTestRunOptions{
				Organization:         "my-org",
				Project:              "my-project",
				PipelineDefinitionID: "abc123",
				ConfigBranch:         "main",
				ConfigTag:            "",
				CheckoutBranch:       "feature",
				CheckoutTag:          "",
				Parameters:           map[string]interface{}{},
				ConfigFilePath:       "",
			},
		},
		{
			name: "valid required arguments",
			args: []string{"my-org", "my-project", "--pipeline-definition-id", "abc123"},
			reader: &mockReader{
				responses: map[string]string{
					"You must specify either a config branch or tag. Enter a branch (or leave blank to enter a tag):":   "main",
					"You must specify either a checkout branch or tag. Enter a branch (or leave blank to enter a tag):": "feature",
				},
				confirmPrompts: map[string]bool{
					"Do you want to test with a local config file? This will bypass the config file in the repository.": false,
				},
			},
			expectedConfig: pipeline.TriggerConfigTestRunOptions{
				Organization:         "my-org",
				Project:              "my-project",
				PipelineDefinitionID: "abc123",
				ConfigBranch:         "main",
				ConfigTag:            "",
				CheckoutBranch:       "feature",
				CheckoutTag:          "",
				Parameters:           map[string]interface{}{},
				ConfigFilePath:       "",
			},
		},
		{
			name: "with config branch and checkout branch",
			args: []string{"my-org", "my-project", "--pipeline-definition-id", "abc123", "--config-branch", "main", "--checkout-branch", "feature"},
			reader: &mockReader{
				confirmPrompts: map[string]bool{
					"Do you want to test with a local config file? This will bypass the config file in the repository.": false,
				},
			},
			expectedConfig: pipeline.TriggerConfigTestRunOptions{
				Organization:         "my-org",
				Project:              "my-project",
				PipelineDefinitionID: "abc123",
				ConfigBranch:         "main",
				ConfigTag:            "",
				CheckoutBranch:       "feature",
				CheckoutTag:          "",
				Parameters:           map[string]interface{}{},
				ConfigFilePath:       "",
			},
		},
		{
			name: "with config tag and checkout tag",
			args: []string{"my-org", "my-project", "--pipeline-definition-id", "abc123", "--config-tag", "v1.0.0", "--checkout-tag", "v1.0.0"},
			reader: &mockReader{
				confirmPrompts: map[string]bool{
					"Do you want to test with a local config file? This will bypass the config file in the repository.": false,
				},
			},
			expectedConfig: pipeline.TriggerConfigTestRunOptions{
				Organization:         "my-org",
				Project:              "my-project",
				PipelineDefinitionID: "abc123",
				ConfigBranch:         "",
				ConfigTag:            "v1.0.0",
				CheckoutBranch:       "",
				CheckoutTag:          "v1.0.0",
				Parameters:           map[string]interface{}{},
				ConfigFilePath:       "",
			},
		},
		{
			name: "with parameters",
			args: []string{"my-org", "my-project", "--pipeline-definition-id", "abc123", "--parameters", "key1=value1", "--parameters", "key2=value2"},
			reader: &mockReader{
				responses: map[string]string{
					"You must specify either a config branch or tag. Enter a branch (or leave blank to enter a tag):":   "main",
					"You must specify either a checkout branch or tag. Enter a branch (or leave blank to enter a tag):": "feature",
				},
				confirmPrompts: map[string]bool{
					"Do you want to test with a local config file? This will bypass the config file in the repository.": false,
				},
			},
			expectedConfig: pipeline.TriggerConfigTestRunOptions{
				Organization:         "my-org",
				Project:              "my-project",
				PipelineDefinitionID: "abc123",
				ConfigBranch:         "main",
				ConfigTag:            "",
				CheckoutBranch:       "feature",
				CheckoutTag:          "",
				Parameters: map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
				ConfigFilePath: "",
			},
		},
		{
			name:          "mutually exclusive config branch and tag",
			args:          []string{"my-org", "my-project", "--pipeline-definition-id", "abc123", "--config-branch", "main", "--config-tag", "v1.0.0"},
			expectedError: "if any flags in the group [config-branch config-tag] are set none of the others can be; [config-branch config-tag] were all set",
		},
		{
			name:          "mutually exclusive checkout branch and tag",
			args:          []string{"my-org", "my-project", "--pipeline-definition-id", "abc123", "--checkout-branch", "main", "--checkout-tag", "v1.0.0"},
			expectedError: "if any flags in the group [checkout-branch checkout-tag] are set none of the others can be; [checkout-branch checkout-tag] were all set",
		},
		{
			name: "with local config file prompt - yes",
			args: []string{"my-org", "my-project", "--pipeline-definition-id", "abc123"},
			reader: &mockReader{
				responses: map[string]string{
					"Enter the path to your local config file":                                                          "/path/to/config.yml",
					"You must specify either a checkout branch or tag. Enter a branch (or leave blank to enter a tag):": "feature",
				},
				confirmPrompts: map[string]bool{
					"Do you want to test with a local config file? This will bypass the config file in the repository.": true,
				},
			},
			expectedConfig: pipeline.TriggerConfigTestRunOptions{
				Organization:         "my-org",
				Project:              "my-project",
				PipelineDefinitionID: "abc123",
				ConfigBranch:         "",
				ConfigTag:            "",
				CheckoutBranch:       "feature",
				CheckoutTag:          "",
				ConfigFilePath:       "/path/to/config.yml",
				Parameters:           map[string]interface{}{},
			},
		},
		{
			name: "with both pipeline definition id prompt and local config file prompt",
			args: []string{"my-org", "my-project"},
			reader: &mockReader{
				responses: map[string]string{
					"Enter the pipeline definition ID for your pipeline":                                                "abc123",
					"Enter the path to your local config file":                                                          "/path/to/config.yml",
					"You must specify either a checkout branch or tag. Enter a branch (or leave blank to enter a tag):": "feature",
				},
				confirmPrompts: map[string]bool{
					"Do you want to test with a local config file? This will bypass the config file in the repository.": true,
				},
			},
			expectedConfig: pipeline.TriggerConfigTestRunOptions{
				Organization:         "my-org",
				Project:              "my-project",
				PipelineDefinitionID: "abc123",
				ConfigBranch:         "",
				ConfigTag:            "",
				CheckoutBranch:       "feature",
				CheckoutTag:          "",
				ConfigFilePath:       "/path/to/config.yml",
				Parameters:           map[string]interface{}{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tempConfigFile string
			var cleanup func()

			// If the test expects a config file, create a temp file
			if tt.expectedConfig.ConfigFilePath != "" {
				tmpfile, err := os.CreateTemp("", "config-*.yml")
				assert.NilError(t, err)
				if _, err := tmpfile.WriteString("version: 2.1\njobs: {}\n"); err != nil {
					assert.NilError(t, err)
				}
				tmpfile.Close()
				tempConfigFile = tmpfile.Name()
				cleanup = func() { os.Remove(tempConfigFile) }
				defer cleanup()
				// Patch the expected config to use the temp file path
				tt.expectedConfig.ConfigFilePath = tempConfigFile
				if tt.reader != nil {
					for k, v := range tt.reader.responses {
						if v == "/path/to/config.yml" {
							tt.reader.responses[k] = tempConfigFile
						}
					}
				}
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if tt.expectedError != "" {
					w.WriteHeader(400)
					if _, err := w.Write([]byte(`{"message": "error"}`)); err != nil {
						panic(err)
					}
					return
				}
				var reqBody map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&reqBody)

				// Assert on API payload fields
				assert.Equal(t, reqBody["definition_id"], tt.expectedConfig.PipelineDefinitionID)
				if config, ok := reqBody["config"].(map[string]interface{}); ok {
					if tt.expectedConfig.ConfigBranch != "" {
						assert.Equal(t, config["branch"], tt.expectedConfig.ConfigBranch)
					} else {
						assert.Assert(t, config["branch"] == nil || config["branch"] == "")
					}
					if tt.expectedConfig.ConfigTag != "" {
						assert.Equal(t, config["tag"], tt.expectedConfig.ConfigTag)
					} else {
						assert.Assert(t, config["tag"] == nil || config["tag"] == "")
					}
					// If content is expected, check it
					if tt.expectedConfig.ConfigFilePath != "" {
						assert.Assert(t, config["content"] != nil && config["content"] != "")
					}
				}
				if checkout, ok := reqBody["checkout"].(map[string]interface{}); ok {
					if tt.expectedConfig.CheckoutBranch != "" {
						assert.Equal(t, checkout["branch"], tt.expectedConfig.CheckoutBranch)
					} else {
						assert.Assert(t, checkout["branch"] == nil || checkout["branch"] == "")
					}
					if tt.expectedConfig.CheckoutTag != "" {
						assert.Equal(t, checkout["tag"], tt.expectedConfig.CheckoutTag)
					} else {
						assert.Assert(t, checkout["tag"] == nil || checkout["tag"] == "")
					}
				}
				if params, ok := reqBody["parameters"].(map[string]interface{}); ok {
					assert.DeepEqual(t, params, tt.expectedConfig.Parameters)
				} else {
					assert.DeepEqual(t, map[string]interface{}{}, tt.expectedConfig.Parameters)
				}

				w.WriteHeader(201)
				if _, err := w.Write([]byte(`{
					"id": "pipeline-id-123",
					"number": 42,
					"state": "created",
					"created_at": "2024-06-01T12:00:00Z"
				}`)); err != nil {
					panic(err)
				}
			}))
			defer server.Close()

			cfg := settings.Config{
				Token:      "testtoken",
				Host:       server.URL,
				HTTPClient: http.DefaultClient,
			}
			client, err := pipeline.NewPipelineRestClient(cfg)
			assert.NilError(t, err)

			ops := &pipelineOpts{
				pipelineClient: client,
				reader:         tt.reader,
			}
			if ops.reader == nil {
				ops.reader = &mockReader{}
			}

			cmd := newRunCommand(ops, validator.Validator(func(cmd *cobra.Command, args []string) error {
				return nil
			}))
			cmd.SetArgs(tt.args)
			err = cmd.Execute()

			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
				return
			}

			assert.NilError(t, err)
		})
	}
}

// Mock reader for testing
type mockReader struct {
	responses      map[string]string
	confirmPrompts map[string]bool
}

func (m *mockReader) ReadStringFromUser(prompt string) string {
	if m.responses != nil {
		if resp, ok := m.responses[prompt]; ok {
			return resp
		}
	}
	return ""
}

func (m *mockReader) AskConfirm(prompt string) bool {
	if m.confirmPrompts != nil {
		if resp, ok := m.confirmPrompts[prompt]; ok {
			return resp
		}
	}
	return true
}
