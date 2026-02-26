package pipeline

import (
	"encoding/json"
	"fmt"

	pipelineapi "github.com/CircleCI-Public/circleci-cli/api/pipeline"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/spf13/cobra"
)

func newRunsLatestCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	var branch string
	var jsonFormat bool

	cmd := &cobra.Command{
		Use:   "latest <project-slug>",
		Short: "Print the latest pipeline run for a project.",
		Long: `Print the latest pipeline run for a project.

Examples:
  circleci pipeline runs latest gh/CircleCI-Public/circleci-cli --branch main`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectSlug := args[0]

			resp, err := ops.pipelineClient.ListPipelinesForProject(projectSlug, pipelineapi.ListPipelinesOptions{
				Branch: branch,
			})
			if err != nil {
				return err
			}

			if len(resp.Items) == 0 {
				return fmt.Errorf("no pipelines found for project %s (branch=%s)", projectSlug, branch)
			}

			latest := resp.Items[0]

			if jsonFormat {
				payload, err := json.Marshal(latest)
				if err != nil {
					return err
				}
				cmd.Println(string(payload))
				return nil
			}

			cmd.Printf("%s\t%d\n", latest.ID, latest.Number)
			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&branch, "branch", "", "The name of a vcs branch.")
	cmd.Flags().BoolVar(&jsonFormat, "json", false, "Return output back in JSON format")
	_ = cmd.MarkFlagRequired("branch")

	return cmd
}
