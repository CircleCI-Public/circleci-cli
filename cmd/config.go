package cmd

import (
	"fmt"
	"os"

	"github.com/CircleCI-Public/circleci-config/generation"
	"github.com/CircleCI-Public/circleci-config/labeling"
	"github.com/CircleCI-Public/circleci-config/labeling/codebase"

	"github.com/CircleCI-Public/circleci-cli/cmd/create_telemetry"
	"github.com/CircleCI-Public/circleci-cli/config"
	"github.com/CircleCI-Public/circleci-cli/filetree"
	"github.com/CircleCI-Public/circleci-cli/proxy"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
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
	var telemetryClient telemetry.Client

	closeTelemetryClient := func() {
		if telemetryClient != nil {
			telemetryClient.Close()
			telemetryClient = nil
		}
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Operate on build config files",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			telemetryClient = create_telemetry.CreateTelemetry(globalConfig)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			closeTelemetryClient()
		},
	}

	packCommand := &cobra.Command{
		Use:   "pack <path>",
		Short: "Pack up your CircleCI configuration into a single file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			defer closeTelemetryClient()
			err := packConfig(args)
			telemetryClient.Track(telemetry.CreateConfigEvent(create_telemetry.GetCommandInformation(cmd, true)))
			return err
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
			defer closeTelemetryClient()

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

			err := compiler.ValidateConfig(config.ValidateConfigOpts{
				ConfigPath:             path,
				OrgID:                  orgID,
				OrgSlug:                orgSlug,
				IgnoreDeprecatedImages: ignoreDeprecatedImages,
				VerboseOutput:          verboseOutput,
			})
			telemetryClient.Track(telemetry.CreateConfigEvent(create_telemetry.GetCommandInformation(cmd, true)))

			return err
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
			defer closeTelemetryClient()

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
			telemetryClient.Track(telemetry.CreateConfigEvent(getCommandInformation(cmd, true)))
			response, err := compiler.ProcessConfig(config.ProcessConfigOpts{
				ConfigPath:             path,
				OrgID:                  orgID,
				OrgSlug:                orgSlug,
				PipelineParamsFilePath: pipelineParamsFilePath,
				VerboseOutput:          verboseOutput,
			})
			telemetryClient.Track(telemetry.CreateConfigEvent(create_telemetry.GetCommandInformation(cmd, true)))
			if err != nil {
				return err
			}
			fmt.Print(response.OutputYaml)
			return nil
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

	generateCommand := &cobra.Command{
		Use:   "generate <path>",
		Short: "Generate a config by analyzing your repository contents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateConfig(args)
		},
	}

	configCmd.AddCommand(packCommand)
	configCmd.AddCommand(validateCommand)
	configCmd.AddCommand(processCommand)
	configCmd.AddCommand(migrateCommand)
	configCmd.AddCommand(generateCommand)

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

func generateConfig(args []string) error {
	var err error
	var path string
	if len(args) == 0 {
		// use working directory as default
		path, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("couldn't get working directory")
		}
	} else {
		path = args[0]
	}

	stat, err := os.Stat(path)

	if os.IsNotExist(err) || !stat.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	if err != nil {
		return fmt.Errorf("error reading from %s: %v", path, err)
	}

	cb := codebase.LocalCodebase{BasePath: path}
	labels := labeling.ApplyAllRules(cb)
	generatedConfig := generation.GenerateConfig(labels)

	fmt.Print(generatedConfig.String())

	return nil
}
