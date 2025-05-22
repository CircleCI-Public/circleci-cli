package trigger

import (
	"fmt"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

type triggerRestClient struct {
	client *rest.Client
}

var _ TriggerClient = &triggerRestClient{}

type Repo struct {
	ExternalID string `json:"external_id"`
}

type RepoResponse struct {
	ExternalID string `json:"external_id"`
}

type EventSource struct {
	Provider string `json:"provider"`
	Repo     Repo   `json:"repo"`
}

type createTriggerRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	EventSource EventSource `json:"event_source"`
	EventPreset string      `json:"event_preset"`
	ConfigRef   *string     `json:"config_ref"`
	CheckoutRef *string     `json:"checkout_ref"`
}

type createTriggerResponse struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	CreatedAt   string      `json:"created_at"`
	EventSource EventSource `json:"event_source"`
	EventPreset string      `json:"event_preset"`
	ConfigRef   *string     `json:"config_ref"`
	CheckoutRef *string     `json:"checkout_ref"`
}

type CreateTriggerOptions struct {
	ProjectID            string
	PipelineDefinitionID string
	Name                 string
	Description          string
	RepoID               string
	EventPreset          string
	ConfigRef            *string
	CheckoutRef          *string
}

// NewTriggerRestClient returns a new triggerRestClient satisfying the api.TriggerInterface
// interface via the REST API.
func NewTriggerRestClient(config settings.Config) (*triggerRestClient, error) {
	client := &triggerRestClient{
		client: rest.NewFromConfig(config.Host, &config),
	}
	return client, nil
}

func (c *triggerRestClient) CreateTrigger(options CreateTriggerOptions) (*CreateTriggerInfo, error) {
	reqBody := createTriggerRequest{
		Name:        options.Name,
		Description: options.Description,
		EventSource: EventSource{
			Provider: "github_app",
			Repo: Repo{
				ExternalID: options.RepoID,
			},
		},
		EventPreset: options.EventPreset,
	}

	// Only set optional fields if they are provided
	if options.ConfigRef != nil {
		reqBody.ConfigRef = options.ConfigRef
	}
	if options.CheckoutRef != nil {
		reqBody.CheckoutRef = options.CheckoutRef
	}

	path := fmt.Sprintf("projects/%s/pipeline-definitions/%s/triggers", options.ProjectID, options.PipelineDefinitionID)
	req, err := c.client.NewRequest("POST", &url.URL{Path: path}, reqBody)
	if err != nil {
		return nil, err
	}

	var resp createTriggerResponse
	_, err = c.client.DoRequest(req, &resp)
	if err != nil {
		return nil, err
	}

	return &CreateTriggerInfo{
		Id:   resp.ID,
		Name: resp.Name,
	}, nil
}
