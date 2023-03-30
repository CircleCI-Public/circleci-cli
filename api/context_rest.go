package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/pkg/errors"
)

// ContextRestClient communicates with the CircleCI REST API to ask questions
// about contexts. It satisfies api.ContextInterface.
type ContextRestClient struct {
	token  string
	server string
	client *http.Client
}

type listEnvironmentVariablesResponse struct {
	Items         []EnvironmentVariable
	NextPageToken *string `json:"next_page_token"`
	client        *ContextRestClient
	params        *listEnvironmentVariablesParams
}

type listContextsResponse struct {
	Items         []Context
	NextPageToken *string `json:"next_page_token"`
	client        *ContextRestClient
	params        *listContextsParams
}

type errorResponse struct {
	Message *string `json:"message"`
}

type listContextsParams struct {
	OwnerID   *string
	OwnerSlug *string
	OwnerType *string
	PageToken *string
}

type listEnvironmentVariablesParams struct {
	ContextID *string
	PageToken *string
}

func toSlug(vcs, org string) *string {
	slug := fmt.Sprintf("%s/%s", vcs, org)
	return &slug
}

// DeleteEnvironmentVariable deletes the environment variable in the context. It
// does not return an error if the environment variable did not exist.
func (c *ContextRestClient) DeleteEnvironmentVariable(contextID, variable string) error {
	req, err := c.newDeleteEnvironmentVariableRequest(contextID, variable)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return err
		}
		return errors.New(*dest.Message)
	}
	return nil
}

func (c *ContextRestClient) CreateContextWithOrgID(orgID *string, name string) error {
	req, err := c.newCreateContextRequestWithOrgID(orgID, name)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)

	if err != nil {
		return err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return err
		}
		return errors.New(*dest.Message)
	}
	var dest Context
	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return err
	}
	return nil
}

// CreateContext creates a new context in the supplied organization.
func (c *ContextRestClient) CreateContext(vcs, org, name string) error {
	req, err := c.newCreateContextRequest(vcs, org, name)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)

	if err != nil {
		return err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return err
		}
		return errors.New(*dest.Message)
	}
	var dest Context
	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return err
	}
	return nil
}

// CreateEnvironmentVariable creates OR UPDATES an environment variable.
func (c *ContextRestClient) CreateEnvironmentVariable(contextID, variable, value string) error {
	req, err := c.newCreateEnvironmentVariableRequest(contextID, variable, value)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return err
		}
		return errors.New(*dest.Message)
	}
	return nil
}

// DeleteContext deletes the context with the given ID.
func (c *ContextRestClient) DeleteContext(contextID string) error {
	req, err := c.newDeleteContextRequest(contextID)

	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return err
		}
		return errors.New(*dest.Message)
	}
	return nil
}

// EnvironmentVariables returns all of the environment variables owned by the
// given context. Note that pagination is not currently supported - we get all
// pages of env vars and return them all.
func (c *ContextRestClient) EnvironmentVariables(contextID string) (*[]EnvironmentVariable, error) {
	envVars, error := c.listAllEnvironmentVariables(
		&listEnvironmentVariablesParams{
			ContextID: &contextID,
		},
	)
	return &envVars, error
}

// Contexts returns all of the contexts owned by the given org. Note that
// pagination is not currently supported - we get all pages of contexts and
// return them all.
func (c *ContextRestClient) Contexts(vcs, org string) (*[]Context, error) {
	contexts, error := c.listAllContexts(
		&listContextsParams{
			OwnerSlug: toSlug(vcs, org),
		},
	)
	return &contexts, error
}

// ContextByName finds a single context by its name and returns it.
func (c *ContextRestClient) ContextByName(vcs, org, name string) (*Context, error) {
	params := &listContextsParams{
		OwnerSlug: toSlug(vcs, org),
	}
	for {
		resp, err := c.listContexts(params)
		if err != nil {
			return nil, err
		}
		for _, context := range resp.Items {
			if context.Name == name {
				return &context, nil
			}
		}
		if resp.NextPageToken == nil {
			return nil, fmt.Errorf("Cannot find context named '%s'", name)
		}
		params.PageToken = resp.NextPageToken
	}
}

func (c *ContextRestClient) listAllEnvironmentVariables(params *listEnvironmentVariablesParams) (envVars []EnvironmentVariable, err error) {
	var resp *listEnvironmentVariablesResponse
	for {
		resp, err = c.listEnvironmentVariables(params)
		if err != nil {
			return nil, err
		}

		envVars = append(envVars, resp.Items...)

		if resp.NextPageToken == nil {
			break
		}

		params.PageToken = resp.NextPageToken
	}
	return envVars, nil
}

func (c *ContextRestClient) listAllContexts(params *listContextsParams) (contexts []Context, err error) {
	var resp *listContextsResponse
	for {
		resp, err = c.listContexts(params)
		if err != nil {
			return nil, err
		}

		contexts = append(contexts, resp.Items...)

		if resp.NextPageToken == nil {
			break
		}

		params.PageToken = resp.NextPageToken
	}
	return contexts, nil
}

func (c *ContextRestClient) listEnvironmentVariables(params *listEnvironmentVariablesParams) (*listEnvironmentVariablesResponse, error) {
	req, err := c.newListEnvironmentVariablesRequest(params)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		return nil, errors.New(*dest.Message)

	}
	dest := listEnvironmentVariablesResponse{
		client: c,
		params: params,
	}
	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return nil, err
	}
	return &dest, nil
}

