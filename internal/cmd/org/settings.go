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
		Use:   "settings <command>",
		Short: "View and update org advanced settings",
		Long: heredoc.Doc(`
			View and update advanced settings for a CircleCI organization.

			Use 'get' to read a setting's current value and 'set' to change it.
			Use 'list' to see all settings at once.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(newOrgSettingsListCmd())
	cmd.AddCommand(newOrgSettingsGetCmd())
	cmd.AddCommand(newOrgSettingsSetCmd())

	return cmd
}

type orgBoolSettingSpec struct {
	use   string
	short string
	get   func(*apiclient.OrgSettingsAttributes) bool
	set   func(*apiclient.OrgSettingsUpdate, bool)
}

var orgBoolSettingSpecs = []orgBoolSettingSpec{
	{
		use:   "ai-error-summarization",
		short: "Enable AI-powered error summarization",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.AIErrorSummarization },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.AIErrorSummarization = &v },
	},
	{
		use:   "ai-agents",
		short: "Enable AI agents for this organization",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.AIAgents },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.AIAgents = &v },
	},
	{
		use:   "unversioned-config",
		short: "Allow triggering pipelines without a config file",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.UnversionedConfig },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.UnversionedConfig = &v },
	},
	{
		use:   "certified-public-orbs",
		short: "Allow use of certified public orbs",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.CertifiedPublicOrbs },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.CertifiedPublicOrbs = &v },
	},
	{
		use:   "chunk-ip-ranges",
		short: "Enable chunk IP ranges for this organization",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.ChunkIPRanges },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.ChunkIPRanges = &v },
	},
	{
		use:   "minor-ai-features",
		short: "Enable minor AI features for this organization",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.MinorAIFeatures },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.MinorAIFeatures = &v },
	},
	{
		use:   "private-orbs",
		short: "Allow use of private orbs",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.PrivateOrbs },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.PrivateOrbs = &v },
	},
	{
		use:   "uncertified-public-orbs",
		short: "Allow use of uncertified public orbs",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.UncertifiedPublicOrbs },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.UncertifiedPublicOrbs = &v },
	},
	{
		use:   "bitbucket-workspace-member-is-org-member",
		short: "Treat Bitbucket workspace members as org members",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.BitbucketWorkspaceMemberIsOrgMember },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.BitbucketWorkspaceMemberIsOrgMember = &v },
	},
	{
		use:   "disable-user-checkout-keys",
		short: "Disable user checkout keys for this organization",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.UserCheckoutKeysDisabled },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.UserCheckoutKeysDisabled = &v },
	},
	{
		use:   "disable-running",
		short: "Disable all builds for this organization",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.DisableRunning },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.DisableRunning = &v },
	},
	{
		use:   "image-brownouts",
		short: "Enable image brownout warnings for this organization",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.ImageBrownouts },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.ImageBrownouts = &v },
	},
	{
		use:   "context-group-restriction",
		short: "Require context group restriction for this organization",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.ContextGroupRestrictionRequired },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.ContextGroupRestrictionRequired = &v },
	},
	{
		use:   "resource-class-brownouts",
		short: "Enable resource class brownout warnings for this organization",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.ResourceClassBrownouts },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.ResourceClassBrownouts = &v },
	},
	{
		use:   "runner-tos-accepted",
		short: "Mark runner terms of service as accepted",
		get:   func(a *apiclient.OrgSettingsAttributes) bool { return a.RunnerTOSAccepted },
		set:   func(u *apiclient.OrgSettingsUpdate, v bool) { u.RunnerTOSAccepted = &v },
	},
}

func findOrgSetting(name string) (orgBoolSettingSpec, bool) {
	for _, s := range orgBoolSettingSpecs {
		if s.use == name {
			return s, true
		}
	}
	return orgBoolSettingSpec{}, false
}

func orgSettingNames() string {
	names := make([]string, len(orgBoolSettingSpecs))
	for i, s := range orgBoolSettingSpecs {
		names[i] = s.use
	}
	return strings.Join(names, ", ")
}

func orgSettingTable() string {
	tbl := mdtable.New("Name", "Description")
	for _, s := range orgBoolSettingSpecs {
		tbl.Row(s.use, s.short)
	}
	return tbl.Render()
}

// --- settings get ---

func newOrgSettingsGetCmd() *cobra.Command {
	var (
		orgSlug string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "get <setting>",
		Short: "Get the current value of an org setting",
		Long: heredoc.Docf(`
			Get the current value of an advanced org setting.

			JSON fields: name, value

			Available settings:
			%s
		`, orgSettingTable()),
		Example: heredoc.Doc(`
			# Get a setting for the current org
			$ circleci org settings get private-orbs

			# Get a setting for a specific org
			$ circleci org settings get private-orbs --org gh/myorg

			# Output as JSON
			$ circleci org settings get private-orbs --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, ok := findOrgSetting(args[0])
			if !ok {
				return clierrors.New("settings.unknown", "Unknown setting",
					fmt.Sprintf("%q is not a known org setting.", args[0])).
					WithSuggestions("Run 'circleci org settings list' to see all available settings",
						"Valid settings: "+orgSettingNames()).
					WithExitCode(clierrors.ExitBadArguments)
			}

			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, orgSlug, "circleci org settings get")
			if err != nil {
				return err
			}
			return runOrgSettingGet(ctx, client, orgID, spec, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &orgSlug, cmdutil.OrgFlag{DefaultsToGitRemote: true})
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// --- settings set ---

