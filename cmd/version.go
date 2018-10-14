package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/spf13/cobra"
)

type versionOptions struct {
	*settings.Config
	args []string
}

func newVersionCommand(config *settings.Config) *cobra.Command {
	opts := versionOptions{
		Config: config,
	}

	return &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		Run: func(_ *cobra.Command, _ []string) {
			opts.Logger.Infof("%s+%s", version.Version, version.Commit)
		},
	}
}
