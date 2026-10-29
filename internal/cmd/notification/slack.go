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

package notification

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
	"github.com/CircleCI-Public/circleci-cli/internal/projectslack"
)

const slackProjectFlagUsage = "project slug (e.g. gh/org/repo); defaults to git remote"

func newSlackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slack <command>",
		Short: "Send a project's notifications to a Slack channel",
		Long: heredoc.Doc(`
			The CircleCI Slack app must already be installed for the organization.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(newSlackSetCmd())
	cmd.AddCommand(newSlackGetCmd())
	cmd.AddCommand(newSlackDeleteCmd())

	return cmd
}

func newSlackSetCmd() *cobra.Command {
	var (
		projectSlug string
		channel     string
		enable      bool
		disable     bool
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "set (--channel <name|ID> | --enable | --disable)",
		Short: "Set, enable or disable a project's Slack channel",
		Long: heredoc.Doc(`
			Setting or re-enabling a channel posts a welcome message to it, which is how
			CircleCI checks it can post there. Disabling keeps the channel configured.

			JSON fields: channel_id, channel_name, enabled, disable_reason
		`),
		Example: heredoc.Doc(`
			# Send the current project's notifications to #ci-alerts
			$ circleci notification slack set --channel '#ci-alerts'

			# Use a channel ID (required for private channels)
			$ circleci notification slack set --channel C0123ABCD --project gh/myorg/myrepo

			# Pause Slack notifications, keeping the channel
			$ circleci notification slack set --disable

			# Resume them and print the result as JSON
			$ circleci notification slack set --enable --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("channel") && strings.TrimSpace(channel) == "" {
				return clierrors.New("slack.channel_empty", "Channel required",
					"--channel must be a Slack channel name or ID (got: \"\").").
					WithExitCode(clierrors.ExitBadArguments)
			}

			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			p, err := projectslack.ResolveProject(ctx, client, projectSlug)
			if err != nil {
				return err
			}

			switch {
			case enable:
				return runSlackSetEnabled(ctx, client, p, true, jsonOut)
			case disable:
				return runSlackSetEnabled(ctx, client, p, false, jsonOut)
			default:
				return runSlackSetChannel(ctx, client, p, strings.TrimSpace(channel), jsonOut)
			}
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", slackProjectFlagUsage)
	cmd.Flags().StringVar(&channel, "channel", "", "Slack channel name (e.g. #ci-alerts) or ID to send notifications to")
	cmd.Flags().BoolVar(&enable, "enable", false, "resume Slack notifications for the configured channel")
	cmd.Flags().BoolVar(&disable, "disable", false, "pause Slack notifications, keeping the configured channel")
	cmd.MarkFlagsOneRequired("channel", "enable", "disable")
	cmd.MarkFlagsMutuallyExclusive("channel", "enable", "disable")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runSlackSetChannel(ctx context.Context, client *apiclient.Client, p projectslack.Project, channel string, jsonOut bool) error {
	cfg, err := projectslack.SetChannel(ctx, client, p, channel)
	if err != nil {
		return err
	}
	if jsonOut {
		return iostream.PrintJSON(ctx, cfg)
	}
	iostream.ErrPrintf(ctx, "%s Connected %s to Slack channel %s\n", iostream.SymbolOK(ctx), p.Slug, slackChannelLabel(cfg))
	iostream.ErrPrintln(ctx, "  A welcome message was posted to the channel.")
	return nil
}

func runSlackSetEnabled(ctx context.Context, client *apiclient.Client, p projectslack.Project, enabled, jsonOut bool) error {
	cfg, err := projectslack.SetEnabled(ctx, client, p, enabled)
	if err != nil {
		return err
	}
	if jsonOut {
		return iostream.PrintJSON(ctx, cfg)
	}
	if enabled {
		iostream.ErrPrintf(ctx, "%s Enabled Slack notifications for %s in %s\n", iostream.SymbolOK(ctx), p.Slug, slackChannelLabel(cfg))
		iostream.ErrPrintln(ctx, "  A welcome message was posted to the channel.")
		return nil
	}
	iostream.ErrPrintf(ctx, "%s Disabled Slack notifications for %s; %s stays configured\n", iostream.SymbolOK(ctx), p.Slug, slackChannelLabel(cfg))
	return nil
}

func newSlackGetCmd() *cobra.Command {
	var (
		projectSlug string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show a project's Slack channel and Slack preferences",
		Long: heredoc.Doc(`
			Show the Slack channel a project's notifications go to, whether they are on,
			and which notifications are sent to Slack.

			JSON fields: channel_id, channel_name, enabled, disable_reason,
			preferences[] (group, name, enabled)
		`),
		Example: heredoc.Doc(`
			# Show the current project's Slack channel
			$ circleci notification slack get

			# Show it for a specific project
			$ circleci notification slack get --project gh/myorg/myrepo

			# Print only the channel ID
			$ circleci notification slack get --json --jq .channel_id
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			p, err := projectslack.ResolveProject(ctx, client, projectSlug)
			if err != nil {
				return err
			}
			return runSlackGet(ctx, client, p, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", slackProjectFlagUsage)
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runSlackGet(ctx context.Context, client *apiclient.Client, p projectslack.Project, jsonOut bool) error {
	status, err := projectslack.Get(ctx, client, p)
	if err != nil {
		return err
	}
	if jsonOut {
		return iostream.PrintJSON(ctx, status)
	}

	channel := mdtable.New("Channel", "Channel ID", "Enabled")
	channel.Row(slackChannelName(status.Config), status.ChannelID, onOff(status.Enabled))
	md := "# Slack Channel\n" + channel.Render()
	if status.DisableReason != "" {
		md += "\nDisabled by CircleCI: " + status.DisableReason + "\n"
	}
	if len(status.Preferences) > 0 {
		prefs := mdtable.New("Group", "Notification", "Enabled")
		for _, pref := range status.Preferences {
			prefs.Row(pref.Group, pref.Name, onOff(pref.Enabled))
		}
		md += "\n# Slack Notifications\n" + prefs.Render()
	}
	iostream.PrintMarkdown(ctx, md)
	return nil
}

func newSlackDeleteCmd() *cobra.Command {
	var (
		projectSlug string
		force       bool
		dryRun      bool
	)

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Remove a project's Slack channel",
		Annotations: map[string]string{
			"destructiveHint": "true",
		},
		Long: heredoc.Doc(`
			Remove the Slack channel from a project. Its Slack notifications stop until a
			channel is set again with 'circleci notification slack set --channel'.
		`),
		Example: heredoc.Doc(`
			# Remove the current project's Slack channel (with confirmation)
			$ circleci notification slack delete

			# Preview what would be removed
			$ circleci notification slack delete --dry-run

			# Remove it from a specific project without confirmation
			$ circleci notification slack delete --project gh/myorg/myrepo --force
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			p, err := projectslack.ResolveProject(ctx, client, projectSlug)
			if err != nil {
				return err
			}
			return runSlackDelete(ctx, client, p, force, dryRun)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", slackProjectFlagUsage)
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would be removed without removing it")

	return cmd
}

func runSlackDelete(ctx context.Context, client *apiclient.Client, p projectslack.Project, force, dryRun bool) error {
	cfg, err := projectslack.Find(ctx, client, p)
	if err != nil {
		return err
	}
	label := slackChannelLabel(cfg)

	if dryRun {
		iostream.ErrPrintf(ctx, "Would remove Slack channel %s from %s\n", label, p.Slug)
		return nil
	}

	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Remove Slack channel %s from %s?", label, p.Slug),
		clierrors.New("slack.delete_aborted", "Deletion aborted",
			"Removing the Slack channel was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("slack.delete_requires_force", "Deletion requires --force",
			fmt.Sprintf("Removing Slack channel %s from %s stops its Slack notifications.", label, p.Slug)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	if _, err := projectslack.Delete(ctx, client, p); err != nil {
		return err
	}
	iostream.ErrPrintf(ctx, "%s Removed Slack channel %s from %s\n", iostream.SymbolOK(ctx), label, p.Slug)
	return nil
}

func slackChannelName(cfg projectslack.Config) string {
	if cfg.ChannelName == "" {
		return ""
	}
	return "#" + cfg.ChannelName
}

func slackChannelLabel(cfg projectslack.Config) string {
	if cfg.ChannelName == "" {
		return cfg.ChannelID
	}
	return fmt.Sprintf("#%s (%s)", cfg.ChannelName, cfg.ChannelID)
}

func onOff(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
