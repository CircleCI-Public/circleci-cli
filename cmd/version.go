package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Run: func(cmd *cobra.Command, args []string) {
			Config.Logger.Infof("%s+%s", version.Version, version.Commit)
		},
	}
}
