package pipeline

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
)

func newCreateCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	var name string
	var description string
	var repoID string
	var filePath string
	var configRepoID string

	cmd := &cobra.Command{
		Use:   "create <project-id> [--name <pipeline-name>] [--description <description>] [--repo-id <github-repo-id>] [--file-path <circleci-config-file-path>] [--config-repo-id <github-repo-id>]",
		Short: "Create a new pipeline for a CircleCI project.",
		Long: `Create a new pipeline for a CircleCI project.

All flags are optional - if not provided, you will be prompted interactively for the required values:
  --name            Name of the pipeline
  --repo-id         GitHub repository ID where the codebase you wish to build a pipeline for
  --file-path       Path to the CircleCI config file
  --config-repo-id  GitHub repository ID where the CircleCI config file is located (if different from code repository)
  --description     Description of the pipeline (will not prompt if omitted)

Examples:
  # Minimal usage (will prompt for required values):
  circleci pipeline create 1662d941-6e28-43ab-bea2-aa678c098d4c
  
  # Full usage with all flags:
  circleci pipeline create 1662d941-6e28-43ab-bea2-aa678c098d4c --name my-pipeline --description "My pipeline description" --repo-id 123456 --file-path .circleci/config.yml --config-repo-id 987654

Note: You will need our GitHub App installed in your repository.

Note: To get the repository id you can use https://docs.github.com/en/rest/repos/repos?apiVersion=2022-11-28#get-a-repository`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]

			if name == "" {
				namePrompt := "Enter a name for the pipeline"
				name = ops.reader.ReadStringFromUser(namePrompt)
			}

			if repoID == "" {
				repoPrompt := "Enter the ID of your github repository"
				repoID = ops.reader.ReadStringFromUser(repoPrompt)
			}

			if configRepoID == "" {
				yOrN := promptTillYOrN(ops.reader)
				if yOrN == "y" {
					configRepoIDPrompt := "Enter the ID of the GitHub repository where the CircleCI config file is located"
					configRepoID = ops.reader.ReadStringFromUser(configRepoIDPrompt)
				} else {
					configRepoID = repoID
				}
			}

			if filePath == "" {
				filePathPrompt := "Enter the path to your circleci config file"
				filePath = ops.reader.ReadStringFromUser(filePathPrompt)
			}
			res, err := ops.pipelineClient.CreatePipeline(projectID, name, description, repoID, configRepoID, filePath)
			if err != nil {
				return err
			}

			cmd.Printf("Pipeline '%s' successfully created for repository '%s'\n", res.Name, res.CheckoutSourceRepoFullName)
			if res.CheckoutSourceRepoFullName != res.ConfigSourceRepoFullName {
				cmd.Printf("Config is successfully referenced from '%s' repository at path '%s'\n", res.ConfigSourceRepoFullName, filePath)
			}
			cmd.Println("You may view your new pipeline in your project settings: https://app.circleci.com/settings/project/<vcs>/<org>/<project>/configurations")
			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the pipeline to create")
	cmd.Flags().StringVar(&description, "description", "", "Description of the pipeline to create")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "Repository ID of the codebase you wish to build a pipeline for")
	cmd.Flags().StringVar(&filePath, "file-path", "", "Path to the circleci config file to create")
	cmd.Flags().StringVar(&configRepoID, "config-repo-id", "", "Repository ID of the CircleCI config file")
	return cmd
}

func promptTillYOrN(reader UserInputReader) string {
	for {
		input := reader.ReadStringFromUser("Does your CircleCI config file exist in a different repository? (y/n)")
		if input == "y" || input == "n" {
			return input
		}
		fmt.Println("Invalid input. Please enter 'y' or 'n'.")
	}
}
