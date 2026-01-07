package pipeline

import (
	"encoding/json"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/spf13/cobra"
)

func newWorkflowsCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	var jsonFormat bool

	cmd := &cobra.Command{
		Use:   "workflows <pipeline-id>",
		Short: "List workflows for a pipeline.",
		Long: `List workflows for a pipeline.

Examples:
  circleci pipeline workflows 5034460f-c7c4-4c43-9457-de07e2029e7b`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			pipelineID := args[0]

			workflows, err := ops.pipelineClient.ListWorkflowsByPipelineId(pipelineID)
			if err != nil {
				return err
			}

			if jsonFormat {
				payload, err := json.Marshal(workflows)
				if err != nil {
					return err
				}
				cmd.Println(string(payload))
				return nil
			}

			for _, w := range workflows {
				cmd.Printf("%s\t%s\t%s\n", w.ID, w.Name, w.Status)
			}

			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().BoolVar(&jsonFormat, "json", false, "Return output back in JSON format")

	return cmd
}
