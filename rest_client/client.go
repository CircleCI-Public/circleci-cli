package rest_client

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"encoding/json"
	"time"
	"io"
	"io/ioutil"
	"strings"
	"github.com/pkg/errors"
)

type Client struct {
	token string
	server string
	client *http.Client
}

type Context struct{
	CreatedAt time.Time `json:"created_at"`
	ID string `json:"id"`
	Name string `json:"name"`
}

type listContextsResponse struct {
	Items []Context
	NextPageToken *string `json:"next_page_token"`
	client *Client
	params *listContextsParams
}

type ErrorResponse struct {
	Message *string `json:"message"`
}

type listContextsParams struct {
	OwnerID *string
	OwnerSlug *string
	OwnerType *string
	PageToken *string
}

type EnvironmentVariable struct {
	Variable string
	ContextID string
	CreatedAt string
}

type ClientInterface interface {
	Contexts(vcs, org string) (*[]Context, error)
	ContextByName(vcs, org, name string) (*Context, error)
	DeleteContext(contextID string) error
	CreateContext(vcs, org, name string) (*Context, error)

	EnvironmentVariables(contextID string) (*[]EnvironmentVariable, error)
	CreateEnvironmentVariable(contextID, variable, value string) (*EnvironmentVariable, error)
	DeleteEnvironmentVariable(contextID, variable string) error
}

func toSlug(vcs, org string) *string {
	slug := fmt.Sprintf("%s/%s", vcs, org)
	return &slug
}

func (c *Client) CreateContext(vcs, org, name string) (*Context, error) {
	req, err := c.newCreateContextRequest(vcs, org, name)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		var dest ErrorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		return nil, errors.New(*dest.Message)
	}
	var dest Context
	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return nil, err
	}
	return &dest, nil
}

func (c *Client) CreateEnvironmentVariable(contextID, variable, value string) (*EnvironmentVariable, error) {
	req, err := c.newCreateEnvironmentVariableRequest(contextID, variable, value)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		var dest ErrorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		return nil, errors.New(*dest.Message)
	}
	var dest EnvironmentVariable
	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return nil, err
	}
	return &dest, nil
}

func (c *Client) DeleteContext(contextID string) error {
	req, err := c.newDeleteContextRequest(contextID)

	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		var dest ErrorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return err
		}
		return errors.New(*dest.Message)
	}
	return nil
}

func (c *Client) Contexts(vcs, org string) (*[]Context, error) {
	contexts, error := c.listAllContexts(
		&listContextsParams{
			OwnerSlug: toSlug(vcs, org),
		},
	)
	return &contexts, error
}

func (c *Client) ContextByName(vcs, org, name string) (*Context, error) {
	return c.getContextByName(
		&listContextsParams{
			OwnerSlug: toSlug(vcs, org),
		},
		name,
	)
}

func (c *Client) listAllContexts(params *listContextsParams) ([]Context, error) {
	resp, err := c.listContexts(params)
	if err != nil {
		return nil, err
	}

	contexts := resp.Items
	if resp.NextPageToken != nil {
		params.PageToken = resp.NextPageToken
		after, err := c.listAllContexts(params)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, after...)
	}
	return contexts, nil
}

func (c *Client) getContextByName(params *listContextsParams, name string) (*Context, error) {
	resp, err := c.listContexts(params)
	if err != nil {
		return nil, err
	}

	for _, context := range resp.Items {
		if context.Name == name {
			return &context, nil
		}
	}
	if resp.NextPageToken != nil {
		params.PageToken = resp.NextPageToken
		context, err := c.getContextByName(params, name)
		if err != nil {
			return nil, err
		}
		return context, nil
	}
	return nil, fmt.Errorf("Cannot find context named '%s'", name)
}

func (c *Client) listContexts (params *listContextsParams) (*listContextsResponse, error) {
	req, err := c.newListContextsRequest(params)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		var dest ErrorResponse
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

func (c *Client) newCreateContextRequest(vcs, org, name string) (*http.Request, error) {
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
		Owner: struct{
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

func (c *Client) newCreateEnvironmentVariableRequest(contextID, variable, value string) (*http.Request, error) {
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
	body := struct{
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

func (c *Client) newDeleteContextRequest(contextID string) (*http.Request, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse(fmt.Sprintf("context/%s", contextID))
	return c.newHTTPRequest("DELETE", queryURL.String(), nil)
}

func (c *Client) newListContextsRequest(params *listContextsParams) (*http.Request, error) {
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

func (c *Client) newHTTPRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("circle-token", c.token)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	return req, nil
}

func NewClient(server, token string) (*Client) {
	// Ensure server ends with a slash
	if !strings.HasSuffix(server, "/") {
		server += "/"
	}
	return &Client{
		token: token,
		server: server,
		client: &http.Client{},
	}
}
