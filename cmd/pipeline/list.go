package pipeline

import (
	"github.com/spf13/cobra"

	pipelineapi "github.com/CircleCI-Public/circleci-cli/api/pipeline"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/slug"
)

func newListCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <project-id|project-slug>",
		Short: "List pipeline definitions for a project.",
		Long: `List pipeline definitions for a project.

The project ID can be found in your project settings or in the URL of your project page.
Alternatively, you can provide a project slug in the format <vcs>/<org>/<repo>.

Examples:
  circleci pipeline list 12345678-1234-1234-1234-123456789012
  circleci pipeline list gh/CircleCI-Public/circleci-cli`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectIDOrSlug := args[0]

			projectID := projectIDOrSlug
			if isProjectSlug(projectIDOrSlug) {
				parsed, err := slug.ParseProject(projectIDOrSlug)
				if err != nil {
					return err
				}

				info, err := ops.projectClient.ProjectInfo(parsed.VCS, parsed.Org, parsed.Repo)
				if err != nil {
					return err
				}
				projectID = info.Id
			}

			definitions, err := ops.pipelineClient.ListPipelineDefinitions(projectID)
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

func printPipelineDefinitions(cmd *cobra.Command, definitions []*pipelineapi.PipelineDefinitionInfo) {
	cmd.Println("Pipeline Definitions:")
	for _, def := range definitions {
		cmd.Printf("\nID: %s\n", def.ID)
		cmd.Printf("Name: %s\n", def.Name)
		if def.Description != "" {
			cmd.Printf("Description: %s\n", def.Description)
		}
		cmd.Printf("Config Source: %s (%s)\n", def.ConfigSource.Repo.FullName, def.ConfigSource.FilePath)
		cmd.Printf("Checkout Source: %s\n", def.CheckoutSource.Repo.FullName)
	}
}

func isProjectSlug(value string) bool {
	return len(value) > 0 && containsRune(value, '/')
}

func containsRune(s string, needle rune) bool {
	for _, r := range s {
		if r == needle {
			return true
		}
	}
	return false
}
