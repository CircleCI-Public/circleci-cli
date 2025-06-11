/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	pipelineapi "github.com/CircleCI-Public/circleci-cli/api/pipeline"
	projectapi "github.com/CircleCI-Public/circleci-cli/api/project"
	triggerapi "github.com/CircleCI-Public/circleci-cli/api/trigger"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

type initOptions struct {
	projectClient  projectapi.ProjectClient
	pipelineClient pipelineapi.PipelineClient
	triggerClient  triggerapi.TriggerClient
	// Project creation options
	vcsType     string
	orgName     string
	projectName string
	// Pipeline creation options
	pipelineName        string
	pipelineDescription string
	repoID              string
	configRepoID        string
	filePath            string
	// Trigger creation options
	triggerName        string
	triggerDescription string
	eventPreset        string
	configRef          string
	checkoutRef        string
}

// UserInputReader interface for prompting user input
type UserInputReader interface {
	ReadStringFromUser(msg string, defaultValue string) string
	AskConfirm(msg string) bool
}

type initPromptReader struct{}

func (p initPromptReader) ReadStringFromUser(msg string, defaultValue string) string {
	return prompt.ReadStringFromUser(msg, defaultValue)
}

func (p initPromptReader) AskConfirm(msg string) bool {
	return prompt.AskUserToConfirm(msg)
}

func newInitCommand(config *settings.Config) *cobra.Command {
	opts := initOptions{}
	reader := &initPromptReader{}

	var initCmd = &cobra.Command{
		Use:   "init [flags]",
		Short: "Initialize a new CircleCI project",
		Long: `Creates a new project, pipeline, and trigger in one easy step.

This command will guide you through setting up a complete CircleCI project by:
1. Creating a new project in your CircleCI organization
2. Setting up a pipeline definition with your repository
3. Creating a trigger to automatically run the pipeline

Examples:
  # Interactive mode - prompts for all required values
  circleci init

  # With flags to skip some prompts
  circleci init --vcs-type github --org-name myorg --project-name myproject --repo-id 123456

  # Full specification with all flags
  circleci init --vcs-type github --org-name myorg --project-name myproject \
    --pipeline-name my-pipeline --repo-id 123456 --trigger-name my-trigger`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Initialize project client
			projectClient, err := projectapi.NewProjectRestClient(*config)
			if err != nil {
				return err
			}
			opts.projectClient = projectClient

			// Initialize pipeline client
			pipelineClient, err := pipelineapi.NewPipelineRestClient(*config)
			if err != nil {
				return err
			}
			opts.pipelineClient = pipelineClient

			// Initialize trigger client
			triggerClient, err := triggerapi.NewTriggerRestClient(*config)
			if err != nil {
				return err
			}
			opts.triggerClient = triggerClient

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return initCmd(opts, reader, cmd)
		},
	}

	// Project creation flags
	initCmd.Flags().StringVar(&opts.vcsType, "vcs-type", "", "Version control system type (e.g., 'github', 'bitbucket')")
	initCmd.Flags().StringVar(&opts.orgName, "org-name", "", "Organization name or slug")
	initCmd.Flags().StringVar(&opts.projectName, "project-name", "", "Name of the project to create")

	// Pipeline creation flags
	initCmd.Flags().StringVar(&opts.pipelineName, "pipeline-name", "", "Name of the pipeline to create")
	initCmd.Flags().StringVar(&opts.pipelineDescription, "pipeline-description", "", "Description of the pipeline")
	initCmd.Flags().StringVar(&opts.repoID, "repo-id", "", "GitHub repository ID of the codebase")
	initCmd.Flags().StringVar(&opts.configRepoID, "config-repo-id", "", "GitHub repository ID where the CircleCI config file is located (defaults to same as repo-id)")
	initCmd.Flags().StringVar(&opts.filePath, "file-path", "", "Path to the CircleCI config file (default: .circleci/config.yml)")

	// Trigger creation flags
	initCmd.Flags().StringVar(&opts.triggerName, "trigger-name", "", "Name of the trigger to create")
	initCmd.Flags().StringVar(&opts.triggerDescription, "trigger-description", "", "Description of the trigger")
	initCmd.Flags().StringVar(&opts.eventPreset, "event-preset", "", "Event preset to filter triggers (e.g., 'all-pushes')")
	initCmd.Flags().StringVar(&opts.configRef, "config-ref", "", "Git ref to use when fetching config (only needed if different from trigger repo)")
	initCmd.Flags().StringVar(&opts.checkoutRef, "checkout-ref", "", "Git ref to use when checking out code (only needed if different from trigger repo)")

	return initCmd
}

