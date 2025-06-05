package orb

import (
	"io"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
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

func NewClient(config *settings.Config) (Client, error) {
	gql := graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug)

	// Since ownerId is optional in the GraphQL schema (UUID, not non-null UUID)
	// and the resolver handles nil ownerId gracefully, we can always use v2 client
	return &v2Client{gql}, nil
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
