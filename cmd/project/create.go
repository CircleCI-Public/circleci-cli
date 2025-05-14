package project

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
)

func newProjectCreateCommand(ops *projectOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <vcs-type> <org-name> <new-project-name>",
		Short: "Create a new project in a CircleCI organization.",
		Long: `Create a new project in a CircleCI organization.

Example:
  circleci project create github my-org my-new-project

Note: For those with the circleci vcs type, you must use the hashed organization name, not the human readable name.
You can get this from the URL of the organization on the web.
Example: 
  https://github.com/organization/circleci/GQERGDFG13454135 -> GQERGDFG13454135
  Not the org name like: "test-org"`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			vcsType := args[0]
			orgName := args[1]
			projectName := args[2]
			res, err := ops.projectClient.CreateProject(vcsType, orgName, projectName)
			if err != nil {
				return err
			}

			cmd.Printf("Project '%s' successfully created in organization '%s'\n", projectName, res.OrgName)
			cmd.Println("You may view your new project at: https://app.circleci.com/projects/" + res.Slug)
			return nil
		},
		Args: cobra.ExactArgs(3),
	}

	return cmd
}
