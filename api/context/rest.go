package context

import (
	"fmt"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
)

type ListContextsWithRestParams struct {
	OwnerID   string
	OwnerSlug string
	OwnerType string
	PageToken string
}

type ListContextsResponse struct {
	Items         []Context `json:"items"`
	NextPageToken string    `json:"next_page_token"`
}

// List all contexts for an owner.
// Binding to https://circleci.com/docs/api/v2/index.html#operation/listContexts
func ListContextsWithRest(client *rest.Client, params ListContextsWithRestParams) (ListContextsResponse, error) {
	queryURL, err := client.BaseURL.Parse("context")
	if err != nil {
		return ListContextsResponse{}, err
	}
	urlParams := url.Values{}
	if params.OwnerID != "" {
		urlParams.Add("owner-id", params.OwnerID)
	}
	if params.OwnerSlug != "" {
		urlParams.Add("owner-slug", params.OwnerSlug)
	}
	if params.OwnerType != "" {
		urlParams.Add("owner-type", params.OwnerType)
	}
	if params.PageToken != "" {
		urlParams.Add("page-token", params.PageToken)
	}
	queryURL.RawQuery = urlParams.Encode()
	req, err := client.NewRequest("GET", queryURL, nil)
	if err != nil {
		return ListContextsResponse{}, err
	}

	resp := ListContextsResponse{}
	if _, err := client.DoRequest(req, &resp); err != nil {
		return ListContextsResponse{}, err
	}
	return resp, nil
}

// Gets all pages of ListContexts
func ListAllContextsWithRest(client *rest.Client, params ListContextsWithRestParams) ([]Context, error) {
	contexts := []Context{}
	for {
		resp, err := ListContextsWithRest(client, params)
		if err != nil {
			return nil, err
		}

		contexts = append(contexts, resp.Items...)

		if resp.NextPageToken == "" {
			break
		}

		params.PageToken = resp.NextPageToken
	}
	return contexts, nil
}

type CreateContextWithRestParams struct {
	Name  string `json:"name"`
	Owner struct {
		Type string `json:"type"`
		Id   string `json:"id,omitempty"`
		Slug string `json:"slug,omitempty"`
	} `json:"owner"`
}

var orgOwnerType = "organization"

// Creates a new context.
// Binding to https://circleci.com/docs/api/v2/index.html#operation/createContext
func CreateContextWithRest(client *rest.Client, params CreateContextWithRestParams) (Context, error) {
	if params.Owner.Id == "" && params.Owner.Slug == "" {
		return Context{}, fmt.Errorf("to create a context, need either org ID or org slug, received none")
	}
	if params.Owner.Type == "" {
		params.Owner.Type = orgOwnerType
	}
	if params.Owner.Type != orgOwnerType && params.Owner.Type != "account" {
		return Context{}, fmt.Errorf("only owner.type values allowed to create a context are \"organization\" or \"account\", received: %s", params.Owner.Type)
	}
	if params.Owner.Id == "" && params.Owner.Slug != "" && params.Owner.Type == "account" {
		return Context{}, fmt.Errorf("when creating a context, owner.type with value \"account\" is only allowed when using owner.id and not when using owner.slug")
	}

	u, err := client.BaseURL.Parse("context")
	if err != nil {
		return Context{}, err
	}
	req, err := client.NewRequest("POST", u, &params)
	if err != nil {
		return Context{}, err
	}

	var context Context
	if _, err := client.DoRequest(req, &context); err != nil {
		return context, err
	}

	return context, nil
}

type DeleteContextWithRestResponse struct {
	Message string `json:"message"`
}

func DeleteContextWithRest(client *rest.Client, contextID string) (DeleteContextWithRestResponse, error) {
	u, err := client.BaseURL.Parse(fmt.Sprintf("context/%s", contextID))
	if err != nil {
		return DeleteContextWithRestResponse{}, err
	}
	req, err := client.NewRequest("DELETE", u, nil)
	if err != nil {
		return DeleteContextWithRestResponse{}, err
	}

	resp := DeleteContextWithRestResponse{}
	if _, err = client.DoRequest(req, &resp); err != nil {
		return DeleteContextWithRestResponse{}, err
	}
	return resp, nil
}

type ListEnvVarsWithRestParams struct {
	ContextID string
	PageToken string
}

type ListEnvVarsResponse struct {
	Items         []EnvironmentVariable `json:"items"`
	NextPageToken string                `json:"next_page_token"`
}

func ListEnvVarsWithRest(client *rest.Client, params ListEnvVarsWithRestParams) (ListEnvVarsResponse, error) {
	u, err := client.BaseURL.Parse(fmt.Sprintf("context/%s/environment-variable", params.ContextID))
	if err != nil {
		return ListEnvVarsResponse{}, err
	}
	qs := u.Query()
	if params.PageToken != "" {
		qs.Add("page-token", params.PageToken)
	}
	u.RawQuery = qs.Encode()

	req, err := client.NewRequest("GET", u, nil)
	if err != nil {
		return ListEnvVarsResponse{}, err
	}

	resp := ListEnvVarsResponse{}
	if _, err := client.DoRequest(req, &resp); err != nil {
		return ListEnvVarsResponse{}, err
	}
	return resp, nil
}

func ListAllEnvVarsWithRest(client *rest.Client, params ListEnvVarsWithRestParams) ([]EnvironmentVariable, error) {
	envVars := []EnvironmentVariable{}
	for {
		resp, err := ListEnvVarsWithRest(client, params)
		if err != nil {
			return nil, err
		}

		envVars = append(envVars, resp.Items...)

		if resp.NextPageToken == "" {
			break
		}

		params.PageToken = resp.NextPageToken
	}
	return envVars, nil
}

type CreateEnvVarWithRestParams struct {
	ContextID string
	Name      string
	Value     string
}

func CreateEnvVarWithRest(client *rest.Client, params CreateEnvVarWithRestParams) (EnvironmentVariable, error) {
	u, err := client.BaseURL.Parse(fmt.Sprintf("context/%s/environment-variable/%s", params.ContextID, params.Name))
	if err != nil {
		return EnvironmentVariable{}, err
	}

	body := struct {
		Value string `json:"value"`
	}{
		Value: params.Value,
	}
	req, err := client.NewRequest("PUT", u, &body)
	if err != nil {
		return EnvironmentVariable{}, err
	}

	resp := EnvironmentVariable{}
	if _, err := client.DoRequest(req, &resp); err != nil {
		return EnvironmentVariable{}, err
	}
	return resp, nil
}

type DeleteEnvVarWithRestParams struct {
	ContextID string
	Name      string
}

type DeleteEnvVarWithRestResponse struct {
	Message string `json:"message"`
}

func DeleteEnvVarWithRest(client *rest.Client, params DeleteEnvVarWithRestParams) (DeleteContextWithRestResponse, error) {
	u, err := client.BaseURL.Parse(fmt.Sprintf("context/%s/environment-variable/%s", params.ContextID, params.Name))
	if err != nil {
		return DeleteContextWithRestResponse{}, err
	}

	req, err := client.NewRequest("DELETE", u, nil)
	if err != nil {
		return DeleteContextWithRestResponse{}, err
	}

	resp := DeleteContextWithRestResponse{}
	if _, err := client.DoRequest(req, &resp); err != nil {
		return DeleteContextWithRestResponse{}, err
	}

	return resp, nil
}
