package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/spf13/cobra"
)

type versionOptions struct {
	cfg  *settings.Config
	log  *logger.Logger
	args []string
}

func newVersionCommand(config *settings.Config) *cobra.Command {
	opts := versionOptions{
		cfg: config,
	}

	return &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			opts.cfg.SkipUpdateCheck = true
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
		},
		Run: func(_ *cobra.Command, _ []string) {
			opts.log.Infof("%s+%s", version.Version, version.Commit)
		},
	}
}
