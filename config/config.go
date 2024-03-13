package config

import (
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"

	"github.com/CircleCI-Public/circleci-cli/api/collaborators"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

var (
	defaultHost    = "https://circleci.com"
	defaultAPIHost = "https://api.circleci.com"

	// Making this the one true source for default config path
	DefaultConfigPath = ".circleci/config.yml"
)

type ConfigCompiler struct {
	apiClient     APIClient
	collaborators collaborators.CollaboratorsClient
}

func NewWithConfig(cfg *settings.Config) (*ConfigCompiler, error) {
	apiClient, err := newAPIClient(cfg)
	if err != nil {
		return nil, err
	}
	collaboratorsClient, err := collaborators.NewCollaboratorsRestClient(*cfg)
	if err != nil {
		return nil, err
	}
	return New(apiClient, collaboratorsClient), nil
}

func New(apiClient APIClient, collaboratorsClient collaborators.CollaboratorsClient) *ConfigCompiler {
	configCompiler := &ConfigCompiler{
		apiClient:     apiClient,
		collaborators: collaboratorsClient,
	}

	return configCompiler
}

func GetCompileHost(cfgHost string) string {
	if cfgHost != defaultHost && cfgHost != "" {
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
	OwnerID string `json:"owner_id,omitempty"`
	// PipelineParameters are deprecated and will be removed in the future.
	// Use PipelineValues instead.
	PipelineParameters map[string]interface{} `json:"pipeline_parameters,omitempty"`
	PipelineValues     map[string]interface{} `json:"pipeline_values,omitempty"`
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

	return c.apiClient.CompileConfig(configString, orgID, params, values)
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
