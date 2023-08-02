package config

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

var (
	compiler      APIClient
	compilerError error
	once          sync.Once

	compilePath = "compile-config-with-defaults"
)

type apiClientVersion string

type APIClient interface {
	CompileConfig(configContent string, orgID string, params Parameters, values Values) (*ConfigResponse, error)
}

func GetAPIClient(config *settings.Config) (APIClient, error) {
	if compiler == nil {
		once.Do(func() {
			compiler, compilerError = newAPIClient(config)
		})
	}
	return compiler, compilerError
}

func newAPIClient(config *settings.Config) (APIClient, error) {
	hostValue := GetCompileHost(config.Host)
	restClient := rest.NewFromConfig(hostValue, config)

	version, err := detectAPIClientVersion(restClient)
	if err != nil {
		return nil, err
	}

	switch version {
	case v1_string:
		return &v1APIClient{graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug)}, nil
	case v2_string:
		return &v2APIClient{restClient}, nil
	default:
		return nil, fmt.Errorf("Unable to recognise your Server's config file API")
	}
}

func detectAPIClientVersion(restClient *rest.Client) (apiClientVersion, error) {
	req, err := restClient.NewRequest("POST", &url.URL{Path: compilePath}, nil)
	if err != nil {
		return "", err
	}

	statusCode, err := restClient.DoRequest(req, nil)
	if _, ok := err.(*rest.HTTPError); !ok {
		return "", err
	}
	if statusCode == 404 {
		return v1_string, nil
	}
	return v2_string, nil
}
