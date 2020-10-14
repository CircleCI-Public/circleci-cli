package runner

import (
	"io"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/api/runner"
)

type AgentConfig struct {
	API    APIConfig    `yaml:"api"`
	Runner RunnerConfig `yaml:"runner"`
}

func NewAgentConfig(t runner.Token) *AgentConfig {
	return &AgentConfig{
		API: APIConfig{
			AuthToken: t.Token,
		},
		Runner: RunnerConfig{
			Name:                    t.Nickname,
			ResourceClass:           t.ResourceClass,
			CommandPrefix:           []string{"/opt/circleci/launch-task"},
			WorkingDirectory:        "/opt/circleci/workdir/%s",
			CleanupWorkingDirectory: true,
		},
	}
}

func (c *AgentConfig) WriteYaml(w io.Writer) error {
	return yaml.NewEncoder(w).Encode(c)
}

type APIConfig struct {
	AuthToken string `yaml:"auth_token"`
}

type RunnerConfig struct {
	Name                    string   `yaml:"name"`
	ResourceClass           string   `yaml:"resource_class"`
	CommandPrefix           []string `yaml:"command_prefix,flow"`
	WorkingDirectory        string   `yaml:"working_directory"`
	CleanupWorkingDirectory bool     `yaml:"cleanup_working_directory"`
}
