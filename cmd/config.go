package cmd

import (
	"context"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/filetree"
	"github.com/CircleCI-Public/circleci-cli/proxy"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

const defaultConfigPath = ".circleci/config.yml"

// Path to the config.yml file to operate on.
// Used to for compatibility with `circleci config validate --path`
var configPath string

var configAnnotations = map[string]string{
	"<path>": "The path to your config (use \"-\" for STDIN)",
}

func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Operate on build config files",
	}

	packCommand := &cobra.Command{
		Use:         "pack <path>",
		Short:       "Pack up your CircleCI configuration into a single file.",
		RunE:        packConfig,
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	packCommand.Annotations["<path>"] = configAnnotations["<path>"]

	validateCommand := &cobra.Command{
		Use:         "validate <path>",
		Aliases:     []string{"check"},
		Short:       "Check that the config file is well formed.",
		RunE:        validateConfig,
		Args:        cobra.MaximumNArgs(1),
		Annotations: make(map[string]string),
	}
	validateCommand.Annotations["<path>"] = configAnnotations["<path>"]
	validateCommand.PersistentFlags().StringVarP(&configPath, "config", "c", ".circleci/config.yml", "path to config file")
	if err := validateCommand.PersistentFlags().MarkHidden("config"); err != nil {
		panic(err)
	}

	processCommand := &cobra.Command{
		Use:         "process <path>",
		Short:       "Process the config.",
		RunE:        processConfig,
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	processCommand.Annotations["<path>"] = configAnnotations["<path>"]

	migrateCommand := &cobra.Command{
		Use:                "migrate",
		Short:              "Migrate a pre-release 2.0 config to the official release version",
		RunE:               migrateConfig,
		Hidden:             true,
		DisableFlagParsing: true,
	}
	// These flags are for documentation and not actually parsed
	migrateCommand.PersistentFlags().StringP("config", "c", ".circleci/config.yml", "path to config file")
	migrateCommand.PersistentFlags().BoolP("in-place", "i", false, "whether to update file in place.  If false, emits to stdout")

	configCmd.AddCommand(packCommand)
	configCmd.AddCommand(validateCommand)
	configCmd.AddCommand(processCommand)
	configCmd.AddCommand(migrateCommand)

	return configCmd
}

// The <path> arg is actually optional, in order to support compatibility with the --path flag.
func validateConfig(cmd *cobra.Command, args []string) error {
	path := defaultConfigPath
	// First, set the path to configPath set by --path flag for compatibility
	if configPath != "" {
		path = configPath
	}

	// Then, if an arg is passed in, choose that instead
	if len(args) == 1 {
		path = args[0]
	}

	ctx := context.Background()
	_, err := api.ConfigQuery(ctx, Config, path)

	if err != nil {
		return err
	}

	Config.Logger.Infof("Config file at %s is valid", path)
	return nil
}

func processConfig(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	response, err := api.ConfigQuery(ctx, Config, args[0])

	if err != nil {
		return err
	}

	Config.Logger.Info(response.OutputYaml)
	return nil
}

func packConfig(cmd *cobra.Command, args []string) error {
	tree, err := filetree.NewTree(args[0])
	if err != nil {
		return errors.Wrap(err, "An error occurred trying to build the tree")
	}

	y, err := yaml.Marshal(&tree)
	if err != nil {
		return errors.Wrap(err, "Failed trying to marshal the tree to YAML ")
	}
	Config.Logger.Infof("%s\n", string(y))
	return nil
}

func migrateConfig(cmd *cobra.Command, args []string) error {
	return proxy.Exec([]string{"config", "migrate"}, args)
}
