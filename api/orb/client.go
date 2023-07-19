package orb

import (
	"fmt"
	"sync"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

var (
	once   sync.Once
	client Client
)

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

func GetClient(config *settings.Config) Client {
	once.Do(func() {
		createClient(config)
	})
	return client
}

func createClient(config *settings.Config) {
	gql := graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug)

	ok, err := orbQueryHandleOwnerId(gql)
	if err != nil {
		fmt.Printf("While requesting orb server: %s", err)
		return
	} else if ok {
		client = &latestClient{gql}
	} else {
		client = &deprecatedClient{gql}
	}
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
	query := `
query ValidateOrb {
  __schema {
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
