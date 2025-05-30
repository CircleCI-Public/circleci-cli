package pipeline_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

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
