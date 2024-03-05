package context

import (
	"fmt"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/pkg/errors"
)

type ListContextsWithGQLParams struct {
	OrgID   string
	OrgName string
	VCSType string
}

type contextsQueryResponse struct {
	Organization struct {
		Id       string
		Contexts struct {
			Edges []struct {
				Node struct {
					ID        string
					Name      string
					CreatedAt string
				}
			}
		}
	}
}

func ListContextsWithGQL(c *graphql.Client, params ListContextsWithGQLParams) ([]Context, error) {
	if params.OrgID == "" && (params.OrgName == "" || params.VCSType == "") {
		return nil, fmt.Errorf("to list context, need either org ID or couple vcs/orgName but got neither")
	}
	useOrgID := params.OrgID != ""
	if !useOrgID && params.VCSType != "github" && params.VCSType != "bitbucket" {
		return nil, fmt.Errorf("only github and bitbucket vcs type are available, got: %s", params.VCSType)
	}
	var query string
	if useOrgID {
		query = `query ContextsQuery($orgId: ID!) {
	organization(id: $orgId) {
		id
		contexts {
			edges {
				node {
					id
					name
					createdAt
				}
			}
		}
	}
}`
	} else {
		query = `query ContextsQuery($name: String!, $vcsType: VCSType) {
	organization(name: $name, vcsType: $vcsType) {
		id
		contexts {
			edges {
				node {
					id
					name
					createdAt
				}
			}
		}
	}
}`
	}
	request := graphql.NewRequest(query)
	if useOrgID {
		request.Var("orgId", params.OrgID)
	} else {
		request.Var("name", params.OrgName)
		request.Var("vcsType", strings.ToUpper(params.VCSType))
	}
	request.SetToken(c.Token)

	var response contextsQueryResponse
	err := c.Run(request, &response)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load context list")
	}
	contexts := make([]Context, len(response.Organization.Contexts.Edges))
	for i, edge := range response.Organization.Contexts.Edges {
		context := edge.Node
		created_at, err := time.Parse(time.RFC3339, context.CreatedAt)
		if err != nil {
			return nil, err
		}
		contexts[i] = Context{
			Name:      context.Name,
			ID:        context.ID,
			CreatedAt: created_at,
		}
	}

	return contexts, nil
}

type CreateContextWithGQLParams struct {
	OwnerId     string `json:"ownerId"`
	OwnerType   string `json:"ownerType"`
	ContextName string `json:"contextName"`
}

func CreateContextWithGQL(c *graphql.Client, params CreateContextWithGQLParams) (Context, error) {
	query := `mutation CreateContext($input: CreateContextInput!) {
	createContext(input: $input) {
		error {
			type
		}
		context {
			createdAt
			id
			name
		}
	}
}`
	request := graphql.NewRequest(query)
	request.SetToken(c.Token)
	request.Var("input", params)

	var response struct {
		CreateContext struct {
			Error *struct {
				Type string
			}
			Context Context
		}
	}
	if err := c.Run(request, &response); err != nil {
		return Context{}, err
	}
	if response.CreateContext.Error.Type != "" {
		return Context{}, fmt.Errorf("Error creating context: %s", response.CreateContext.Error.Type)
	}

	return response.CreateContext.Context, nil
}

func DeleteContextWithGQL(c *graphql.Client, contextID string) error {
	query := `mutation DeleteContext($contextId: UUID) {
	deleteContext(input: { contextId: $contextId }) {
		clientMutationId
	}
}`
	request := graphql.NewRequest(query)
	request.SetToken(c.Token)

	request.Var("contextId", contextID)

	var response struct {
	}

	err := c.Run(request, &response)

	return errors.Wrap(err, "failed to delete context")
}

func ListEnvVarsWithGQL(c *graphql.Client, contextID string) ([]EnvironmentVariable, error) {
	query := `query Context($id: ID!) {
	context(id: $id) {
		resources {
			variable
			createdAt
			updatedAt
		}
	}
}`
	request := graphql.NewRequest(query)
	request.SetToken(c.Token)
	request.Var("id", contextID)
	var resp struct {
		Context struct {
			Resources []EnvironmentVariable
		}
	}
	err := c.Run(request, &resp)

	if err != nil {
		return nil, err
	}
	for _, ev := range resp.Context.Resources {
		ev.ContextID = contextID
	}
	return resp.Context.Resources, nil
}

type CreateEnvVarWithGQLParams struct {
	ContextID string `json:"contextId"`
	Variable  string `json:"variable"`
	Value     string `json:"value"`
}

func CreateEnvVarWithGQL(c *graphql.Client, params CreateEnvVarWithRestParams) error {
	query := `mutation CreateEnvVar($input: StoreEnvironmentVariableInput!) {
	storeEnvironmentVariable(input: $input) {
		error {
			type
		}
	}
}`
	request := graphql.NewRequest(query)
	request.SetToken(c.Token)
	request.Var("input", params)

	var response struct {
		StoreEnvironmentVariable struct {
			Error struct {
				Type string
			}
		}
	}

	if err := c.Run(request, &response); err != nil {
		return errors.Wrap(err, "failed to store environment variable in context")
	}

	if response.StoreEnvironmentVariable.Error.Type != "" {
		return fmt.Errorf("Error storing environment variable: %s", response.StoreEnvironmentVariable.Error.Type)
	}

	return nil
}

type DeleteEnvVarWithGQLParams struct {
	ContextID string `json:"contextId"`
	Variable  string `json:"variable"`
}

func DeleteEnvVarWithGQL(c *graphql.Client, params DeleteEnvVarWithRestParams) error {
	query := `mutation DeleteEnvVar($input: RemoveEnvironmentVariableInput!) {
	removeEnvironmentVariable(input: $input) {
		context {
			id
		}
	}
}`
	request := graphql.NewRequest(query)
	request.SetToken(c.Token)
	request.Var("input", params)

	var response struct {
		RemoveEnvironmentVariable struct{ Context struct{ Id string } }
	}

	err := c.Run(request, &response)
	return errors.Wrap(err, "failed to delete environment variable")
}
