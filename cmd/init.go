package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/CircleCI-Public/circleci-cli/api/collaborators"
	pipelineapi "github.com/CircleCI-Public/circleci-cli/api/pipeline"
	projectapi "github.com/CircleCI-Public/circleci-cli/api/project"
	"github.com/CircleCI-Public/circleci-cli/api/repository"
	triggerapi "github.com/CircleCI-Public/circleci-cli/api/trigger"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

type initOptions struct {
	projectClient       projectapi.ProjectClient
	pipelineClient      pipelineapi.PipelineClient
	triggerClient       triggerapi.TriggerClient
	collaboratorsClient collaborators.CollaboratorsClient
	repositoryClient    repository.RepositoryClient
	// Project creation options
	vcsType     string
	orgName     string
	orgID       string
	projectName string
	// Pipeline creation options
	pipelineName        string
	pipelineDescription string
	repoID              string
	configRepoID        string
	filePath            string
	// Trigger creation options
	eventPreset string
	configRef   string
	checkoutRef string
	// Local onboarding options
	localOnly  bool
	dir        string
	skipDocker bool
	skipConfig bool
	verbose    bool
}

type UserInputReader interface {
	ReadStringFromUser(msg string, defaultValue string, validator ...func(string) error) string
	AskConfirm(msg string) bool
}

type initPromptReader struct{}

const (
	githubVCS   = "github"
	circleciVCS = "circleci"
)

func (p initPromptReader) ReadStringFromUser(msg string, defaultValue string, validator ...func(string) error) string {
	return prompt.ReadStringFromUser(msg, defaultValue, validator...)
}

func (p initPromptReader) AskConfirm(msg string) bool {
	return prompt.AskUserToConfirm(msg)
}

