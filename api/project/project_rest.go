package project

import (
	"fmt"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

type projectRestClient struct {
	token  string
	server string
	client *rest.Client
}

var _ ProjectClient = &projectRestClient{}

type listProjectEnvVarsParams struct {
	vcs       string
	org       string
	project   string
	pageToken string
}

type projectEnvVarResponse struct {
	Name  string
	Value string
}

type listAllProjectEnvVarsResponse struct {
	Items         []projectEnvVarResponse
	NextPageToken string `json:"next_page_token"`
}

// NewProjectRestClient returns a new projectRestClient satisfying the api.ProjectInterface
// interface via the REST API.
func NewProjectRestClient(config settings.Config) (*projectRestClient, error) {
	serverURL, err := config.ServerURL()
	if err != nil {
		return nil, err
	}

	client := &projectRestClient{
		token:  config.Token,
		server: serverURL.String(),
		client: rest.New(config.Host, &config),
	}

	return client, nil
}

// ListAllEnvironmentVariables returns all of the environment variables owned by the
// given project. Note that pagination is not supported - we get all
// pages of env vars and return them all.
func (p *projectRestClient) ListAllEnvironmentVariables(vcs, org, project string) ([]*ProjectEnvironmentVariable, error) {
	res := make([]*ProjectEnvironmentVariable, 0)
	var nextPageToken string
	for {
		resp, err := p.listEnvironmentVariables(&listProjectEnvVarsParams{
			vcs:       vcs,
			org:       org,
			project:   project,
			pageToken: nextPageToken,
		})
		if err != nil {
			return nil, err
		}

		for _, ev := range resp.Items {
			res = append(res, &ProjectEnvironmentVariable{
				Name:  ev.Name,
				Value: ev.Value,
			})
		}

		if resp.NextPageToken == "" {
			break
		}

		nextPageToken = resp.NextPageToken
	}
	return res, nil
}

func (c *projectRestClient) listEnvironmentVariables(params *listProjectEnvVarsParams) (*listAllProjectEnvVarsResponse, error) {
	path := fmt.Sprintf("project/%s/%s/%s/envvar", params.vcs, params.org, params.project)
	urlParams := url.Values{}
	if params.pageToken != "" {
		urlParams.Add("page-token", params.pageToken)
	}

	req, err := c.client.NewRequest("GET", &url.URL{Path: path, RawQuery: urlParams.Encode()}, nil)
	if err != nil {
		return nil, err
	}

	var resp listAllProjectEnvVarsResponse
	_, err = c.client.DoRequest(req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
