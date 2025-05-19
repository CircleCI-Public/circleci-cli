package project

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/prompt"
)

var projectName string

func newProjectCreateCommand(ops *projectOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <vcs-type> <orgSlug> [--name <project-name>]",
		Short: "Create a new project in a CircleCI organization.",
		Long: `Create a new project in a CircleCI organization.

The project name can be provided using the --name flag. If not provided, you will be prompted to enter it.

Examples:
  circleci project create github orgSlug --name my-new-project
  circleci project create circleci orgSlug --name my-new-project

The orgSlug can be retrieved from the CircleCI web app > Organization Settings > Organization slug.

Example orgSlug: 
  in the CircleCI web app, circleci/9YytKzouJxzu4TjCRFqAoD -> 9YytKzouJxzu4TjCRFqAoD is the organization slug`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			vcsType := args[0]
			orgName := args[1]
			if projectName == "" {
				projectName = prompt.ReadStringFromUser("Enter a name for the project", "")
			}
			res, err := ops.projectClient.CreateProject(vcsType, orgName, projectName)
			if err != nil {
				return err
			}

			cmd.Printf("Project '%s' successfully created in organization '%s'\n", projectName, res.OrgName)
			cmd.Println("You may view your new project at: https://app.circleci.com/projects/" + res.Slug)
			return nil
		},
		Args: cobra.ExactArgs(2),
	}

	cmd.Flags().StringVar(&projectName, "name", "", "Name of the project to create")

	return cmd
}
