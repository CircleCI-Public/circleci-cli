package runner

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/api/runner"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

type runnerOpts struct {
	r *runner.Runner
}

func NewCommand(config *settings.Config, preRunE validator) *cobra.Command {
	var opts runnerOpts
	cmd := &cobra.Command{
		Use:   "runner",
		Short: "Operate on runners",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			opts.r = runner.New(rest.New(config.Host, config.RestEndpoint, config.Token))
		},
	}
	cmd.AddCommand(newResourceClassCommand(&opts, preRunE))
	cmd.AddCommand(newTokenCommand(&opts, preRunE))
	cmd.AddCommand(newRunnerInstanceCommand(&opts, preRunE))
	return cmd
}

type validator func(cmd *cobra.Command, args []string) error