func (c *ContextRestClient) listContexts(params *listContextsParams) (*listContextsResponse, error) {
	req, err := c.newListContextsRequest(params)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err

		}
		return nil, errors.New(*dest.Message)

	}

	dest := listContextsResponse{
		client: c,
		params: params,
	}
	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return nil, err
	}
	return &dest, nil
}

// newCreateContextRequest posts a new context creation with orgname and vcs type using a slug
func (c *ContextRestClient) newCreateContextRequest(vcs, org, name string) (*http.Request, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse("context")
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader

	var body = struct {
		Name  string `json:"name"`
		Owner struct {
			Slug *string `json:"slug,omitempty"`
		} `json:"owner"`
	}{
		Name: name,
		Owner: struct {
			Slug *string `json:"slug,omitempty"`
		}{

			Slug: toSlug(vcs, org),
		},
	}
	buf, err := json.Marshal(body)

	if err != nil {
		return nil, err
	}

	bodyReader = bytes.NewReader(buf)

	return c.newHTTPRequest("POST", queryURL.String(), bodyReader)
}

// newCreateContextRequestWithOrgID posts a new context creation with an orgID
func (c *ContextRestClient) newCreateContextRequestWithOrgID(orgID *string, name string) (*http.Request, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse("context")
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader

	var body = struct {
		Name  string `json:"name"`
		Owner struct {
			ID *string `json:"id,omitempty"`
		} `json:"owner"`
	}{
		Name: name,
		Owner: struct {
			ID *string `json:"id,omitempty"`
		}{

			ID: orgID,
		},
	}
	buf, err := json.Marshal(body)

	if err != nil {
		return nil, err
	}

	bodyReader = bytes.NewReader(buf)

	return c.newHTTPRequest("POST", queryURL.String(), bodyReader)
}

func (c *ContextRestClient) newCreateEnvironmentVariableRequest(contextID, variable, value string) (*http.Request, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse(fmt.Sprintf("context/%s/environment-variable/%s", contextID, variable))
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	body := struct {
		Value string `json:"value"`
	}{
		Value: value,
	}
	buf, err := json.Marshal(body)

	if err != nil {
		return nil, err
	}

	bodyReader = bytes.NewReader(buf)

	return c.newHTTPRequest("PUT", queryURL.String(), bodyReader)
}

func (c *ContextRestClient) newDeleteEnvironmentVariableRequest(contextID, name string) (*http.Request, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse(fmt.Sprintf("context/%s/environment-variable/%s", contextID, name))
	if err != nil {
		return nil, err
	}
	return c.newHTTPRequest("DELETE", queryURL.String(), nil)
}

func (c *ContextRestClient) newDeleteContextRequest(contextID string) (*http.Request, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse(fmt.Sprintf("context/%s", contextID))
	if err != nil {
		return nil, err
	}
	return c.newHTTPRequest("DELETE", queryURL.String(), nil)
}

func (c *ContextRestClient) newListEnvironmentVariablesRequest(params *listEnvironmentVariablesParams) (*http.Request, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse(fmt.Sprintf("context/%s/environment-variable", *params.ContextID))
	if err != nil {
		return nil, err
	}
	urlParams := url.Values{}
	if params.PageToken != nil {
		urlParams.Add("page-token", *params.PageToken)
	}
	queryURL.RawQuery = urlParams.Encode()

	return c.newHTTPRequest("GET", queryURL.String(), nil)
}

func (c *ContextRestClient) newListContextsRequest(params *listContextsParams) (*http.Request, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse("context")
	if err != nil {
		return nil, err
	}

	urlParams := url.Values{}
	if params.OwnerID != nil {
		urlParams.Add("owner-id", *params.OwnerID)
	}
	if params.OwnerSlug != nil {
		urlParams.Add("owner-slug", *params.OwnerSlug)
	}
	if params.OwnerType != nil {
		urlParams.Add("owner-type", *params.OwnerType)
	}
	if params.PageToken != nil {
		urlParams.Add("page-token", *params.PageToken)
	}

	queryURL.RawQuery = urlParams.Encode()

	return c.newHTTPRequest("GET", queryURL.String(), nil)
}

func (c *ContextRestClient) newHTTPRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("circle-token", c.token)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", version.UserAgent())
	commandStr := header.GetCommandStr()
	if commandStr != "" {
		req.Header.Add("Circleci-Cli-Command", commandStr)
	}
	return req, nil
}

// EnsureExists verifies that the REST API exists and has the necessary
// endpoints to interact with contexts and env vars.
func (c *ContextRestClient) EnsureExists() error {
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return err
	}
	queryURL, err = queryURL.Parse("openapi.json")
	if err != nil {
		return err
	}
	req, err := c.newHTTPRequest("GET", queryURL.String(), nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New("API v2 test request failed.")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	var respBody struct {
		Paths struct {
			ContextEndpoint interface{} `json:"/context"`
		}
	}
	if err := json.Unmarshal(bodyBytes, &respBody); err != nil {
		return err
	}

	if respBody.Paths.ContextEndpoint == nil {
		return errors.New("No context endpoint exists")
	}

	return nil
}

// NewContextRestClient returns a new client satisfying the api.ContextInterface
// interface via the REST API.
func NewContextRestClient(config settings.Config) (*ContextRestClient, error) {
	serverURL, err := config.ServerURL()
	if err != nil {
		return nil, err
	}

	client := &ContextRestClient{
		token:  config.Token,
		server: serverURL.String(),
		client: config.HTTPClient,
	}

	return client, nil
}
