package orb

import (
	"fmt"
	"io"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
)

type clientVersion string

// ConfigResponse is a structure that matches the result of the GQL
// query, so that we can use mapstructure to convert from
// nested maps to a strongly typed struct.
type QueryResponse struct {
	OrbConfig struct {
		api.ConfigResponse
	}
}

type Client interface {
	OrbQuery(configPath string, ownerId string) (*api.ConfigResponse, error)
}

func NewClient(config *settings.Config) (Client, error) {
	gql := graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug)

	clientVersion, err := detectClientVersion(gql)
	if err != nil {
		return &v1Client{gql}, nil
	}

	switch clientVersion {
	case v1_string:
		return &v1Client{gql}, nil
	case v2_string:
		return &v2Client{gql}, nil
	default:
		return nil, fmt.Errorf("Unable to recognise your server orb API")
	}
}

// detectClientVersion returns the highest available version of the orb API
//
// To do that it checks that whether the GraphQL query has the parameter "ownerId" or not.
// If it does not have the parameter, the function returns `v1_string` else it returns `v2_string`
func detectClientVersion(gql *graphql.Client) (clientVersion, error) {
	handlesOwnerId, err := orbQueryHandleOwnerId(gql)
	if err != nil {
		return "", err
	}
	if !handlesOwnerId {
		return v1_string, nil
	}
	return v2_string, nil
}

type OrbIntrospectionResponse struct {
	Schema struct {
		Query struct {
			Fields []struct {
				Name string `json:"name"`
				Args []struct {
					Name string `json:"name"`
				} `json:"args"`
			} `json:"fields"`
		} `json:"queryType"`
	} `json:"__schema"`
}

func orbQueryHandleOwnerId(gql *graphql.Client) (bool, error) {
	query := `query IntrospectionQuery {
	_schema {
		queryType {
			fields(includeDeprecated: true) {
				name
				args {
					name
					__typename
					type {
						name
					}
				}
			}
		}
	}
}`
	request := graphql.NewRequest(query)
	response := OrbIntrospectionResponse{}
	err := gql.Run(request, &response)
	if err != nil {
		return false, err
	}

	request.SetToken(gql.Token)

	// Find the orbConfig query method, look at its arguments, if it has the "ownerId" argument, return true
	for _, field := range response.Schema.Query.Fields {
		if field.Name == "orbConfig" {
			for _, arg := range field.Args {
				if arg.Name == "ownerId" {
					return true, nil
				}
			}
		}
	}

	// else return false, ownerId is not supported

	return false, nil
}

func loadYaml(path string) (string, error) {
	var err error
	var config []byte
	if path == "-" {
		config, err = io.ReadAll(os.Stdin)
	} else {
		config, err = os.ReadFile(path)
	}

	if err != nil {
		return "", errors.Wrapf(err, "Could not load config file at %s", path)
	}

	return string(config), nil
}
