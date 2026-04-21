// Package workflow implements the "circleci workflow" command group.
package workflow

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
)

// NewWorkflowCmd returns the "circleci workflow" command group.
func NewWorkflowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow <command>",
		Short: "Manage workflows",
		Long: heredoc.Doc(`
			Work with CircleCI workflows.

			Workflows orchestrate jobs within a pipeline. Use these commands to
			inspect workflow status, rerun failed jobs, or cancel a running workflow.

			Workflow IDs are shown in the output of 'circleci pipeline get'.
		`),
	}

	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newRerunCmd())
	cmd.AddCommand(newCancelCmd())

	return cmd
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"workflow.not_found", "No workflow found for %q.",
		"Check the workflow ID with: circleci pipeline get")
}
