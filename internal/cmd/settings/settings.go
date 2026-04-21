package settings

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

// NewSettingsCmd returns the "circleci settings" command group.
func NewSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings <command>",
		Short: "Manage CLI settings",
		Long: heredoc.Doc(`
			View and modify settings for the circleci CLI tool.

			Use 'circleci settings set token' to configure your personal API token.
			Use 'circleci settings list' to view current settings.

			For pipeline YAML operations, see 'circleci config'.
		`),
	}

	cmd.AddCommand(newSetCmd())
	cmd.AddCommand(newListCmd())

	return cmd
}