func newOrgSettingsSetCmd() *cobra.Command {
	var (
		orgSlug string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "set <setting> <true|false>",
		Short: "Set an org setting",
		Long: heredoc.Docf(`
			Set an advanced org setting to true or false.

			JSON fields: name, value

			Available settings:
			%s
		`, orgSettingTable()),
		Example: heredoc.Doc(`
			# Enable a setting for the current org
			$ circleci org settings set private-orbs true

			# Disable a setting for a specific org
			$ circleci org settings set private-orbs false --org gh/myorg

			# Output the updated value as JSON
			$ circleci org settings set ai-error-summarization true --json
		`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, ok := findOrgSetting(args[0])
			if !ok {
				return clierrors.New("settings.unknown", "Unknown setting",
					fmt.Sprintf("%q is not a known org setting.", args[0])).
					WithSuggestions("Run 'circleci org settings list' to see all available settings",
						"Valid settings: "+orgSettingNames()).
					WithExitCode(clierrors.ExitBadArguments)
			}

			value, err := parseBoolOrg(args[1])
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, orgSlug, "circleci org settings set")
			if err != nil {
				return err
			}
			return runOrgSettingSet(ctx, client, orgID, spec, value, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &orgSlug, cmdutil.OrgFlag{DefaultsToGitRemote: true})
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func parseBoolOrg(s string) (bool, error) {
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
		return cmdutil.APIErr(err, orgID.String(), "org.settings_failed", "Failed to get settings for organization %q.")
	}

	val := spec.get(attrs)

	if jsonOut {
		return iostream.PrintJSON(ctx, orgSettingValueOutput{Name: spec.use, Value: val})
	}

	iostream.Printf(ctx, "%s: %v\n", spec.use, val)
	return nil
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
		return cmdutil.APIErr(err, orgID.String(), "org.settings_failed", "Failed to update settings for organization %q.")
	}

	val := spec.get(attrs)

	if jsonOut {
		return iostream.PrintJSON(ctx, orgSettingValueOutput{Name: spec.use, Value: val})
	}

	iostream.Printf(ctx, "%s %s: %v\n", iostream.SymbolOK(ctx), spec.use, val)
	return nil
}

// --- settings list ---

type orgSettingsListOutput struct {
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
	RunnerTOSAccepted                   bool `json:"is_runner_terms_of_service_accepted"`
}

func newOrgSettingsListCmd() *cobra.Command {
	var (
		orgSlug string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all advanced settings for an org",
		Long: heredoc.Doc(`
			List all advanced settings for a CircleCI organization.

			The organization is inferred from the current git repository's remote
			unless overridden with --org.

			JSON fields: enable_ai_error_summarization, enable_ai_agents,
			             enable_unversioned_config, enable_certified_public_orbs,
			             enable_chunk_ip_ranges, enable_minor_ai_features,
			             enable_private_orbs, enable_uncertified_public_orbs,
			             is_bitbucket_workspace_member_org_member,
			             is_user_checkout_keys_disabled, is_running_disabled,
			             enable_image_brownouts, is_context_group_restriction_required,
			             enable_resource_class_brownouts, is_runner_terms_of_service_accepted
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

	cmdutil.AddOrgFlag(cmd, &orgSlug, cmdutil.OrgFlag{DefaultsToGitRemote: true})
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
		return cmdutil.APIErr(err, orgID.String(), "org.settings_failed", "Failed to get settings for organization %q.")
	}

	out := orgSettingsListOutput{
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
		RunnerTOSAccepted:                   attrs.RunnerTOSAccepted,
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
