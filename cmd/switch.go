package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

type switchOptions struct {
	cfg  *settings.Config
	log  *logger.Logger
	args []string
}

func newSwitchCommand(config *settings.Config) *cobra.Command {
	opts := switchOptions{
		cfg: config,
	}

	return &cobra.Command{
		Use: "switch",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSwitch(opts)
		},
		Hidden: true,
	}
}

func runSwitch(opts switchOptions) error {
	opts.log.Infoln("You've already updated to the latest CLI. Please see `circleci help` for usage.")
	return nil
}
