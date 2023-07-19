package config

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/pkg/errors"
)

const v1_string apiClientVersion = "v1"

type v1APIClient struct {
	gql *graphql.Client
}

// GQLErrorsCollection is a slice of errors returned by the GraphQL server.
// Each error is made up of a GQLResponseError type.
type GQLErrorsCollection []GQLResponseError

// BuildConfigResponse wraps the GQL result of the ConfigQuery
type BuildConfigResponse struct {
	BuildConfig struct {
		LegacyConfigResponse
	}
}

// Error turns a GQLErrorsCollection into an acceptable error string that can be printed to the user.
func (errs GQLErrorsCollection) Error() string {
	message := "config compilation contains errors:"

	for i := range errs {
		message += fmt.Sprintf("\n\t- %s", errs[i].Message)
	}

	return message
}

// LegacyConfigResponse is a structure that matches the result of the GQL
// query, so that we can use mapstructure to convert from
// nested maps to a strongly typed struct.
type LegacyConfigResponse struct {
	Valid      bool
	SourceYaml string
	OutputYaml string

	Errors GQLErrorsCollection
}

// GQLResponseError is a mapping of the data returned by the GraphQL server of key-value pairs.
// Typically used with the structure "Message: string", but other response errors provide additional fields.
type GQLResponseError struct {
	Message       string
	Value         string
	AllowedValues []string
	EnumType      string
	Type          string
}

// KeyVal is a data structure specifically for passing pipeline data to GraphQL which doesn't support free-form maps.
type KeyVal struct {
	Key string      `json:"key"`
	Val interface{} `json:"val"`
}

// PrepareForGraphQL takes a golang homogenous map, and transforms it into a list of keyval pairs, since GraphQL does not support homogenous maps.
func PrepareForGraphQL(kvMap Values) []KeyVal {
	// we need to create the slice of KeyVals in a deterministic order for testing purposes
	keys := make([]string, 0, len(kvMap))
	for k := range kvMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	kvs := make([]KeyVal, 0, len(kvMap))
	for _, k := range keys {
		kvs = append(kvs, KeyVal{Key: k, Val: kvMap[k]})
	}
	return kvs
}

func (client *v1APIClient) CompileConfig(configContent string, orgID string, params Parameters, values Values) (*ConfigResponse, error) {
	var response BuildConfigResponse
	// GraphQL isn't forwards-compatible, so we are unusually selective here about
	// passing only non-empty fields on to the API, to minimize user impact if the
	// backend is out of date.
	var fieldAddendums string
	if orgID != "" {
		fieldAddendums += ", orgId: $orgId"
	}
	if len(params) > 0 {
		fieldAddendums += ", pipelineParametersJson: $pipelineParametersJson"
	}
	query := fmt.Sprintf(
		`query ValidateConfig ($config: String!, $pipelineParametersJson: String, $pipelineValues: [StringKeyVal!], $orgSlug: String) {
			buildConfig(configYaml: $config, pipelineValues: $pipelineValues%s) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`,
		fieldAddendums,
	)

	request := graphql.NewRequest(query)
	request.SetToken(client.gql.Token)
	request.Var("config", configContent)

	if values != nil {
		request.Var("pipelineValues", PrepareForGraphQL(values))
	}
	if params != nil {
		pipelineParameters, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("unable to serialize pipeline values: %s", err.Error())
		}
		request.Var("pipelineParametersJson", string(pipelineParameters))
	}

	if orgID != "" {
		request.Var("orgId", orgID)
	}

	err := client.gql.Run(request, &response)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to validate config")
	}
	if len(response.BuildConfig.LegacyConfigResponse.Errors) > 0 {
		return nil, &response.BuildConfig.LegacyConfigResponse.Errors
	}

	return &ConfigResponse{
		Valid:      response.BuildConfig.LegacyConfigResponse.Valid,
		SourceYaml: response.BuildConfig.LegacyConfigResponse.SourceYaml,
		OutputYaml: response.BuildConfig.LegacyConfigResponse.OutputYaml,
	}, nil
}
