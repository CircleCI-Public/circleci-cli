package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/config"
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

var (
	CollaborationsPath = "me/collaborations"
)

type configOptions struct {
	cfg  *settings.Config
	rest *rest.Client
	args []string
}

// Path to the config.yml file to operate on.
// Used to for compatibility with `circleci config validate --path`
var configPath string
var ignoreDeprecatedImages bool // should we ignore deprecated images warning
var verboseOutput bool          // Enable extra debugging output

var configAnnotations = map[string]string{
	"<path>": "The path to your config (use \"-\" for STDIN)",
}

func GetConfigAPIHost(cfg *settings.Config) string {
	if cfg.Host != defaultHost {
		return cfg.Host
	} else {
		return cfg.ConfigAPIHost
	}
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
			opts.rest = rest.NewFromConfig(GetConfigAPIHost(opts.cfg), config)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return validateConfig(opts, cmd.Flags())
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
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.rest = rest.NewFromConfig(GetConfigAPIHost(opts.cfg), config)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return processConfig(opts, cmd.Flags())
		},
		Args:        cobra.ExactArgs(1),
		Annotations: make(map[string]string),
	}
	processCommand.Annotations["<path>"] = configAnnotations["<path>"]
	processCommand.Flags().StringP("org-slug", "o", "", "organization slug (for example: github/example-org), used when a config depends on private orbs belonging to that org")
	processCommand.Flags().String("org-id", "", "organization id used when a config depends on private orbs belonging to that org")
	processCommand.Flags().StringP("pipeline-parameters", "", "", "YAML/JSON map of pipeline parameters, accepts either YAML/JSON directly or file path (for example: my-params.yml)")
	processCommand.Flags().StringP("circleci-api-host", "", "", "the api-host you want to use for config processing and validation")

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
	var err error
	var response *config.ConfigResponse
	path := local.DefaultConfigPath
	// First, set the path to configPath set by --path flag for compatibility
	if configPath != "" {
		path = configPath
	}

	// Then, if an arg is passed in, choose that instead
	if len(opts.args) == 1 {
		path = opts.args[0]
	}

	//if no orgId provided use org slug
	values := pipeline.LocalPipelineValues()
	if verboseOutput {
		printValues(values)
	}

	var orgID string
	orgID, _ = flags.GetString("org-id")
	if strings.TrimSpace(orgID) != "" {
		response, err = config.ConfigQuery(opts.rest, path, orgID, nil, pipeline.LocalPipelineValues())
		if err != nil {
			return err
		}
	} else {
		orgSlug, _ := flags.GetString("org-slug")
		orgs, err := GetOrgCollaborations(opts.cfg)
		if err != nil {
			fmt.Println(err.Error())
		}
		orgID = GetOrgIdFromSlug(orgSlug, orgs)
		response, err = config.ConfigQuery(opts.rest, path, orgID, nil, pipeline.LocalPipelineValues())
		if err != nil {
			return err
		}
	}

	// check if a deprecated Linux VM image is being used
	// link here to blog post when available
	// returns an error if a deprecated image is used
	if !ignoreDeprecatedImages {
		err := config.DeprecatedImageCheck(response)
		if err != nil {
			return err
		}
	}

	if path == "-" {
		fmt.Printf("\nConfig input is valid.\n")
	} else {
		fmt.Printf("\nConfig file at %s is valid.\n", path)
	}

	return nil
}

func processConfig(opts configOptions, flags *pflag.FlagSet) error {
	paramsYaml, _ := flags.GetString("pipeline-parameters")
	var response *config.ConfigResponse
	var params pipeline.Parameters
	var err error

	if len(paramsYaml) > 0 {
		// The 'src' value can be a filepath, or a yaml string. If the file cannot be read successfully,
		// proceed with the assumption that the value is already valid yaml.
		raw, err := os.ReadFile(paramsYaml)
		if err != nil {
			raw = []byte(paramsYaml)
		}

		err = yaml.Unmarshal(raw, &params)
		if err != nil {
			return fmt.Errorf("invalid 'pipeline-parameters' provided: %s", err.Error())
		}
	}

	//if no orgId provided use org slug
	values := pipeline.LocalPipelineValues()
	printValues(values)

	orgID, _ := flags.GetString("org-id")
	if strings.TrimSpace(orgID) != "" {
		response, err = config.ConfigQuery(opts.rest, opts.args[0], orgID, params, values)
		if err != nil {
			return err
		}
	} else {
		orgSlug, _ := flags.GetString("org-slug")
		orgs, err := GetOrgCollaborations(opts.cfg)
		if err != nil {
			fmt.Println(err.Error())
		}
		orgID = GetOrgIdFromSlug(orgSlug, orgs)
		response, err = config.ConfigQuery(opts.rest, opts.args[0], orgID, params, values)
		if err != nil {
			return err
		}
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

func printValues(values pipeline.Values) {
	fmt.Fprintln(os.Stderr, "Processing config with following values:")
	for key, value := range values {
		fmt.Fprintf(os.Stderr, "%-18s %s\n", key+":", value)
	}
}

type CollaborationResult struct {
	VcsTye    string `json:"vcs_type"`
	OrgSlug   string `json:"slug"`
	OrgName   string `json:"name"`
	OrgId     string `json:"id"`
	AvatarUrl string `json:"avatar_url"`
}

// GetOrgCollaborations - fetches all the collaborations for a given user.
func GetOrgCollaborations(cfg *settings.Config) ([]CollaborationResult, error) {
	baseClient := rest.NewFromConfig(cfg.Host, cfg)
	req, err := baseClient.NewRequest("GET", &url.URL{Path: CollaborationsPath}, nil)
	if err != nil {
		return nil, err
	}

	var resp []CollaborationResult
	_, err = baseClient.DoRequest(req, &resp)
	return resp, err
}

// GetOrgIdFromSlug - converts a slug into an orgID.
func GetOrgIdFromSlug(slug string, collaborations []CollaborationResult) string {
	for _, v := range collaborations {
		if v.OrgSlug == slug {
			return v.OrgId
		}
	}
	return ""
}
