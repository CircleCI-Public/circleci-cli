package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/pipeline"
	"github.com/pkg/errors"
)

type ConfigError struct {
	Message string `json:"message"`
}

type ConfigResponse struct {
	Valid      bool          `json:"valid"`
	SourceYaml string        `json:"source-yaml"`
	OutputYaml string        `json:"output-yaml"`
	Errors     []ConfigError `json:"errors"`
}

type CompileConfigRequest struct {
	ConfigYaml string  `json:"config_yaml"`
	Options    Options `json:"options"`
}

type Options struct {
	OwnerID            string            `json:"owner_id,omitempty"`
	PipelineParameters string            `json:"pipeline_parameters,omitempty"`
	PipelineValues     map[string]string `json:"pipeline_values,omitempty"`
}

// #nosec
func loadYaml(path string) (string, error) {
	var err error
	var config []byte
	if path == "-" {
		config, err = ioutil.ReadAll(os.Stdin)
	} else {
		config, err = ioutil.ReadFile(path)
	}

	if err != nil {
		return "", errors.Wrapf(err, "Could not load config file at %s", path)
	}

	return string(config), nil
}

// ConfigQuery - attempts to compile or validate a given config file with the
// passed in params/values.
// If the orgID is passed in, the config-compilation with private orbs should work.
func ConfigQuery(
	rest *rest.Client,
	configPath string,
	orgID string,
	params pipeline.Parameters,
	values pipeline.Values,
) (*ConfigResponse, error) {

	configString, err := loadYaml(configPath)
	if err != nil {
		return nil, err
	}

	compileRequest := CompileConfigRequest{
		ConfigYaml: configString,
		Options: Options{
			PipelineValues: values,
		},
	}

	if orgID != "" {
		compileRequest.Options.OwnerID = orgID
	}

	if len(params) >= 1 {
		pipelineParamsString, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		compileRequest.Options.PipelineParameters = string(pipelineParamsString)
	}

	req, err := rest.NewAPIRequest(
		"POST",
		&url.URL{
			Path: "compile-config-with-defaults",
		},
		compileRequest,
	)
	if err != nil {
		return nil, err
	}

	configCompilationResp := &ConfigResponse{}
	statusCode, err := rest.DoRequest(req, configCompilationResp)
	if err != nil {
		return nil, err
	}
	if statusCode != 200 {
		return nil, errors.New("non 200 status code")
	}

	if len(configCompilationResp.Errors) > 0 {
		return nil, errors.New(fmt.Sprintf("config compilation contains errors: %s", configCompilationResp.Errors))
	}

	return configCompilationResp, nil
}
