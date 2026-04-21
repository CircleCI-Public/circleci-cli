package job

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

// NewJobCmd returns the "circleci job" command group.
func NewJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job <command>",
		Short: "Manage jobs",
		Long: heredoc.Doc(`
			Work with CircleCI jobs.

			Jobs are the individual units of work within a workflow.
		`),
	}

	cmd.AddCommand(newArtifactsCmd())
	cmd.AddCommand(newLogsCmd())

	return cmd
}
