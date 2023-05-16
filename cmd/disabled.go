package cmd

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type disableOptions struct {
	cfg     *settings.Config
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
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			opts.cfg.SkipUpdateCheck = true
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
		},
		Run: func(_ *cobra.Command, _ []string) {
			disableCommand(opts)
		},
	}

	return disable
}

func disableCommand(opts disableOptions) {
	bold := color.New(color.Bold).SprintFunc()
	fmt.Printf("%s is not available because this tool was installed using %s.\n", bold(opts.command), bold(version.PackageManager()))

	if opts.command == "update" {
		fmt.Println("Please consult the package manager's documentation on how to update the CLI.")
	}
}
