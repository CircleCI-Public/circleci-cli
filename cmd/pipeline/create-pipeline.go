package pipeline

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
)

func newPipelineCreateCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <project-id>",
		Short: "Create a new pipeline in a CircleCI project.",
		Long: `Create a new pipeline in a CircleCI project.

Example:
  circleci pipeline create 1662d941-6e28-43ab-bea2-aa678c098d4c

Note: You will need our Github App installed in your repository.

Note: To get the repository id you can use https://docs.github.com/en/rest/repos/repos?apiVersion=2022-11-28#get-a-repository`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]

			// Use custom reader to get pipeline inputs
			namePrompt := "Enter a name for the pipeline"
			name, err := ops.reader.ReadSecretString(namePrompt)
			if err != nil {
				return err
			}

			descPrompt := "Enter a description for the pipeline (optional)"
			description, err := ops.reader.ReadSecretString(descPrompt)
			if err != nil {
				return err
			}

			repoPrompt := "Enter the ID of your github repository"
			repoID, err := ops.reader.ReadSecretString(repoPrompt)
			if err != nil {
				return err
			}

			filePathPrompt := "Enter the path to your circleci config file"
			filePath, err := ops.reader.ReadSecretString(filePathPrompt)
			if err != nil {
				// Default to .circleci/config.yml if no input
				filePath = ".circleci/config.yml"
			}

			res, err := ops.pipelineClient.CreatePipeline(projectID, name, description, repoID, filePath)
			if err != nil {
				return err
			}

			cmd.Printf("Pipeline '%s' successfully created in repository '%s'\n", res.Name, res.RepoFullName)
			cmd.Println("You may view your new pipeline at: https://app.circleci.com/projects/" + projectID + "/pipelines")
			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	return cmd
}
