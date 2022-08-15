package project

import (
	projectapi "github.com/CircleCI-Public/circleci-cli/api/project"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

type projectOpts struct {
	client projectapi.ProjectClient
}

// NewProjectCommand generates a cobra command for managing projects
func NewProjectCommand(config *settings.Config, preRunE validator.Validator) *cobra.Command {
	var opts projectOpts
	command := &cobra.Command{
		Use:   "project",
		Short: "Operate on projects",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			client, err := projectapi.NewProjectRestClient(*config)
			if err != nil {
				return err
			}
			opts.client = client
			return nil
		},
	}

	command.AddCommand(newProjectEnvironmentVariableCommand(&opts, preRunE))

	return command
}
