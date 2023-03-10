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

type createProjectEnvVarRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
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

// GetEnvironmentVariable retrieves and returns a variable with the given name.
// If the response status code is 404, nil is returned.
func (c *projectRestClient) GetEnvironmentVariable(vcs string, org string, project string, envName string) (*ProjectEnvironmentVariable, error) {
	path := fmt.Sprintf("project/%s/%s/%s/envvar/%s", vcs, org, project, envName)
	req, err := c.client.NewRequest("GET", &url.URL{Path: path}, nil)
	if err != nil {
		return nil, err
	}

	var resp projectEnvVarResponse
	code, err := c.client.DoRequest(req, &resp)
	if err != nil {
		if code == 404 {
			// Note: 404 may mean that the project isn't found.
			// The cause can't be distinguished except by the response text.
			return nil, nil
		}
		return nil, err
	}
	return &ProjectEnvironmentVariable{
		Name:  resp.Name,
		Value: resp.Value,
	}, nil
}

// CreateEnvironmentVariable creates a variable on the given project.
// This returns the variable if successfully created.
func (c *projectRestClient) CreateEnvironmentVariable(vcs string, org string, project string, v ProjectEnvironmentVariable) (*ProjectEnvironmentVariable, error) {
	path := fmt.Sprintf("project/%s/%s/%s/envvar", vcs, org, project)
	req, err := c.client.NewRequest("POST", &url.URL{Path: path}, &createProjectEnvVarRequest{
		Name:  v.Name,
		Value: v.Value,
	})
	if err != nil {
		return nil, err
	}

	var resp projectEnvVarResponse
	_, err = c.client.DoRequest(req, &resp)
	if err != nil {
		return nil, err
	}
	return &ProjectEnvironmentVariable{
		Name:  resp.Name,
		Value: resp.Value,
	}, nil
}
