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
	"strings"

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
		Use:   "setting <command>",
		Short: "View and update project advanced settings",
		Long: heredoc.Doc(`
			View and update advanced settings for a CircleCI project.

			Use 'get' to read a setting's current value and 'set' to change it.
			Use 'list' to see all settings at once.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(newSettingsListCmd())
	cmd.AddCommand(newSettingsGetCmd())
	cmd.AddCommand(newSettingsSetCmd())

	return cmd
}

// boolSettingSpec describes a single boolean advanced setting.
type boolSettingSpec struct {
	use   string // name used on the CLI, e.g. "build-forked-pull-requests"
	short string // one-line description
	get   func(*apiclient.ProjectSettingsAttributes) bool
	set   func(*apiclient.ProjectSettingsUpdate, bool)
}

var boolSettingSpecs = []boolSettingSpec{
	{
		use:   "build-forked-pull-requests",
		short: "Build pull requests from forked repositories",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.BuildForkPRs },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.BuildForkPRs = &v },
	},
	{
		use:   "forks-receive-secret-env-vars",
		short: "Pass secret environment variables to forked-PR builds",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.CanPassSecretsToForkPR },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.CanPassSecretsToForkPR = &v },
	},
	{
		use:   "oss",
		short: "Mark project as free and open source",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.IsOSS },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.IsOSS = &v },
	},
	{
		use:   "auto-cancel-builds",
		short: "Automatically cancel redundant workflows",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.AutoCancelBuilds },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.AutoCancelBuilds = &v },
	},
	{
		use:   "set-github-status",
		short: "Report build status back to GitHub",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.CanSetGitHubStatus },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.CanSetGitHubStatus = &v },
	},
	{
		use:   "build-prs-only",
		short: "Only build branches that have open pull requests",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.BuildPRsOnly },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.BuildPRsOnly = &v },
	},
	{
		use:   "disable-ssh",
		short: "Disable SSH access to build containers",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.DisableSSH },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.DisableSSH = &v },
	},
	{
		use:   "write-settings-requires-admin",
		short: "Require admin role to change project settings",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.IsAdminRequiredForWriting },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.IsAdminRequiredForWriting = &v },
	},
	{
		use:   "ai-error-summarization",
		short: "Enable AI-powered error summarization",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.AIErrorSummarization },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.AIErrorSummarization = &v },
	},
	{
		use:   "dynamic-config",
		short: "Enable dynamic configuration (setup workflows)",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.DynamicConfig },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.DynamicConfig = &v },
	},
	{
		use:   "unversioned-config",
		short: "Allow triggering pipelines without a config file",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.UnversionedConfig },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.UnversionedConfig = &v },
	},
	{
		use:   "disable-running",
		short: "Disable all builds for this project",
		get:   func(a *apiclient.ProjectSettingsAttributes) bool { return a.DisableRunning },
		set:   func(u *apiclient.ProjectSettingsUpdate, v bool) { u.DisableRunning = &v },
	},
}

func findProjectSetting(name string) (boolSettingSpec, bool) {
	for _, s := range boolSettingSpecs {
		if s.use == name {
			return s, true
		}
	}
	return boolSettingSpec{}, false
}

func projectSettingNames() string {
	names := make([]string, len(boolSettingSpecs))
	for i, s := range boolSettingSpecs {
		names[i] = s.use
	}
	return strings.Join(names, ", ")
}

func projectSettingTable() string {
	tbl := mdtable.New("Name", "Description")
	for _, s := range boolSettingSpecs {
		tbl.Row(s.use, s.short)
	}
	return tbl.Render()
}

// resolveProjectIDFromSlug resolves a project UUID from a slug flag or git remote.
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

// --- settings get ---

func newSettingsGetCmd() *cobra.Command {
	var (
		projectSlug string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "get <setting>",
		Short: "Get the current value of a project setting",
		Long: heredoc.Docf(`
			Get the current value of an advanced project setting.

			JSON fields: name, value

			Available settings:
			%s
		`, projectSettingTable()),
		Example: heredoc.Doc(`
			# Get a setting for the current project
			$ circleci project settings get build-forked-pull-requests

			# Get a setting for a specific project
			$ circleci project settings get build-forked-pull-requests --project gh/myorg/myrepo

			# Output as JSON
			$ circleci project settings get build-forked-pull-requests --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, ok := findProjectSetting(args[0])
			if !ok {
				return clierrors.New("settings.unknown", "Unknown setting",
					fmt.Sprintf("%q is not a known project setting.", args[0])).
					WithSuggestions("Run 'circleci project settings list' to see all available settings",
						"Valid settings: "+projectSettingNames()).
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
			return runProjectSettingGet(ctx, client, projectID, spec, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// --- settings set ---

func newSettingsSetCmd() *cobra.Command {
	var (
		projectSlug string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "set <setting> <true|false>",
		Short: "Set a project setting",
		Long: heredoc.Docf(`
			Set an advanced project setting to true or false.

			JSON fields: name, value

			Available settings:
			%s
		`, projectSettingTable()),
		Example: heredoc.Doc(`
			# Enable a setting for the current project
			$ circleci project settings set build-forked-pull-requests true

			# Disable a setting for a specific project
			$ circleci project settings set build-forked-pull-requests false --project gh/myorg/myrepo

			# Output the updated value as JSON
			$ circleci project settings set auto-cancel-builds true --json
		`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, ok := findProjectSetting(args[0])
			if !ok {
				return clierrors.New("settings.unknown", "Unknown setting",
					fmt.Sprintf("%q is not a known project setting.", args[0])).
					WithSuggestions("Run 'circleci project settings list' to see all available settings",
						"Valid settings: "+projectSettingNames()).
					WithExitCode(clierrors.ExitBadArguments)
			}

			value, err := parseBool(args[1])
			if err != nil {
				return err
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
			return runProjectSettingSet(ctx, client, projectID, spec, value, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// parseBool accepts "true"/"false" and returns a structured error for anything else.
func parseBool(s string) (bool, error) {
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, clierrors.New("args.invalid_value", "Invalid value",
			fmt.Sprintf("%q is not a valid boolean value. Use 'true' or 'false'.", s)).
			WithExitCode(clierrors.ExitBadArguments)
	}
}

type settingValueOutput struct {
	Name  string `json:"name"`
	Value bool   `json:"value"`
}

func runProjectSettingGet(ctx context.Context, client *apiclient.Client, projectID uuid.UUID, spec boolSettingSpec, jsonOut bool) error {
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

	iostream.Printf(ctx, "%s: %v\n", spec.use, val)
	return nil
}

func runProjectSettingSet(ctx context.Context, client *apiclient.Client, projectID uuid.UUID, spec boolSettingSpec, value bool, jsonOut bool) error {
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
