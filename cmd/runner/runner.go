package runner

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/api/runner"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

type runnerOpts struct {
	r running
}

func NewCommand(config *settings.Config, preRunE validator.Validator) *cobra.Command {
	var opts runnerOpts
	cmd := &cobra.Command{
		Use:   "runner",
		Short: "Operate on runners",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			var host string
			if strings.Contains(config.Host, "https://circleci.com") {
				host = "https://runner.circleci.com"
			} else {
				host = config.Host
			}
			opts.r = runner.New(rest.NewFromConfig(host, config))
		},
	}

	cmd.AddCommand(newResourceClassCommand(&opts, preRunE))
	cmd.AddCommand(newTokenCommand(&opts, preRunE))
	cmd.AddCommand(newRunnerInstanceCommand(&opts, preRunE))

	return cmd
}

type running interface {
	CreateResourceClass(resourceClass, desc string) (rc *runner.ResourceClass, err error)
	GetResourceClassByName(resourceClass string) (rc *runner.ResourceClass, err error)
	GetNamespaceByResourceClass(resourceClass string) (ns string, err error)
	GetResourceClassesByNamespace(namespace string) ([]runner.ResourceClass, error)
	DeleteResourceClass(id string, force bool) error
	CreateToken(resourceClass, nickname string) (token *runner.Token, err error)
	GetRunnerTokensByResourceClass(resourceClass string) ([]runner.Token, error)
	DeleteToken(id string) error
	GetRunnerInstances(query string) ([]runner.RunnerInstance, error)
}
