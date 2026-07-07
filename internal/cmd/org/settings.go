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

package org

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
		Short: "View and update org advanced settings",
		Long: heredoc.Doc(`
			View and update advanced settings for a CircleCI organization.

			Each subcommand corresponds to one advanced setting. In a terminal,
			running a subcommand with no flags shows the current value and prompts
			you to pick a new one. In non-interactive mode (CI, scripts) it prints
			the current value and shows the exact flags to change it.

			Use --enable or --disable to set a value directly without a prompt.

			To list all settings at once, use 'circleci org settings list'.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(newOrgSettingsListCmd())
	for _, spec := range orgBoolSettingSpecs {
		cmd.AddCommand(newOrgBoolSettingCmd(spec))
	}

	return cmd
}

// orgBoolSettingSpec describes a single boolean advanced setting for an org.
type orgBoolSettingSpec struct {
	use   string // cobra Use name, e.g. "ai-error-summarization"
	short string // one-line description
	long  string // multi-line Long help
	// get returns the value of this field from the settings attributes.
	get func(*apiclient.OrgSettingsAttributes) bool
	// set writes the value into an update payload.
	set func(*apiclient.OrgSettingsUpdate, bool)
}

var orgBoolSettingSpecs = []orgBoolSettingSpec{
	{
		use:   "runner-tos-accepted",
		short: "Runner terms of service accepted",
		long: heredoc.Doc(`
			Indicates whether the runner terms of service have been accepted for
			this organization.

			JSON fields: is_runner_terms_of_service_accepted
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.RunnerTOSAccepted },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.RunnerTOSAccepted = &v },
	},
	{
		use:   "ai-error-summarization",
		short: "Enable AI-powered error summarization",
		long: heredoc.Doc(`
			Control whether CircleCI uses AI to summarize failed build errors for
			this organization.

			When enabled, CircleCI generates an AI summary of failure output
			shown in the UI alongside the raw logs.

			JSON fields: enable_ai_error_summarization
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.AIErrorSummarization },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.AIErrorSummarization = &v },
	},
	{
		use:   "ai-agents",
		short: "Enable AI agents for this organization",
		long: heredoc.Doc(`
			Control whether AI agents are enabled for this organization.

			JSON fields: enable_ai_agents
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.AIAgents },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.AIAgents = &v },
	},
	{
		use:   "unversioned-config",
		short: "Allow triggering pipelines without a config file",
		long: heredoc.Doc(`
			Control whether pipelines can be triggered via the API without a
			config file in the repository for this organization.

			When enabled, API-triggered pipelines may supply their configuration
			inline at trigger time rather than reading it from the repo.

			JSON fields: enable_unversioned_config
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.UnversionedConfig },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.UnversionedConfig = &v },
	},
	{
		use:   "certified-public-orbs",
		short: "Allow use of certified public orbs",
		long: heredoc.Doc(`
			Control whether projects in this organization can use certified
			public orbs from the CircleCI orb registry.

			JSON fields: enable_certified_public_orbs
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.CertifiedPublicOrbs },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.CertifiedPublicOrbs = &v },
	},
	{
		use:   "chunk-ip-ranges",
		short: "Enable chunk IP ranges for this organization",
		long: heredoc.Doc(`
			Control whether chunk IP ranges are enabled for this organization.

			JSON fields: enable_chunk_ip_ranges
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.ChunkIPRanges },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.ChunkIPRanges = &v },
	},
	{
		use:   "minor-ai-features",
		short: "Enable minor AI features for this organization",
		long: heredoc.Doc(`
			Control whether minor AI features are enabled for this organization.

			JSON fields: enable_minor_ai_features
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.MinorAIFeatures },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.MinorAIFeatures = &v },
	},
	{
		use:   "private-orbs",
		short: "Allow use of private orbs",
		long: heredoc.Doc(`
			Control whether projects in this organization can use private orbs.

			JSON fields: enable_private_orbs
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.PrivateOrbs },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.PrivateOrbs = &v },
	},
	{
		use:   "uncertified-public-orbs",
		short: "Allow use of uncertified public orbs",
		long: heredoc.Doc(`
			Control whether projects in this organization can use uncertified
			public orbs from the CircleCI orb registry.

			JSON fields: enable_uncertified_public_orbs
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.UncertifiedPublicOrbs },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.UncertifiedPublicOrbs = &v },
	},
	{
		use:   "bitbucket-workspace-member-is-org-member",
		short: "Treat Bitbucket workspace members as org members",
		long: heredoc.Doc(`
			Control whether Bitbucket workspace members are automatically treated
			as organization members in CircleCI.

			JSON fields: is_bitbucket_workspace_member_org_member
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool {
			return a.BitbucketWorkspaceMemberIsOrgMember
		},
		set: func(u *apiclient.OrgSettingsUpdate, v bool) {
			u.BitbucketWorkspaceMemberIsOrgMember = &v
		},
	},
	{
		use:   "disable-user-checkout-keys",
		short: "Disable user checkout keys for this organization",
		long: heredoc.Doc(`
			Control whether user checkout keys are disabled for this organization.

			JSON fields: is_user_checkout_keys_disabled
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.UserCheckoutKeysDisabled },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.UserCheckoutKeysDisabled = &v },
	},
	{
		use:   "disable-running",
		short: "Disable all builds for this organization",
		long: heredoc.Doc(`
			Control whether builds are disabled for all projects in this
			organization.

			When enabled, all new pipeline runs are dropped immediately. Use this
			as an emergency stop when an organization is producing unexpected or
			runaway builds.

			JSON fields: is_running_disabled
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.DisableRunning },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.DisableRunning = &v },
	},
	{
		use:   "image-brownouts",
		short: "Enable image brownout warnings for this organization",
		long: heredoc.Doc(`
			Control whether image brownout warnings are enabled for this
			organization.

			JSON fields: enable_image_brownouts
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.ImageBrownouts },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.ImageBrownouts = &v },
	},
	{
		use:   "context-group-restriction",
		short: "Require context group restriction for this organization",
		long: heredoc.Doc(`
			Control whether context group restrictions are required for this
			organization.

			JSON fields: is_context_group_restriction_required
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool {
			return a.ContextGroupRestrictionRequired
		},
		set: func(u *apiclient.OrgSettingsUpdate, v bool) {
			u.ContextGroupRestrictionRequired = &v
		},
	},
	{
		use:   "resource-class-brownouts",
		short: "Enable resource class brownout warnings for this organization",
		long: heredoc.Doc(`
			Control whether resource class brownout warnings are enabled for this
			organization.

			JSON fields: enable_resource_class_brownouts
		`),
		get: func(a *apiclient.OrgSettingsAttributes) bool { return a.ResourceClassBrownouts },
		set: func(u *apiclient.OrgSettingsUpdate, v bool) { u.ResourceClassBrownouts = &v },
	},
}

// newOrgBoolSettingCmd returns a Cobra command for one boolean advanced org setting.
func newOrgBoolSettingCmd(spec orgBoolSettingSpec) *cobra.Command {
	var (
		orgSlug string
		enable  bool
		disable bool
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long:  spec.long,
		Example: heredoc.Docf(`
			# Show current value and pick a new one interactively (TTY)
			$ circleci org settings %[1]s

			# Show current value for a specific org (non-interactive)
			$ circleci org settings %[1]s --org gh/myorg

			# Enable the setting directly (non-interactive / scripting)
			$ circleci org settings %[1]s --enable

			# Disable the setting directly (non-interactive / scripting)
			$ circleci org settings %[1]s --disable

			# Output the current value as JSON
			$ circleci org settings %[1]s --json
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

			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, orgSlug, "circleci org settings "+spec.use)
			if err != nil {
				return err
			}

			if !enable && !disable {
				return runOrgSettingGet(ctx, client, orgID, spec, jsonOut)
			}
			return runOrgSettingSet(ctx, client, orgID, spec, enable, jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "Org slug or UUID (e.g. gh/myorg); defaults to git remote")
	cmd.Flags().BoolVar(&enable, "enable", false, "Enable the setting")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable the setting")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type orgSettingValueOutput struct {
	Name  string `json:"name"`
	Value bool   `json:"value"`
}