func newInitCommand(config *settings.Config) *cobra.Command {
	opts := initOptions{}
	reader := &initPromptReader{}

	var initCmd = &cobra.Command{
		Use:   "init [org-slug] [flags]",
		Short: "Initialize a new CircleCI project",
		Long: `Detect your project's tech stack, run tests locally, and generate a CircleCI config.

When run without authentication, this command will:
1. Scan your repository to detect the language, dependencies, and test commands
2. Select the best matching CircleCI convenience image
3. Run tests locally in a Docker container
4. Generate a .circleci/config.yml
5. Suggest CI optimizations (caching, linting, parallelism)

When authenticated, it can additionally create the project on CircleCI.

Examples:
  # Local onboarding (no account needed)
  circleci init

  # Skip Docker test execution
  circleci init --skip-docker

  # Scan a different directory
  circleci init --dir ./my-project

  # Force local-only mode even when authenticated
  circleci init --local

  # With org slug for remote project creation
  circleci init github/myorg`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine if we should run the local onboarding flow.
			// Run local flow if: --local flag is set, OR no auth token is configured.
			runLocal := opts.localOnly || config.Token == ""

			if runLocal {
				err := runOnboard(cmd.Context(), onboardOptions{
					dir:        opts.dir,
					skipDocker: opts.skipDocker,
					skipConfig: opts.skipConfig,
					verbose:    opts.verbose,
				}, cmd.OutOrStdout())
				if err != nil {
					return err
				}

				if config.Token == "" {
					return nil
				}

				// User is authenticated but used --local; don't chain into API flow
				if opts.localOnly {
					return nil
				}
			}

			// If we reach here, the user is authenticated. Offer to create the project on CircleCI.
			if !runLocal {
				// Run local flow first, then offer remote setup
				err := runOnboard(cmd.Context(), onboardOptions{
					dir:        opts.dir,
					skipDocker: opts.skipDocker,
					skipConfig: opts.skipConfig,
					verbose:    opts.verbose,
				}, cmd.OutOrStdout())
				if err != nil {
					return err
				}

				if !reader.AskConfirm("Would you like to also create the project on CircleCI?") {
					return nil
				}
			}

			// Initialize API clients for remote project creation
			if err := initAPIClients(config, &opts); err != nil {
				return err
			}

			// Parse org slug argument if provided
			if len(args) > 0 {
				orgSlug := args[0]

				// Validate org slug format
				if !strings.HasPrefix(orgSlug, "github/") && !strings.HasPrefix(orgSlug, "circleci/") {
					return fmt.Errorf("org slug must start with 'github/' or 'circleci/', got: %s", orgSlug)
				}

				// Split org slug to get vcs type and org name
				parts := strings.Split(orgSlug, "/")
				if len(parts) != 2 || parts[1] == "" {
					return fmt.Errorf("invalid org slug format. Expected 'github/orgname' or 'circleci/orgname', got: %s", orgSlug)
				}

				// Set vcs type and org name if not already provided via flags
				if opts.vcsType == "" {
					opts.vcsType = parts[0]
				}
				if opts.orgName == "" {
					opts.orgName = parts[1]
				}
			}

			// Validate event preset
			if err := validateEventPreset(opts.eventPreset); err != nil {
				return err
			}

			return runInitRemote(opts, reader, cmd)
		},
	}

	// Local onboarding flags
	initCmd.Flags().BoolVar(&opts.localOnly, "local", false, "Run only the local onboarding flow (detect, test, generate config)")
	initCmd.Flags().StringVar(&opts.dir, "dir", ".", "Directory to scan for tech stack detection")
	initCmd.Flags().BoolVar(&opts.skipDocker, "skip-docker", false, "Skip running tests in Docker")
	initCmd.Flags().BoolVar(&opts.skipConfig, "skip-config", false, "Skip generating .circleci/config.yml")
	initCmd.Flags().BoolVar(&opts.verbose, "verbose", false, "Show full Docker build and test output")

	// Project creation flags
	initCmd.Flags().StringVar(&opts.vcsType, "vcs-type", "", "Version control system type (e.g., 'github', 'circleci')")
	initCmd.Flags().StringVar(&opts.orgName, "org-name", "", "Organization name or slug")
	initCmd.Flags().StringVar(&opts.projectName, "project-name", "", "Name of the project to create")

	// Pipeline creation flags
	initCmd.Flags().StringVar(&opts.pipelineName, "pipeline-name", "", "Name of the pipeline to create")
	initCmd.Flags().StringVar(&opts.pipelineDescription, "pipeline-description", "", "Description of the pipeline")
	initCmd.Flags().StringVar(&opts.repoID, "repo-id", "", "GitHub repository ID of the codebase")
	initCmd.Flags().StringVar(&opts.configRepoID, "config-repo-id", "", "GitHub repository ID where the CircleCI config file is located (defaults to same as repo-id)")
	initCmd.Flags().StringVar(&opts.filePath, "file-path", "", "Path to the CircleCI config file (default: .circleci/config.yml)")

	// Trigger creation flags
	initCmd.Flags().StringVar(&opts.eventPreset, "event-preset", "", "Event preset to filter triggers. Valid values: all-pushes, only-tags, default-branch-pushes, only-build-prs, only-open-prs, only-merged-prs, only-ready-for-review-prs, only-labeled-prs, only-build-pushes-to-non-draft-prs")
	initCmd.Flags().StringVar(&opts.configRef, "config-ref", "", "Git ref to use when fetching config (only needed if different from trigger repo)")
	initCmd.Flags().StringVar(&opts.checkoutRef, "checkout-ref", "", "Git ref to use when checking out code (only needed if different from trigger repo)")

	return initCmd
}

func initAPIClients(config *settings.Config, opts *initOptions) error {
	projectClient, err := projectapi.NewProjectRestClient(*config)
	if err != nil {
		return err
	}
	opts.projectClient = projectClient

	pipelineClient, err := pipelineapi.NewPipelineRestClient(*config)
	if err != nil {
		return err
	}
	opts.pipelineClient = pipelineClient

	triggerClient, err := triggerapi.NewTriggerRestClient(*config)
	if err != nil {
		return err
	}
	opts.triggerClient = triggerClient

	collaboratorsClient, err := collaborators.NewCollaboratorsRestClient(*config)
	if err != nil {
		return err
	}
	opts.collaboratorsClient = collaboratorsClient

	repositoryClient, err := repository.NewRepositoryRestClient(*config)
	if err != nil {
		return err
	}
	opts.repositoryClient = repositoryClient

	return nil
}

