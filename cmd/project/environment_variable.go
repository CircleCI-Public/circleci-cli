package project

import (
	projectapi "github.com/CircleCI-Public/circleci-cli/api/project"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func newProjectEnvironmentVariableCommand(ops *projectOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Operate on environment variables of projects",
	}

	listVarsCommand := &cobra.Command{
		Short:   "List all environment variables of a project",
		Use:     "list <vcs-type> <org-name> <project-name>",
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listProjectEnvironmentVariables(cmd, ops.client, args[0], args[1], args[2])
		},
		Args: cobra.ExactArgs(3),
	}

	cmd.AddCommand(listVarsCommand)
	return cmd
}

func listProjectEnvironmentVariables(cmd *cobra.Command, client projectapi.ProjectClient, vcsType, orgName, projName string) error {
	envVars, err := client.ListAllEnvironmentVariables(vcsType, orgName, projName)
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(cmd.OutOrStdout())

	table.SetHeader([]string{"Environment Variable", "Value"})

	for _, envVar := range envVars {
		table.Append([]string{envVar.Name, envVar.Value})
	}
	table.Render()

	return nil
}
