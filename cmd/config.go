package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/filetree"
	"github.com/CircleCI-Public/circleci-cli/local"
	"github.com/CircleCI-Public/circleci-cli/pipeline"
	"github.com/CircleCI-Public/circleci-cli/proxy"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

type configOptions struct {
	cfg  *settings.Config
	cl   *graphql.Client
	args []string
}

// Path to the config.yml file to operate on.
// Used to for compatibility with `circleci config validate --path`
var configPath string

var configAnnotations = map[string]string{
	"<path>": "The path to your config (use \"-\" for STDIN)",
}

func newConfigCommand(config *settings.Config) *cobra.Command {
	opts := configOptions{
		cfg: config,
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Operate on build config files",
	}

	packCommand := &cobra.Command{
		Use:   "pack <path>",
		Short: "Pack up your CircleCI configuration into a single file.",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return packConfig(opts)
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	packCommand.Annotations["<path>"] = configAnnotations["<path>"]

	validateCommand := &cobra.Command{
		Use:     "validate <path>",
		Aliases: []string{"check"},
		Short:   "Check that the config file is well formed.",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.cl = graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return validateConfig(opts, cmd.Flags())
		},
		Args:        cobra.MaximumNArgs(1),
		Annotations: make(map[string]string),
	}
	validateCommand.Annotations["<path>"] = configAnnotations["<path>"]
	validateCommand.PersistentFlags().StringVarP(&configPath, "config", "c", ".circleci/config.yml", "path to config file")
	if err := validateCommand.PersistentFlags().MarkHidden("config"); err != nil {
		panic(err)
	}
	validateCommand.Flags().StringP("org-slug", "o", "", "organization slug (for example: github/example-org), used when a config depends on private orbs belonging to that org")

	processCommand := &cobra.Command{
		Use:   "process <path>",
		Short: "Validate config and display expanded configuration.",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.cl = graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return processConfig(opts, cmd.Flags())
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	processCommand.Annotations["<path>"] = configAnnotations["<path>"]
	processCommand.Flags().StringP("org-slug", "o", "", "organization slug (for example: github/example-org), used when a config depends on private orbs belonging to that org")
	processCommand.Flags().StringP("pipeline-parameters", "", "", "JSON map of pipeline parameters. Use @filename.json to read from a file.")

	migrateCommand := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate a pre-release 2.0 config to the official release version",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return migrateConfig(opts)
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

// The <path> arg is actually optional, in order to support compatibility with the --path flag.
func validateConfig(opts configOptions, flags *pflag.FlagSet) error {
	path := local.DefaultConfigPath
	// First, set the path to configPath set by --path flag for compatibility
	if configPath != "" {
		path = configPath
	}

	// Then, if an arg is passed in, choose that instead
	if len(opts.args) == 1 {
		path = opts.args[0]
	}

	orgSlug, _ := flags.GetString("org-slug")

	_, err := api.ConfigQuery(opts.cl, path, orgSlug, pipeline.LocalPipelineVars(map[string]string{}))

	if err != nil {
		return err
	}

	if path == "-" {
		fmt.Printf("Config input is valid.\n")
	} else {
		fmt.Printf("Config file at %s is valid.\n", path)
	}

	return nil
}

func processConfig(opts configOptions, flags *pflag.FlagSet) error {
	orgSlug, _ := flags.GetString("org-slug")
	paramsJson, _ := flags.GetString("pipeline-parameters")

	var params map[string]string

	if len(paramsJson) > 0 {
		if paramsJson[0] == '@' {
			data, fileErr := ioutil.ReadFile(paramsJson[1:])
			if fileErr != nil {
				return fileErr
			}
			jsonErr := json.Unmarshal([]byte(data), &params)

			if jsonErr != nil {
				return jsonErr
			}
		} else {
			jsonErr := json.Unmarshal([]byte(paramsJson), &params)

			if jsonErr != nil {
				return jsonErr
			}
		}
	}

	response, err := api.ConfigQuery(opts.cl, opts.args[0], orgSlug, pipeline.LocalPipelineVars(params))

	if err != nil {
		return err
	}

	fmt.Print(response.OutputYaml)
	return nil
}

func packConfig(opts configOptions) error {
	tree, err := filetree.NewTree(opts.args[0])
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

func migrateConfig(opts configOptions) error {
	return proxy.Exec([]string{"config", "migrate"}, opts.args)
}