func promptTillYOrN(reader UserInputReader) string {
	for {
		input := reader.ReadStringFromUser("Does your CircleCI config file exist in a different repository? (y/n)", "", nil)
		if input == "y" || input == "n" {
			return input
		}
		fmt.Println("Invalid input. Please enter 'y' or 'n'.")
	}
}

func selectOrganization(collaboratorsClient collaborators.CollaboratorsClient) (string, string, string, error) {
	fmt.Println("🔍 Fetching your organizations...")
	collaborations, err := collaboratorsClient.GetOrgCollaborations()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch organizations: %w", err)
	}

	if len(collaborations) == 0 {
		return "", "", "", fmt.Errorf("no organizations found for your account")
	}

	options := make([]string, len(collaborations))
	orgMap := make(map[string]collaborators.CollaborationResult)

	for i, collab := range collaborations {
		displayName := fmt.Sprintf("%s (%s)", collab.OrgName, collab.OrgSlug)
		options[i] = displayName
		orgMap[displayName] = collab
	}

	var selectedOption string
	prompt := &survey.Select{
		Message: "Select an organization:",
		Options: options,
	}

	err = survey.AskOne(prompt, &selectedOption)
	if err != nil {
		return "", "", "", fmt.Errorf("organization selection failed: %w", err)
	}

	selectedOrg := orgMap[selectedOption]

	slugParts := strings.Split(selectedOrg.OrgSlug, "/")
	if len(slugParts) != 2 {
		return "", "", "", fmt.Errorf("invalid organization slug format: %s", selectedOrg.OrgSlug)
	}

	vcsType := slugParts[0]
	orgName := slugParts[1]
	orgID := selectedOrg.OrgId

	return vcsType, orgName, orgID, nil
}

func selectRepository(reader UserInputReader, repositoryClient repository.RepositoryClient, orgID string) (string, error) {
	if orgID == "" {
		fmt.Println("📝 Organization ID not available, using manual repository ID input...")
		return selectRepositoryManually(reader)
	}

	fmt.Println("🔍 Fetching available repositories...")

	repositories, err := repositoryClient.GetGitHubRepositories(orgID)
	if err != nil {
		fmt.Printf("⚠️  Unable to fetch repositories from GitHub (%v)\n", err)
		fmt.Println("📝 Falling back to manual repository ID input...")
		return selectRepositoryManually(reader)
	}

	if len(repositories.Repositories) == 0 {
		fmt.Println("📝 No repositories found for this organization. Please enter repository ID manually...")
		return selectRepositoryManually(reader)
	}

	fmt.Printf("✅ Found %d repositories\n", repositories.TotalCount)

	maxOptions := 50
	repoCount := len(repositories.Repositories)
	if repoCount > maxOptions {
		repoCount = maxOptions
		fmt.Printf("   Showing first %d repositories (you can enter ID manually if needed)\n", maxOptions)
	}

	options := make([]string, repoCount+1) // +1 for manual input option
	repoMap := make(map[string]repository.Repository)

	for i := range repoCount {
		repo := repositories.Repositories[i]
		displayName := fmt.Sprintf("%s (%s) - %s", repo.FullName, repo.Language, repo.Description)
		if len(displayName) > 80 {
			displayName = displayName[:77] + "..."
		}
		options[i] = displayName
		repoMap[displayName] = repo
	}

	options[repoCount] = "📝 Enter repository ID manually"

	var selectedOption string
	prompt := &survey.Select{
		Message:  "Select the repository you want this pipeline to build, test, and deploy:",
		Options:  options,
		PageSize: 10,
	}

	err = survey.AskOne(prompt, &selectedOption)
	if err != nil {
		return "", fmt.Errorf("repository selection failed: %w", err)
	}

	if selectedOption == "📝 Enter repository ID manually" {
		return selectRepositoryManually(reader)
	}

	selectedRepo := repoMap[selectedOption]
	fmt.Printf("✅ Selected repository: %s (ID: %d)\n", selectedRepo.FullName, selectedRepo.ID)

	return strconv.Itoa(selectedRepo.ID), nil
}