func runOrgSettingGet(ctx context.Context, client *apiclient.Client, orgID uuid.UUID, spec orgBoolSettingSpec, jsonOut bool) error {
	attrs, err := client.GetOrgSettings(ctx, orgID)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("org.not_found", "Organization not found",
				fmt.Sprintf("No organization found for ID %q.", orgID)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return cmdutil.APIErr(err, orgID.String(), "org.settings_failed", "Failed to get settings for org %q.")
	}

	val := spec.get(attrs)

	if jsonOut {
		return iostream.PrintJSON(ctx, orgSettingValueOutput{Name: spec.use, Value: val})
	}

	if iostream.IsInteractive(ctx) {
		return runOrgSettingPrompt(ctx, client, orgID, spec, val)
	}

	iostream.Printf(ctx, "%s: %v\n", spec.use, val)
	iostream.ErrPrintf(ctx, "To change this setting, run:\n  circleci org settings %s --enable\n  circleci org settings %s --disable\n", spec.use, spec.use)
	return nil
}

// runOrgSettingPrompt offers an interactive enable/disable picker pre-highlighted
// at the current value. If the user picks a different value it is applied immediately.
func runOrgSettingPrompt(ctx context.Context, client *apiclient.Client, orgID uuid.UUID, spec orgBoolSettingSpec, current bool) error {
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
	return runOrgSettingSet(ctx, client, orgID, spec, newVal, false)
}

