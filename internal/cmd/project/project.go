// Package project implements the "circleci project" command group.
package project

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

// NewProjectCmd returns the "circleci project" parent command.
func NewProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <command>",
		Short: "Manage CircleCI projects",
		Long: heredoc.Doc(`
			List, follow, and manage settings for CircleCI projects.

			A project corresponds to a version-control repository connected
			to CircleCI. Use 'circleci project list' to see all followed projects,
			'circleci project follow' to start following a new project, and
			'circleci project env' to manage environment variables.

			To manage environment variables directly, use the top-level alias:
			  circleci envvar list --project gh/org/repo
		`),
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newFollowCmd())
	cmd.AddCommand(newEnvCmd())

	return cmd
}
