// Go functions that expose the Context-related calls in the GraphQL API.
package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/pkg/errors"
)

type GraphQLContextClient struct {
	Client *client.Client
}

type Resource struct {
	Variable       string
	CreatedAt      string
	TruncatedValue string
}

type CircleCIContext struct {
	ID        string
	Name      string
	CreatedAt string
	Groups    struct {
	}
	Resources []Resource
}

type ContextsQueryResponse struct {
	Organization struct {
		Id       string
		Contexts struct {
			Edges []struct {
				Node CircleCIContext
			}
		}
	}
}

func improveVcsTypeError(err error) error {
	if responseErrors, ok := err.(client.ResponseErrorsCollection); ok {
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

func CreateContext(cl *client.Client, vcsType, orgName, contextName string) (error) {

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

	request := client.NewRequest(query)
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

	request := client.NewRequest(query)
	request.SetToken(cl.Token)

	request.Var("orgId", org.Organization.ID)

	var response ContextsQueryResponse
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

func DeleteEnvironmentVariable(cl *client.Client, contextId, variableName string) error {
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

	request := client.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("input", input)

	var response struct {
		RemoveEnvironmentVariable struct {
			Context CircleCIContext
		}
	}

	err := cl.Run(request, &response)
	return errors.Wrap(improveVcsTypeError(err), "failed to delete environment varaible")
}

func StoreEnvironmentVariable(cl *client.Client, contextId, variableName, secretValue string) error {
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

	request := client.NewRequest(query)
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
			Context CircleCIContext
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

func DeleteContext(cl *client.Client, contextId string) error {
	query := `
	mutation DeleteContext($input: DeleteContextInput!) {
		deleteContext(input: $input) {
		  clientMutationId
		}
	  }`

	request := client.NewRequest(query)
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
