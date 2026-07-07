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
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings <command>",
		Short: "View and update project advanced settings",
		Long: heredoc.Doc(`
			View and update advanced settings for a CircleCI project.

			Each subcommand corresponds to one advanced setting. In a terminal,
			running a subcommand with no flags shows the current value and prompts
			you to pick a new one. In non-interactive mode (CI, scripts) it prints
			the current value and shows the exact flags to change it.

			Use --enable or --disable to set a value directly without a prompt.

			To list all settings at once, use 'circleci project settings list'.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(newSettingsListCmd())
	for _, spec := range boolSettingSpecs {
		cmd.AddCommand(newBoolSettingCmd(spec))
	}

	return cmd
}

// boolSettingSpec describes a single boolean advanced setting.
type boolSettingSpec struct {
	use   string // cobra Use name, e.g. "build-forked-pull-requests"
	short string // one-line description
	long  string // multi-line Long help
	// get returns the value of this field from the settings attributes.
	get func(*apiclient.ProjectSettingsAttributes) bool
	// set writes the value into an update payload.
	set func(*apiclient.ProjectSettingsUpdate, bool)
}

var boolSettingSpecs = []boolSettingSpec{
	{
		use:   "build-forked-pull-requests",
		short: "Build pull requests from forked repositories",
		long: heredoc.Doc(`
			Control whether CircleCI builds pull requests opened from forked
			repositories.

			When enabled, commits pushed to a fork that open a PR against this
			repository will trigger a pipeline. Disable this on private repositories
			if you do not want forked-repo PRs to have access to project secrets.

			JSON fields: enable_building_fork_prs
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.BuildForkPRs },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.BuildForkPRs = &v },
	},
	{
		use:   "forks-receive-secret-env-vars",
		short: "Pass secret environment variables to forked-PR builds",
		long: heredoc.Doc(`
			Control whether secret environment variables are passed to pipelines
			triggered by pull requests from forked repositories.

			This setting only applies when build-forked-pull-requests is also
			enabled. Enabling it on public repositories exposes your project
			secrets to all fork contributors.

			JSON fields: can_pass_secrets_to_fork_pr_jobs
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.CanPassSecretsToForkPR },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.CanPassSecretsToForkPR = &v },
	},
	{
		use:   "oss",
		short: "Mark project as free and open source",
		long: heredoc.Doc(`
			Mark this project as free and open source (OSS).

			OSS projects receive extra free build minutes on CircleCI and are
			visible on the public builds page. Enable this only for genuinely
			open-source repositories.

			JSON fields: is_oss
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.IsOSS },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.IsOSS = &v },
	},
	{
		use:   "auto-cancel-builds",
		short: "Automatically cancel redundant workflows",
		long: heredoc.Doc(`
			Control whether CircleCI automatically cancels queued or running
			workflows when a newer commit is pushed to the same branch.

			Enabling this reduces CI queue time on busy branches at the cost of
			not retaining build history for superseded commits.

			JSON fields: enable_auto_cancel_redundant_workflows
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.AutoCancelBuilds },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.AutoCancelBuilds = &v },
	},
	{
		use:   "set-github-status",
		short: "Report build status back to GitHub",
		long: heredoc.Doc(`
			Control whether CircleCI posts build status checks to GitHub.

			When enabled, CircleCI updates the commit status on GitHub so that
			pull requests show a pass/fail indicator. Disabling this prevents
			CircleCI from writing to the GitHub Checks or Statuses API.

			JSON fields: can_set_github_status
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.CanSetGitHubStatus },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.CanSetGitHubStatus = &v },
	},
	{
		use:   "build-prs-only",
		short: "Only build branches that have open pull requests",
		long: heredoc.Doc(`
			Control whether CircleCI only runs pipelines for branches that have
			an open pull request.

			When enabled, pushes to branches without an open PR are ignored.
			This reduces build usage on feature branches that have not yet
			opened a PR.

			JSON fields: is_build_prs_only
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.BuildPRsOnly },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.BuildPRsOnly = &v },
	},
	{
		use:   "disable-ssh",
		short: "Disable SSH access to build containers",
		long: heredoc.Doc(`
			Control whether users can SSH into build containers for debugging.

			When disabled, the "Rerun with SSH" button is removed from the
			CircleCI UI and no SSH keys are added to running containers. Enable
			this for projects with strict security requirements.

			JSON fields: is_ssh_disabled
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.DisableSSH },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.DisableSSH = &v },
	},
	{
		use:   "write-settings-requires-admin",
		short: "Require admin role to change project settings",
		long: heredoc.Doc(`
			Control whether only organization admins can modify project settings.

			When enabled, project-level settings (including environment variables
			and advanced settings) can only be changed by users with admin access
			to the organization.

			JSON fields: is_admin_required_for_writing_settings
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.IsAdminRequiredForWriting },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.IsAdminRequiredForWriting = &v },
	},
	{
		use:   "ai-error-summarization",
		short: "Enable AI-powered error summarization",
		long: heredoc.Doc(`
			Control whether CircleCI uses AI to summarize failed build errors.

			When enabled, CircleCI generates an AI summary of failure output
			shown in the UI alongside the raw logs.

			JSON fields: enable_ai_error_summarization
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.AIErrorSummarization },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.AIErrorSummarization = &v },
	},
	{
		use:   "dynamic-config",
		short: "Enable dynamic configuration (setup workflows)",
		long: heredoc.Doc(`
			Control whether this project can use dynamic configuration with
			setup workflows.

			Dynamic configuration allows a setup workflow to generate and run
			a secondary configuration at runtime based on changed files or other
			conditions.

			JSON fields: enable_dynamic_config
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.DynamicConfig },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.DynamicConfig = &v },
	},
	{
		use:   "unversioned-config",
		short: "Allow triggering pipelines without a config file",
		long: heredoc.Doc(`
			Control whether pipelines can be triggered via the API without a
			config file in the repository.

			When enabled, API-triggered pipelines may supply their configuration
			inline at trigger time rather than reading it from the repo.

			JSON fields: enable_unversioned_config
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.UnversionedConfig },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.UnversionedConfig = &v },
	},
	{
		use:   "disable-running",
		short: "Disable all builds for this project",
		long: heredoc.Doc(`
			Control whether builds are disabled for this project.

			When enabled, all new pipeline runs are dropped immediately. Use this
			as an emergency stop when a project is producing unexpected or
			runaway builds.

			JSON fields: is_running_disabled
		`),
		get: func(a *apiclient.ProjectSettingsAttributes) bool { return a.DisableRunning },
		set: func(u *apiclient.ProjectSettingsUpdate, v bool) { u.DisableRunning = &v },
	},
}

