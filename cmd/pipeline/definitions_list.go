package pipeline

import (
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/slug"
	"github.com/spf13/cobra"
)

func newDefinitionsListCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <project-slug>",
		Short: "List pipeline definitions for a project.",
		Long: `List pipeline definitions for a project.

Examples:
  circleci pipeline definitions list gh/CircleCI-Public/circleci-cli`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectSlug := args[0]

			parsed, err := slug.ParseProject(projectSlug)
			if err != nil {
				return err
			}

			projectInfo, err := ops.projectClient.ProjectInfo(parsed.VCS, parsed.Org, parsed.Repo)
			if err != nil {
				return err
			}

			definitions, err := ops.pipelineClient.ListPipelineDefinitions(projectInfo.Id)
			if err != nil {
				return err
			}

			if len(definitions) == 0 {
				cmd.Println("No pipeline definitions found for this project.")
				return nil
			}

			printPipelineDefinitions(cmd, definitions)
			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	return cmd
}
