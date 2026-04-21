// Package envvar implements the "circleci envvar" top-level command, which is the
// primary user-facing alias for "circleci project envvar".
package envvar

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/project"
)

// NewEnvVarCmd returns the "circleci envvar" command, the primary entry point for
// managing project environment variables. It is an alias for "circleci project envvar"
// and exists to satisfy the 2-level nesting rule (circleci project envvar list
// is 3 levels deep).
func NewEnvVarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "envvar <command>",
		Short: "Manage project environment variables",
		Long: heredoc.Doc(`
			List, set, and delete environment variables for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project.

			Environment variable values are masked in list output (shown as "xxxx").
			The full value is never retrievable after it has been set.

			Also available as: circleci project envvar <command>
		`),
		Example: heredoc.Doc(`
			# List all environment variables for the current project
			$ circleci envvar list

			# Set an environment variable
			$ circleci envvar set MY_SECRET s3cr3t --project gh/myorg/myrepo

			# Delete an environment variable
			$ circleci envvar delete MY_SECRET
		`),
	}

	cmd.AddCommand(project.NewEnvListCmd())
	cmd.AddCommand(project.NewEnvSetCmd())
	cmd.AddCommand(project.NewEnvDeleteCmd())

	return cmd
}