// newBoolSettingCmd returns a Cobra command for one boolean advanced setting.
func newBoolSettingCmd(spec boolSettingSpec) *cobra.Command {
	var (
		projectSlug string
		enable      bool
		disable     bool
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long:  spec.long,
		Example: heredoc.Docf(`
			# Show current value and pick a new one interactively (TTY)
			$ circleci project settings %[1]s

			# Show current value for a specific project (non-interactive)
			$ circleci project settings %[1]s --project gh/myorg/myrepo

			# Enable the setting directly (non-interactive / scripting)
			$ circleci project settings %[1]s --enable

			# Disable the setting directly (non-interactive / scripting)
			$ circleci project settings %[1]s --disable

			# Output the current value as JSON
			$ circleci project settings %[1]s --json
		`, spec.use),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if enable && disable {
				return clierrors.New("args.conflicting_flags", "Conflicting flags",
					"--enable and --disable cannot be used together.").
					WithExitCode(clierrors.ExitBadArguments)
			}

			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}

			projectID, err := resolveProjectIDFromSlug(ctx, client, projectSlug)
			if err != nil {
				return err
			}

			if !enable && !disable {
				return runSettingGet(ctx, client, projectID, spec, jsonOut)
			}
			return runSettingSet(ctx, client, projectID, spec, enable, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().BoolVar(&enable, "enable", false, "Enable the setting")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable the setting")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// resolveProjectIDFromSlug resolves a project UUID from a slug flag or git remote.
// It reuses resolveProjectID from trigger.go (same package).
func resolveProjectIDFromSlug(ctx context.Context, client *apiclient.Client, projectSlug string) (uuid.UUID, error) {
	idStr, err := resolveProjectID(ctx, client, projectSlug, "")
	if err != nil {
		return uuid.UUID{}, err
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.UUID{}, clierrors.New("project.invalid_id", "Invalid project ID",
			fmt.Sprintf("Could not parse project UUID %q.", idStr)).
			WithExitCode(clierrors.ExitAPIError)
	}
	return id, nil
}

type settingValueOutput struct {
	Name  string `json:"name"`
	Value bool   `json:"value"`
}

func runSettingGet(ctx context.Context, client *apiclient.Client, projectID uuid.UUID, spec boolSettingSpec, jsonOut bool) error {
	attrs, err := client.GetProjectSettings(ctx, projectID)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("project.not_found", "Project not found",
				fmt.Sprintf("No project found for ID %q.", projectID)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return cmdutil.APIErr(err, projectID.String(), "project.settings_failed", "Failed to get settings for project %q.")
	}

	val := spec.get(attrs)

	if jsonOut {
		return iostream.PrintJSON(ctx, settingValueOutput{Name: spec.use, Value: val})
	}

	if iostream.IsInteractive(ctx) {
		return runSettingPrompt(ctx, client, projectID, spec, val)
	}

	iostream.Printf(ctx, "%s: %v\n", spec.use, val)
	iostream.ErrPrintf(ctx, "To change this setting, run:\n  circleci project settings %s --enable\n  circleci project settings %s --disable\n", spec.use, spec.use)
	return nil
}

// runSettingPrompt offers an interactive enable/disable picker pre-highlighted
// at the current value. If the user picks a different value it is applied immediately.
func runSettingPrompt(ctx context.Context, client *apiclient.Client, projectID uuid.UUID, spec boolSettingSpec, current bool) error {
	options := []string{"enable", "disable"}
	defaultIdx := 1 // disable
	if current {
		defaultIdx = 0 // enable
	}

	idx, err := iostream.PromptSelectDefault(ctx, fmt.Sprintf("Set %s", spec.use), options, defaultIdx)
	if err != nil {
		return err
	}
	if idx < 0 {
		return clierrors.New("settings.cancelled", "Aborted",
			"No selection made.").
			WithExitCode(clierrors.ExitCancelled)
	}

	newVal := idx == 0 // 0 = "enable" → true
	if newVal == current {
		iostream.Printf(ctx, "%s: %v (unchanged)\n", spec.use, current)
		return nil
	}
	return runSettingSet(ctx, client, projectID, spec, newVal, false)
}

func runSettingSet(ctx context.Context, client *apiclient.Client, projectID uuid.UUID, spec boolSettingSpec, value bool, jsonOut bool) error {
	var update apiclient.ProjectSettingsUpdate
	spec.set(&update, value)

	attrs, err := client.UpdateProjectSettings(ctx, projectID, update)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("project.not_found", "Project not found",
				fmt.Sprintf("No project found for ID %q.", projectID)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return cmdutil.APIErr(err, projectID.String(), "project.settings_failed", "Failed to update settings for project %q.")
	}

	val := spec.get(attrs)

	if jsonOut {
		return iostream.PrintJSON(ctx, settingValueOutput{Name: spec.use, Value: val})
	}

	iostream.Printf(ctx, "%s %s: %v\n", iostream.SymbolOK(ctx), spec.use, val)
	return nil
}

// --- settings list ---

type settingsListOutput struct {
	AIErrorSummarization      bool     `json:"enable_ai_error_summarization"`
	AutoCancelBuilds          bool     `json:"enable_auto_cancel_redundant_workflows"`
	BuildForkPRs              bool     `json:"enable_building_fork_prs"`
	BuildPRsOnly              bool     `json:"is_build_prs_only"`
	CanPassSecretsToForkPR    bool     `json:"can_pass_secrets_to_fork_pr_jobs"`
	CanSetGitHubStatus        bool     `json:"can_set_github_status"`
	DisableRunning            bool     `json:"is_running_disabled"`
	DisableSSH                bool     `json:"is_ssh_disabled"`
	DynamicConfig             bool     `json:"enable_dynamic_config"`
	IsAdminRequiredForWriting bool     `json:"is_admin_required_for_writing_settings"`
	IsOSS                     bool     `json:"is_oss"`
	PROnlyBranchOverrides     []string `json:"pr_only_branch_overrides"`
	UnversionedConfig         bool     `json:"enable_unversioned_config"`
}

func newSettingsListCmd() *cobra.Command {
	var (
		projectSlug string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all advanced settings for a project",
		Long: heredoc.Doc(`
			List all advanced settings for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project.

			JSON fields: enable_ai_error_summarization, enable_auto_cancel_redundant_workflows,
			             enable_building_fork_prs, is_build_prs_only, can_pass_secrets_to_fork_pr_jobs,
			             can_set_github_status, is_running_disabled, is_ssh_disabled,
			             enable_dynamic_config, is_admin_required_for_writing_settings,
			             is_oss, pr_only_branch_overrides, enable_unversioned_config
		`),
		Example: heredoc.Doc(`
			# List settings for the current project
			$ circleci project settings list

			# List settings for a specific project
			$ circleci project settings list --project gh/myorg/myrepo

			# Output as JSON
			$ circleci project settings list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}

			projectID, err := resolveProjectIDFromSlug(ctx, client, projectSlug)
			if err != nil {
				return err
			}

			return runSettingsList(ctx, client, projectID, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runSettingsList(ctx context.Context, client *apiclient.Client, projectID uuid.UUID, jsonOut bool) error {
	attrs, err := client.GetProjectSettings(ctx, projectID)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("project.not_found", "Project not found",
				fmt.Sprintf("No project found for ID %q.", projectID)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return cmdutil.APIErr(err, projectID.String(), "project.settings_failed", "Failed to get settings for project %q.")
	}

	overrides := attrs.PROnlyBranchOverrides
	if overrides == nil {
		overrides = []string{}
	}

	out := settingsListOutput{
		AIErrorSummarization:      attrs.AIErrorSummarization,
		AutoCancelBuilds:          attrs.AutoCancelBuilds,
		BuildForkPRs:              attrs.BuildForkPRs,
		BuildPRsOnly:              attrs.BuildPRsOnly,
		CanPassSecretsToForkPR:    attrs.CanPassSecretsToForkPR,
		CanSetGitHubStatus:        attrs.CanSetGitHubStatus,
		DisableRunning:            attrs.DisableRunning,
		DisableSSH:                attrs.DisableSSH,
		DynamicConfig:             attrs.DynamicConfig,
		IsAdminRequiredForWriting: attrs.IsAdminRequiredForWriting,
		IsOSS:                     attrs.IsOSS,
		PROnlyBranchOverrides:     overrides,
		UnversionedConfig:         attrs.UnversionedConfig,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	tbl := mdtable.New("Setting", "Value")
	tbl.Row("ai-error-summarization", fmt.Sprintf("%v", out.AIErrorSummarization))
	tbl.Row("auto-cancel-builds", fmt.Sprintf("%v", out.AutoCancelBuilds))
	tbl.Row("build-forked-pull-requests", fmt.Sprintf("%v", out.BuildForkPRs))
	tbl.Row("build-prs-only", fmt.Sprintf("%v", out.BuildPRsOnly))
	tbl.Row("disable-running", fmt.Sprintf("%v", out.DisableRunning))
	tbl.Row("disable-ssh", fmt.Sprintf("%v", out.DisableSSH))
	tbl.Row("dynamic-config", fmt.Sprintf("%v", out.DynamicConfig))
	tbl.Row("forks-receive-secret-env-vars", fmt.Sprintf("%v", out.CanPassSecretsToForkPR))
	tbl.Row("oss", fmt.Sprintf("%v", out.IsOSS))
	tbl.Row("set-github-status", fmt.Sprintf("%v", out.CanSetGitHubStatus))
	tbl.Row("unversioned-config", fmt.Sprintf("%v", out.UnversionedConfig))
	tbl.Row("write-settings-requires-admin", fmt.Sprintf("%v", out.IsAdminRequiredForWriting))
	iostream.PrintMarkdown(ctx, "# Advanced Settings\n"+tbl.Render())
	return nil
}
