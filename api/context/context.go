package context

import (
	"errors"
	"fmt"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

// An EnvironmentVariable has a Variable, a ContextID (its owner), and a
// CreatedAt date.
type EnvironmentVariable struct {
	Variable  string    `json:"variable"`
	ContextID string    `json:"context_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// A Context is the owner of EnvironmentVariables.
type Context struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
}

// ContextInterface is the interface to interact with contexts and environment
// variables.
type ContextInterface interface {
	Contexts() ([]Context, error)
	ContextByName(name string) (Context, error)
	CreateContext(name string) error
	DeleteContext(contextID string) error
	EnvironmentVariables(contextID string) ([]EnvironmentVariable, error)
	CreateEnvironmentVariable(contextID, variable, value string) error
	DeleteEnvironmentVariable(contextID, variable string) error
}

func NewContextClient(config *settings.Config, orgID, vcsType, orgName string) ContextInterface {
	restClient := restClient{
		client:  rest.NewFromConfig(config.Host, config),
		orgID:   orgID,
		vcsType: vcsType,
		orgName: orgName,
	}

	if config.Host == "https://circleci.com" {
		return restClient
	}
	if err := IsRestAPIAvailable(restClient); err != nil {
		fmt.Printf("err = %+v\n", err)
		return &gqlClient{
			client:  graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug),
			orgID:   orgID,
			vcsType: vcsType,
			orgName: orgName,
		}
	}
	return restClient
}

func IsRestAPIAvailable(c restClient) error {
	u, err := c.client.BaseURL.Parse("openapi.json")
	if err != nil {
		return err
	}
	req, err := c.client.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}

	var resp struct {
		Paths struct {
			ContextEndpoint interface{} `json:"/context"`
		}
	}
	if _, err := c.client.DoRequest(req, &resp); err != nil {
		return err
	}
	if resp.Paths.ContextEndpoint == nil {
		return errors.New("No context endpoint exists")
	}

	return nil
}
