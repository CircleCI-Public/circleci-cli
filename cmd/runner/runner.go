package runner

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/api/runner"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

func NewCommand(config *settings.Config, preRunE validator) *cobra.Command {
	r := runner.New(rest.New(config.Host, config.RestEndpoint, config.Token))
	cmd := &cobra.Command{
		Use:    "runner",
		Short:  "Operate on runners",
		Hidden: true,
	}
	cmd.AddCommand(newResourceClassCommand(r, preRunE))
	cmd.AddCommand(newTokenCommand(r, preRunE))
	cmd.AddCommand(newRunnerInstanceCommand(r, preRunE))
	return cmd
}

type validator func(cmd *cobra.Command, args []string) error
