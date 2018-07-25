package cmd

import (
	"context"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/spf13/cobra"
)

const defaultConfigPath = ".circleci/config.yml"

func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Operate on build config files",
	}

	validateCommand := &cobra.Command{
		Use:     "validate [config.yml]",
		Aliases: []string{"check"},
		Short:   "Check that the config file is well formed.",
		RunE:    validateConfig,
		Args:    cobra.MaximumNArgs(1),
	}

	expandCommand := &cobra.Command{
		Use:   "expand [config.yml]",
		Short: "Expand the config.",
		RunE:  expandConfig,
		Args:  cobra.MaximumNArgs(1),
	}

	configCmd.AddCommand(validateCommand)
	configCmd.AddCommand(expandCommand)

	return configCmd
}

func validateConfig(cmd *cobra.Command, args []string) error {

	configPath := defaultConfigPath
	if len(args) == 1 {
		configPath = args[0]
	}
	ctx := context.Background()
	response, err := api.ConfigQuery(ctx, Logger, configPath)

	if err != nil {
		return err
	}

	if !response.Valid {
		return response.ToError()
	}

	Logger.Infof("Config file at %s is valid", configPath)
	return nil
}

func expandConfig(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	configPath := defaultConfigPath
	if len(args) == 1 {
		configPath = args[0]
	}
	response, err := api.ConfigQuery(ctx, Logger, configPath)

	if err != nil {
		return err
	}

	if !response.Valid {
		return response.ToError()
	}

	Logger.Info(response.OutputYaml)
	return nil
}
