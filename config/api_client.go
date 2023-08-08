package config

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

const compilePath = "compile-config-with-defaults"

type apiClientVersion string

type APIClient interface {
	CompileConfig(configContent string, orgID string, params Parameters, values Values) (*ConfigResponse, error)
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

// detectAPIClientVersion returns the highest available version of the config API.
//
// To do that it tries to request the `compilePath` API route.
// If the route returns a 404, this means the route does not exist on the requested host and the function returns
// `v1_string` indicating that the deprecated GraphQL endpoint should be used instead.
// Else if the route returns any other status, this means it is available for request and the function returns
// `v2_string` indicating that the route can be used
func detectAPIClientVersion(restClient *rest.Client) (apiClientVersion, error) {
	req, err := restClient.NewRequest("POST", &url.URL{Path: compilePath}, nil)
	if err != nil {
		return "", err
	}

	_, err = restClient.DoRequest(req, nil)
	httpErr, ok := err.(*rest.HTTPError)
	if !ok {
		return "", err
	}
	if httpErr.Code == http.StatusNotFound {
		return v1_string, nil
	}
	return v2_string, nil
}
