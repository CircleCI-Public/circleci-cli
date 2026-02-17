package trigger

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/pipeline"
	"github.com/CircleCI-Public/circleci-cli/api/trigger"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

func newCreateCommand(ops *triggerOpts, preRunE validator.Validator) *cobra.Command {
	var pipelineDefinitionID string
	var repoID string
	var eventPreset string
	var configRef string
	var checkoutRef string

	cmd := &cobra.Command{
		Use:   "create <project-id> [--repo-id <github-repo-id>] [--event-preset <preset-to-filter-triggers>] [--config-ref <ref-to-fetch-config>] [--checkout-ref <ref-to-checkout-code>]",
		Short: "Create a new trigger for a CircleCI project.",
		Long: `Create a new trigger for a CircleCI project.

All flags are optional - if not provided, you will be prompted interactively for
  the required values:

  --pipeline-definition-id  Pipeline definition ID you wish to create a trigger for
                           (required)
  --name                    Name of the trigger (required)
  --description             Description of the trigger (will not prompt if omitted)
  --repo-id                 GitHub repository ID you wish to create a trigger for
                           (required)
  --event-preset            The name of the event preset to use when filtering events
                           for this trigger (will not prompt if omitted)
  --checkout-ref            Git ref (branch, or tag for example) to check out code
                           for pipeline runs (only required if different repository,
                           will not prompt if omitted)
  --config-ref              Git ref (branch, or tag for example) to fetch config for
                           pipeline runs (only required if different repository,
                           will not prompt if omitted)

To api/v2 documentation for creating a trigger, see:
  https://circleci.com/docs/api/v2/index.html#tag/Trigger/operation/createTrigger

Examples:
  # Minimal usage (will prompt for required values):
  circleci trigger create 1662d941-6e28-43ab-bea2-aa678c098d4c

  # Full usage with all flags:
  circleci trigger create 1662d941-6e28-43ab-bea2-aa678c098d4c --name my-trigger \
    --description "Trigger description" --repo-id 123456 --event-preset all-pushes \
    --config-ref my-config --checkout-ref my-checkout

Notes:
  - This is only supporting pipeline definitions created with GitHub App provider.
    You will need our GitHub App installed in your repository.
  - To get the repository id you can use:
    https://docs.github.com/en/rest/repos/repos?apiVersion=2022-11-28#get-a-repository
  - To get the pipeline definition id you can visit the Pipelines tab in project
    settings:
    https://app.circleci.com/settings/project/circleci/<org>/<project>/configurations
  - --config_ref and --checkout_ref flags are only required if your config source or
    checkout source exist in a different repo to your trigger repo
`,
		PreRunE: preRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]

			client, _ := telemetry.FromContext(cmd.Context())
			invID, _ := telemetry.InvocationIDFromContext(cmd.Context())

			if pipelineDefinitionID == "" {
				trackTriggerCreateStep(client, "definition_id_prompt_shown", invID, nil)
				pipelineDefinitionIDPrompt := "Enter the pipeline definition ID you wish to create a trigger for"
				pipelineDefinitionID = ops.reader.ReadStringFromUser(pipelineDefinitionIDPrompt)
				trackTriggerCreateStep(client, "definition_id_prompt_answered", invID, nil)
			}

			if repoID == "" {
				trackTriggerCreateStep(client, "repo_id_prompt_shown", invID, nil)
				repoPrompt := "Enter the ID of the triggering github repository"
				repoID = ops.reader.ReadStringFromUser(repoPrompt)
				trackTriggerCreateStep(client, "repo_id_prompt_answered", invID, nil)
			}

			pipelineOptions := pipeline.GetPipelineDefinitionOptions{
				ProjectID:            projectID,
				PipelineDefinitionID: pipelineDefinitionID,
			}

			trackTriggerCreateStep(client, "pipeline_definition_api_called", invID, nil)
			pipelineResp, pipelineErr := ops.pipelineClient.GetPipelineDefinition(pipelineOptions)
			if pipelineErr != nil {
				trackTriggerCreateStep(client, "pipeline_definition_fetch_failed", invID, nil)
				cmd.Println("\nThere was an error fetching your pipeline definition")
				return pipelineErr
			}

			if configRef == "" && pipelineResp.ConfigSourceId != repoID {
				trackTriggerCreateStep(client, "config_ref_prompt_shown", invID, map[string]interface{}{
					"different_config_repo": true,
				})
				configRefPrompt := "Your pipeline repo and config source repo are different. Enter the branch or tag to use when fetching config for pipeline runs"
				configRef = ops.reader.ReadStringFromUser(configRefPrompt)
				trackTriggerCreateStep(client, "config_ref_prompt_answered", invID, nil)
			}

			if checkoutRef == "" && pipelineResp.CheckoutSourceId != repoID {
				trackTriggerCreateStep(client, "checkout_ref_prompt_shown", invID, map[string]interface{}{
					"different_checkout_repo": true,
				})
				checkoutRefPrompt := "Your pipeline repo and checkout source repo are different. Enter the branch or tag to use when checking out code for pipeline runs"
				checkoutRef = ops.reader.ReadStringFromUser(checkoutRefPrompt)
				trackTriggerCreateStep(client, "checkout_ref_prompt_answered", invID, nil)
			}

			triggerOptions := trigger.CreateTriggerOptions{
				ProjectID:            projectID,
				PipelineDefinitionID: pipelineDefinitionID,
				RepoID:               repoID,
				EventPreset:          eventPreset,
				ConfigRef:            configRef,
				CheckoutRef:          checkoutRef,
			}

			trackTriggerCreateStep(client, "api_called", invID, nil)
			_, err := ops.triggerClient.CreateTrigger(triggerOptions)
			if err != nil {
				trackTriggerCreateStep(client, "failed", invID, nil)
				cmd.Println("\nThere was an error creating your trigger. Do you have Github App installed in your repository?")
				return err
			}

			trackTriggerCreateStep(client, "succeeded", invID, nil)
			cmd.Printf("Trigger created successfully\n")
			cmd.Println("You may view your new trigger in your project settings: https://app.circleci.com/settings/project/circleci/<org>/<project>/triggers")
			return nil
		},
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&pipelineDefinitionID, "pipeline-definition-id", "", "Pipeline definition ID you wish to create a trigger for")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "Repository ID of the codebase you wish to create a trigger for")
	cmd.Flags().StringVar(&eventPreset, "event-preset", "", "The name of the event preset to use when filtering events for this trigger")
	cmd.Flags().StringVar(&configRef, "config-ref", "", "Git ref (branch, or tag for example) to use when fetching config for pipeline runs created from this trigger")
	cmd.Flags().StringVar(&checkoutRef, "checkout-ref", "", "Git ref (branch, or tag for example) to use when checking out code for pipeline runs created from this trigger")
	return cmd
}

func trackTriggerCreateStep(client telemetry.Client, step, invocationID string, extra map[string]interface{}) {
	telemetry.TrackWorkflowStep(client, "trigger_create", step, invocationID, extra)
}
