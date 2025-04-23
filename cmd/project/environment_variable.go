package project

import (
	"encoding/json"
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

	jsonFormat := false

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

	listVarsCommand.PersistentFlags().BoolVar(&jsonFormat, "json", false,
		"Return output back in JSON format")
	createVarCommand.PersistentFlags().BoolVar(&jsonFormat, "json", false,
		"Return output back in JSON format")

	cmd.AddCommand(listVarsCommand)
	cmd.AddCommand(createVarCommand)
	return cmd
}

func listProjectEnvironmentVariables(cmd *cobra.Command, client projectapi.ProjectClient, vcsType, orgName, projName string) error {
	envVars, err := client.ListAllEnvironmentVariables(vcsType, orgName, projName)
	if err != nil {
		return err
	}

	jsonVal, err := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}

	if jsonVal {
		// return JSON formatted for output
		jsonEnvVars, err := json.Marshal(envVars)
		if err != nil {
			return err
		}
		jsonWriter := cmd.OutOrStdout()
		if _, err := jsonWriter.Write(jsonEnvVars); err != nil {
			return err
		}
	} else {
		table := tablewriter.NewWriter(cmd.OutOrStdout())

		table.SetHeader([]string{"Environment Variable", "Value"})

		for _, envVar := range envVars {
			table.Append([]string{envVar.Name, envVar.Value})
		}
		table.Render()
	}

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

	jsonVal, err := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}

	if jsonVal {
		// return JSON formatted for output
		jsonV, err := json.Marshal(v)
		if err != nil {
			return err
		}
		jsonWriter := cmd.OutOrStdout()
		if _, err := jsonWriter.Write(jsonV); err != nil {
			return err
		}
	} else {
		table := tablewriter.NewWriter(cmd.OutOrStdout())

		table.SetHeader([]string{"Environment Variable", "Value"})
		table.Append([]string{v.Name, v.Value})
		table.Render()
	}

	return nil
}
