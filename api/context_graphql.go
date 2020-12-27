// Go functions that expose the Context-related calls in the GraphQL API.
package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/pkg/errors"
)

type GraphQLContextClient struct {
	Client *graphql.Client
}

type circleCIContext struct {
	ID        string
	Name      string
	CreatedAt string
	Groups    struct {
	}
}

type contextsQueryResponse struct {
	Organization struct {
		Id       string
		Contexts struct {
			Edges []struct {
				Node circleCIContext
			}
		}
	}
}

func improveVcsTypeError(err error) error {
	if responseErrors, ok := err.(graphql.ResponseErrorsCollection); ok {
		if len(responseErrors) > 0 {
			details := responseErrors[0].Extensions
			if details.EnumType == "VCSType" {
				allowedValues := strings.ToLower(strings.Join(details.AllowedValues[:], ", "))
				return fmt.Errorf("Invalid vcs-type '%s' provided, expected one of %s", strings.ToLower(details.Value), allowedValues)
			}
		}
	}
	return err
}

// CreateContext creates a new Context in the supplied organization.
func (c *GraphQLContextClient) CreateContext(vcsType, orgName, contextName string) (error) {
	cl := c.Client

	org, err := getOrganization(cl, orgName, vcsType)

	if err != nil {
		return err
	}

	query := `
	mutation CreateContext($input: CreateContextInput!) {
		createContext(input: $input) {
		  ...CreateButton
		}
	  }

	  fragment CreateButton on CreateContextPayload {
		error {
		  type
		}
	  }

	`

	var input struct {
		OwnerId     string `json:"ownerId"`
		OwnerType   string `json:"ownerType"`
		ContextName string `json:"contextName"`
	}

	input.OwnerId = org.Organization.ID
	input.OwnerType = "ORGANIZATION"
	input.ContextName = contextName

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("input", input)

	var response struct {
		CreateContext struct {
			Error struct {
				Type string
			}
		}
	}

	if err = cl.Run(request, &response); err != nil {
		return improveVcsTypeError(err)
	}

	if response.CreateContext.Error.Type != "" {
		return fmt.Errorf("Error creating context: %s", response.CreateContext.Error.Type)
	}

	return nil
}

// ContextByName returns the Context in the given organization with the given
// name.
func (c *GraphQLContextClient) ContextByName(vcs, org, name string) (*Context, error) {
	contexts , err := c.Contexts(vcs, org)
	if err != nil {
		return nil, err
	}
	for _, c := range *contexts {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, errors.New("No context found with that name")
}

// EnvironmentVariables returns all of the environment variables in this
// context.
func (c *GraphQLContextClient) EnvironmentVariables(contextID string) (*[]EnvironmentVariable, error) {
	cl := c.Client
	query := `
	query Context($id: ID!) {
		context(id: $id) {
			resources {
				variable
				createdAt
			}
		}
	}`
	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("id", contextID)
	var resp struct{
		Context struct{
			Resources []EnvironmentVariable
		}
	}
	err := cl.Run(request, &resp)

	if err != nil {
		return nil, err
	}
	for _, ev := range resp.Context.Resources {
		ev.ContextID = contextID
	}
	return &resp.Context.Resources, nil
}

// Contexts returns all of the Contexts owned by this organization.
func (c *GraphQLContextClient) Contexts(vcsType, orgName string) (*[]Context, error) {
	cl := c.Client
	// In theory we can lookup the organization by name and its contexts in
	// the same query, but using separate requests to circumvent a bug in
	// the API
	org, err := getOrganization(cl, orgName, vcsType)

	if err != nil {
		return nil, err
	}

	query := `
	query ContextsQuery($orgId: ID!) {
		organization(id: $orgId) {
			id
			contexts {
				edges {
					node {
						...Context
					}
				}
			}
		}
	}

	fragment Context on Context {
		id
		name
		createdAt
		groups {
			edges {
				node {
					...SecurityGroups
				}
			}
		}
		resources {
			...EnvVars
		}
	}

	fragment EnvVars on EnvironmentVariable {
		variable
		createdAt
		truncatedValue
	}

	fragment SecurityGroups on Group {
		id
		name
	}
	`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("orgId", org.Organization.ID)

	var response contextsQueryResponse
	err = cl.Run(request, &response)
	if err != nil {
		return nil, errors.Wrapf(improveVcsTypeError(err), "failed to load context list")
	}
	var contexts []Context
        for _, edge := range response.Organization.Contexts.Edges {
		context := edge.Node
		created_at, err := time.Parse(time.RFC3339, context.CreatedAt)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, Context{
			Name: context.Name,
			ID: context.ID,
			CreatedAt: created_at,
		})
	}

	return &contexts, nil
}

