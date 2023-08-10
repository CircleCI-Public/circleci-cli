package config

import (
	"fmt"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/pkg/errors"
)

const v2_string apiClientVersion = "v2"

type v2APIClient struct {
	restClient *rest.Client
}

func configErrorsAsError(configErrors []ConfigError) error {
	message := "config compilation contains errors:"
	for _, err := range configErrors {
		message += fmt.Sprintf("\n\t- %s", err.Message)
	}
	return errors.New(message)
}

func (client *v2APIClient) CompileConfig(configContent string, orgID string, params Parameters, values Values) (*ConfigResponse, error) {
	compileRequest := CompileConfigRequest{
		ConfigYaml: configContent,
		Options: Options{
			OwnerID:            orgID,
			PipelineValues:     values,
			PipelineParameters: params,
		},
	}

	req, err := client.restClient.NewRequest(
		"POST",
		&url.URL{Path: compilePath},
		compileRequest,
	)
	if err != nil {
		return nil, fmt.Errorf("an error occurred creating the request: %w", err)
	}

	configCompilationResp := &ConfigResponse{}
	statusCode, originalErr := client.restClient.DoRequest(req, configCompilationResp)

	if originalErr != nil {
		return nil, fmt.Errorf("config compilation request returned an error: %w", originalErr)
	}

	if statusCode != 200 {
		return nil, errors.New("unable to validate or compile config")
	}

	if len(configCompilationResp.Errors) > 0 {
		return nil, fmt.Errorf("config compilation contains errors: %s", configErrorsAsError(configCompilationResp.Errors))
	}

	return configCompilationResp, nil
}
