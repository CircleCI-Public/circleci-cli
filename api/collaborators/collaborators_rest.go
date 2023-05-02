package collaborators

import (
	"net/url"
	"strings"

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

func (c *collaboratorsRestClient) GetCollaborationBySlug(slug string) (*CollaborationResult, error) {
	// Support for <vcs-name>/<org-name> as well as <vcs-short>/<org-name> for the slug
	// requires splitting
	collaborations, err := c.GetOrgCollaborations()

	if err != nil {
		return nil, err
	}

	slugParts := strings.Split(slug, "/")

	for _, v := range collaborations {
		// The rest-api allways returns <vsc-short>/<org-name> as a slug
		if v.OrgSlug == slug {
			return &v, nil
		}

		// Compare first part of argument slug with the VCSType
		splitted := strings.Split(v.OrgSlug, "/")
		if len(slugParts) >= 2 && len(splitted) >= 2 && slugParts[0] == v.VcsType && slugParts[1] == splitted[1] {
			return &v, nil
		}
	}

	return nil, nil
}