func selectRepositoryManually(reader UserInputReader) (string, error) {
	fmt.Println("📝 Repository ID Input")
	fmt.Println("   You can get your repository ID using the appropriate API:")
	fmt.Println("   For GitHub: curl -H \"Accept: application/vnd.github.v3+json\" https://api.github.com/repos/{owner}/{repo}")
	fmt.Println("   Or visit: https://docs.github.com/en/rest/repos/repos?apiVersion=2022-11-28#get-a-repository")
	fmt.Println()

	repoID := reader.ReadStringFromUser("Enter the repository ID", "", func(s string) error {
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("repository ID must be a number, got: %s", s)
		}
		return nil
	})

	return repoID, nil
}

// selectEventPreset prompts the user to select an event preset
func selectEventPreset() (string, error) {
	presetOptions := []string{
		"all-pushes",
		"only-tags",
		"default-branch-pushes",
		"only-build-prs",
		"only-open-prs",
		"only-merged-prs",
		"only-ready-for-review-prs",
		"only-labeled-prs",
		"only-build-pushes-to-non-draft-prs",
	}

	presetDescriptions := map[string]string{
		"all-pushes":                         "All pushes - Trigger your pipeline on all pushes to your repo",
		"only-tags":                          "Only tags - Trigger on pushes to tags",
		"default-branch-pushes":              "Default branch pushes - Trigger only on pushes to the default branch",
		"only-build-prs":                     "PR opened or pushed to, default branch and tag pushes",
		"only-open-prs":                      "Only open PRs - Trigger only when pull requests are opened",
		"only-merged-prs":                    "Only merged PRs - Trigger only when pull requests are merged",
		"only-ready-for-review-prs":          "Only ready for review PRs - Trigger when PRs are marked ready for review",
		"only-labeled-prs":                   "Only labeled PRs - Trigger only when `run-ci` label added to PR",
		"only-build-pushes-to-non-draft-prs": "Only build pushes to non-draft PRs - Trigger on pushes to non-draft pull requests",
	}

	displayOptions := make([]string, len(presetOptions))
	for i, preset := range presetOptions {
		displayOptions[i] = fmt.Sprintf("%s - %s", preset, presetDescriptions[preset])
	}

	var selectedOption string
	prompt := &survey.Select{
		Message:  "Select an event preset for your trigger:",
		Options:  displayOptions,
		PageSize: 10,
	}

	err := survey.AskOne(prompt, &selectedOption)
	if err != nil {
		return "", fmt.Errorf("event preset selection failed: %w", err)
	}

	// Extract the actual preset value from the display string
	selectedPreset := strings.Split(selectedOption, " - ")[0]
	return selectedPreset, nil
}

// validateEventPreset validates that the event preset is one of the allowed values
func validateEventPreset(preset string) error {
	if preset == "" {
		return nil // Allow empty preset, will be prompted for
	}

	validPresets := map[string]bool{
		"all-pushes":                         true,
		"only-tags":                          true,
		"default-branch-pushes":              true,
		"only-build-prs":                     true,
		"only-open-prs":                      true,
		"only-merged-prs":                    true,
		"only-ready-for-review-prs":          true,
		"only-labeled-prs":                   true,
		"only-build-pushes-to-non-draft-prs": true,
	}

	if !validPresets[preset] {
		return fmt.Errorf("invalid event preset '%s'. Valid values are: all-pushes, only-tags, default-branch-pushes, only-build-prs, only-open-prs, only-merged-prs, only-ready-for-review-prs, only-labeled-prs, only-build-pushes-to-non-draft-prs", preset)
	}

	return nil
}

func validateProjectName(name string) error {
	if len(name) < 3 || len(name) > 40 {
		return fmt.Errorf("project name must be between 3 and 40 characters long, got %d characters", len(name))
	}

	allowedPattern := regexp.MustCompile(`^[a-zA-Z0-9 \-_.:!&+\[\]]+$`)
	if !allowedPattern.MatchString(name) {
		return fmt.Errorf("project name can only contain letters, numbers, spaces, and the following characters: -_.:!&+[]")
	}

	return nil
}