func runOrgSettingSet(ctx context.Context, client *apiclient.Client, orgID uuid.UUID, spec orgBoolSettingSpec, value bool, jsonOut bool) error {
	var update apiclient.OrgSettingsUpdate
	spec.set(&update, value)

	attrs, err := client.UpdateOrgSettings(ctx, orgID, update)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("org.not_found", "Organization not found",
				fmt.Sprintf("No organization found for ID %q.", orgID)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return cmdutil.APIErr(err, orgID.String(), "org.settings_failed", "Failed to update settings for org %q.")
	}

	val := spec.get(attrs)

	if jsonOut {
		return iostream.PrintJSON(ctx, orgSettingValueOutput{Name: spec.use, Value: val})
	}

	iostream.Printf(ctx, "%s %s: %v\n", iostream.SymbolOK(ctx), spec.use, val)
	return nil
}

// --- org settings list ---

type orgSettingsListOutput struct {
	RunnerTOSAccepted                   bool `json:"is_runner_terms_of_service_accepted"`
	AIErrorSummarization                bool `json:"enable_ai_error_summarization"`
	AIAgents                            bool `json:"enable_ai_agents"`
	UnversionedConfig                   bool `json:"enable_unversioned_config"`
	CertifiedPublicOrbs                 bool `json:"enable_certified_public_orbs"`
	ChunkIPRanges                       bool `json:"enable_chunk_ip_ranges"`
	MinorAIFeatures                     bool `json:"enable_minor_ai_features"`
	PrivateOrbs                         bool `json:"enable_private_orbs"`
	UncertifiedPublicOrbs               bool `json:"enable_uncertified_public_orbs"`
	BitbucketWorkspaceMemberIsOrgMember bool `json:"is_bitbucket_workspace_member_org_member"`
	UserCheckoutKeysDisabled            bool `json:"is_user_checkout_keys_disabled"`
	DisableRunning                      bool `json:"is_running_disabled"`
	ImageBrownouts                      bool `json:"enable_image_brownouts"`
	ContextGroupRestrictionRequired     bool `json:"is_context_group_restriction_required"`
	ResourceClassBrownouts              bool `json:"enable_resource_class_brownouts"`
}

