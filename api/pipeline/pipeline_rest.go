package pipeline

import (
	"fmt"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

type pipelineRestClient struct {
	client *rest.Client
}

var _ PipelineClient = &pipelineRestClient{}

type Repo struct {
	ExternalID string `json:"external_id"`
}

type RepoResponse struct {
	ExternalID string `json:"external_id"`
	FullName   string `json:"full_name"`
}

type ConfigSource struct {
	Provider string `json:"provider"`
	Repo     Repo   `json:"repo"`
	FilePath string `json:"file_path"`
}

type ConfigSourceResponse struct {
	Provider string       `json:"provider"`
	Repo     RepoResponse `json:"repo"`
	FilePath string       `json:"file_path"`
}

type CheckoutSource struct {
	Provider string `json:"provider"`
	Repo     Repo   `json:"repo"`
}

type CheckoutSourceResponse struct {
	Provider string       `json:"provider"`
	Repo     RepoResponse `json:"repo"`
}

type createPipelineDefinitionRequest struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	ConfigSource   ConfigSource   `json:"config_source"`
	CheckoutSource CheckoutSource `json:"checkout_source"`
}

type createPipelineDefinitionResponse struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	ConfigSource   ConfigSourceResponse   `json:"config_source"`
	CheckoutSource CheckoutSourceResponse `json:"checkout_source"`
}

type GetPipelineDefinitionResponse struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	CreatedAt      string                 `json:"created_at"`
	ConfigSource   ConfigSourceResponse   `json:"config_source"`
	CheckoutSource CheckoutSourceResponse `json:"checkout_source"`
}

// NewPipelineRestClient returns a new pipelineRestClient satisfying the api.PipelineInterface
// interface via the REST API.
func NewPipelineRestClient(config settings.Config) (*pipelineRestClient, error) {
	client := &pipelineRestClient{
		client: rest.NewFromConfig(config.Host, &config),
	}
	return client, nil
}

func (c *pipelineRestClient) CreatePipeline(projectID string, name string, description string, repoID string, configRepoID string, filePath string) (*CreatePipelineInfo, error) {
	reqBody := createPipelineDefinitionRequest{
		Name:        name,
		Description: description,
		ConfigSource: ConfigSource{
			Provider: "github_app",
			Repo: Repo{
				ExternalID: configRepoID,
			},
			FilePath: filePath,
		},
		CheckoutSource: CheckoutSource{
			Provider: "github_app",
			Repo: Repo{
				ExternalID: repoID,
			},
		},
	}

	path := fmt.Sprintf("projects/%s/pipeline-definitions", projectID)
	req, err := c.client.NewRequest("POST", &url.URL{Path: path}, reqBody)
	if err != nil {
		return nil, err
	}

	var resp createPipelineDefinitionResponse
	_, err = c.client.DoRequest(req, &resp)
	if err != nil {
		return nil, err
	}

	return &CreatePipelineInfo{
		Id:                         resp.ID,
		Name:                       resp.Name,
		CheckoutSourceRepoFullName: resp.CheckoutSource.Repo.FullName,
		ConfigSourceRepoFullName:   resp.ConfigSource.Repo.FullName,
	}, nil
}

func (c *pipelineRestClient) GetPipelineDefinition(options GetPipelineDefinitionOptions) (*PipelineDefinition, error) {
	path := fmt.Sprintf("projects/%s/pipeline-definitions/%s", options.ProjectID, options.PipelineDefinitionID)
	req, err := c.client.NewRequest("GET", &url.URL{Path: path}, nil)
	if err != nil {
		return nil, err
	}

	var resp GetPipelineDefinitionResponse
	_, err = c.client.DoRequest(req, &resp)
	if err != nil {
		return nil, err
	}

	return &PipelineDefinition{
		ConfigSourceId:   resp.ConfigSource.Repo.ExternalID,
		CheckoutSourceId: resp.CheckoutSource.Repo.ExternalID,
	}, nil
}
