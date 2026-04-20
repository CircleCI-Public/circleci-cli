package workflow

import (
	"encoding/json"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/spf13/cobra"
)

func newJobsCommand(ops *workflowOpts, preRunE validator.Validator) *cobra.Command {
	var jsonFormat bool

	cmd := &cobra.Command{
		Use:   "jobs <workflow-id>",
		Short: "List jobs for a workflow.",
		Long: `List jobs for a workflow.

Examples:
  circleci workflow jobs 93f07932-8924-42cb-88d3-9e0c4ad75905`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			workflowID := args[0]

			jobs, err := ops.workflowClient.ListWorkflowJobs(workflowID)
			if err != nil {
				return err
			}

			if jsonFormat {
				payload, err := json.Marshal(jobs)
				if err != nil {
					return err
				}
				cmd.Println(string(payload))
				return nil
			}

			for _, j := range jobs {
				cmd.Printf("%s\t%s\t%d\n", j.Name, j.Status, j.JobNumber)
			}

			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().BoolVar(&jsonFormat, "json", false, "Return output back in JSON format")

	return cmd
}