func promptTillYOrN(reader UserInputReader) string {
	for {
		input := reader.ReadStringFromUser("Does your CircleCI config file exist in a different repository? (y/n)", "")
		if input == "y" || input == "n" {
			return input
		}
		fmt.Println("Invalid input. Please enter 'y' or 'n'.")
	}
}

func initCmd(opts initOptions, reader UserInputReader, cmd *cobra.Command) error {
	// Step 1: Create project
	fmt.Println("🚀 Initializing CircleCI project...")
	fmt.Println()

	// Prompt for missing project values
	if opts.vcsType == "" {
		opts.vcsType = reader.ReadStringFromUser("Enter VCS type (github/bitbucket)", "github")
	}

	if opts.orgName == "" {
		opts.orgName = reader.ReadStringFromUser("Enter organization name", "")
	}

	if opts.projectName == "" {
		opts.projectName = reader.ReadStringFromUser("Enter project name", "")
	}

	// Validate required project fields
	if opts.vcsType == "" || opts.orgName == "" || opts.projectName == "" {
		return fmt.Errorf("all project fields are required: vcs-type, org-name, and project-name")
	}

	// Create the project
	fmt.Printf("📁 Creating project '%s' in organization '%s'...\n", opts.projectName, opts.orgName)

	projectRes, err := opts.projectClient.CreateProject(opts.vcsType, opts.orgName, opts.projectName)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	fmt.Printf("✅ Project '%s' successfully created in organization '%s'\n", projectRes.Name, projectRes.OrgName)
	fmt.Printf("   Project ID: %s\n", projectRes.Id)
	fmt.Printf("   View project: https://app.circleci.com/projects/%s\n", projectRes.Slug)
	fmt.Println()

	// Step 2: Create pipeline
	fmt.Println("📋 Creating pipeline definition...")

	// Prompt for missing pipeline values
	if opts.pipelineName == "" {
		opts.pipelineName = reader.ReadStringFromUser("Enter a name for the pipeline", fmt.Sprintf("%s-pipeline", opts.projectName))
	}

	if opts.repoID == "" {
		fmt.Println("ℹ️  You can get your GitHub repository ID using: https://docs.github.com/en/rest/repos/repos?apiVersion=2022-11-28#get-a-repository")
		opts.repoID = reader.ReadStringFromUser("Enter the ID of your GitHub repository", "")
	}

	if opts.configRepoID == "" {
		yOrN := promptTillYOrN(reader)
		if yOrN == "y" {
			opts.configRepoID = reader.ReadStringFromUser("Enter the ID of the GitHub repository where the CircleCI config file is located", "")
		} else {
			opts.configRepoID = opts.repoID
		}
	}

	if opts.filePath == "" {
		opts.filePath = reader.ReadStringFromUser("Enter the path to your CircleCI config file", ".circleci/config.yml")
	}

	// Validate required pipeline fields
	if opts.pipelineName == "" || opts.repoID == "" || opts.configRepoID == "" || opts.filePath == "" {
		return fmt.Errorf("all pipeline fields are required: pipeline-name, repo-id, config-repo-id, and file-path")
	}

	// Create the pipeline
	fmt.Printf("📋 Creating pipeline '%s' for project '%s'...\n", opts.pipelineName, projectRes.Id)

	pipelineRes, err := opts.pipelineClient.CreatePipeline(
		projectRes.Id,
		opts.pipelineName,
		opts.pipelineDescription,
		opts.repoID,
		opts.configRepoID,
		opts.filePath,
	)
	if err != nil {
		fmt.Printf("❌ Failed to create pipeline: %v\n", err)
		fmt.Println("💡 Make sure you have the GitHub App installed in your repository")
		fmt.Println("   Visit: https://github.com/apps/circleci")
		return fmt.Errorf("failed to create pipeline: %w", err)
	}

	fmt.Printf("✅ Pipeline '%s' successfully created for repository '%s'\n", pipelineRes.Name, pipelineRes.CheckoutSourceRepoFullName)
	if pipelineRes.CheckoutSourceRepoFullName != pipelineRes.ConfigSourceRepoFullName {
		fmt.Printf("   Config referenced from '%s' repository at path '%s'\n", pipelineRes.ConfigSourceRepoFullName, opts.filePath)
	}
	fmt.Printf("   Pipeline ID: %s\n", pipelineRes.Id)
	fmt.Println()

	// Step 3: Create trigger
	fmt.Println("⚡ Creating trigger for the pipeline...")

	// Prompt for missing trigger values
	if opts.triggerName == "" {
		opts.triggerName = reader.ReadStringFromUser("Enter a name for the trigger", fmt.Sprintf("%s-trigger", opts.pipelineName))
	}

	// Get pipeline definition to check if we need config/checkout refs
	pipelineOptions := pipelineapi.GetPipelineDefinitionOptions{
		ProjectID:            projectRes.Id,
		PipelineDefinitionID: pipelineRes.Id,
	}
	pipelineResp, err := opts.pipelineClient.GetPipelineDefinition(pipelineOptions)
	if err != nil {
		fmt.Printf("❌ Failed to get pipeline definition: %v\n", err)
		return fmt.Errorf("failed to get pipeline definition: %w", err)
	}

	// Check if we need config ref (only if config source is different from trigger repo)
	if opts.configRef == "" && pipelineResp.ConfigSourceId != opts.repoID {
		opts.configRef = reader.ReadStringFromUser("Your pipeline repo and config source repo are different. Enter the branch or tag to use when fetching config for pipeline runs", "")
	}

	// Check if we need checkout ref (only if checkout source is different from trigger repo)
	if opts.checkoutRef == "" && pipelineResp.CheckoutSourceId != opts.repoID {
		opts.checkoutRef = reader.ReadStringFromUser("Your pipeline repo and checkout source repo are different. Enter the branch or tag to use when checking out code for pipeline runs", "")
	}

	// Create the trigger
	fmt.Printf("⚡ Creating trigger '%s' for pipeline '%s'...\n", opts.triggerName, pipelineRes.Id)

	triggerOptions := triggerapi.CreateTriggerOptions{
		ProjectID:            projectRes.Id,
		PipelineDefinitionID: pipelineRes.Id,
		Name:                 opts.triggerName,
		Description:          opts.triggerDescription,
		RepoID:               opts.repoID,
		EventPreset:          opts.eventPreset,
		ConfigRef:            opts.configRef,
		CheckoutRef:          opts.checkoutRef,
	}

	triggerRes, err := opts.triggerClient.CreateTrigger(triggerOptions)
	if err != nil {
		fmt.Printf("❌ Failed to create trigger: %v\n", err)
		fmt.Println("💡 Make sure you have the GitHub App installed in your repository")
		fmt.Println("   Visit: https://github.com/apps/circleci")
		return fmt.Errorf("failed to create trigger: %w", err)
	}

	fmt.Printf("✅ Trigger '%s' successfully created!\n", triggerRes.Name)
	fmt.Printf("   Trigger ID: %s\n", triggerRes.Id)
	fmt.Println()

	fmt.Println("🎉 Project initialization completed successfully! Summary:")
	fmt.Printf("   ✅ Project: %s (ID: %s)\n", projectRes.Name, projectRes.Id)
	fmt.Printf("   ✅ Pipeline: %s (ID: %s)\n", pipelineRes.Name, pipelineRes.Id)
	fmt.Printf("   ✅ Trigger: %s (ID: %s)\n", triggerRes.Name, triggerRes.Id)
	fmt.Println()
	fmt.Println("🔗 Useful links:")
	fmt.Printf("   Project: https://app.circleci.com/projects/%s\n", projectRes.Slug)
	fmt.Println("   Pipeline settings: https://app.circleci.com/settings/project/<vcs>/<org>/<project>/configurations")
	fmt.Println("   Trigger settings: https://app.circleci.com/settings/project/<vcs>/<org>/<project>/triggers")
	fmt.Println()
	fmt.Println("📝 Next steps:")
	fmt.Printf("   1. Make sure you have a '%s' file in your repository\n", opts.filePath)
	fmt.Println("   2. Push code to your repository to trigger your first pipeline run")
	fmt.Println("   3. Monitor your pipeline runs in the CircleCI dashboard")
	fmt.Println()
	fmt.Println("🎊 Your CircleCI project is now fully configured and ready to use!")

	return nil
}
