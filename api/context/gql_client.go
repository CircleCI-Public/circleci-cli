package context

import (
	"errors"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/errs"
)

type gqlClient struct {
	client *graphql.Client

	orgID   string
	vcsType string
	orgName string
}

func (c *gqlClient) ensureOrgIDIsDefined() error {
	if c.orgID != "" {
		return nil
	}
	if c.vcsType == "" || c.orgName == "" {
		return errors.New("need org id or vcs type and org name to be defined")
	}
	org, err := api.GetOrganization(c.client, api.GetOrganizationParams{OrgName: c.orgName, VCSType: c.vcsType})
	if err != nil {
		return err
	}
	c.orgID = org.Organization.ID
	return nil
}

func (c *gqlClient) Contexts() ([]Context, error) {
	return ListContextsWithGQL(c.client, ListContextsWithGQLParams{
		OrgID:   c.orgID,
		VCSType: c.vcsType,
		OrgName: c.orgName,
	})
}

func (c *gqlClient) ContextByName(name string) (Context, error) {
	contexts, err := ListContextsWithGQL(c.client, ListContextsWithGQLParams{
		OrgID:   c.orgID,
		VCSType: c.vcsType,
		OrgName: c.orgName,
	})
	if err != nil {
		return Context{}, err
	}

	for _, context := range contexts {
		if context.Name == name {
			return context, nil
		}
	}
	return Context{}, errs.NotFoundf("No context found with that name")
}

func (c *gqlClient) CreateContext(name string) error {
	if err := c.ensureOrgIDIsDefined(); err != nil {
		return err
	}
	_, err := CreateContextWithGQL(c.client, CreateContextWithGQLParams{
		OwnerId:     c.orgID,
		OwnerType:   "ORGANIZATION",
		ContextName: name,
	})
	return err
}

func (c *gqlClient) DeleteContext(contextID string) error {
	return DeleteContextWithGQL(c.client, contextID)
}

func (c *gqlClient) EnvironmentVariables(contextID string) ([]EnvironmentVariable, error) {
	return ListEnvVarsWithGQL(c.client, contextID)
}

func (c *gqlClient) CreateEnvironmentVariable(contextID, variable, value string) error {
	return CreateEnvVarWithGQL(c.client, CreateEnvVarWithRestParams{contextID, variable, value})
}

func (c *gqlClient) DeleteEnvironmentVariable(contextID, variable string) error {
	return DeleteEnvVarWithGQL(c.client, DeleteEnvVarWithRestParams{contextID, variable})
}
