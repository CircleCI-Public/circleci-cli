package pipeline

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
)

func newListCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <project-id>",
		Short: "List pipeline definitions for a project.",
		Long: `List pipeline definitions for a project.

The project ID can be found in your project settings or in the URL of your project page.

Examples:
  circleci pipeline list 12345678-1234-1234-1234-123456789012`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]

			definitions, err := ops.pipelineClient.ListPipelineDefinitions(projectID)
			if err != nil {
				return err
			}

			if len(definitions) == 0 {
				cmd.Println("No pipeline definitions found for this project.")
				return nil
			}

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

			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	return cmd
}
