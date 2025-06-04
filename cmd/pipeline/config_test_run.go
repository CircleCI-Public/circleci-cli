package pipeline

import (
	"github.com/spf13/cobra"

	"strings"

	"github.com/CircleCI-Public/circleci-cli/api/pipeline"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
)

func newConfigTestRunCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	var organization string
	var project string
	var pipelineDefinitionID string
	var configBranch string
	var configTag string
	var checkoutBranch string
	var checkoutTag string
	var configFilePath string
	var parameters map[string]string

	cmd := &cobra.Command{
		Use:   "config-test-run <organization> <project> --pipeline-definition-id <id> [options...]",
		Short: "Test run a pipeline configuration. When a local config is supplied, it runs it via a ci pipeline in circleci's cloud.",
		Long: `Test run a pipeline configuration. When a local config is supplied, it runs it via a ci pipeline in circleci's cloud.

Required arguments:
  organization             Depending on the organization type, this may be the org name (e.g. my-org)
                           or an ID (e.g. 43G3lM5RtFE7v5sa4nWAU). The second segment of the slash-separated project
                           slug, as shown in Project Settings > Overview. 													
  project                  Depending on the organization type, this may be the project name (e.g. my-project)
                           or an ID (e.g. 44n9wujWcTnVZ2b5S8Fnat). The third segment of the slash-separated
                           project slug, as shown in Project Settings > Overview. 													
  --pipeline-definition-id The unique id for the pipeline definition. This can be found in the page
                           Project Settings > Pipelines.

Optional flags:
  --config-branch          Branch to use for config (mutually exclusive with --config-tag)
  --config-tag             Tag to use for config (mutually exclusive with --config-branch)
  --checkout-branch        Branch to checkout (mutually exclusive with --checkout-tag)
  --checkout-tag           Tag to checkout (mutually exclusive with --checkout-branch)
  --config-file            Path to a local config file to use. If not provided, the config file in the repository
                           will be used. Please note you must have "Allow triggering pipelines with unversioned
                           config" enabled in Organization Settings > Advanced.
  --parameters             Pipeline parameters in key=value format (can be specified multiple times)

Examples:
  # Test with a config branch and checkout branch:
  circleci pipeline config-test-run my-org my-project --pipeline-definition-id abc123 \
    --config-branch main --checkout-branch feature-branch

  # Test with a config file and parameters:
  circleci pipeline config-test-run my-org my-project --pipeline-definition-id abc123 \
    --config-file .circleci/config.yml --parameters "key1=value1" --parameters "key2=value2"

  # Test with tags instead of branches:
  circleci pipeline config-test-run my-org my-project --pipeline-definition-id abc123 \
    --config-tag v1.0.0 --checkout-tag v1.0.0
`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			organization = args[0]
			project = args[1]

			if pipelineDefinitionID == "" {
				pipelineDefinitionIDPrompt := "Enter the pipeline definition ID for your pipeline"
				pipelineDefinitionID = ops.reader.ReadStringFromUser(pipelineDefinitionIDPrompt)
			}

			// If no config file is specified, ask if user wants to use a local config
			if configFilePath == "" {
				useLocalConfigPrompt := "Do you want to test with a local config file? This will override the config file in the repository. (Y/n)"
				if ops.reader.AskConfirm(useLocalConfigPrompt) {
					configFilePathPrompt := "Enter the path to your local config file"
					configFilePath = ops.reader.ReadStringFromUser(configFilePathPrompt)
				}
			}

			// Prompt for config branch or tag if neither is provided
			if configBranch == "" && configTag == "" {
				configPrompt := "You must specify either a config branch or tag. Enter a branch (or leave blank to enter a tag):"
				configBranch = ops.reader.ReadStringFromUser(configPrompt)
				if configBranch == "" {
					configTagPrompt := "Enter a config tag:"
					configTag = ops.reader.ReadStringFromUser(configTagPrompt)
				}
			}

			// Prompt for checkout branch or tag if neither is provided
			if checkoutBranch == "" && checkoutTag == "" {
				checkoutPrompt := "You must specify either a checkout branch or tag. Enter a branch (or leave blank to enter a tag):"
				checkoutBranch = ops.reader.ReadStringFromUser(checkoutPrompt)
				if checkoutBranch == "" {
					checkoutTagPrompt := "Enter a checkout tag:"
					checkoutTag = ops.reader.ReadStringFromUser(checkoutTagPrompt)
				}
			}

			// Convert string parameters to interface map
			paramMap := make(map[string]interface{})
			for k, v := range parameters {
				paramMap[k] = v
			}

			options := pipeline.TriggerConfigTestRunOptions{
				Organization:         organization,
				Project:              project,
				PipelineDefinitionID: pipelineDefinitionID,
				ConfigBranch:         configBranch,
				ConfigTag:            configTag,
				CheckoutBranch:       checkoutBranch,
				CheckoutTag:          checkoutTag,
				Parameters:           paramMap,
				ConfigFilePath:       configFilePath,
			}

			resp, err := ops.pipelineClient.TriggerConfigTestRun(options)
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
	cmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to a local config file to use")
	cmd.Flags().StringToStringVar(&parameters, "parameters", nil, "Pipeline parameters in key=value format (can be specified multiple times)")

	// Add mutual exclusivity rules
	cmd.MarkFlagsMutuallyExclusive("config-branch", "config-tag")
	cmd.MarkFlagsMutuallyExclusive("checkout-branch", "checkout-tag")

	return cmd
}
