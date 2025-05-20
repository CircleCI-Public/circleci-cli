package pipeline

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
)

func newCreateCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	var name string
	var description string
	var repoID string
	var filePath string

	cmd := &cobra.Command{
		Use:   "create <project-id> [--name <pipeline-name>] [--description <description>] [--repo-id <github-repo-id>] [--file-path <circleci-config-file-path>]",
		Short: "Create a new pipeline in a CircleCI project.",
		Long: `Create a new pipeline in a CircleCI project.

This command allows you to create a pipeline with the following options:
  --name        Name of the pipeline
  --repo-id     GitHub repository ID where the pipeline is configured
  --file-path   Path to the CircleCI config file
  --description Description of the pipeline (optional, you will not be prompted if this flag doesn't exist)

If flags are not provided, the command will prompt for input interactively.

Example:
  circleci pipeline create 1662d941-6e28-43ab-bea2-aa678c098d4c --name my-pipeline --description "My pipeline description" --repo-id 123456 --file-path .circleci/config.yml

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

			if filePath == "" {
				filePathPrompt := "Enter the path to your circleci config file"
				filePath = ops.reader.ReadStringFromUser(filePathPrompt)
			}

			res, err := ops.pipelineClient.CreatePipeline(projectID, name, description, repoID, filePath)
			if err != nil {
				return err
			}

			cmd.Printf("Pipeline '%s' successfully created for repository '%s'\n", res.Name, res.RepoFullName)
			cmd.Println("You may view your new pipeline in your project settings: https://app.circleci.com/settings/project/<vcs>/<org>/<project>/configurations")
			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the pipeline to create")
	cmd.Flags().StringVar(&description, "description", "", "Description of the pipeline to create")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "Repository ID of the pipeline to create")
	cmd.Flags().StringVar(&filePath, "file-path", "", "Path to the circleci config file to create")

	return cmd
}
