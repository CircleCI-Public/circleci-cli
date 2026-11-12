// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package pipeline

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/provider"
)

var validConfigFileTypes = []string{configFileTypeGitHubActions}

const (
	configFileTypeGitHubActions = "github-actions"
	// workflowDir is where GitHub reads workflow files from, and where the
	// CircleCI docs and web UI put a github-actions config.
	workflowDir          = ".github/workflows"
	defaultWorkflowPath  = workflowDir + "/ci.yml"
	gitHubActionsDocsURL = "https://circleci.com/docs/guides/orchestrate/use-a-github-actions-workflow-file-as-your-pipeline-configuration/"
)

// createInput is the flag set of `pipeline create`. The flags are passed as one
// value rather than a dozen positional strings, so two adjacent ones cannot be
// transposed at the call site without the compiler noticing.
type createInput struct {
	projectSlug          string
	projectID            string
	name                 string
	description          string
	configProvider       string
	configRepoID         string
	configRepoFullName   string
	configFile           string
	configFileType       string
	checkoutProvider     string
	checkoutRepoID       string
	checkoutRepoFullName string
	allowNonstandardPath bool
	jsonOut              bool
}

func newCreateCmd() *cobra.Command {
	var in createInput

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pipeline definition",
		Long: heredoc.Docf(`
			Set where CircleCI finds config and what it checks out. Next: %[1]sproject trigger create%[1]s.

			JSON fields: id, name, description, created_at, config_source.provider, config_source.file_path, config_source.repo.external_id, config_source.repo.full_name, checkout_source.provider, checkout_source.repo.external_id, checkout_source.repo.full_name
		`, "`"),
		Example: heredoc.Doc(`
			# Create a pipeline definition using GitHub App
			$ circleci pipeline create \
			    --project gh/myorg/myrepo \
			    --name "my-pipeline" \
			    --config-provider github_app \
			    --config-repo-id 123456789 \
			    --config-file .circleci/config.yml \
			    --checkout-provider github_app \
			    --checkout-repo-id 123456789

			# Create with a description and output as JSON
			$ circleci pipeline create \
			    --name "release-pipeline" \
			    --description "Runs on tagged releases" \
			    --config-provider github_app \
			    --config-repo-id 123456789 \
			    --config-file .circleci/release.yml \
			    --checkout-provider github_app \
			    --checkout-repo-id 123456789 \
			    --json

			# Create for a Cursor repository (Origin needs the owner/name too)
			$ circleci pipeline create \
			    --project-id a1b2c3d4-... \
			    --name "my-pipeline" \
			    --config-provider origin \
			    --config-repo-id repo_01abc \
			    --config-repo-full-name myorg/myrepo \
			    --config-file .circleci/config.yml \
			    --checkout-provider origin \
			    --checkout-repo-id repo_01abc \
			    --checkout-repo-full-name myorg/myrepo
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runCreate(ctx, client, in)
		},
	}

	cmd.Flags().StringVar(&in.projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&in.projectID, "project-id", "", "Project UUID (overrides --project)")
	cmd.Flags().StringVar(&in.name, "name", "", "Pipeline definition name (required)")
	cmd.Flags().StringVar(&in.description, "description", "", "Pipeline definition description")
	cmd.Flags().StringVar(&in.configProvider, "config-provider", "", fmt.Sprintf("Config source provider (one of: %s)", provider.List(provider.ConfigSource)))
	cmd.Flags().StringVar(&in.configRepoID, "config-repo-id", "", fmt.Sprintf("Config source repo external ID (required for %s)",
		provider.List(provider.ConfigSource, provider.Provider.NeedsRepoID)))
	cmd.Flags().StringVar(&in.configRepoFullName, "config-repo-full-name", "", fmt.Sprintf("Config source repo owner/name (required for %s; enough on its own for %s)",
		provider.List(provider.ConfigSource, provider.Provider.NeedsFullName),
		provider.List(provider.ConfigSource, provider.Provider.FullNameIsEnough)))
	cmd.Flags().StringVar(&in.configFile, "config-file", "", "Config file path (e.g. .circleci/config.yml)")
	cmd.Flags().StringVar(&in.configFileType, "config-file-type", "", fmt.Sprintf("Config file type, omit for standard CircleCI YAML (one of: %s)", strings.Join(validConfigFileTypes, ", ")))
	cmd.Flags().StringVar(&in.checkoutProvider, "checkout-provider", "", fmt.Sprintf("Checkout source provider (one of: %s)", provider.List(provider.CheckoutSource)))
	cmd.Flags().StringVar(&in.checkoutRepoID, "checkout-repo-id", "", "Checkout source repo external ID")
	cmd.Flags().StringVar(&in.checkoutRepoFullName, "checkout-repo-full-name", "", fmt.Sprintf("Checkout source repo owner/name (required for %s; enough on its own for %s)",
		provider.List(provider.CheckoutSource, provider.Provider.NeedsFullName),
		provider.List(provider.CheckoutSource, provider.Provider.FullNameIsEnough)))
	cmd.Flags().BoolVar(&in.allowNonstandardPath, "allow-nonstandard-path", false, "allow a github-actions config file outside .github/workflows/")
	cmdutil.AddJSONFlag(cmd, &in.jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type createOutput struct {
	ID             string                              `json:"id"`
	Name           string                              `json:"name"`
	Description    string                              `json:"description,omitempty"`
	CreatedAt      string                              `json:"created_at"`
	ConfigSource   *apiclient.PipelineDefinitionSource `json:"config_source,omitempty"`
	CheckoutSource *apiclient.PipelineDefinitionSource `json:"checkout_source,omitempty"`
}

func runCreate(ctx context.Context, client *apiclient.Client, in createInput) error {
	resolvedProjectID, err := cmdutil.ResolveProjectID(ctx, client, in.projectSlug, in.projectID)
	if err != nil {
		return err
	}

	in.name, err = resolveRequired(ctx, in.name, "Pipeline definition name", "e.g. my-pipeline", "", "--name is required")
	if err != nil {
		return err
	}

	in.configProvider, err = resolveRequiredSelect(ctx, in.configProvider, "Config source provider", provider.Names(provider.ConfigSource), "--config-provider is required")
	if err != nil {
		return err
	}
	configP, err := validateConfigProvider(in.configProvider)
	if err != nil {
		return err
	}
	if in.configFileType != "" {
		if err := validateConfigFileType(in.configFileType); err != nil {
			return err
		}
	}

	// Fetch existing pipeline definitions once for interactive repo selection.
	// Errors are ignored — we fall back gracefully to manual entry.
	var repoOpts []repoOption
	if iostream.IsInteractive(ctx) {
		defs, _ := client.ListPipelineDefinitions(ctx, resolvedProjectID)
		repoOpts = collectRepoOptions(defs)
	}

	// configRepo stays zero for a hosted config (the circleci provider), which has
	// no repository at all; the checkout then has nothing to default to.
	var configRepo repoOption
	if configP.HasRepo() {
		configRepo, err = resolveRepoFromOptions(ctx, configP, repoOpts, in.configRepoID, in.configRepoFullName, configRepoFlags)
		if err != nil {
			return err
		}
		in.configRepoID = configRepo.id
		in.configRepoFullName, err = resolveRepoFullName(ctx, configP, in.configRepoFullName, configRepo,
			"Config repo owner/name", "--config-repo-full-name is required for provider "+in.configProvider)
		if err != nil {
			return err
		}
	}

	defaultConfigFile := ".circleci/config.yml"
	if in.configFileType == configFileTypeGitHubActions {
		defaultConfigFile = defaultWorkflowPath
	}
	in.configFile, err = resolveRequired(ctx, in.configFile, "Config file path", defaultConfigFile, defaultConfigFile, "--config-file is required")
	if err != nil {
		return err
	}
	if err := checkWorkflowPath(ctx, in); err != nil {
		return err
	}

	in.checkoutProvider, err = resolveRequiredSelect(ctx, in.checkoutProvider, "Checkout source provider", provider.Names(provider.CheckoutSource), "--checkout-provider is required")
	if err != nil {
		return err
	}
	checkoutP, err := validateCheckoutProvider(in.checkoutProvider)
	if err != nil {
		return err
	}

	checkoutRepo, err := resolveCheckoutRepo(ctx, checkoutP, repoOpts, in.checkoutRepoID, in.checkoutRepoFullName, configRepo)
	if err != nil {
		return err
	}
	in.checkoutRepoID = checkoutRepo.id
	in.checkoutRepoFullName, err = resolveRepoFullName(ctx, checkoutP, in.checkoutRepoFullName, checkoutRepo,
		"Checkout repo owner/name", "--checkout-repo-full-name is required for provider "+in.checkoutProvider)
	if err != nil {
		return err
	}

	resp, err := client.CreatePipelineDefinition(ctx, resolvedProjectID, apiclient.CreatePipelineDefinitionInput{
		Name:                 in.name,
		Description:          in.description,
		ConfigProvider:       in.configProvider,
		ConfigRepoID:         in.configRepoID,
		ConfigRepoFullName:   in.configRepoFullName,
		ConfigFilePath:       in.configFile,
		ConfigFileType:       in.configFileType,
		CheckoutProvider:     in.checkoutProvider,
		CheckoutRepoID:       in.checkoutRepoID,
		CheckoutRepoFullName: in.checkoutRepoFullName,
	})
	if err != nil {
		return cmdutil.APIErr(err, resolvedProjectID,
			"pipeline_definition.create_failed",
			"Failed to create pipeline definition for project %q.",
			"Check that the project ID and repo external IDs are correct",
			"Visit: https://app.circleci.com/settings/project/circleci/<org>/<project>/configurations")
	}

	out := createOutput{
		ID:             resp.ID,
		Name:           resp.Name,
		Description:    resp.Description,
		CreatedAt:      cmdutil.JSONTime(resp.CreatedAt),
		ConfigSource:   resp.ConfigSource,
		CheckoutSource: resp.CheckoutSource,
	}

	if in.jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	iostream.Printf(ctx, "Pipeline definition created (ID: %s)\n", out.ID)
	return nil
}

func resolveRequired(ctx context.Context, val, prompt, placeholder, defaultVal, errMsg string) (string, error) {
	if val != "" {
		return val, nil
	}
	if !iostream.IsInteractive(ctx) {
		return "", clierrors.New("args.missing_flag", "Missing required flag", errMsg).
			WithSuggestions(
				"Pass "+strings.Fields(errMsg)[0]+" <value>",
				"Run with --help for flag descriptions",
			).
			WithExitCode(clierrors.ExitBadArguments)
	}
	v, err := iostream.PromptText(ctx, prompt, placeholder, defaultVal)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", clierrors.New("create.cancelled", "Aborted", "No value entered for: "+prompt).
			WithExitCode(clierrors.ExitCancelled)
	}
	return v, nil
}

func resolveRequiredSelect(ctx context.Context, val, prompt string, options []string, errMsg string) (string, error) {
	if val != "" {
		return val, nil
	}
	if !iostream.IsInteractive(ctx) {
		return "", clierrors.New("args.missing_flag", "Missing required flag", errMsg).
			WithSuggestions(
				"Pass "+strings.Fields(errMsg)[0]+" <value>",
				"Valid values: "+strings.Join(options, ", "),
			).
			WithExitCode(clierrors.ExitBadArguments)
	}
	idx, err := iostream.PromptSelect(ctx, prompt, options)
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", clierrors.New("create.cancelled", "Aborted", "No value selected for: "+prompt).
			WithExitCode(clierrors.ExitCancelled)
	}
	return options[idx], nil
}

// repoOption is a repository the project already has a pipeline definition for.
// It carries the full name as well as the id because a provider that addresses
// repositories by owner and name needs both, and an existing definition is an
// authoritative source for the pairing.
type repoOption struct {
	id       string
	fullName string
	label    string
}

func collectRepoOptions(defs []apiclient.PipelineDefinition) []repoOption {
	seen := map[string]bool{}
	var opts []repoOption
	for _, d := range defs {
		for _, src := range []*apiclient.PipelineDefinitionSource{d.ConfigSource, d.CheckoutSource} {
			if src == nil || src.Repo == nil || src.Repo.ExternalID == "" || seen[src.Repo.ExternalID] {
				continue
			}
			seen[src.Repo.ExternalID] = true
			label := src.Repo.ExternalID
			if src.Repo.FullName != "" {
				label = fmt.Sprintf("%s (%s)", src.Repo.FullName, src.Repo.ExternalID)
			}
			opts = append(opts, repoOption{
				id:       src.Repo.ExternalID,
				fullName: src.Repo.FullName,
				label:    label,
			})
		}
	}
	return opts
}

// repoFor returns the option describing the repository with this id, so a
// flag-supplied id still picks up the full name and label the project already
// knows. An id no definition names yields an option carrying just the id.
func repoFor(opts []repoOption, id string) repoOption {
	for _, o := range opts {
		if o.id == id {
			return o
		}
	}
	return repoOption{id: id}
}

func pickFromRepoOptions(ctx context.Context, p provider.Provider, opts []repoOption, f repoFlags) (repoOption, error) {
	prompt := f.subject + " external ID"
	if len(opts) == 0 {
		return promptRepo(ctx, p, f)
	}

	const enterManually = "Enter manually..."
	labels := make([]string, len(opts)+1)
	for i, o := range opts {
		labels[i] = o.label
	}
	labels[len(opts)] = enterManually

	idx, err := iostream.PromptSelect(ctx, prompt, labels)
	if err != nil {
		return repoOption{}, err
	}
	if idx < 0 {
		return repoOption{}, clierrors.New("create.cancelled", "Aborted", "No repo selected.").
			WithExitCode(clierrors.ExitCancelled)
	}
	if labels[idx] == enterManually {
		return promptRepo(ctx, p, f)
	}
	return opts[idx], nil
}

// repoFlags names the flags and prompt subject for one source's repository.
type repoFlags struct {
	subject  string // "Config repo", the start of every prompt for it
	id       string // the external id flag
	fullName string // the owner/name flag
}

var (
	configRepoFlags   = repoFlags{subject: "Config repo", id: "--config-repo-id", fullName: "--config-repo-full-name"}
	checkoutRepoFlags = repoFlags{subject: "Checkout repo", id: "--checkout-repo-id", fullName: "--checkout-repo-full-name"}
)

// repoFromFlags returns the repository the flags name, and whether they name
// one: an id, or an owner/name alone for a provider that resolves by name.
func repoFromFlags(p provider.Provider, opts []repoOption, id, fullName string) (repoOption, bool) {
	if id != "" {
		return repoFor(opts, id), true
	}
	if fullName != "" && p.FullNameIsEnough() {
		return repoOption{fullName: fullName, label: fullName}, true
	}
	return repoOption{}, false
}

// missingRepoErr is the non-interactive error for a source that names no
// repository. A provider that also resolves a repository by owner/name accepts
// either flag, so the error names both.
func missingRepoErr(p provider.Provider, f repoFlags) error {
	if p.FullNameIsEnough() {
		return clierrors.New("args.missing_flag", "Missing required flag",
			f.id+" or "+f.fullName+" is required for provider "+p.Name).
			WithSuggestions(
				"Pass "+f.fullName+" <owner>/<repo>",
				"Or pass "+f.id+" <value>",
			).
			WithExitCode(clierrors.ExitBadArguments)
	}
	return clierrors.New("args.missing_flag", "Missing required flag",
		f.id+" is required for provider "+p.Name).
		WithSuggestions(
			"Pass "+f.id+" <value>",
			"Run with --help for flag descriptions",
		).
		WithExitCode(clierrors.ExitBadArguments)
}

func resolveRepoFromOptions(ctx context.Context, p provider.Provider, opts []repoOption, id, fullName string, f repoFlags) (repoOption, error) {
	if repo, ok := repoFromFlags(p, opts, id, fullName); ok {
		return repo, nil
	}
	if !iostream.IsInteractive(ctx) {
		return repoOption{}, missingRepoErr(p, f)
	}
	return pickFromRepoOptions(ctx, p, opts, f)
}

func resolveCheckoutRepo(ctx context.Context, p provider.Provider, opts []repoOption, id, fullName string, configRepo repoOption) (repoOption, error) {
	if repo, ok := repoFromFlags(p, opts, id, fullName); ok {
		return repo, nil
	}
	if !iostream.IsInteractive(ctx) {
		return repoOption{}, missingRepoErr(p, checkoutRepoFlags)
	}

	// A config repo named by owner/name alone has no id, but is still the
	// checkout's natural default.
	if configRepo.id == "" && configRepo.fullName == "" {
		return pickFromRepoOptions(ctx, p, opts, checkoutRepoFlags)
	}

	configLabel := configRepo.label
	if configLabel == "" {
		configLabel = configRepo.id
	}

	const other = "Other..."
	idx, err := iostream.PromptSelect(ctx, "Checkout repo external ID", []string{configLabel, other})
	if err != nil {
		return repoOption{}, err
	}
	switch idx {
	case -1:
		return repoOption{}, clierrors.New("create.cancelled", "Aborted", "No repo selected.").
			WithExitCode(clierrors.ExitCancelled)
	case 0:
		return configRepo, nil
	default:
		return pickFromRepoOptions(ctx, p, opts, checkoutRepoFlags)
	}
}

// resolveRepoFullName returns the repo full name to send for a source. For a
// provider that keys repositories by id the name is decorative, so whatever was
// passed is forwarded untouched and nothing is guessed. For a provider that
// addresses repositories by owner and name it is used when known: an explicit
// flag wins, then the name carried by the repository that was resolved. Only a
// provider that needs the name prompts for it.
func resolveRepoFullName(
	ctx context.Context, p provider.Provider, val string, repo repoOption, prompt, errMsg string,
) (string, error) {
	if val != "" || (!p.NeedsFullName() && !p.FullNameIsEnough()) {
		return val, nil
	}
	if repo.fullName != "" {
		return repo.fullName, nil
	}
	if !p.NeedsFullName() {
		return "", nil
	}
	return resolveRequired(ctx, "", prompt, "e.g. myorg/myrepo", "", errMsg)
}

// promptRepo asks for the repository by hand: by owner/name for a provider that
// resolves repositories that way, since that is what a user knows, else by id.
func promptRepo(ctx context.Context, p provider.Provider, f repoFlags) (repoOption, error) {
	if p.FullNameIsEnough() {
		prompt := f.subject + " owner/name"
		v, err := iostream.PromptText(ctx, prompt, "e.g. myorg/myrepo")
		if err != nil {
			return repoOption{}, err
		}
		if v == "" {
			return repoOption{}, clierrors.New("create.cancelled", "Aborted", "No value entered for: "+prompt).
				WithExitCode(clierrors.ExitCancelled)
		}
		return repoOption{fullName: v, label: v}, nil
	}
	prompt := f.subject + " external ID"
	v, err := iostream.PromptText(ctx, prompt, p.RepoIDExample)
	if err != nil {
		return repoOption{}, err
	}
	if v == "" {
		return repoOption{}, clierrors.New("create.cancelled", "Aborted", "No value entered for: "+prompt).
			WithExitCode(clierrors.ExitCancelled)
	}
	return repoOption{id: v}, nil
}

// isStandardWorkflowPath reports whether p is a YAML file directly in
// .github/workflows/. GitHub does not read subdirectories of it.
func isStandardWorkflowPath(p string) bool {
	p = path.Clean(p)
	ext := path.Ext(p)
	return path.Dir(p) == workflowDir && (ext == ".yml" || ext == ".yaml")
}

// checkWorkflowPath guards against a github-actions definition pointing at a
// file that is probably a CircleCI config. CircleCI would translate that file
// as a GitHub Actions workflow and produce jobs that do nothing and pass.
func checkWorkflowPath(ctx context.Context, in createInput) error {
	if in.configFileType != configFileTypeGitHubActions || in.allowNonstandardPath || isStandardWorkflowPath(in.configFile) {
		return nil
	}

	streams := iostream.Get(ctx)
	if !streams.IsInteractive() {
		return clierrors.New("args.nonstandard_workflow_path", "Config file is not a GitHub Actions workflow path",
			fmt.Sprintf("%s is not in %s/, where GitHub Actions workflow files live. CircleCI will run it as a GitHub Actions workflow; a CircleCI config run this way produces jobs that do nothing and pass.",
				in.configFile, workflowDir)).
			WithSuggestions(
				"Omit --config-file-type if this is a CircleCI config",
				"Pass --allow-nonstandard-path to create it anyway",
			).
			WithRef(gitHubActionsDocsURL).
			WithExitCode(clierrors.ExitBadArguments)
	}

	iostream.ErrPrintf(ctx, "%s %s\n", iostream.SymbolWarn(ctx), iostream.Warning(ctx,
		fmt.Sprintf("%s is not in %s/. CircleCI will run it as a GitHub Actions workflow; a CircleCI config run this way produces jobs that do nothing and pass.",
			in.configFile, workflowDir)))
	if !streams.Confirm(ctx, "Create the pipeline definition anyway?") {
		return clierrors.New("pipeline_definition.create_aborted", "Create aborted", "User cancelled the create.").
			WithRef(gitHubActionsDocsURL).
			WithExitCode(clierrors.ExitCancelled)
	}
	return nil
}

func validateConfigProvider(v string) (provider.Provider, error) {
	if p, ok := provider.Lookup(v, provider.ConfigSource); ok {
		return p, nil
	}
	return provider.Provider{}, clierrors.New("args.invalid_config_provider", "Invalid --config-provider value",
		fmt.Sprintf("%q is not a valid config provider.", v)).
		WithSuggestions("Valid values: " + provider.List(provider.ConfigSource)).
		WithExitCode(clierrors.ExitBadArguments)
}

func validateConfigFileType(v string) *clierrors.CLIError {
	for _, valid := range validConfigFileTypes {
		if v == valid {
			return nil
		}
	}
	return clierrors.New("args.invalid_config_file_type", "Invalid --config-file-type value",
		fmt.Sprintf("%q is not a valid config file type.", v)).
		WithSuggestions("Valid values: " + strings.Join(validConfigFileTypes, ", ")).
		WithExitCode(clierrors.ExitBadArguments)
}

func validateCheckoutProvider(v string) (provider.Provider, error) {
	if p, ok := provider.Lookup(v, provider.CheckoutSource); ok {
		return p, nil
	}
	return provider.Provider{}, clierrors.New("args.invalid_checkout_provider", "Invalid --checkout-provider value",
		fmt.Sprintf("%q is not a valid checkout provider.", v)).
		WithSuggestions("Valid values: " + provider.List(provider.CheckoutSource)).
		WithExitCode(clierrors.ExitBadArguments)
}
