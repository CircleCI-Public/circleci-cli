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

package project

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/provider"
)

// validEventPresets lists the allowed values for --event-preset.
var validEventPresets = []string{
	"all-pushes",
	"only-tags",
	"default-branch-pushes",
	"only-build-prs",
	"only-open-prs",
	"only-labeled-prs",
	"only-merged-prs",
	"only-ready-for-review-prs",
	"only-branch-delete",
	"only-build-pushes-to-non-draft-prs",
	"only-merged-or-closed-prs",
	"pr-comment-equals-run-ci",
	"non-draft-pr-opened",
	"pushes-to-merge-queues",
}

func newTriggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger <command>",
		Short: "Manage project triggers",
		Long: heredoc.Doc(`
			List, create, update and delete triggers for a CircleCI project.

			Triggers watch a GitHub repository for events and automatically run
			a pipeline definition when matching events occur. Only projects
			connected via the CircleCI GitHub App are supported.
		`),
	}

	cmd.AddCommand(newTriggerListCmd())
	cmd.AddCommand(newTriggerCreateCmd())
	cmd.AddCommand(newTriggerUpdateCmd())
	cmd.AddCommand(newTriggerDeleteCmd())

	return cmd
}

// resolveProjectID returns the project UUID, looking it up from the slug if needed.
// It uses --project-id directly, then --project slug, then the git remote.
func resolveProjectID(ctx context.Context, client *apiclient.Client, projectSlug, projectID string) (string, error) {
	if projectID != "" {
		return projectID, nil
	}
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return "", cmdutil.GitDetectErr(err, "Or specify the project with --project gh/org/repo or --project-id <uuid>")
		}
		projectSlug = info.Slug
	}
	proj, err := client.GetProjectBySlug(ctx, projectSlug)
	if err != nil {
		return "", cmdutil.APIErr(err, projectSlug, "project.not_found", "No project found for %q.",
			"Run 'circleci project link' to bind this repository to a CircleCI project",
			"Check the project slug and try again",
			"Use 'circleci project list' to see followed projects")
	}
	return proj.ID.String(), nil
}

// resolveOptionalProjectID resolves --project or --project-id when one was given
// and returns "" otherwise. Unlike resolveProjectID it never falls back to the git
// remote: a trigger addressed by its ID needs no project, and the lookup is only
// scoped to one to reach a project's synthetic OAuth trigger.
func resolveOptionalProjectID(ctx context.Context, client *apiclient.Client, projectSlug, projectID string) (string, error) {
	if projectSlug == "" && projectID == "" {
		return "", nil
	}
	return resolveProjectID(ctx, client, projectSlug, projectID)
}

// selectPipelineDefinition fetches the list of pipeline definitions for the project
// and presents an interactive picker. Returns the chosen pipeline definition ID.
// Only called when in interactive mode and --pipeline-definition-id was not given.
func selectPipelineDefinition(ctx context.Context, client *apiclient.Client, projectID string) (string, error) {
	defs, err := client.ListPipelineDefinitions(ctx, projectID)
	if err != nil {
		return "", cmdutil.APIErr(err, projectID, "pipeline_definition.list_failed",
			"Failed to list pipeline definitions for project %q.",
			"Check that the project ID is correct",
			"Visit: https://app.circleci.com/settings/project/circleci/<org>/<project>/configurations")
	}

	if len(defs) == 0 {
		return "", clierrors.New("pipeline_definition.none_found", "No pipeline definitions found",
			"This project has no pipeline definitions yet.").
			WithSuggestions(
				"Create a pipeline definition in your CircleCI project settings under Pipelines",
				"Visit: https://app.circleci.com/settings/project/circleci/<org>/<project>/configurations",
			).
			WithExitCode(clierrors.ExitNotFound)
	}

	labels := make([]string, len(defs))
	for i, d := range defs {
		label := d.Name
		if d.Description != "" {
			label = fmt.Sprintf("%s — %s", d.Name, d.Description)
		}
		labels[i] = label
	}

	idx, err := iostream.PromptSelect(ctx, "Select a pipeline definition", labels)
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", clierrors.New("trigger.cancelled", "Aborted",
			"No pipeline definition selected.").
			WithExitCode(clierrors.ExitCancelled)
	}
	return defs[idx].ID, nil
}

