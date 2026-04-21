package workflow

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newRerunCmd() *cobra.Command {
	var fromFailed bool

	cmd := &cobra.Command{
		Use:   "rerun <workflow-id>",
		Short: "Rerun a workflow",
		Long: heredoc.Doc(`
			Rerun a CircleCI workflow.

			By default all jobs in the workflow are rerun from scratch. Use
			--from-failed to rerun only the jobs that failed, leaving successful
			jobs untouched.

			Workflow IDs are shown in the output of 'circleci pipeline get'.
		`),
		Example: heredoc.Doc(`
			# Rerun all jobs in a workflow from scratch
			$ circleci workflow rerun 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Rerun only the failed jobs
			$ circleci workflow rerun 5034460f-c7c4-4c43-9457-de07e2029e7b --from-failed

			# Find a workflow ID from the latest pipeline
			$ circleci pipeline get --json | jq -r '.workflows[].id'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "workflow-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			return runRerun(ctx, streams, args[0], fromFailed)
		},
	}

	cmd.Flags().BoolVar(&fromFailed, "from-failed", false, "Rerun only failed jobs")
	return cmd
}

func runRerun(ctx context.Context, streams iostream.Streams, id string, fromFailed bool) error {
	client, cliErr := cmdutil.LoadClient()
	if cliErr != nil {
		return cliErr
	}

	if err := client.RerunWorkflow(ctx, id, fromFailed); err != nil {
		return apiErr(err, id)
	}

	if fromFailed {
		streams.Printf("Rerunning failed jobs in workflow %s\n", id)
	} else {
		streams.Printf("Rerunning workflow %s from scratch\n", id)
	}
	return nil
}
