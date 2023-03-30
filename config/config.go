package config

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
)

var (
	defaultHost    = "https://circleci.com"
	defaultAPIHost = "https://api.circleci.com"

	// Making this the one true source for default config path
	DefaultConfigPath = ".circleci/config.yml"
)

type ConfigCompiler struct {
	host                   string
	compileRestClient      *rest.Client
	collaboratorRestClient *rest.Client
}

func New(cfg *settings.Config) *ConfigCompiler {
	hostValue := getCompileHost(cfg.Host)
	configCompiler := &ConfigCompiler{
		host:                   hostValue,
		compileRestClient:      rest.NewFromConfig(hostValue, cfg),
		collaboratorRestClient: rest.NewFromConfig(cfg.Host, cfg),
	}
	return configCompiler
}

func getCompileHost(cfgHost string) string {
	if cfgHost != defaultHost {
		return cfgHost
	} else {
		return defaultAPIHost
	}
}

type ConfigError struct {
	Message string `json:"message"`
}

// ConfigResponse - the structure of what is returned from the downstream compilation endpoint
type ConfigResponse struct {
	Valid      bool          `json:"valid"`
	SourceYaml string        `json:"source-yaml"`
	OutputYaml string        `json:"output-yaml"`
	Errors     []ConfigError `json:"errors"`
}

// CompileConfigRequest - the structure of the data we send to the downstream compilation service.
type CompileConfigRequest struct {
	ConfigYaml string  `json:"config_yaml"`
	Options    Options `json:"options"`
}

type Options struct {
	OwnerID            string                 `json:"owner_id,omitempty"`
	PipelineParameters map[string]interface{} `json:"pipeline_parameters,omitempty"`
	PipelineValues     map[string]string      `json:"pipeline_values,omitempty"`
}

// ConfigQuery - attempts to compile or validate a given config file with the
// passed in params/values.
// If the orgID is passed in, the config-compilation with private orbs should work.
func (c *ConfigCompiler) ConfigQuery(
	configPath string,
	orgID string,
	params Parameters,
	values Values,
) (*ConfigResponse, error) {
	configString, err := loadYaml(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load yaml config from config path provider: %w", err)
	}

	compileRequest := CompileConfigRequest{
		ConfigYaml: configString,
		Options: Options{
			OwnerID:            orgID,
			PipelineValues:     values,
			PipelineParameters: params,
		},
	}

	req, err := c.compileRestClient.NewRequest(
		"POST",
		&url.URL{
			Path: "compile-config-with-defaults",
		},
		compileRequest,
	)
	if err != nil {
		return nil, fmt.Errorf("an error occurred creating the request: %w", err)
	}

	configCompilationResp := &ConfigResponse{}
	statusCode, err := c.compileRestClient.DoRequest(req, configCompilationResp)
	if err != nil {
		if statusCode == 404 {
			return nil, errors.New("this version of the CLI does not support your instance of server, please refer to https://github.com/CircleCI-Public/circleci-cli for version compatibility")
		}
		return nil, fmt.Errorf("config compilation request returned an error: %w", err)
	}

	if statusCode != 200 {
		return nil, errors.New("unable to validate or compile config")
	}

	if len(configCompilationResp.Errors) > 0 {
		return nil, fmt.Errorf("config compilation contains errors: %s", configCompilationResp.Errors)
	}

	return configCompilationResp, nil
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
