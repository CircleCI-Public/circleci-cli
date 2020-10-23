package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

func newAdminCommand(config *settings.Config) *cobra.Command {
	opts := orbOptions{
		cfg: config,
		tty: createOrbInteractiveUI{},
	}

	importOrbCommand := &cobra.Command{
		Use:   "import-orb <namespace>[/<orb>[@<version>]]",
		Short: "Import an orb version from circleci.com into a CircleCI Server installation",
		RunE: func(_ *cobra.Command, _ []string) error {
			return importOrb(opts)
		},
		Args:   cobra.MinimumNArgs(1),
		Hidden: true,
	}
	importOrbCommand.Flags().BoolVar(&opts.integrationTesting, "integration-testing", false, "Enable test mode to bypass interactive UI.")

	adminCommand := &cobra.Command{
		Use:   "admin",
		Short: "Administrative operations for a CircleCI Server installation.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			opts.args = args
			opts.cl = graphql.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)

			// PersistentPreRunE overwrites the inherited persistent hook from rootCmd
			// So we explicitly call it here to retain that behavior.
			// As of writing this comment, that is only for daily update checks.
			return rootCmdPreRun(rootOptions)
		},
	}

	adminCommand.AddCommand(importOrbCommand)

	return adminCommand
}
