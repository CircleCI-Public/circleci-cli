package pipeline

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"strings"

	"github.com/CircleCI-Public/circleci-cli/api/pipeline"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
)

func newRunCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	var orgSlug string
	var projectID string
	var pipelineDefinitionID string
	var configBranch string
	var configTag string
	var checkoutBranch string
	var checkoutTag string
	var localConfigFilePath string
	var parameters map[string]string
	var repoConfig bool

	cmd := &cobra.Command{
		Use:   "run <orgSlug> <project-id> --pipeline-definition-id <id> [options...]",
		Short: "Run a pipeline configuration. When a local config is supplied, it runs it via a ci pipeline in circleci's cloud.",
		Long: `Run a pipeline configuration. When a local config is supplied, it runs it via a ci pipeline in circleci's cloud.

Required arguments:
  orgSlug                 The second segment of the slash-separated project slug, as shown in 
                          Project Settings > Overview. For example, in circleci/6phtklsjdlskE/cLKSdlksdn
                          it would be 6phtklsjdlskE 													
  project-id              Project ID (e.g. 44n9wujWcTnVZ2b5S8Fnat). You can view it in Project Settings > Overview. 													
  --pipeline-definition-id The unique id for the pipeline definition. This can be found in the page
                           Project Settings > Pipelines.

Optional flags:
  # Note for config-* and checkout-*: Both required if running a pipeline run without a local config file.
	If you are using a local config file, you can omit the config-* flags.
  # Note for --local-config-file: If you want to run a pipeline with a local config file, you need to enable
	"Allow triggering pipelines with unversioned config" in Organization Settings > Advanced.
  --config-branch          Branch to use for config (mutually exclusive with --config-tag)
  --config-tag             Tag to use for config (mutually exclusive with --config-branch)
  --checkout-branch        Branch to checkout (mutually exclusive with --checkout-tag)
  --checkout-tag           Tag to checkout (mutually exclusive with --checkout-branch)
  --local-config-file      Path to a local config file to use
  --parameters             Pipeline parameters in key=value format (can be specified multiple times)
  --repo-config            Use repository config

Examples:
  # Minimal usage (will prompt for required values):
  circleci pipeline run circleci/6phtklsjdlskE/cLKSdlksdn 44n9wujWcTnVZ2b5S8Fnat

  # Full usage with all flags:
  circleci pipeline run 5e16180a-023b-4c3v-9bd9-43a8eb6cdb8f 44n9wujWcTnVZ2b5S8Fnat --pipeline-definition-id abc123 \
    --config-branch main --checkout-branch feature-branch --local-config-file .circleci/config.yml \ 
    "--parameters", "key1=value1", "--parameters", "key2=value2"
`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			orgSlug = args[0]
			projectID = args[1]

			if pipelineDefinitionID == "" {
				pipelineDefinitionIDPrompt := "Enter the pipeline definition ID for your pipeline"
				pipelineDefinitionID = ops.reader.ReadStringFromUser(pipelineDefinitionIDPrompt)
			}

			// Prompt logic for local config file
			useLocalConfig := false
			if (localConfigFilePath == "" && !repoConfig) || (localConfigFilePath != "" && repoConfig) {
				useLocalConfigPrompt := "Do you want to test run with a local config file?"
				if ops.reader.AskConfirm(useLocalConfigPrompt) {
					configFilePathPrompt := "Enter the path to your local config file"
					localConfigFilePath = ops.reader.ReadStringFromUser(configFilePathPrompt)
					useLocalConfig = true
				} else {
					localConfigFilePath = ""
				}
			} else if localConfigFilePath != "" {
				useLocalConfig = true
			}

			// If a config file path is supplied and confirmed, check if the file is empty
			if useLocalConfig && localConfigFilePath != "" {
				configBranch = "cli-run"
				data, err := os.ReadFile(localConfigFilePath)
				if err != nil {
					return fmt.Errorf("failed to read config file: %w", err)
				}
				if len(data) == 0 {
					cmd.Println("The supplied config file is empty. Please provide a valid CircleCI config file, and try again.")
					return fmt.Errorf("empty config file")
				}
			} else {
				// Only prompt for config branch/tag if no config file is supplied
				for configBranch == "" && configTag == "" {
					configBranch = ops.reader.ReadStringFromUser("You must specify either a config branch or tag. Enter a branch (or leave blank to enter a tag):")
					if configBranch == "" {
						configTag = ops.reader.ReadStringFromUser("Enter a config tag:")
					}
				}
			}

			// Always prompt for checkout branch/tag if neither is provided
			for checkoutBranch == "" && checkoutTag == "" {
				checkoutBranch = ops.reader.ReadStringFromUser("You must specify either a checkout branch or tag. Enter a branch (or leave blank to enter a tag):")
				if checkoutBranch == "" {
					checkoutTag = ops.reader.ReadStringFromUser("Enter a checkout tag:")
				}
			}

			// Convert string parameters to interface map
			paramMap := make(map[string]interface{})
			for k, v := range parameters {
				paramMap[k] = v
			}

			options := pipeline.PipelineRunOptions{
				Organization:         orgSlug,
				Project:              projectID,
				PipelineDefinitionID: pipelineDefinitionID,
				ConfigBranch:         configBranch,
				ConfigTag:            configTag,
				CheckoutBranch:       checkoutBranch,
				CheckoutTag:          checkoutTag,
				Parameters:           paramMap,
				ConfigFilePath:       localConfigFilePath,
			}

			resp, err := ops.pipelineClient.PipelineRun(options)
			if err != nil {
				cmd.Println("\nThere was an error running the config test")
				if strings.Contains(err.Error(), "Permission denied") {
					cmd.Printf("Please ensure you have \"Allow triggering pipelines with unversioned config\" enabled in Organization Settings > Advanced\n")
				}
				return err
			}

			if resp.Created != nil {
				cmd.Printf("Pipeline created successfully\n")
				cmd.Printf("Pipeline ID: %s\n", resp.Created.ID)
				cmd.Printf("Pipeline Number: %d\n", resp.Created.Number)
				cmd.Printf("State: %s\n", resp.Created.State)
				cmd.Printf("Created at: %s\n", resp.Created.CreatedAt)
				cmd.Printf("You may view your pipeline run on the pipelines page: https://app.circleci.com/pipelines/circleci/%s\n", orgSlug)
			} else if resp.Message != nil {
				cmd.Printf("Message: %s\n", resp.Message.Message)
			}

			return nil
		},
		Args: cobra.ExactArgs(2),
	}

	cmd.Flags().StringVar(&pipelineDefinitionID, "pipeline-definition-id", "", "Pipeline definition ID to test")
	cmd.Flags().StringVar(&configBranch, "config-branch", "", "Branch to use for config (mutually exclusive with --config-tag)")
	cmd.Flags().StringVar(&configTag, "config-tag", "", "Tag to use for config (mutually exclusive with --config-branch)")
	cmd.Flags().StringVar(&checkoutBranch, "checkout-branch", "", "Branch to checkout (mutually exclusive with --checkout-tag)")
	cmd.Flags().StringVar(&checkoutTag, "checkout-tag", "", "Tag to checkout (mutually exclusive with --checkout-branch)")
	cmd.Flags().StringVar(&localConfigFilePath, "local-config-file", "", "Path to a local config file to use")
	cmd.Flags().StringToStringVar(&parameters, "parameters", nil, "Pipeline parameters in key=value format (can be specified multiple times)")
	cmd.Flags().BoolVar(&repoConfig, "repo-config", false, "Use repository config")

	// Add mutual exclusivity rules
	cmd.MarkFlagsMutuallyExclusive("config-branch", "config-tag")
	cmd.MarkFlagsMutuallyExclusive("checkout-branch", "checkout-tag")

	return cmd
}
