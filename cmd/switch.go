package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

type switchOptions struct {
	*settings.Config
	args []string
}

func newSwitchCommand(config *settings.Config) *cobra.Command {
	opts := switchOptions{
		Config: config,
	}

	return &cobra.Command{
		Use: "switch",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSwitch(opts)
		},
		Hidden: true,
	}
}

func runSwitch(opts switchOptions) error {
	opts.Logger.Infoln("You've already updated to the latest CLI. Please see `circleci help` for usage.")
	return nil
}