func runInitRemote(opts initOptions, reader UserInputReader, cmd *cobra.Command) error {
	fmt.Println("🚀 Initializing CircleCI project...")
	fmt.Println()

	if opts.vcsType == "" || opts.orgName == "" {
		fmt.Println("📋 Organization Selection")
		selectedVCS, selectedOrg, selectedOrgID, err := selectOrganization(opts.collaboratorsClient)
		if err != nil {
			fmt.Printf("⚠️  Unable to fetch organizations (%v)\n", err)
			fmt.Println("📝 Please enter organization details manually:")

			if opts.vcsType == "" {
				opts.vcsType = reader.ReadStringFromUser("Enter VCS type (github/circleci)", "github", nil)
			}
			if opts.orgName == "" {
				opts.orgName = reader.ReadStringFromUser("Enter organization name", "", nil)
			}
		} else {
			fmt.Printf("✅ Selected organization: %s (%s/%s)\n", selectedOrg, selectedVCS, selectedOrg)
			if opts.vcsType == "" {
				opts.vcsType = selectedVCS
			}
			if opts.orgName == "" {
				opts.orgName = selectedOrg
			}
			if opts.orgID == "" {
				opts.orgID = selectedOrgID
			}
		}
		fmt.Println()
	}

	if opts.vcsType == circleciVCS {
		vcsConnectionsURL := fmt.Sprintf("https://app.circleci.com/settings/organization/%s/%s/vcs-connections", opts.vcsType, opts.orgName)

		fmt.Println("🔗 GitHub App Installation Check")
		fmt.Printf("   The CLI currently only supports GitHub repositories with the CircleCI GitHub App installed. We do not currently offer Bitbucket or GitLab support via the CLI.\n")
		fmt.Printf("   Checking app installation for organization: %s\n", opts.orgName)
		fmt.Println()

		// Check if GitHub App is installed using the API
		installation, err := opts.repositoryClient.CheckGitHubAppInstallation(opts.orgID)
		if err != nil {
			fmt.Printf("⚠️  Unable to check GitHub App installation status: %v\n", err)
			fmt.Printf("   Please verify manually: %s\n", vcsConnectionsURL)
			fmt.Println()

			// Fall back to manual confirmation if API check fails
			hasApp := reader.AskConfirm("Have you installed the CircleCI GitHub App for this organization?")
			if !hasApp {
				fmt.Println("⚠️  You'll need to install the CircleCI GitHub App before proceeding.")
				fmt.Println("   Please install it and run this command again.")
				fmt.Printf("   Install at: %s\n", vcsConnectionsURL)
				return fmt.Errorf("CircleCI GitHub App is required for GitHub organizations")
			}
			fmt.Println("✅ GitHub App installation confirmed!")
		} else if installation.ID == 0 {
			// App is not installed
			fmt.Printf("❌ CircleCI GitHub App is not installed for organization: %s\n", opts.orgName)
			fmt.Printf("   Visit: %s\n", vcsConnectionsURL)
			fmt.Println()

			openBrowser := reader.AskConfirm("Would you like to open the GitHub App installation page in your browser?")

			if openBrowser {
				err := browser.OpenURL(vcsConnectionsURL)
				if err != nil {
					fmt.Printf("⚠️  Could not open browser automatically: %v\n", err)
					fmt.Printf("   Please manually visit: %s\n", vcsConnectionsURL)
				} else {
					fmt.Println("✅ Opened GitHub App installation page in your browser")
				}
			} else {
				fmt.Printf("   You can manually visit: %s\n", vcsConnectionsURL)
			}
			fmt.Println()

			hasApp := reader.AskConfirm("Have you installed the CircleCI GitHub App for this organization?")
			if !hasApp {
				fmt.Println("⚠️  You'll need to install the CircleCI GitHub App before proceeding.")
				fmt.Println("   Please install it and run this command again.")
				fmt.Printf("   Install at: %s\n", vcsConnectionsURL)
				return fmt.Errorf("CircleCI GitHub App is required for GitHub organizations")
			}
			fmt.Println("✅ GitHub App installation confirmed!")
		} else {
			// App is installed
			fmt.Printf("✅ GitHub App is installed for organization: %s\n", opts.orgName)
		}
		fmt.Println()
	}

	if opts.projectName == "" {
		for {
			opts.projectName = reader.ReadStringFromUser("Enter your new project name", "", func(s string) error {
				if err := validateProjectName(s); err != nil {
					return err
				}
				return nil
			})
			if opts.projectName == "" {
				fmt.Println("Project name is required.")
				continue
			}
			break
		}
	} else {
		if err := validateProjectName(opts.projectName); err != nil {
			return fmt.Errorf("invalid project name: %w", err)
		}
	}

	if opts.vcsType == "" || opts.orgName == "" || opts.projectName == "" {
		return fmt.Errorf("all project fields are required: vcs-type, org-name, and project-name")
	}

	fmt.Printf("📁 Creating project '%s' in organization '%s'...\n", opts.projectName, opts.orgName)
	trackInitStep(cmd, "project_creating", nil)

	projectRes, err := opts.projectClient.CreateProject(opts.vcsType, opts.orgName, opts.projectName)
	if err != nil {
		trackInitStep(cmd, "project_failed", nil)
		return fmt.Errorf("failed to create project: %w", err)
	}

	trackInitStep(cmd, "project_created", nil)
	fmt.Printf("✅ Project '%s' successfully created in organization '%s'\n", projectRes.Name, projectRes.OrgName)
	fmt.Printf("   Project ID: %s\n", projectRes.Id)
	fmt.Printf("   View project: https://app.circleci.com/projects/%s\n", projectRes.Slug)
	fmt.Println()

	fmt.Println("📋 Creating pipeline definition...")
	fmt.Println("   Pipelines orchestrate the execution of workflows, which run jobs to build, test, and deploy your code.")

	if opts.pipelineName == "" {
		opts.pipelineName = reader.ReadStringFromUser("Enter a name for the pipeline", "build-and-test", nil)
	}

	if opts.repoID == "" {
		fmt.Println()
		selectedRepoID, err := selectRepository(reader, opts.repositoryClient, opts.orgID)
		if err != nil {
			return fmt.Errorf("repository selection failed: %w", err)
		}
		opts.repoID = selectedRepoID
		fmt.Println()
	}

	if opts.configRepoID == "" {
		yOrN := promptTillYOrN(reader)
		if yOrN == "y" {
			fmt.Println("📝 For the config repository, you'll need to provide another repository ID.")
			configRepoID, err := selectRepository(reader, opts.repositoryClient, opts.orgID)
			if err != nil {
				return fmt.Errorf("config repository selection failed: %w", err)
			}
			opts.configRepoID = configRepoID
		} else {
			opts.configRepoID = opts.repoID
		}
	}

	if opts.filePath == "" {
		opts.filePath = reader.ReadStringFromUser("Enter the path to your CircleCI config file", ".circleci/config.yml", nil)
	}

	if opts.pipelineName == "" || opts.repoID == "" || opts.configRepoID == "" || opts.filePath == "" {
		return fmt.Errorf("all pipeline fields are required: pipeline-name, repo-id, config-repo-id, and file-path")
	}

	fmt.Printf("📋 Creating pipeline '%s' for project '%s'...\n", opts.pipelineName, projectRes.Name)
	trackInitStep(cmd, "pipeline_creating", nil)

	pipelineRes, err := opts.pipelineClient.CreatePipeline(
		projectRes.Id,
		opts.pipelineName,
		opts.pipelineDescription,
		opts.repoID,
		opts.configRepoID,
		opts.filePath,
	)
	if err != nil {
		trackInitStep(cmd, "pipeline_failed", nil)
		fmt.Printf("❌ Failed to create pipeline: %v\n", err)
		fmt.Println("💡 Make sure you have the GitHub App installed in your repository")
		fmt.Println("   Visit: https://github.com/apps/circleci")
		return fmt.Errorf("failed to create pipeline: %w", err)
	}

	trackInitStep(cmd, "pipeline_created", nil)
	fmt.Printf("✅ Pipeline '%s' successfully created for repository '%s'\n", pipelineRes.Name, pipelineRes.CheckoutSourceRepoFullName)
	if pipelineRes.CheckoutSourceRepoFullName != pipelineRes.ConfigSourceRepoFullName {
		fmt.Printf("   Config referenced from '%s' repository at path '%s'\n", pipelineRes.ConfigSourceRepoFullName, opts.filePath)
	}

	fmt.Println("⚡ Creating trigger for the pipeline...")
	fmt.Println("   Triggers determine when your pipeline runs - on code pushes, pull requests, or custom webhooks.")

	if opts.eventPreset == "" {
		fmt.Println("📋 Event Preset Selection")
		selectedPreset, err := selectEventPreset()
		if err != nil {
			return fmt.Errorf("event preset selection failed: %w", err)
		}
		opts.eventPreset = selectedPreset
		fmt.Println()
	}

	fmt.Printf("✅ Using trigger event: %s\n", opts.eventPreset)

	pipelineOptions := pipelineapi.GetPipelineDefinitionOptions{
		ProjectID:            projectRes.Id,
		PipelineDefinitionID: pipelineRes.Id,
	}
	pipelineResp, err := opts.pipelineClient.GetPipelineDefinition(pipelineOptions)
	if err != nil {
		fmt.Printf("❌ Failed to get pipeline definition: %v\n", err)
		return fmt.Errorf("failed to get pipeline definition: %w", err)
	}

	if opts.configRef == "" && pipelineResp.ConfigSourceId != opts.repoID {
		opts.configRef = reader.ReadStringFromUser("Your pipeline repo and config source repo are different. Enter the branch or tag to use when fetching config for pipeline runs", "", nil)
	}

	if opts.checkoutRef == "" && pipelineResp.CheckoutSourceId != opts.repoID {
		opts.checkoutRef = reader.ReadStringFromUser("Your pipeline repo and checkout source repo are different. Enter the branch or tag to use when checking out code for pipeline runs", "", nil)
	}

	fmt.Printf("⚡ Creating trigger for pipeline '%s'...\n", pipelineRes.Name)
	trackInitStep(cmd, "trigger_creating", nil)

	triggerOptions := triggerapi.CreateTriggerOptions{
		ProjectID:            projectRes.Id,
		PipelineDefinitionID: pipelineRes.Id,
		RepoID:               opts.repoID,
		EventPreset:          opts.eventPreset,
		ConfigRef:            opts.configRef,
		CheckoutRef:          opts.checkoutRef,
	}

	triggerRes, err := opts.triggerClient.CreateTrigger(triggerOptions)
	if err != nil {
		trackInitStep(cmd, "trigger_failed", nil)
		fmt.Printf("❌ Failed to create trigger: %v\n", err)
		fmt.Println("💡 Make sure you have the GitHub App installed in your repository")
		fmt.Println("   Visit: https://github.com/apps/circleci")
		return fmt.Errorf("failed to create trigger: %w", err)
	}

	trackInitStep(cmd, "trigger_created", nil)
	fmt.Printf("✅ Trigger successfully created!\n")
	fmt.Println()

	fmt.Println("🎉 Project initialization completed successfully! Summary:")
	fmt.Printf("   ✅ Project: %s (ID: %s)\n", projectRes.Name, projectRes.Id)
	fmt.Printf("   ✅ Pipeline: %s (ID: %s)\n", pipelineRes.Name, pipelineRes.Id)
	fmt.Printf("   ✅ Trigger: (ID: %s)\n", triggerRes.Id)
	fmt.Println()
	fmt.Println("🔗 Useful links:")
	fmt.Printf("   Project: https://app.circleci.com/projects/%s\n", projectRes.Slug)
	fmt.Printf("   Pipeline settings: https://app.circleci.com/settings/project/%s/configurations\n", projectRes.Slug)
	fmt.Printf("   Trigger settings: https://app.circleci.com/settings/project/%s/triggers\n", projectRes.Slug)
	fmt.Println()
	fmt.Println("📝 Next steps:")
	fmt.Printf("   1. Make sure you have a '%s' file in your repository\n", opts.filePath)
	fmt.Println("   2. Push code to your repository to trigger your first pipeline run")
	fmt.Println("   3. Monitor your pipeline runs in the CircleCI dashboard")
	fmt.Println()
	fmt.Println("🎊 Your CircleCI project is now fully configured and ready to use!")
	fmt.Println("   To create additional pipelines or triggers, run the `circleci pipeline create` or `circleci trigger create` commands.")

	trackInitStep(cmd, "succeeded", nil)
	return nil
}

func trackInitStep(cmd *cobra.Command, step string, extra map[string]interface{}) {
	client, ok := telemetry.FromContext(cmd.Context())
	if !ok {
		return
	}
	invID, _ := telemetry.InvocationIDFromContext(cmd.Context())
	telemetry.TrackWorkflowStep(client, "init", step, invID, extra)
}
