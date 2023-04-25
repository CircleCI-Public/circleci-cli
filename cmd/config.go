package cmd

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/config"
	"github.com/CircleCI-Public/circleci-cli/filetree"
	"github.com/CircleCI-Public/circleci-cli/proxy"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Path to the config.yml file to operate on.
// Used to for compatibility with `circleci config validate --path`
var configPath string
var ignoreDeprecatedImages bool // should we ignore deprecated images warning
var verboseOutput bool          // Enable extra debugging output

var configAnnotations = map[string]string{
	"<path>": "The path to your config (use \"-\" for STDIN)",
}

func newConfigCommand(globalConfig *settings.Config) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Operate on build config files",
	}

	packCommand := &cobra.Command{
		Use:   "pack <path>",
		Short: "Pack up your CircleCI configuration into a single file.",
		RunE: func(_ *cobra.Command, args []string) error {
			return packConfig(args)
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	packCommand.Annotations["<path>"] = configAnnotations["<path>"]

	validateCommand := &cobra.Command{
		Use:     "validate <path>",
		Aliases: []string{"check"},
		Short:   "Check that the config file is well formed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			compiler := config.New(globalConfig)
			orgID, _ := cmd.Flags().GetString("org-id")
			orgSlug, _ := cmd.Flags().GetString("org-slug")
			path := config.DefaultConfigPath
			if configPath != "" {
				path = configPath
			}
			if len(args) == 1 {
				path = args[0]
			}
			return compiler.ValidateConfig(config.ValidateConfigOpts{
				ConfigPath:             path,
				OrgID:                  orgID,
				OrgSlug:                orgSlug,
				IgnoreDeprecatedImages: ignoreDeprecatedImages,
				VerboseOutput:          verboseOutput,
			})
		},
		Args:        cobra.MaximumNArgs(1),
		Annotations: make(map[string]string),
	}
	validateCommand.Annotations["<path>"] = configAnnotations["<path>"]
	validateCommand.PersistentFlags().StringVarP(&configPath, "config", "c", ".circleci/config.yml", "path to config file")
	validateCommand.PersistentFlags().BoolVarP(&verboseOutput, "verbose", "v", false, "Enable verbose output")
	validateCommand.PersistentFlags().BoolVar(&ignoreDeprecatedImages, "ignore-deprecated-images", false, "ignores the deprecated images error")

	if err := validateCommand.PersistentFlags().MarkHidden("config"); err != nil {
		panic(err)
	}
	validateCommand.Flags().StringP("org-slug", "o", "", "organization slug (for example: github/example-org), used when a config depends on private orbs belonging to that org")
	validateCommand.Flags().String("org-id", "", "organization id used when a config depends on private orbs belonging to that org")

	processCommand := &cobra.Command{
		Use:   "process <path>",
		Short: "Validate config and display expanded configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			compiler := config.New(globalConfig)
			pipelineParamsFilePath, _ := cmd.Flags().GetString("pipeline-parameters")
			orgID, _ := cmd.Flags().GetString("org-id")
			orgSlug, _ := cmd.Flags().GetString("org-slug")
			path := config.DefaultConfigPath
			if configPath != "" {
				path = configPath
			}
			if len(args) == 1 {
				path = args[0]
			}
			return compiler.ProcessConfig(config.ProcessConfigOpts{
				ConfigPath:             path,
				OrgID:                  orgID,
				OrgSlug:                orgSlug,
				PipelineParamsFilePath: pipelineParamsFilePath,
				VerboseOutput:          verboseOutput,
			})
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	processCommand.Annotations["<path>"] = configAnnotations["<path>"]
	processCommand.Flags().StringP("org-slug", "o", "", "organization slug (for example: github/example-org), used when a config depends on private orbs belonging to that org")
	processCommand.Flags().String("org-id", "", "organization id used when a config depends on private orbs belonging to that org")
	processCommand.Flags().StringP("pipeline-parameters", "", "", "YAML/JSON map of pipeline parameters, accepts either YAML/JSON directly or file path (for example: my-params.yml)")
	processCommand.PersistentFlags().BoolVar(&verboseOutput, "verbose", false, "adds verbose output to the command")

	migrateCommand := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate a pre-release 2.0 config to the official release version",
		RunE: func(_ *cobra.Command, args []string) error {
			return migrateConfig(args)
		},
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

func packConfig(args []string) error {
	tree, err := filetree.NewTree(args[0])
	if err != nil {
		return errors.Wrap(err, "An error occurred trying to build the tree")
	}

	y, err := yaml.Marshal(&tree)
	if err != nil {
		return errors.Wrap(err, "Failed trying to marshal the tree to YAML ")
	}
	fmt.Printf("%s\n", string(y))
	return nil
}

func migrateConfig(args []string) error {
	return proxy.Exec([]string{"config", "migrate"}, args)
}
