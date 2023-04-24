package project

import (
	"fmt"
	"strings"

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
			return listProjectEnvironmentVariables(cmd, ops.projectClient, args[0], args[1], args[2])
		},
		Args: cobra.ExactArgs(3),
	}

	var envValue string
	createVarCommand := &cobra.Command{
		Short:   "Create an environment variable of a project. The value is read from stdin.",
		Use:     "create <vcs-type> <org-name> <project-name> <env-name>",
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createProjectEnvironmentVariable(cmd, ops.projectClient, ops.reader, args[0], args[1], args[2], args[3], envValue)
		},
		Args: cobra.ExactArgs(4),
	}

	createVarCommand.Flags().StringVar(&envValue, "env-value", "", "An environment variable value to be created. You can also pass it by stdin without this option.")

	cmd.AddCommand(listVarsCommand)
	cmd.AddCommand(createVarCommand)
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

func createProjectEnvironmentVariable(cmd *cobra.Command, client projectapi.ProjectClient, r UserInputReader, vcsType, orgName, projName, name, value string) error {
	if value == "" {
		val, err := r.ReadSecretString("Enter an environment variable value and press enter")
		if err != nil {
			return err
		}
		if val == "" {
			return fmt.Errorf("the environment variable value must not be empty")
		}
		value = val
	}
	value = strings.Trim(value, "\r\n")

	existV, err := client.GetEnvironmentVariable(vcsType, orgName, projName, name)
	if err != nil {
		return err
	}
	if existV != nil {
		msg := fmt.Sprintf("The environment variable name=%s value=%s already exists. Do you overwrite it?", existV.Name, existV.Value)
		if !r.AskConfirm(msg) {
			fmt.Fprintln(cmd.OutOrStdout(), "Canceled")
			return nil
		}
	}

	v, err := client.CreateEnvironmentVariable(vcsType, orgName, projName, projectapi.ProjectEnvironmentVariable{
		Name:  name,
		Value: value,
	})
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(cmd.OutOrStdout())

	table.SetHeader([]string{"Environment Variable", "Value"})
	table.Append([]string{v.Name, v.Value})
	table.Render()

	return nil
}
