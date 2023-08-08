package orb

import (
	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/pkg/errors"
)

// This client makes request to servers that **DON'T** have the field `ownerId` in the GraphQL query method: `orbConfig`

const v1_string clientVersion = "v1"

type v1Client struct {
	gql *graphql.Client
}

func (client *v1Client) OrbQuery(configPath string, ownerId string) (*api.ConfigResponse, error) {
	if ownerId != "" {
		return nil, errors.New("Your version of Server does not support validating orbs that refer to other private orbs. Please see the README for more information on server compatibility: https://github.com/CircleCI-Public/circleci-cli#server-compatibility")
	}

	var response QueryResponse

	configContent, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}

	query := `query ValidateOrb ($config: String!) {
	orbConfig(orbYaml: $config) {
		valid,
		errors { message },
		sourceYaml,
		outputYaml
	}
}`

	request := graphql.NewRequest(query)
	request.Var("config", configContent)

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
