package runner

import (
	"io"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/api/runner"
)

func generateConfig(t runner.Token, w io.Writer) (err error) {
	return yaml.NewEncoder(w).Encode(&agentConfig{
		API: apiConfig{
			AuthToken: t.Token,
		},
		Runner: runnerConfig{
			Name: t.Nickname,
		},
	})
}

type agentConfig struct {
	API    apiConfig    `yaml:"api"`
	Runner runnerConfig `yaml:"runner"`
}

type apiConfig struct {
	AuthToken string `yaml:"auth_token"`
}

type runnerConfig struct {
	Name string `yaml:"name"`
}
