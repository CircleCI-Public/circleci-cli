package cmd

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/spf13/cobra"
)

type versionOptions struct {
	cfg  *settings.Config
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
		PreRun: func(_ *cobra.Command, args []string) {
			opts.args = args
		},
		Run: func(cmd *cobra.Command, _ []string) {
			version := fmt.Sprintf("%s+%s (%s)", version.Version, version.Commit, version.PackageManager())

			telemetryClient, ok := telemetry.FromContext(cmd.Context())
			fmt.Printf("telemetryClient = %+v\n", telemetryClient)
			fmt.Printf("ok = %+v\n", ok)
			if ok {
				_ = telemetryClient.Track(telemetry.CreateVersionEvent(version))
			}

			fmt.Printf("%s\n", version)
		},
	}
}
