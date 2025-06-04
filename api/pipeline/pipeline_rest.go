package pipeline

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

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

// PipelineDefinitionInfo represents a pipeline definition in a project
type PipelineDefinitionInfo struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	ConfigSource   ConfigSourceResponse   `json:"config_source"`
	CheckoutSource CheckoutSourceResponse `json:"checkout_source"`
}

type listPipelineDefinitionsResponse struct {
	Items []PipelineDefinitionInfo `json:"items"`
}

type pipelineRunRequest struct {
	DefinitionID string                 `json:"definition_id"`
	Config       *Config                `json:"config,omitempty"`
	Checkout     *Checkout              `json:"checkout,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
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

// ListPipelineDefinitions returns a list of pipeline definitions for a project
func (c *pipelineRestClient) ListPipelineDefinitions(projectID string) ([]*PipelineDefinitionInfo, error) {
	path := fmt.Sprintf("projects/%s/pipeline-definitions", projectID)
	req, err := c.client.NewRequest("GET", &url.URL{Path: path}, nil)
	if err != nil {
		return nil, err
	}

	var response listPipelineDefinitionsResponse
	_, err = c.client.DoRequest(req, &response)
	if err != nil {
		return nil, err
	}

	items := make([]*PipelineDefinitionInfo, len(response.Items))
	for i := range response.Items {
		items[i] = &response.Items[i]
	}

	return items, nil
}

// PipelineRun triggers a new pipeline run
func (c *pipelineRestClient) PipelineRun(options PipelineRunOptions) (*PipelineRunResponse, error) {
	var fileContent string
	if options.ConfigFilePath != "" {
		bytes, err := os.ReadFile(options.ConfigFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		fileContent = string(bytes)
	}

	configRef := &Config{
		Branch: options.ConfigBranch,
		Tag:    options.ConfigTag,
	}
	if fileContent != "" {
		configRef.Content = fileContent
	}

	reqBody := pipelineRunRequest{
		DefinitionID: options.PipelineDefinitionID,
		Config:       configRef,
		Checkout: &Checkout{
			Branch: options.CheckoutBranch,
			Tag:    options.CheckoutTag,
		},
		Parameters: options.Parameters,
	}

	path := fmt.Sprintf("project/circleci/%s/%s/pipeline/run", options.Organization, options.Project)
	req, err := c.client.NewRequest("POST", &url.URL{Path: path}, reqBody)
	if err != nil {
		return nil, err
	}

	var rawResp map[string]interface{}
	status, err := c.client.DoRequest(req, &rawResp)
	if err != nil {
		return nil, err
	}

	response := &PipelineRunResponse{}
	if _, ok := rawResp["message"]; ok && status == 200 {
		b, _ := json.Marshal(rawResp)
		var msgResp PipelineRunMessageResponse
		if err := json.Unmarshal(b, &msgResp); err != nil {
			return nil, err
		}
		response.Message = &msgResp
	} else if status == 201 {
		b, _ := json.Marshal(rawResp)
		var createdResp PipelineRunCreatedResponse
		if err := json.Unmarshal(b, &createdResp); err != nil {
			return nil, err
		}
		response.Created = &createdResp
	} else {
		return nil, fmt.Errorf("unexpected status code or response: %d", status)
	}

	return response, nil
}