// DeleteEnvironmentVariable deletes the environment variable from the context.
// It returns an error if one occurred. It does not return an error if the
// environment variable did not exist.
func (c *GraphQLContextClient) DeleteEnvironmentVariable(contextId, variableName string) error {
	cl := c.Client
	query := `
	mutation DeleteEnvVar($input: RemoveEnvironmentVariableInput!) {
		removeEnvironmentVariable(input: $input) {
			context {
				id
				resources {
					...EnvVars
				}
			}
		}
	}

	fragment EnvVars on EnvironmentVariable {
		variable
		createdAt
		truncatedValue
	}`

	var input struct {
		ContextId string `json:"contextId"`
		Variable  string `json:"variable"`
	}

	input.ContextId = contextId
	input.Variable = variableName

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("input", input)

	var response struct {
		RemoveEnvironmentVariable struct {
			Context circleCIContext
		}
	}

	err := cl.Run(request, &response)
	return errors.Wrap(improveVcsTypeError(err), "failed to delete environment varaible")
}

// CreateEnvironmentVariable creates a new environment variable in the given
// context. Note that the GraphQL API does not support upsert, so an error will
// be returned if the env var already exists.
func (c *GraphQLContextClient) CreateEnvironmentVariable(contextId, variableName, secretValue string) error {
	cl := c.Client
	query := `
	mutation CreateEnvVar($input: StoreEnvironmentVariableInput!) {
		storeEnvironmentVariable(input: $input) {
		  context {
			id
			resources {
			  ...EnvVars
			}
		  }
		  ...CreateEnvVarButton
		}
	  }

	  fragment EnvVars on EnvironmentVariable {
		variable
		createdAt
		truncatedValue
	  }

	  fragment CreateEnvVarButton on StoreEnvironmentVariablePayload {
		error {
		  type
		}
	  }`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	var input struct {
		ContextId string `json:"contextId"`
		Variable  string `json:"variable"`
		Value     string `json:"value"`
	}

	input.ContextId = contextId
	input.Variable = variableName
	input.Value = secretValue

	request.Var("input", input)

	var response struct {
		StoreEnvironmentVariable struct {
			Context circleCIContext
			Error   struct {
				Type string
			}
		}
	}

	if err := cl.Run(request, &response); err != nil {
		return errors.Wrap(improveVcsTypeError(err), "failed to store environment varaible in context")
	}

	if response.StoreEnvironmentVariable.Error.Type != "" {
		return fmt.Errorf("Error storing environment variable: %s", response.StoreEnvironmentVariable.Error.Type)
	}

	return nil
}

// DeleteContext will delete the context with the given ID.
func (c *GraphQLContextClient) DeleteContext(contextId string) error {
	cl := c.Client
	query := `
	mutation DeleteContext($input: DeleteContextInput!) {
		deleteContext(input: $input) {
		  clientMutationId
		}
	  }`

	request := graphql.NewRequest(query)
	request.SetToken(cl.Token)

	var input struct {
		ContextId string `json:"contextId"`
	}

	input.ContextId = contextId
	request.Var("input", input)

	var response struct {
	}

	err := cl.Run(request, &response)

	return errors.Wrap(improveVcsTypeError(err), "failed to delete context")
}

// NewContextGraphqlClient returns a new client satisfying the
// api.ContextInterface interface via the GraphQL API.
func NewContextGraphqlClient(host, endpoint, token string, debug bool) *GraphQLContextClient {
	return &GraphQLContextClient{
		Client: graphql.NewClient(host, endpoint, token, debug),
	}
}
