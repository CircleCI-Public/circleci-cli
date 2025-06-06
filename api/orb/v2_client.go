package orb

import (
	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/pkg/errors"
)

type v2Client struct {
	gql *graphql.Client
}

func (client *v2Client) OrbQuery(configPath string, ownerId string) (*api.ConfigResponse, error) {
	var response QueryResponse

	configContent, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}

	query := `query ValidateOrb ($config: String!, $owner: UUID) {
	orbConfig(orbYaml: $config, ownerId: $owner) {
		valid,
		errors { message },
		sourceYaml,
		outputYaml
	}
}`

	request := graphql.NewRequest(query)
	request.Var("config", configContent)

	if ownerId != "" {
		request.Var("owner", ownerId)
	}
	request.SetToken(client.gql.Token)

	err = client.gql.Run(request, &response)
	if err != nil {
		return nil, errors.Wrap(err, "Validating config")
	}

	if len(response.OrbConfig.ConfigResponse.Errors) > 0 {
		return nil, response.OrbConfig.ConfigResponse.Errors
	}

	return &response.OrbConfig.ConfigResponse, nil
}
