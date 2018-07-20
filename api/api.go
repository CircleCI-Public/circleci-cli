package api

import (
	"context"
	"io/ioutil"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// GQLResponseErrors is a slice of errors returned by the GraphQL server. Each
// error message is a key-value pair with the structure "Message: string"
type GQLResponseErrors struct {
	Errors []struct {
		Message string
	}
}

// ConfigResponse is a structure that matches the result of the GQL
// query, so that we can use mapstructure to convert from
// nested maps to a strongly typed struct.
type ConfigResponse struct {
	Valid      bool
	SourceYaml string
	OutputYaml string

	GQLResponseErrors
}

// The PublishOrbResponse type matches the data shape of the GQL response for
// publishing an orb.
type PublishOrbResponse struct {
	Orb struct {
		CreatedAt string
		Version   string
	}

	GQLResponseErrors
}

// ToError returns all GraphQL errors for a single response concatenated, or
// nil.
func (response GQLResponseErrors) ToError() error {
	messages := []string{}

	for i := range response.Errors {
		messages = append(messages, response.Errors[i].Message)
	}

	return errors.New(strings.Join(messages, ": "))
}

func loadYaml(path string) (string, error) {

	config, err := ioutil.ReadFile(path)

	if err != nil {
		return "", errors.Wrapf(err, "Could not load config file at %s", path)
	}

	return string(config), nil
}

func buildAndOrbQuery(ctx context.Context, logger *logger.Logger, configPath string, response interface{}, query string) error {
	config, err := loadYaml(configPath)
	if err != nil {
		return err
	}

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("config", config)
	graphQLclient := client.NewClient(viper.GetString("endpoint"), logger)

	err = graphQLclient.Run(ctx, request, response)

	if err != nil {
		return errors.Wrap(err, "Unable to validate config")
	}

	return nil
}

// ConfigQuery calls the GQL API to validate and expand config
func ConfigQuery(ctx context.Context, logger *logger.Logger, configPath string) (*ConfigResponse, error) {
	var response struct {
		BuildConfig struct {
			ConfigResponse
		}
	}
	return &response.BuildConfig.ConfigResponse, buildAndOrbQuery(ctx, logger, configPath, &response, `
		query ValidateConfig ($config: String!) {
			buildConfig(configYaml: $config) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`)
}

// OrbQuery validated and expands an orb.
func OrbQuery(ctx context.Context, logger *logger.Logger, configPath string) (*ConfigResponse, error) {
	var response struct {
		OrbConfig struct {
			ConfigResponse
		}
	}

	return &response.OrbConfig.ConfigResponse, buildAndOrbQuery(ctx, logger, configPath, &response, `
		query ValidateOrb ($config: String!) {
			orbConfig(orbYaml: $config) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`)
}

// OrbPublish publishes a new version of an orb
func OrbPublish(ctx context.Context, logger *logger.Logger,
	configPath string, orbVersion string, orbID string) (*PublishOrbResponse, error) {
	var response struct {
		PublishOrb struct {
			PublishOrbResponse
		}
	}

	config, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}

	query := `
		mutation($config: String!, $orbId: UUID!, $version: String!) {
			publishOrb(
				orbId: $orbId,
				orbYaml: $config,
				version: $version
			) {
				orb {
					version
					createdAt
				}
				errors { message }
			}
		}
	`

	request := client.NewAuthorizedRequest(viper.GetString("token"), query)
	request.Var("config", config)
	request.Var("orbId", orbID)
	request.Var("version", orbVersion)

	graphQLclient := client.NewClient(viper.GetString("endpoint"), logger)

	err = graphQLclient.Run(ctx, request, &response)

	if err != nil {
		err = errors.Wrap(err, "Unable to publish orb")
	}
	return &response.PublishOrb.PublishOrbResponse, err
}
