package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

func newAdminCommand(config *settings.Config) *cobra.Command {
	orbOpts := orbOptions{
		cfg: config,
		tty: createOrbInteractiveUI{},
	}
	nsOpts := namespaceOptions{
		cfg: config,
		tty: createNamespaceInteractiveUI{},
	}

	importOrbCommand := &cobra.Command{
		Use:   "import-orb <namespace>[/<orb>[@<version>]]",
		Short: "Import an orb version from circleci.com into a CircleCI Server installation",
		RunE: func(_ *cobra.Command, _ []string) error {
			return importOrb(orbOpts)
		},
		Args: cobra.MinimumNArgs(1),
	}
	importOrbCommand.Flags().BoolVar(&orbOpts.integrationTesting, "integration-testing", false, "Enable test mode to bypass interactive UI.")
	if err := importOrbCommand.Flags().MarkHidden("integration-testing"); err != nil {
		panic(err)
	}
	importOrbCommand.Flags().BoolVar(&orbOpts.noPrompt, "no-prompt", false, "Disable prompt to bypass interactive UI.")

	renameCommand := &cobra.Command{
		Use:   "rename-namespace <old-name> <new-name>",
		Short: "Rename a namespace",
		PreRunE: func(_ *cobra.Command, args []string) error {
			nsOpts.args = args
			nsOpts.cl = graphql.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)

			return validateToken(nsOpts.cfg)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return renameNamespace(nsOpts)
		},
		Args:        cobra.ExactArgs(2),
		Annotations: make(map[string]string),
	}
	renameCommand.Flags().BoolVar(&nsOpts.noPrompt, "no-prompt", false, "Disable prompt to bypass interactive UI.")

	renameCommand.Annotations["<old-name>"] = "The current name of the namespace"
	renameCommand.Annotations["<new-name>"] = "The new name you want to give the namespace"

	deleteAliasCommand := &cobra.Command{
		Use:   "delete-namespace-alias <name>",
		Short: "Delete a namespace alias",
		Long: `Delete a namespace alias.

A namespace can have multiple aliases (names). This command deletes an alias left behind by a rename. The most recent alias cannot be deleted.

Example:
- namespace A is renamed to B
- alias B is created, coexisting with alias A
- after migrating config accordingly, we can delete the A alias.`,
		PreRunE: func(_ *cobra.Command, args []string) error {
			return validateToken(nsOpts.cfg)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			if nsOpts.integrationTesting {
				nsOpts.tty = createNamespaceTestUI{
					confirm: true,
				}
			}
			return deleteNamespaceAlias(nsOpts)
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}

	deleteAliasCommand.Annotations["<name>"] = "The name of the alias to delete"
	deleteAliasCommand.Flags().BoolVar(&nsOpts.noPrompt, "no-prompt", false, "Disable prompt to bypass interactive UI.")
	deleteAliasCommand.Flags().BoolVar(&nsOpts.integrationTesting, "integration-testing", false, "Enable test mode to bypass interactive UI.")
	if err := deleteAliasCommand.Flags().MarkHidden("integration-testing"); err != nil {
		panic(err)
	}

	adminCommand := &cobra.Command{
		Use:   "admin",
		Short: "Administrative operations for a CircleCI Server installation.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			orbOpts.args = args
			nsOpts.args = args
			orbOpts.cl = graphql.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)
			nsOpts.cl = orbOpts.cl

			// PersistentPreRunE overwrites the inherited persistent hook from rootCmd
			// So we explicitly call it here to retain that behavior.
			// As of writing this comment, that is only for daily update checks.
			return rootCmdPreRun(rootOptions)
		},
		Hidden: true,
	}

	adminCommand.AddCommand(importOrbCommand)
	adminCommand.AddCommand(renameCommand)
	adminCommand.AddCommand(deleteAliasCommand)

	return adminCommand
}
