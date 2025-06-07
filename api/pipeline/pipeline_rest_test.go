package pipeline_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"encoding/json"
	"io"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api/pipeline"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"gotest.tools/v3/assert"
)

func getPipelineRestClient(server *httptest.Server) (pipeline.PipelineClient, error) {
	client := &http.Client{}

	return pipeline.NewPipelineRestClient(settings.Config{
		RestEndpoint: "api/v2",
		Host:         server.URL,
		HTTPClient:   client,
		Token:        "token",
	})
}

func Test_pipelineRestClient_CreatePipeline(t *testing.T) {
	const (
		vcsType      = "github"
		orgName      = "test-org"
		projectID    = "test-project-id"
		repoID       = "test-repo-id"
		configRepoID = "test-config-repo-id"
		filePath     = ".circleci/config.yml"
		testName     = "test-pipeline"
		description  = "test-description"
	)
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    *pipeline.CreatePipelineInfo
		wantErr bool
	}{
		{
			name: "Should handle a successful request with CreatePipeline",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/projects/%s/pipeline-definitions", projectID))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
				{
					"id": "123",
					"name": "test-pipeline",
					"description": "test-description",
					"checkout_source": {
						"provider": "github_app",
						"repo": {
							"external_id": "test-repo-id",
							"full_name": "test-repo"
						}
					},
					"config_source": {
						"provider": "github_app",
						"repo": {
							"external_id": "test-repo-id",
							"full_name": "test-repo"
						}
					}
				}`))
				assert.NilError(t, err)
			},
			want: &pipeline.CreatePipelineInfo{
				Id:                         "123",
				Name:                       testName,
				CheckoutSourceRepoFullName: "test-repo",
				ConfigSourceRepoFullName:   "test-repo",
			},
		},
		{
			name: "Should handle an error request with CreatePipeline",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"message": "error"}`))
				assert.NilError(t, err)
			},
			wantErr: true,
		},
		{
			name: "Should handle a successful request with CreatePipeline with configRepoID",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "POST")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/projects/%s/pipeline-definitions", projectID))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
				{
					"id": "123",
					"name": "test-pipeline",
					"description": "test-description",
					"checkout_source": {
						"provider": "github_app",
						"repo": {
							"external_id": "test-repo-id",
							"full_name": "test-repo"
						}
					},
					"config_source": {
						"provider": "github_app",
						"repo": {
							"external_id": "test-config-repo-id",
							"full_name": "test-config-repo"
						}
					}
				}`))
				assert.NilError(t, err)
			},
			want: &pipeline.CreatePipelineInfo{
				Id:                         "123",
				Name:                       testName,
				CheckoutSourceRepoFullName: "test-repo",
				ConfigSourceRepoFullName:   "test-config-repo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			p, err := getPipelineRestClient(server)
			assert.NilError(t, err)

			got, err := p.CreatePipeline(projectID, testName, description, repoID, configRepoID, filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("pipelineRestClient.CreatePipeline() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("pipelineRestClient.CreatePipeline() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_pipelineRestClient_GetPipelineDefinition(t *testing.T) {
	const (
		projectID            = "test-project-id"
		pipelineDefinitionID = "test-pipeline-definition-id"
	)
	tests := []struct {
		name    string
		options pipeline.GetPipelineDefinitionOptions
		handler http.HandlerFunc
		want    *pipeline.PipelineDefinition
		wantErr bool
	}{
		{
			name: "Should handle a successful request with GetPipelineDefinition",
			options: pipeline.GetPipelineDefinitionOptions{
				ProjectID:            projectID,
				PipelineDefinitionID: pipelineDefinitionID,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/projects/%s/pipeline-definitions/%s", projectID, pipelineDefinitionID))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`
				{
					"id": "123",
					"name": "test-pipeline",
					"description": "test-description",
					"created_at": "2024-01-01T00:00:00Z",
					"checkout_source": {
						"provider": "github_app",
						"repo": {
							"external_id": "test-repo-id",
							"full_name": "test-repo"
						}
					},
					"config_source": {
						"provider": "github_app",
						"repo": {
							"external_id": "test-config-repo-id",
							"full_name": "test-config-repo"
						}
					}
				}`))
				assert.NilError(t, err)
			},
			want: &pipeline.PipelineDefinition{
				ConfigSourceId:   "test-config-repo-id",
				CheckoutSourceId: "test-repo-id",
			},
		},
		{
			name: "Should handle an error request with GetPipelineDefinition",
			options: pipeline.GetPipelineDefinitionOptions{
				ProjectID:            projectID,
				PipelineDefinitionID: pipelineDefinitionID,
			},
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

			p, err := getPipelineRestClient(server)
			assert.NilError(t, err)

			got, err := p.GetPipelineDefinition(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("pipelineRestClient.GetPipelineDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("pipelineRestClient.GetPipelineDefinition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_pipelineRestClient_ListPipelineDefinitions(t *testing.T) {
	const (
		projectID = "test-project-id"
	)
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    []*pipeline.PipelineDefinitionInfo
		wantErr bool
	}{
		{
			name: "Should handle a successful request",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/projects/%s/pipeline-definitions", projectID))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{
					"items": [
						{
							"id": "123",
							"name": "test-pipeline",
							"description": "test-description",
							"config_source": {
								"provider": "github_app",
								"repo": {
									"external_id": "test-repo-id",
									"full_name": "test-repo"
								},
								"file_path": ".circleci/config.yml"
							},
							"checkout_source": {
								"provider": "github_app",
								"repo": {
									"external_id": "test-repo-id",
									"full_name": "test-repo"
								}
							}
						}
					]
				}`))
				assert.NilError(t, err)
			},
			want: []*pipeline.PipelineDefinitionInfo{
				{
					ID:          "123",
					Name:        "test-pipeline",
					Description: "test-description",
					ConfigSource: pipeline.ConfigSourceResponse{
						Provider: "github_app",
						Repo: pipeline.RepoResponse{
							ExternalID: "test-repo-id",
							FullName:   "test-repo",
						},
						FilePath: ".circleci/config.yml",
					},
					CheckoutSource: pipeline.CheckoutSourceResponse{
						Provider: "github_app",
						Repo: pipeline.RepoResponse{
							ExternalID: "test-repo-id",
							FullName:   "test-repo",
						},
					},
				},
			},
		},
		{
			name: "Should handle empty list response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/projects/%s/pipeline-definitions", projectID))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"items": []}`))
				assert.NilError(t, err)
			},
			want: []*pipeline.PipelineDefinitionInfo{},
		},
		{
			name: "Should handle an error request",
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

			p, err := getPipelineRestClient(server)
			assert.NilError(t, err)

			got, err := p.ListPipelineDefinitions(projectID)
			if (err != nil) != tt.wantErr {
				t.Errorf("pipelineRestClient.ListPipelineDefinitions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assert.DeepEqual(t, got, tt.want)
			}
		})
	}
}

func Test_pipelineRestClient_PipelineRun(t *testing.T) {
	const (
		organization = "test-org"
		project      = "test-project"
		definitionID = "pipeline-def-id"
	)
	expectedContent := "version: 2.1\njobs:\n  test:\n    docker:\n      - image: cimg/base:stable\n    steps:\n      - run: echo Hello, world!"

	tests := []struct {
		name         string
		setupFile    bool
		filePath     string
		wantContent  string
		wantErr      bool
		statusCode   int
		responseBody string
		wantCreated  bool
		wantMessage  bool
	}{
		{
			name:         "no config file, pipeline created",
			setupFile:    false,
			filePath:     "",
			wantContent:  "",
			wantErr:      false,
			statusCode:   201,
			responseBody: `{"id": "pipeline-id-123", "number": 42, "state": "created", "created_at": "2024-06-01T12:00:00Z"}`,
			wantCreated:  true,
		},
		{
			name:         "no config file, no pipeline created",
			setupFile:    false,
			filePath:     "",
			wantContent:  "",
			wantErr:      false,
			statusCode:   200,
			responseBody: `{"message": "No pipeline was triggered"}`,
			wantMessage:  true,
		},
		{
			name:         "valid config file, pipeline created",
			setupFile:    true,
			wantContent:  expectedContent,
			wantErr:      false,
			statusCode:   201,
			responseBody: `{"id": "pipeline-id-123", "number": 42, "state": "created", "created_at": "2024-06-01T12:00:00Z"}`,
			wantCreated:  true,
		},
		{
			name:         "invalid config file path",
			setupFile:    false,
			filePath:     "/tmp/this-file-should-not-exist-1234567890.yml",
			wantContent:  "",
			wantErr:      true,
			statusCode:   201, // won't matter, file read fails first
			responseBody: `{"id": "pipeline-id-123", "number": 42, "state": "created", "created_at": "2024-06-01T12:00:00Z"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configPath string
			if tt.setupFile {
				tmpDir := t.TempDir()
				configDir := tmpDir + "/.circleci"
				err := os.MkdirAll(configDir, 0755)
				assert.NilError(t, err)
				configPath = configDir + "/config.yml"
				tmpFile, err := os.Create(configPath)
				assert.NilError(t, err)
				_, err = tmpFile.WriteString(expectedContent)
				assert.NilError(t, err)
				tmpFile.Close()
			} else {
				configPath = tt.filePath
			}

			var receivedContent string
			handler := func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				assert.NilError(t, err)
				var reqBody map[string]interface{}
				_ = json.Unmarshal(body, &reqBody)
				if configObj, ok := reqBody["config"].(map[string]interface{}); ok {
					receivedContent, _ = configObj["content"].(string)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, err = w.Write([]byte(tt.responseBody))
				assert.NilError(t, err)
			}

			server := httptest.NewServer(http.HandlerFunc(handler))
			defer server.Close()

			client, err := pipeline.NewPipelineRestClient(settings.Config{
				RestEndpoint: "api/v2",
				Host:         server.URL,
				HTTPClient:   &http.Client{},
				Token:        "token",
			})
			assert.NilError(t, err)

			resp, err := client.PipelineRun(pipeline.PipelineRunOptions{
				Organization:         organization,
				Project:              project,
				PipelineDefinitionID: definitionID,
				ConfigBranch:         "main",
				ConfigTag:            "",
				CheckoutBranch:       "main",
				CheckoutTag:          "",
				Parameters:           map[string]interface{}{"foo": "bar"},
				ConfigFilePath:       configPath,
			})

			if tt.wantErr {
				assert.ErrorContains(t, err, "failed to read config file")
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, receivedContent, tt.wantContent)
			if tt.wantCreated {
				assert.Assert(t, resp.Created != nil)
			}
			if tt.wantMessage {
				assert.Assert(t, resp.Message != nil)
			}
		})
	}
}