func newOrgSettingsListCmd() *cobra.Command {
	var (
		orgSlug string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all advanced settings for an organization",
		Long: heredoc.Doc(`
			List all advanced settings for a CircleCI organization.

			The organization is inferred from the current git repository's remote
			unless overridden with --org.

			JSON fields: is_runner_terms_of_service_accepted, enable_ai_error_summarization,
			             enable_ai_agents, enable_unversioned_config, enable_certified_public_orbs,
			             enable_chunk_ip_ranges, enable_minor_ai_features, enable_private_orbs,
			             enable_uncertified_public_orbs, is_bitbucket_workspace_member_org_member,
			             is_user_checkout_keys_disabled, is_running_disabled,
			             enable_image_brownouts, is_context_group_restriction_required,
			             enable_resource_class_brownouts
		`),
		Example: heredoc.Doc(`
			# List settings for the current org
			$ circleci org settings list

			# List settings for a specific org
			$ circleci org settings list --org gh/myorg

			# Output as JSON
			$ circleci org settings list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}

			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, orgSlug, "circleci org settings list")
			if err != nil {
				return err
			}

			return runOrgSettingsList(ctx, client, orgID, jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "Org slug or UUID (e.g. gh/myorg); defaults to git remote")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runOrgSettingsList(ctx context.Context, client *apiclient.Client, orgID uuid.UUID, jsonOut bool) error {
	attrs, err := client.GetOrgSettings(ctx, orgID)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("org.not_found", "Organization not found",
				fmt.Sprintf("No organization found for ID %q.", orgID)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return cmdutil.APIErr(err, orgID.String(), "org.settings_failed", "Failed to get settings for org %q.")
	}

	out := orgSettingsListOutput{
		RunnerTOSAccepted:                   attrs.RunnerTOSAccepted,
		AIErrorSummarization:                attrs.AIErrorSummarization,
		AIAgents:                            attrs.AIAgents,
		UnversionedConfig:                   attrs.UnversionedConfig,
		CertifiedPublicOrbs:                 attrs.CertifiedPublicOrbs,
		ChunkIPRanges:                       attrs.ChunkIPRanges,
		MinorAIFeatures:                     attrs.MinorAIFeatures,
		PrivateOrbs:                         attrs.PrivateOrbs,
		UncertifiedPublicOrbs:               attrs.UncertifiedPublicOrbs,
		BitbucketWorkspaceMemberIsOrgMember: attrs.BitbucketWorkspaceMemberIsOrgMember,
		UserCheckoutKeysDisabled:            attrs.UserCheckoutKeysDisabled,
		DisableRunning:                      attrs.DisableRunning,
		ImageBrownouts:                      attrs.ImageBrownouts,
		ContextGroupRestrictionRequired:     attrs.ContextGroupRestrictionRequired,
		ResourceClassBrownouts:              attrs.ResourceClassBrownouts,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	tbl := mdtable.New("Setting", "Value")
	tbl.Row("ai-agents", fmt.Sprintf("%v", out.AIAgents))
	tbl.Row("ai-error-summarization", fmt.Sprintf("%v", out.AIErrorSummarization))
	tbl.Row("bitbucket-workspace-member-is-org-member", fmt.Sprintf("%v", out.BitbucketWorkspaceMemberIsOrgMember))
	tbl.Row("certified-public-orbs", fmt.Sprintf("%v", out.CertifiedPublicOrbs))
	tbl.Row("chunk-ip-ranges", fmt.Sprintf("%v", out.ChunkIPRanges))
	tbl.Row("context-group-restriction", fmt.Sprintf("%v", out.ContextGroupRestrictionRequired))
	tbl.Row("disable-running", fmt.Sprintf("%v", out.DisableRunning))
	tbl.Row("disable-user-checkout-keys", fmt.Sprintf("%v", out.UserCheckoutKeysDisabled))
	tbl.Row("image-brownouts", fmt.Sprintf("%v", out.ImageBrownouts))
	tbl.Row("minor-ai-features", fmt.Sprintf("%v", out.MinorAIFeatures))
	tbl.Row("private-orbs", fmt.Sprintf("%v", out.PrivateOrbs))
	tbl.Row("resource-class-brownouts", fmt.Sprintf("%v", out.ResourceClassBrownouts))
	tbl.Row("runner-tos-accepted", fmt.Sprintf("%v", out.RunnerTOSAccepted))
	tbl.Row("uncertified-public-orbs", fmt.Sprintf("%v", out.UncertifiedPublicOrbs))
	tbl.Row("unversioned-config", fmt.Sprintf("%v", out.UnversionedConfig))
	iostream.PrintMarkdown(ctx, "# Advanced Settings\n"+tbl.Render())
	return nil
}
