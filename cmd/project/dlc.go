package project

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/dl"
	projectapi "github.com/CircleCI-Public/circleci-cli/api/project"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

func newProjectDLCCommand(config *settings.Config, ops *projectOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dlc",
		Short: "Manage dlc for projects",
	}

	purgeCommand := &cobra.Command{
		Short:   "Purge DLC for a project",
		Use:     "purge <vcs-type> <org-name> <project-name>",
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			dlClient, err := dl.NewDlRestClient(*config)
			if err != nil {
				if dl.IsCloudOnlyErr(err) {
					cmd.SilenceUsage = true
				}
				return err
			}

			return dlcPurge(cmd, ops.projectClient, dlClient, args[0], args[1], args[2])
		},
		Args: cobra.ExactArgs(3),
	}

	cmd.AddCommand(purgeCommand)
	return cmd
}

func dlcPurge(cmd *cobra.Command, projClient projectapi.ProjectClient, dlClient dl.DlClient, vcsType, orgName, projName string) error {
	// first we need to work out the project id
	projectInfo, err := projClient.ProjectInfo(vcsType, orgName, projName)
	if err != nil {
		return err
	}
	projectId := projectInfo.Id

	// now we issue the purge request
	err = dlClient.PurgeDLC(projectId)
	if err != nil {
		if dl.IsGoneErr(err) {
			cmd.SilenceUsage = true
		}
		return err
	}

	cmd.Println("Purged DLC for project")
	return nil
}
