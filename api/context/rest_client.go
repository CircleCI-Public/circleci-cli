package context

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
)

type restClient struct {
	client *rest.Client

	orgID   string
	vcsType string
	orgName string
}

func toSlug(vcs, org string) string {
	slug := fmt.Sprintf("%s/%s", vcs, org)
	return slug
}

// createListContextsParams is a helper to create ListContextsParams from the content of the ContextRestClient
func (c restClient) createListContextsParams() (ListContextsWithRestParams, error) {
	params := ListContextsWithRestParams{}
	if c.orgID != "" {
		params.OwnerID = c.orgID
	} else if c.vcsType != "" && c.orgName != "" {
		params.OwnerSlug = toSlug(c.vcsType, c.orgName)
	} else {
		return params, fmt.Errorf("to list context, need either org ID or couple vcs/orgName but got neither")
	}
	return params, nil
}

func (c restClient) Contexts() ([]Context, error) {
	params, err := c.createListContextsParams()
	if err != nil {
		return nil, err
	}
	return ListAllContextsWithRest(c.client, params)
}

func (c restClient) ContextByName(name string) (Context, error) {
	params, err := c.createListContextsParams()
	if err != nil {
		return Context{}, err
	}

	for {
		resp, err := ListContextsWithRest(c.client, params)
		if err != nil {
			return Context{}, err
		}

		for _, context := range resp.Items {
			if context.Name == name {
				return context, nil
			}
		}

		if resp.NextPageToken == "" {
			break
		}

		params.PageToken = resp.NextPageToken
	}
	return Context{}, fmt.Errorf("context with name %s not found", name)
}

func (c restClient) CreateContext(name string) error {
	params := CreateContextWithRestParams{
		Name: name,
	}
	params.Owner.Type = "organization"
	if c.orgID != "" {
		params.Owner.Id = c.orgID
	} else if c.vcsType != "" && c.orgName != "" {
		params.Owner.Slug = toSlug(c.vcsType, c.orgName)
	} else {
		return fmt.Errorf("need either org ID or vcs type and org name to create a context, received none")
	}
	_, err := CreateContextWithRest(c.client, params)
	return err
}

func (c restClient) DeleteContext(contextID string) error {
	_, err := DeleteContextWithRest(c.client, contextID)
	return err
}

func (c restClient) EnvironmentVariables(contextID string) ([]EnvironmentVariable, error) {
	return ListAllEnvVarsWithRest(c.client, ListEnvVarsWithRestParams{ContextID: contextID})
}

func (c restClient) CreateEnvironmentVariable(contextID, variable, value string) error {
	_, err := CreateEnvVarWithRest(c.client, CreateEnvVarWithRestParams{
		ContextID: contextID,
		Name:      variable,
		Value:     value,
	})
	return err
}

func (c restClient) DeleteEnvironmentVariable(contextID, variable string) error {
	_, err := DeleteEnvVarWithRest(c.client, DeleteEnvVarWithRestParams{ContextID: contextID, Name: variable})
	return err
}
