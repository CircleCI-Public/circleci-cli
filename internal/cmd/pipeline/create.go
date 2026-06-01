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

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

var validConfigProviders = []string{"github_app", "github_server", "circleci"}
var validCheckoutProviders = []string{"github_app", "github_server"}

// repoConfigProviders are config source providers that require a repo external ID.
var repoConfigProviders = map[string]bool{
	"github_app":    true,
	"github_server": true,
}

func newCreateCmd() *cobra.Command {
	var (
		projectSlug      string
		projectID        string
		name             string
		description      string
		configProvider   string
		configRepoID     string
		configFile       string
		checkoutProvider string
		checkoutRepoID   string
		jsonOut          bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pipeline definition",
		Long: heredoc.Docf(`
			Create a new pipeline definition for a CircleCI project.

			A pipeline definition specifies where CircleCI finds the config YAML
			(%[1]s--config-file%[1]s) and which repository to use for code checkout.
			Once created, you can attach triggers to it with %[1]scircleci project trigger create%[1]s.

			The project is resolved from --project-id if provided; otherwise the
			project slug (--project or git remote) is used to look up the project UUID.

			All required flags must be provided in non-interactive mode (CI, agents).
			In a terminal, missing values are prompted interactively.

			Valid --config-provider values: %[2]s
			Valid --checkout-provider values: %[3]s

			JSON fields: id, name, description, created_at,
			             config_source.provider, config_source.file_path,
			             config_source.repo.external_id, config_source.repo.full_name,
			             checkout_source.provider,
			             checkout_source.repo.external_id, checkout_source.repo.full_name
		`, "`", strings.Join(validConfigProviders, ", "), strings.Join(validCheckoutProviders, ", ")),
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

			# Create using a direct project UUID (skips project info lookup)
			$ circleci pipeline create \
			    --project-id a1b2c3d4-... \
			    --name "nightly" \
			    --config-provider github_app \
			    --config-repo-id 123456789 \
			    --config-file .circleci/nightly.yml \
			    --checkout-provider github_app \
			    --checkout-repo-id 123456789
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runCreate(ctx, client, projectSlug, projectID, name, description,
				configProvider, configRepoID, configFile,
				checkoutProvider, checkoutRepoID, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project UUID (overrides --project)")
	cmd.Flags().StringVar(&name, "name", "", "Pipeline definition name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Pipeline definition description")
	cmd.Flags().StringVar(&configProvider, "config-provider", "", fmt.Sprintf("Config source provider (one of: %s)", strings.Join(validConfigProviders, ", ")))
	cmd.Flags().StringVar(&configRepoID, "config-repo-id", "", "Config source repo external ID (required for github_app, github_server)")
	cmd.Flags().StringVar(&configFile, "config-file", "", "Config file path (e.g. .circleci/config.yml)")
	cmd.Flags().StringVar(&checkoutProvider, "checkout-provider", "", fmt.Sprintf("Checkout source provider (one of: %s)", strings.Join(validCheckoutProviders, ", ")))
	cmd.Flags().StringVar(&checkoutRepoID, "checkout-repo-id", "", "Checkout source repo external ID")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
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

func runCreate(
	ctx context.Context,
	client *apiclient.Client,
	projectSlug, projectID string,
	name, description string,
	configProvider, configRepoID, configFile string,
	checkoutProvider, checkoutRepoID string,
	jsonOut bool,
) error {
	resolvedProjectID, err := cmdutil.ResolveProjectID(ctx, client, projectSlug, projectID)
	if err != nil {
		return err
	}

	name, err = resolveRequired(ctx, name, "Pipeline definition name", "e.g. my-pipeline", "", "--name is required")
	if err != nil {
		return err
	}

	configProvider, err = resolveRequiredSelect(ctx, configProvider, "Config source provider", validConfigProviders, "--config-provider is required")
	if err != nil {
		return err
	}
	if err := validateConfigProvider(configProvider); err != nil {
		return err
	}

	// Fetch existing pipeline definitions once for interactive repo selection.
	// Errors are ignored — we fall back gracefully to manual entry.
	var repoOpts []repoOption
	if iostream.IsInteractive(ctx) {
		defs, _ := client.ListPipelineDefinitions(ctx, resolvedProjectID)
		repoOpts = collectRepoOptions(defs)
	}

	if repoConfigProviders[configProvider] {
		configRepoID, err = resolveRepoIDFromOptions(ctx, repoOpts, configRepoID, "Config repo external ID", "--config-repo-id is required for provider "+configProvider)
		if err != nil {
			return err
		}
	}

	configFile, err = resolveRequired(ctx, configFile, "Config file path", ".circleci/config.yml", ".circleci/config.yml", "--config-file is required")
	if err != nil {
		return err
	}

	checkoutProvider, err = resolveRequiredSelect(ctx, checkoutProvider, "Checkout source provider", validCheckoutProviders, "--checkout-provider is required")
	if err != nil {
		return err
	}
	if err := validateCheckoutProvider(checkoutProvider); err != nil {
		return err
	}

	checkoutRepoID, err = resolveCheckoutRepoID(ctx, repoOpts, checkoutRepoID, configRepoID)
	if err != nil {
		return err
	}

	resp, err := client.CreatePipelineDefinition(ctx, resolvedProjectID, apiclient.CreatePipelineDefinitionInput{
		Name:             name,
		Description:      description,
		ConfigProvider:   configProvider,
		ConfigRepoID:     configRepoID,
		ConfigFilePath:   configFile,
		CheckoutProvider: checkoutProvider,
		CheckoutRepoID:   checkoutRepoID,
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

	if jsonOut {
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

type repoOption struct {
	id    string
	label string
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
			opts = append(opts, repoOption{id: src.Repo.ExternalID, label: label})
		}
	}
	return opts
}

func pickFromRepoOptions(ctx context.Context, opts []repoOption, prompt string) (string, error) {
	if len(opts) == 0 {
		return promptRepoID(ctx, prompt)
	}

	const enterManually = "Enter manually..."
	labels := make([]string, len(opts)+1)
	for i, o := range opts {
		labels[i] = o.label
	}
	labels[len(opts)] = enterManually

	idx, err := iostream.PromptSelect(ctx, prompt, labels)
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", clierrors.New("create.cancelled", "Aborted", "No repo selected.").
			WithExitCode(clierrors.ExitCancelled)
	}
	if labels[idx] == enterManually {
		return promptRepoID(ctx, prompt)
	}
	return opts[idx].id, nil
}

func resolveRepoIDFromOptions(ctx context.Context, opts []repoOption, val, prompt, errMsg string) (string, error) {
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
	return pickFromRepoOptions(ctx, opts, prompt)
}

func resolveCheckoutRepoID(ctx context.Context, opts []repoOption, val, configRepoID string) (string, error) {
	if val != "" {
		return val, nil
	}
	if !iostream.IsInteractive(ctx) {
		return "", clierrors.New("args.missing_flag", "Missing required flag", "--checkout-repo-id is required").
			WithSuggestions(
				"Pass --checkout-repo-id <value>",
				"Run with --help for flag descriptions",
			).
			WithExitCode(clierrors.ExitBadArguments)
	}

	if configRepoID == "" {
		return pickFromRepoOptions(ctx, opts, "Checkout repo external ID")
	}

	configLabel := configRepoID
	for _, o := range opts {
		if o.id == configRepoID {
			configLabel = o.label
			break
		}
	}

	const other = "Other..."
	idx, err := iostream.PromptSelect(ctx, "Checkout repo external ID", []string{configLabel, other})
	if err != nil {
		return "", err
	}
	switch idx {
	case -1:
		return "", clierrors.New("create.cancelled", "Aborted", "No repo selected.").
			WithExitCode(clierrors.ExitCancelled)
	case 0:
		return configRepoID, nil
	default:
		return pickFromRepoOptions(ctx, opts, "Checkout repo external ID")
	}
}

func promptRepoID(ctx context.Context, prompt string) (string, error) {
	v, err := iostream.PromptText(ctx, prompt, "e.g. 123456789")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", clierrors.New("create.cancelled", "Aborted", "No value entered for: "+prompt).
			WithExitCode(clierrors.ExitCancelled)
	}
	return v, nil
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
