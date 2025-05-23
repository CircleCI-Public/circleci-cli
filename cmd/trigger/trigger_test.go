package trigger_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/cmd/trigger"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

type testCreateTriggerArgs struct {
	projectID            string
	pipelineDefinitionID string
	statusCode           int
	triggerName          string
	description          string
	repoID               string
	eventPreset          string
	configRef            string
	checkoutRef          string
}

type triggerTestReader struct {
	inputs map[string]string
}

func (r triggerTestReader) ReadStringFromUser(msg string) string {
	if val, ok := r.inputs[msg]; ok {
		return val
	}
	return ""
}

func (r triggerTestReader) AskConfirm(msg string) bool {
	return true
}

func scaffoldCMD(
	baseURL string,
	validator validator.Validator,
	opts ...trigger.TriggerOption,
) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	config := &settings.Config{
		Token:      "testtoken",
		HTTPClient: http.DefaultClient,
		Host:       baseURL,
		DlHost:     baseURL,
	}
	cmd := trigger.NewTriggerCommand(config, validator, opts...)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd, stdout, stderr
}

func TestCreateTrigger(t *testing.T) {
	const (
		pipelineDefinitionID = "test-pipeline-definition-id"
		projectID            = "test-project-id"
		triggerName          = "Test Trigger"
		description          = "Test trigger description"
		repoID               = "123456"
		eventPreset          = "all-pushes"
		configRef            = "main"
		checkoutRef          = "main"
	)

	tests := []struct {
		name    string
		args    testCreateTriggerArgs
		want    string
		wantErr bool
	}{
		{
			name: "Create trigger successfully",
			args: testCreateTriggerArgs{
				projectID:            projectID,
				pipelineDefinitionID: pipelineDefinitionID,
				statusCode:           http.StatusOK,
				triggerName:          triggerName,
				description:          description,
				repoID:               repoID,
				eventPreset:          eventPreset,
				configRef:            configRef,
				checkoutRef:          checkoutRef,
			},
			want: fmt.Sprintf("Trigger '%s' created successfully\nYou may view your new trigger in your project settings: https://app.circleci.com/settings/project/circleci/<org>/<project>/triggers\n", triggerName),
		},
		{
			name: "Handle API error when creating trigger",
			args: testCreateTriggerArgs{
				projectID:            projectID,
				pipelineDefinitionID: pipelineDefinitionID,
				statusCode:           http.StatusInternalServerError,
				triggerName:          triggerName,
				description:          description,
				repoID:               repoID,
				eventPreset:          eventPreset,
				configRef:            configRef,
				checkoutRef:          checkoutRef,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.String(), fmt.Sprintf("/projects/%s/pipeline-definitions/%s/triggers", tt.args.projectID, tt.args.pipelineDefinitionID))

				body, err := io.ReadAll(r.Body)
				assert.NilError(t, err)

				var requestBody map[string]interface{}
				err = json.Unmarshal(body, &requestBody)
				assert.NilError(t, err)

				assert.Equal(t, requestBody["name"].(string), tt.args.triggerName)

				eventSource, ok := requestBody["event_source"].(map[string]interface{})
				assert.Assert(t, ok, "event_source should be a map")

				repo, ok := eventSource["repo"].(map[string]interface{})
				assert.Assert(t, ok, "repo should be a map")

				expectedRepoID := tt.args.repoID
				assert.Equal(t, repo["external_id"].(string), expectedRepoID)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.args.statusCode)

				if tt.args.statusCode == http.StatusOK {

					responseBody := map[string]interface{}{
						"id":   "trigger-id",
						"name": tt.args.triggerName,
						"repo": map[string]interface{}{
							"external_id": tt.args.repoID,
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
				"Enter a name for the trigger":                                      tt.args.triggerName,
				"Enter the pipeline definition ID you wish to create a trigger for": tt.args.pipelineDefinitionID,
				"Enter the ID of your github repository":                            tt.args.repoID,
			}

			opts := []trigger.TriggerOption{
				trigger.CustomReader(triggerTestReader{inputs: inputs}),
			}

			noValidator := func(_ *cobra.Command, _ []string) error {
				return nil
			}

			cmd, stdout, _ := scaffoldCMD(server.URL, noValidator, opts...)

			cmdArgs := []string{"create", tt.args.projectID}
			if tt.args.triggerName != "" {
				cmdArgs = append(cmdArgs, "--name", tt.args.triggerName)
			}
			if tt.args.pipelineDefinitionID != "" {
				cmdArgs = append(cmdArgs, "--pipeline-definition-id", tt.args.pipelineDefinitionID)
			}
			if tt.args.repoID != "" {
				cmdArgs = append(cmdArgs, "--repo-id", tt.args.repoID)
			}

			cmd.SetArgs(cmdArgs)

			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Create trigger command test failed: error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				got := stdout.String()
				if got != tt.want {
					t.Errorf("Create trigger command output = %q, want %q", got, tt.want)
				}
			}
		})
	}
}
