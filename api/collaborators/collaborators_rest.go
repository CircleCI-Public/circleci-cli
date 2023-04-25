package collaborators

import (
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

var (
	CollaborationsPath = "me/collaborations"
)

type collaboratorsRestClient struct {
	client *rest.Client
}

// NewCollaboratorsRestClient returns a new collaboratorsRestClient satisfying the api.CollaboratorsClient
// interface via the REST API.
func NewCollaboratorsRestClient(config settings.Config) (*collaboratorsRestClient, error) {
	client := &collaboratorsRestClient{
		client: rest.NewFromConfig(config.Host, &config),
	}
	return client, nil
}

func (c *collaboratorsRestClient) GetOrgCollaborations() ([]CollaborationResult, error) {
	req, err := c.client.NewRequest("GET", &url.URL{Path: CollaborationsPath}, nil)
	if err != nil {
		return nil, err
	}

	var resp []CollaborationResult
	_, err = c.client.DoRequest(req, &resp)
	return resp, err
}
