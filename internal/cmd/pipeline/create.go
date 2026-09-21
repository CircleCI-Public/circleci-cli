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
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

var validConfigProviders = []string{"github_app", "github_server", "circleci", "origin"}
var validCheckoutProviders = []string{"github_app", "github_server", "origin"}
var validConfigFileTypes = []string{"github-actions"}

// repoConfigProviders are config source providers that require a repo external ID.
var repoConfigProviders = map[string]bool{
	"github_app":    true,
	"github_server": true,
	"origin":        true,
}

// fullNameProviders are providers that address a repository by owner and name
// rather than by id, and so need the full name as well as the external ID.
// Origin exposes no lookup by id, so the full name is what resolves the
// repository at all; the API rejects a write without it and rejects a pair that
// names two different repositories.
var fullNameProviders = map[string]bool{
	"origin": true,
}

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
	jsonOut              bool
}

func newCreateCmd() *cobra.Command {
	var in createInput

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pipeline definition",
		Long: heredoc.Docf(`
			Create a pipeline definition: where CircleCI finds the config YAML and which
			repository to check out. Add a trigger next with %[1]sproject trigger create%[1]s.

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
	cmd.Flags().StringVar(&in.configProvider, "config-provider", "", fmt.Sprintf("Config source provider (one of: %s)", strings.Join(validConfigProviders, ", ")))
	cmd.Flags().StringVar(&in.configRepoID, "config-repo-id", "", "Config source repo external ID (required for github_app, github_server, origin)")
	cmd.Flags().StringVar(&in.configRepoFullName, "config-repo-full-name", "", "Config source repo owner/name (required for origin)")
	cmd.Flags().StringVar(&in.configFile, "config-file", "", "Config file path (e.g. .circleci/config.yml)")
	cmd.Flags().StringVar(&in.configFileType, "config-file-type", "", fmt.Sprintf("Config file type, omit for standard CircleCI YAML (one of: %s)", strings.Join(validConfigFileTypes, ", ")))
	cmd.Flags().StringVar(&in.checkoutProvider, "checkout-provider", "", fmt.Sprintf("Checkout source provider (one of: %s)", strings.Join(validCheckoutProviders, ", ")))
	cmd.Flags().StringVar(&in.checkoutRepoID, "checkout-repo-id", "", "Checkout source repo external ID")
	cmd.Flags().StringVar(&in.checkoutRepoFullName, "checkout-repo-full-name", "", "Checkout source repo owner/name (required for origin)")
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

	in.configProvider, err = resolveRequiredSelect(ctx, in.configProvider, "Config source provider", validConfigProviders, "--config-provider is required")
	if err != nil {
		return err
	}
	if err := validateConfigProvider(in.configProvider); err != nil {
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
	if repoConfigProviders[in.configProvider] {
		configRepo, err = resolveRepoFromOptions(ctx, in.configProvider, repoOpts, in.configRepoID, "Config repo external ID", "--config-repo-id is required for provider "+in.configProvider)
		if err != nil {
			return err
		}
		in.configRepoID = configRepo.id
		in.configRepoFullName, err = resolveRepoFullName(ctx, in.configProvider, in.configRepoFullName, configRepo,
			"Config repo owner/name", "--config-repo-full-name is required for provider "+in.configProvider)
		if err != nil {
			return err
		}
	}

	in.configFile, err = resolveRequired(ctx, in.configFile, "Config file path", ".circleci/config.yml", ".circleci/config.yml", "--config-file is required")
	if err != nil {
		return err
	}

	in.checkoutProvider, err = resolveRequiredSelect(ctx, in.checkoutProvider, "Checkout source provider", validCheckoutProviders, "--checkout-provider is required")
	if err != nil {
		return err
	}
	if err := validateCheckoutProvider(in.checkoutProvider); err != nil {
		return err
	}

	checkoutRepo, err := resolveCheckoutRepo(ctx, in.checkoutProvider, repoOpts, in.checkoutRepoID, configRepo)
	if err != nil {
		return err
	}
	in.checkoutRepoID = checkoutRepo.id
	in.checkoutRepoFullName, err = resolveRepoFullName(ctx, in.checkoutProvider, in.checkoutRepoFullName, checkoutRepo,
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
		CreatedAt:      resp.CreatedAt.Format(time.RFC3339),
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

func pickFromRepoOptions(ctx context.Context, provider string, opts []repoOption, prompt string) (repoOption, error) {
	if len(opts) == 0 {
		return promptRepoID(ctx, provider, prompt)
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
		return promptRepoID(ctx, provider, prompt)
	}
	return opts[idx], nil
}

func resolveRepoFromOptions(ctx context.Context, provider string, opts []repoOption, val, prompt, errMsg string) (repoOption, error) {
	if val != "" {
		return repoFor(opts, val), nil
	}
	if !iostream.IsInteractive(ctx) {
		return repoOption{}, clierrors.New("args.missing_flag", "Missing required flag", errMsg).
			WithSuggestions(
				"Pass "+strings.Fields(errMsg)[0]+" <value>",
				"Run with --help for flag descriptions",
			).
			WithExitCode(clierrors.ExitBadArguments)
	}
	return pickFromRepoOptions(ctx, provider, opts, prompt)
}

func resolveCheckoutRepo(ctx context.Context, provider string, opts []repoOption, val string, configRepo repoOption) (repoOption, error) {
	if val != "" {
		return repoFor(opts, val), nil
	}
	if !iostream.IsInteractive(ctx) {
		return repoOption{}, clierrors.New("args.missing_flag", "Missing required flag", "--checkout-repo-id is required").
			WithSuggestions(
				"Pass --checkout-repo-id <value>",
				"Run with --help for flag descriptions",
			).
			WithExitCode(clierrors.ExitBadArguments)
	}

	if configRepo.id == "" {
		return pickFromRepoOptions(ctx, provider, opts, "Checkout repo external ID")
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
		return pickFromRepoOptions(ctx, provider, opts, "Checkout repo external ID")
	}
}

// resolveRepoFullName returns the repo full name to send for a source. For a
// provider that keys repositories by id the name is decorative, so whatever was
// passed is forwarded untouched and nothing is guessed. For a provider that
// addresses repositories by owner and name it is required: an explicit flag wins,
// then the name carried by the repository that was resolved, then a prompt.
func resolveRepoFullName(
	ctx context.Context, provider, val string, repo repoOption, prompt, errMsg string,
) (string, error) {
	if !fullNameProviders[provider] {
		return val, nil
	}
	if val != "" {
		return val, nil
	}
	if repo.fullName != "" {
		return repo.fullName, nil
	}
	return resolveRequired(ctx, "", prompt, "e.g. myorg/myrepo", "", errMsg)
}

// repoIDExample is a sample external repository id in the shape the provider
// uses: opaque text for Origin, a number for the GitHub providers.
func repoIDExample(provider string) string {
	if provider == "origin" {
		return "e.g. repo_01abc"
	}
	return "e.g. 123456789"
}

func promptRepoID(ctx context.Context, provider, prompt string) (repoOption, error) {
	v, err := iostream.PromptText(ctx, prompt, repoIDExample(provider))
	if err != nil {
		return repoOption{}, err
	}
	if v == "" {
		return repoOption{}, clierrors.New("create.cancelled", "Aborted", "No value entered for: "+prompt).
			WithExitCode(clierrors.ExitCancelled)
	}
	return repoOption{id: v}, nil
}

func validateConfigProvider(v string) *clierrors.CLIError {
	for _, valid := range validConfigProviders {
		if v == valid {
			return nil
		}
	}
	return clierrors.New("args.invalid_config_provider", "Invalid --config-provider value",
		fmt.Sprintf("%q is not a valid config provider.", v)).
		WithSuggestions("Valid values: " + strings.Join(validConfigProviders, ", ")).
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

func validateCheckoutProvider(v string) *clierrors.CLIError {
	for _, valid := range validCheckoutProviders {
		if v == valid {
			return nil
		}
	}
	return clierrors.New("args.invalid_checkout_provider", "Invalid --checkout-provider value",
		fmt.Sprintf("%q is not a valid checkout provider.", v)).
		WithSuggestions("Valid values: " + strings.Join(validCheckoutProviders, ", ")).
		WithExitCode(clierrors.ExitBadArguments)
}
