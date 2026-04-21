package pipeline

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
)

// NewPipelineCmd returns the "circleci pipeline" command group.
func NewPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline <command>",
		Short: "Manage pipelines",
		Long: heredoc.Doc(`
			Work with CircleCI pipelines.

			Pipelines are the top-level unit of work in CircleCI — they contain
			one or more workflows, each of which contains jobs.
		`),
	}

	cmd.AddCommand(newCancelCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newTriggerCmd())
	cmd.AddCommand(newWatchCmd())

	return cmd
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"pipeline.not_found", "No pipeline found for %q.",
		"Check the pipeline UUID or branch name and try again")
}
