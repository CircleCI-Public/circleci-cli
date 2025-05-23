package trigger

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/trigger"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
)

func newCreateCommand(ops *triggerOpts, preRunE validator.Validator) *cobra.Command {
	var name string
	var description string
	var pipelineDefinitionID string
	var repoID string
	var eventPreset string
	var configRef string
	var checkoutRef string

	cmd := &cobra.Command{
		Use:   "create <project-id> [--name <trigger-name>] [--description <description>] [--repo-id <github-repo-id>] [--event-preset <preset-to-filter-triggers>] [--config-ref <ref-to-fetch-config>] [--checkout-ref <ref-to-checkout-code>]",
		Short: "Create a new trigger for a CircleCI project.",
		Long: `Create a new trigger for a CircleCI project.
All flags are optional - if not provided, you will be prompted interactively for the required values:
	--pipeline-definition-id  Pipeline definition ID you wish to create a trigger for (required)
	--name                    Name of the trigger (required)
	--description             Description of the trigger (will not prompt if omitted)
	--repo-id                 GitHub repository ID you wish to create a trigger for (required)
	--event-preset            The name of the event preset to use when filtering events for this trigger (will not prompt if omitted)
	--checkout-ref            Git ref to check out code for pipeline runs (only required different repository, will not prompt if omitted)
	--config-ref              Git ref to fetch config for pipeline runs (only required different repository, will not prompt if omitted)
To api/v2 documentation for creating a trigger, see: https://circleci.com/docs/api/v2/index.html#tag/Trigger/operation/createTrigger

Examples:
  # Minimal usage (will prompt for required values):
  circleci trigger create 1662d941-6e28-43ab-bea2-aa678c098d4c
  
  # Full usage with all flags:
  circleci trigger create 1662d941-6e28-43ab-bea2-aa678c098d4c --name my-trigger --description "Trigger description" --repo-id 123456 --event-preset all-pushes --config-ref my-config --checkout-ref my-checkout
Note: This is only supporting pipeline definitions created with GitHub App provider. You will need our GitHub App installed in your repository.
Note: To get the repository id you can use https://docs.github.com/en/rest/repos/repos?apiVersion=2022-11-28#get-a-repository
Note: To get the pipeline definition id you can visit the Pipelines tab in project settings: https://app.circleci.com/settings/project/circleci/<org>/<project>/configurations
Note: --config_ref and --checkout_ref flags are only required if your config source or checkout source exist in a different repo to your trigger repo
`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]

			if name == "" {
				namePrompt := "Enter a name for the trigger"
				name = ops.reader.ReadStringFromUser(namePrompt)
			}

			if pipelineDefinitionID == "" {
				pipelineDefinitionIDPrompt := "Enter the pipeline definition ID you wish to create a trigger for"
				pipelineDefinitionID = ops.reader.ReadStringFromUser(pipelineDefinitionIDPrompt)
			}

			if repoID == "" {
				repoPrompt := "Enter the ID of your github repository"
				repoID = ops.reader.ReadStringFromUser(repoPrompt)
			}

			if checkoutRef == "" || configRef == "" {
				refPrompt := "Is your config source and/or checkout source repository different repository for this triggger (y/n)?"
				refPromptResponse := ops.reader.ReadStringFromUser(refPrompt)
				if refPromptResponse == "y" {
					configRefPrompt := "Enter the git ref to use when fetching config for pipeline runs created from this trigger"
					configRef = ops.reader.ReadStringFromUser(configRefPrompt)
					checkoutRefPrompt := "Enter the git ref to use when checking out code for pipeline runs created from this trigger"
					checkoutRef = ops.reader.ReadStringFromUser(checkoutRefPrompt)
				}
			}

			triggerOptions := trigger.CreateTriggerOptions{
				ProjectID:            projectID,
				PipelineDefinitionID: pipelineDefinitionID,
				Name:                 name,
				Description:          description,
				RepoID:               repoID,
				EventPreset:          eventPreset,
				ConfigRef:            configRef,
				CheckoutRef:          checkoutRef,
			}

			res, err := ops.triggerClient.CreateTrigger(triggerOptions)
			if err != nil {
				cmd.Println("\nThere was an error creating your trigger. Do you have Github App installed in your repository?")
				return err
			}

			cmd.Printf("Trigger '%s' created successfully\n", res.Name)
			cmd.Println("You may view your new trigger in your project settings: https://app.circleci.com/settings/project/circleci/<org>/<project>/triggers")
			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the trigger to create")
	cmd.Flags().StringVar(&pipelineDefinitionID, "pipeline-definition-id", "", "Pipeline definition ID you wish to create a trigger for")
	cmd.Flags().StringVar(&description, "description", "", "Description of the trigger to create")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "Repository ID of the codebase you wish to create a trigger for")
	cmd.Flags().StringVar(&eventPreset, "event-preset", "", "The name of the event preset to use when filtering events for this trigger")
	cmd.Flags().StringVar(&configRef, "config-ref", "", "Git ref to use when fetching config for pipeline runs created from this trigger")
	cmd.Flags().StringVar(&checkoutRef, "checkout-ref", "", "Git ref to use when checking out code for pipeline runs created from this trigger")
	return cmd
}