// --- trigger list ---

func newTriggerListCmd() *cobra.Command {
	var (
		projectSlug          string
		projectID            string
		pipelineDefinitionID string
		jsonOut              bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List triggers for a pipeline definition",
		Long: heredoc.Doc(`
			List all triggers attached to a pipeline definition.

			--pipeline-definition-id is required. In a terminal it will be prompted
			interactively if omitted; in non-interactive mode (CI, agents) it must
			be passed as a flag.

			JSON fields: id, created_at, event_name, event_preset, config_ref, checkout_ref, disabled, event_source.provider, event_source.repo.external_id, event_source.repo.full_name
		`),
		Example: heredoc.Doc(`
			# List triggers for the current repository's project
			$ circleci project trigger list \
			    --pipeline-definition-id a1b2c3d4-...

			# List triggers for a specific project
			$ circleci project trigger list \
			    --project gh/myorg/myrepo \
			    --pipeline-definition-id a1b2c3d4-...

			# Output as JSON for scripting
			$ circleci project trigger list \
			    --pipeline-definition-id a1b2c3d4-... \
			    --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runTriggerList(ctx, client, projectSlug, projectID, pipelineDefinitionID, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project UUID (overrides --project)")
	cmd.Flags().StringVar(&pipelineDefinitionID, "pipeline-definition-id", "", "Pipeline definition ID (required)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type triggerListEntry struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	EventName   string `json:"event_name,omitempty"`
	EventPreset string `json:"event_preset,omitempty"`
	ConfigRef   string `json:"config_ref,omitempty"`
	CheckoutRef string `json:"checkout_ref,omitempty"`
	Disabled    bool   `json:"disabled"`
	Provider    string `json:"provider"`
	RepoID      string `json:"repo_external_id,omitempty"`
	RepoName    string `json:"repo_full_name,omitempty"`
}

func runTriggerList(ctx context.Context, client *apiclient.Client, projectSlug, projectID, pipelineDefinitionID string, jsonOut bool) error {
	resolvedProjectID, err := resolveProjectID(ctx, client, projectSlug, projectID)
	if err != nil {
		return err
	}

	if pipelineDefinitionID == "" {
		if !iostream.IsInteractive(ctx) {
			return clierrors.New("args.missing_flag", "Missing required flag",
				"--pipeline-definition-id is required in non-interactive mode.").
				WithSuggestions(
					"Pass --pipeline-definition-id <uuid>",
					"Find the ID in your CircleCI project settings under Pipelines",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		pipelineDefinitionID, err = selectPipelineDefinition(ctx, client, resolvedProjectID)
		if err != nil {
			return err
		}
	}

	triggers, err := client.ListTriggers(ctx, resolvedProjectID, pipelineDefinitionID)
	if err != nil {
		return cmdutil.APIErr(err, resolvedProjectID, "trigger.list_failed", "Failed to list triggers for project %q.",
			"Check that the pipeline definition ID is correct",
			"Visit: https://app.circleci.com/settings/project/circleci/<org>/<project>/configurations")
	}

	entries := make([]triggerListEntry, len(triggers))
	for i, t := range triggers {
		e := triggerListEntry{
			ID:          t.ID,
			CreatedAt:   cmdutil.JSONTime(t.CreatedAt),
			EventName:   t.EventName,
			EventPreset: t.EventPreset,
			ConfigRef:   t.ConfigRef,
			CheckoutRef: t.CheckoutRef,
			Disabled:    t.Disabled,
			Provider:    t.EventSource.Provider,
		}
		if t.EventSource.Repo != nil {
			e.RepoID = t.EventSource.Repo.ExternalID
			e.RepoName = t.EventSource.Repo.FullName
		}
		entries[i] = e
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(entries) == 0 {
		iostream.ErrPrintln(ctx, "No triggers found.")
		return nil
	}

	tbl := mdtable.New("ID", "Provider", "Repository", "Event Preset", "Disabled")
	for _, e := range entries {
		repo := e.RepoName
		if repo == "" {
			repo = e.RepoID
		}
		disabled := "no"
		if e.Disabled {
			disabled = "yes"
		}
		tbl.Row(e.ID, e.Provider, repo, e.EventPreset, disabled)
	}
	iostream.PrintMarkdown(ctx, "# Triggers\n"+tbl.Render())
	return nil
}

// --- trigger create ---

func newTriggerCreateCmd() *cobra.Command {
	var (
		projectSlug          string
		projectID            string
		pipelineDefinitionID string
		providerName         string
		repoID               string
		repoFullName         string
		eventPreset          string
		configRef            string
		checkoutRef          string
		jsonOut              bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new project trigger",
		Long: heredoc.Docf(`
			Create a new trigger for a CircleCI project, connecting an event source to a
			pipeline definition so that matching events start a pipeline run. Required
			values prompt in a terminal. Run %[1]scircleci help triggers%[1]s for what each
			provider and event preset means.

			JSON fields: id, created_at, event_name, event_preset, config_ref, checkout_ref, disabled
		`, "`"),
		Example: heredoc.Doc(`
			# Create a GitHub App trigger (provider defaults to github_app)
			$ circleci project trigger create \
			    --pipeline-definition-id a1b2c3d4-... \
			    --repo-id 123456789

			# Create a trigger for a Cursor repository (Origin needs the owner/name too)
			$ circleci project trigger create \
			    --provider origin \
			    --pipeline-definition-id a1b2c3d4-... \
			    --repo-id repo_01abc \
			    --repo-full-name myorg/myrepo

			# Create a trigger with event filtering and output as JSON
			$ circleci project trigger create \
			    --pipeline-definition-id a1b2c3d4-... \
			    --repo-id 123456789 \
			    --event-preset all-pushes \
			    --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runTriggerCreate(ctx, client, projectSlug, projectID, pipelineDefinitionID, providerName, repoID, repoFullName, eventPreset, configRef, checkoutRef, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project UUID (overrides --project)")
	cmd.Flags().StringVar(&pipelineDefinitionID, "pipeline-definition-id", "", "Pipeline definition ID (required)")
	cmd.Flags().StringVar(&providerName, "provider", "github_app", fmt.Sprintf("Event source provider (one of: %s)",
		provider.List(provider.TriggerSource)))
	cmd.Flags().StringVar(&repoID, "repo-id", "", fmt.Sprintf("Repository external ID (required for %s)",
		provider.List(provider.TriggerSource, provider.Provider.NeedsRepoID)))
	cmd.Flags().StringVar(&repoFullName, "repo-full-name", "", fmt.Sprintf("Repository owner/name (required for %s; enough on its own for %s)",
		provider.List(provider.TriggerSource, provider.Provider.NeedsFullName),
		provider.List(provider.TriggerSource, provider.Provider.FullNameIsEnough)))
	cmd.Flags().StringVar(&eventPreset, "event-preset", "", fmt.Sprintf("Event preset for filtering trigger events (one of: %s)", strings.Join(validEventPresets, ", ")))
	cmd.Flags().StringVar(&configRef, "config-ref", "", "Git ref for fetching config (only needed when config repo differs from event source repo)")
	cmd.Flags().StringVar(&checkoutRef, "checkout-ref", "", "Git ref for checking out code (only needed when checkout repo differs from event source repo)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type triggerCreateOutput struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	EventName   string `json:"event_name,omitempty"`
	EventPreset string `json:"event_preset,omitempty"`
	ConfigRef   string `json:"config_ref,omitempty"`
	CheckoutRef string `json:"checkout_ref,omitempty"`
	Disabled    bool   `json:"disabled"`
}

func runTriggerCreate(
	ctx context.Context,
	client *apiclient.Client,
	projectSlug, projectID, pipelineDefinitionID, providerName, repoID, repoFullName, eventPreset, configRef, checkoutRef string,
	jsonOut bool,
) error {
	p, err := validateProvider(providerName)
	if err != nil {
		return err
	}

	resolvedProjectID, err := resolveProjectID(ctx, client, projectSlug, projectID)
	if err != nil {
		return err
	}

	if pipelineDefinitionID == "" {
		if !iostream.IsInteractive(ctx) {
			return clierrors.New("args.missing_flag", "Missing required flag",
				"--pipeline-definition-id is required in non-interactive mode.").
				WithSuggestions(
					"Pass --pipeline-definition-id <uuid>",
					"Find the ID in your CircleCI project settings under Pipelines",
					"Visit: https://app.circleci.com/settings/project/circleci/<org>/<project>/configurations",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		pipelineDefinitionID, err = selectPipelineDefinition(ctx, client, resolvedProjectID)
		if err != nil {
			return err
		}
	}

	// A provider that resolves repositories by owner/name is satisfied by
	// --repo-full-name alone, and is asked for the name rather than an id.
	namedByFullName := p.FullNameIsEnough() && repoFullName != ""
	if repoID == "" && p.HasRepo() && !namedByFullName {
		if !iostream.IsInteractive(ctx) {
			return missingRepoIDErr(p)
		}
		if p.FullNameIsEnough() {
			var promptErr error
			repoFullName, promptErr = iostream.PromptText(ctx, "Repository owner/name", "e.g. myorg/myrepo")
			if promptErr != nil {
				return promptErr
			}
			if repoFullName == "" {
				return clierrors.New("trigger.create_cancelled", "Aborted",
					"No repository owner/name entered.").
					WithExitCode(clierrors.ExitCancelled)
			}
		} else {
			var promptErr error
			repoID, promptErr = iostream.PromptText(ctx,
				"Repository external ID",
				p.RepoIDExample)
			if promptErr != nil {
				return promptErr
			}
			if repoID == "" {
				return clierrors.New("trigger.create_cancelled", "Aborted",
					"No repository ID entered.").
					WithExitCode(clierrors.ExitCancelled)
			}
		}
	}

	// Origin addresses a repository by owner and name and offers no lookup by id,
	// so the full name is required alongside --repo-id rather than decorative.
	if repoFullName == "" && p.NeedsFullName() {
		if !iostream.IsInteractive(ctx) {
			return clierrors.New("args.missing_flag", "Missing required flag",
				fmt.Sprintf("--repo-full-name is required for provider %q in non-interactive mode.", p.Name)).
				WithSuggestions(
					"Pass --repo-full-name <owner>/<repo>",
					"It must name the same repository as --repo-id",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		var promptErr error
		repoFullName, promptErr = iostream.PromptText(ctx,
			"Repository owner/name",
			"e.g. myorg/myrepo")
		if promptErr != nil {
			return promptErr
		}
		if repoFullName == "" {
			return clierrors.New("trigger.create_cancelled", "Aborted",
				"No repository owner/name entered.").
				WithExitCode(clierrors.ExitCancelled)
		}
	}

	if eventPreset != "" {
		if err := validateEventPreset(eventPreset); err != nil {
			return err
		}
	}

	resp, err := client.CreateTrigger(ctx, apiclient.CreateTriggerInput{
		ProjectID:            resolvedProjectID,
		PipelineDefinitionID: pipelineDefinitionID,
		Provider:             p.Name,
		RepoID:               repoID,
		RepoFullName:         repoFullName,
		EventPreset:          eventPreset,
		ConfigRef:            configRef,
		CheckoutRef:          checkoutRef,
	})
	if err != nil {
		return cmdutil.APIErr(err, resolvedProjectID, "trigger.create_failed", "Failed to create trigger for project %q.",
			"Ensure the GitHub App is installed in your repository",
			"Check that the pipeline definition ID and repository ID are correct",
			"Visit: https://app.circleci.com/settings/project/circleci/<org>/<project>/configurations")
	}

	out := triggerCreateOutput{
		ID:          resp.ID,
		CreatedAt:   cmdutil.JSONTime(resp.CreatedAt),
		EventName:   resp.EventName,
		EventPreset: resp.EventPreset,
		ConfigRef:   resp.ConfigRef,
		CheckoutRef: resp.CheckoutRef,
		Disabled:    resp.Disabled,
	}

	if jsonOut {
		return cmdutil.WriteJSON(iostream.Out(ctx), out)
	}

	iostream.Printf(ctx, "Trigger created (ID: %s)\n", out.ID)
	return nil
}

// --- trigger update ---

func newTriggerUpdateCmd() *cobra.Command {
	var (
		projectSlug string
		projectID   string
		enable      bool
		disable     bool
		eventPreset string
		configRef   string
		checkoutRef string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "update <trigger-id>",
		Short: "Update a project trigger",
		Long: heredoc.Docf(`
			Change an existing trigger. Only the flags you pass are changed. The event
			source (provider and repository) is fixed when the trigger is created; to
			change it, delete the trigger and create a new one. Run %[1]scircleci help triggers%[1]s
			for what each event preset means.

			JSON fields: id, created_at, event_name, event_preset, config_ref, checkout_ref, disabled
		`, "`"),
		Example: heredoc.Doc(`
			# Pause a trigger without deleting it
			$ circleci project trigger update a1b2c3d4-... --disable

			# Resume it
			$ circleci project trigger update a1b2c3d4-... --enable

			# Only build tag pushes, and output the result as JSON
			$ circleci project trigger update a1b2c3d4-... --event-preset only-tags --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "trigger-id"); err != nil {
				return err
			}
			input := apiclient.UpdateTriggerInput{}
			flags := cmd.Flags()
			if flags.Changed("enable") || flags.Changed("disable") {
				input.Disabled = &disable
			}
			if flags.Changed("event-preset") {
				input.EventPreset = &eventPreset
			}
			if flags.Changed("config-ref") {
				input.ConfigRef = &configRef
			}
			if flags.Changed("checkout-ref") {
				input.CheckoutRef = &checkoutRef
			}
			if err := validateTriggerUpdate(input); err != nil {
				return err
			}

			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runTriggerUpdate(ctx, client, args[0], projectSlug, projectID, input, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "project slug (e.g. gh/org/repo); only needed for a project's OAuth trigger")
	cmd.Flags().StringVar(&projectID, "project-id", "", "project UUID (overrides --project)")
	cmd.Flags().BoolVar(&enable, "enable", false, "resume the trigger so matching events start pipelines")
	cmd.Flags().BoolVar(&disable, "disable", false, "pause the trigger without deleting it")
	cmd.Flags().StringVar(&eventPreset, "event-preset", "", fmt.Sprintf("event preset for filtering trigger events (one of: %s)", strings.Join(validEventPresets, ", ")))
	cmd.Flags().StringVar(&configRef, "config-ref", "", "git ref for fetching config (only when the config repo differs from the event source repo)")
	cmd.Flags().StringVar(&checkoutRef, "checkout-ref", "", "git ref for checking out code (only when the checkout repo differs from the event source repo)")
	cmd.MarkFlagsMutuallyExclusive("enable", "disable")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// validateTriggerUpdate rejects an update that changes nothing, or that names an
// unknown event preset, before any request is made.
func validateTriggerUpdate(input apiclient.UpdateTriggerInput) error {
	if input.Disabled == nil && input.EventPreset == nil && input.ConfigRef == nil && input.CheckoutRef == nil {
		return clierrors.New("args.nothing_to_update", "Nothing to update",
			"No changes were requested.").
			WithSuggestions("Pass at least one of --enable, --disable, --event-preset, --config-ref or --checkout-ref").
			WithExitCode(clierrors.ExitBadArguments)
	}
	if input.EventPreset != nil && *input.EventPreset != "" {
		if err := validateEventPreset(*input.EventPreset); err != nil {
			return err
		}
	}
	return nil
}

func runTriggerUpdate(ctx context.Context, client *apiclient.Client, triggerID, projectSlug, projectID string, input apiclient.UpdateTriggerInput, jsonOut bool) error {
	var err error
	input.ProjectID, err = resolveOptionalProjectID(ctx, client, projectSlug, projectID)
	if err != nil {
		return err
	}

	resp, err := client.UpdateTrigger(ctx, triggerID, input)
	if err != nil {
		return cmdutil.APIErr(err, triggerID, "trigger.not_found", "No trigger found with ID %q.",
			"Run 'circleci project trigger list --pipeline-definition-id <id>' to see trigger IDs")
	}

	out := triggerCreateOutput{
		ID:          resp.ID,
		CreatedAt:   cmdutil.JSONTime(resp.CreatedAt),
		EventName:   resp.EventName,
		EventPreset: resp.EventPreset,
		ConfigRef:   resp.ConfigRef,
		CheckoutRef: resp.CheckoutRef,
		Disabled:    resp.Disabled,
	}

	if jsonOut {
		return cmdutil.WriteJSON(iostream.Out(ctx), out)
	}

	iostream.Printf(ctx, "%s Updated trigger %s\n", iostream.SymbolOK(ctx), out.ID)
	return nil
}

// --- trigger delete ---

func newTriggerDeleteCmd() *cobra.Command {
	var (
		projectSlug string
		projectID   string
		force       bool
	)

	cmd := &cobra.Command{
		Use:     "delete <trigger-id>",
		Aliases: []string{"rm"},
		Short:   "Delete a project trigger",
		Annotations: map[string]string{
			"destructiveHint": "true",
		},
		Long: heredoc.Doc(`
			Delete a trigger. Events from its event source stop starting pipelines.
			To pause a trigger without losing it, use 'trigger update --disable'.
		`),
		Example: heredoc.Doc(`
			# Delete a trigger (with confirmation)
			$ circleci project trigger delete a1b2c3d4-...

			# Delete without confirmation, for scripts
			$ circleci project trigger delete a1b2c3d4-... --force

			# Pause it instead of deleting it
			$ circleci project trigger update a1b2c3d4-... --disable
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "trigger-id"); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runTriggerDelete(ctx, client, args[0], projectSlug, projectID, force)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "project slug (e.g. gh/org/repo); only needed for a project's OAuth trigger")
	cmd.Flags().StringVar(&projectID, "project-id", "", "project UUID (overrides --project)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}

func runTriggerDelete(ctx context.Context, client *apiclient.Client, triggerID, projectSlug, projectID string, force bool) error {
	// Resolve before confirming so a bad project fails before the user is asked.
	resolvedProjectID, err := resolveOptionalProjectID(ctx, client, projectSlug, projectID)
	if err != nil {
		return err
	}

	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Delete trigger %s? Events from its event source will stop starting pipelines.", triggerID),
		clierrors.New("trigger.delete_aborted", "Deletion aborted",
			"Trigger deletion was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("trigger.delete_requires_force", "Deletion requires --force",
			fmt.Sprintf("Deleting trigger %s stops its events from starting pipelines.", triggerID)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	if err := client.DeleteTrigger(ctx, triggerID, resolvedProjectID); err != nil {
		return cmdutil.APIErr(err, triggerID, "trigger.not_found", "No trigger found with ID %q.",
			"Run 'circleci project trigger list --pipeline-definition-id <id>' to see trigger IDs")
	}

	iostream.Printf(ctx, "%s Deleted trigger %s\n", iostream.SymbolOK(ctx), triggerID)
	return nil
}

// missingRepoIDErr is the non-interactive error for a repository-backed trigger
// that names no repository.
func missingRepoIDErr(p provider.Provider) error {
	if p.FullNameIsEnough() {
		return clierrors.New("args.missing_flag", "Missing required flag",
			fmt.Sprintf("--repo-id or --repo-full-name is required for provider %q in non-interactive mode.", p.Name)).
			WithSuggestions(
				"Pass --repo-full-name <owner>/<repo>",
				"Or pass --repo-id <external-repo-id>",
			).
			WithExitCode(clierrors.ExitBadArguments)
	}
	return clierrors.New("args.missing_flag", "Missing required flag",
		fmt.Sprintf("--repo-id is required for provider %q in non-interactive mode.", p.Name)).
		WithSuggestions(
			"Pass --repo-id <external-repo-id>",
			"Find the repository ID via the GitHub API: GET /repos/{owner}/{repo}",
			"See: https://docs.github.com/en/rest/repos/repos#get-a-repository",
		).
		WithExitCode(clierrors.ExitBadArguments)
}

func validateProvider(v string) (provider.Provider, error) {
	if p, ok := provider.Lookup(v, provider.TriggerSource); ok {
		return p, nil
	}
	return provider.Provider{}, clierrors.New("args.invalid_provider", "Invalid --provider value",
		fmt.Sprintf("%q is not a valid provider.", v)).
		WithSuggestions("Valid values: " + provider.List(provider.TriggerSource)).
		WithExitCode(clierrors.ExitBadArguments)
}

func validateEventPreset(v string) *clierrors.CLIError {
	for _, valid := range validEventPresets {
		if v == valid {
			return nil
		}
	}
	return clierrors.New("args.invalid_event_preset", "Invalid --event-preset value",
		fmt.Sprintf("%q is not a valid event preset.", v)).
		WithSuggestions("Valid values: " + strings.Join(validEventPresets, ", ")).
		WithExitCode(clierrors.ExitBadArguments)
}
