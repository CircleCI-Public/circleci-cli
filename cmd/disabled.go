package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

type disableOptions struct {
	cfg     *settings.Config
	log     *logger.Logger
	command string
	args    []string
}

// nolint: unparam
func newDisabledCommand(config *settings.Config, command string) *cobra.Command {
	opts := disableOptions{
		cfg:     config,
		command: command,
	}

	disable := &cobra.Command{
		Use:   opts.command,
		Short: "This command is unavailable on your platform",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
		},
		Run: func(_ *cobra.Command, _ []string) {
			disableCommand(opts)
		},
	}

	return disable
}

func disableCommand(opts disableOptions) {
	opts.log.Infof("`%s` is not available because this tool was installed using `%s`.", opts.command, PackageManager)

	if opts.command == "update" {
		opts.log.Info("Please consult the package manager's documentation on how to update the CLI.")
	}
}
