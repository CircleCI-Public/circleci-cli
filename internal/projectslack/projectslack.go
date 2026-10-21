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

package projectslack

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

const channelTypeSlack = "slack"

const channelNotFoundPrefix = "Channel not found"

type Project struct {
	Slug  string
	ID    uuid.UUID
	OrgID uuid.UUID
}

type Config struct {
	ChannelID     string `json:"channel_id"`
	ChannelName   string `json:"channel_name"`
	Enabled       bool   `json:"enabled"`
	DisableReason string `json:"disable_reason,omitempty"`
}

type Preference struct {
	Group   string `json:"group"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type Status struct {
	Config
	Preferences []Preference `json:"preferences"`
}

func ResolveProject(ctx context.Context, client *apiclient.Client, projectSlug string) (Project, error) {
	slug, err := cmdutil.ResolveProjectSlug(projectSlug)
	if err != nil {
		return Project{}, err
	}
	ref, err := client.GetProjectBySlug(ctx, slug)
	if err != nil {
		return Project{}, cmdutil.APIErr(err, slug, "project.not_found", "No project found for %q.",
			"Run 'circleci project link' to bind this repository to a CircleCI project",
			"Check the project slug and try again")
	}
	return Project{Slug: slug, ID: ref.ID, OrgID: ref.OrgID}, nil
}

func SetChannel(ctx context.Context, client *apiclient.Client, p Project, channel string) (Config, error) {
	cfg, err := client.CreateProjectChannelConfig(ctx, p.OrgID, p.ID, channelTypeSlack, channel)
	if err != nil {
		return Config{}, slackErr(ctx, client, p, err, true)
	}
	return toConfig(*cfg), nil
}

func SetEnabled(ctx context.Context, client *apiclient.Client, p Project, enabled bool) (Config, error) {
	existing, err := find(ctx, client, p)
	if err != nil {
		return Config{}, err
	}
	cfg, err := client.SetChannelConfigEnabled(ctx, existing.ID, enabled)
	if err != nil {
		return Config{}, slackErr(ctx, client, p, err, enabled)
	}
	return toConfig(*cfg), nil
}

func Get(ctx context.Context, client *apiclient.Client, p Project) (Status, error) {
	existing, err := find(ctx, client, p)
	if err != nil {
		return Status{}, err
	}
	prefs, err := client.ListProjectNotificationPreferences(ctx, p.OrgID, p.ID)
	if err != nil {
		return Status{}, slackErr(ctx, client, p, err, false)
	}
	status := Status{Config: toConfig(*existing), Preferences: []Preference{}}
	for _, pref := range prefs {
		if pref.Channel != channelTypeSlack {
			continue
		}
		status.Preferences = append(status.Preferences, Preference{
			Group:   pref.Group,
			Name:    pref.Name,
			Enabled: pref.Enabled,
		})
	}
	return status, nil
}

func Delete(ctx context.Context, client *apiclient.Client, p Project) (Config, error) {
	existing, err := find(ctx, client, p)
	if err != nil {
		return Config{}, err
	}
	if err := client.DeleteChannelConfig(ctx, existing.ID); err != nil {
		return Config{}, slackErr(ctx, client, p, err, false)
	}
	return toConfig(*existing), nil
}

func Find(ctx context.Context, client *apiclient.Client, p Project) (Config, error) {
	existing, err := find(ctx, client, p)
	if err != nil {
		return Config{}, err
	}
	return toConfig(*existing), nil
}

func find(ctx context.Context, client *apiclient.Client, p Project) (*apiclient.ChannelConfig, error) {
	configs, err := client.ListProjectChannelConfigs(ctx, p.OrgID, p.ID)
	if err != nil {
		return nil, slackErr(ctx, client, p, err, false)
	}
	for i := range configs {
		if configs[i].ChannelType == channelTypeSlack {
			return &configs[i], nil
		}
	}
	return nil, clierrors.New("slack.not_configured", "No Slack channel configured",
		fmt.Sprintf("Project %q has no Slack channel configured.", p.Slug)).
		WithSuggestions(fmt.Sprintf("Run: circleci notification slack set --channel <name or ID> --project %s", p.Slug)).
		WithExitCode(clierrors.ExitNotFound)
}

func toConfig(c apiclient.ChannelConfig) Config {
	return Config{
		ChannelID:     c.Target,
		ChannelName:   c.ChannelName,
		Enabled:       c.Enabled,
		DisableReason: c.DisableReason,
	}
}

func slackErr(ctx context.Context, client *apiclient.Client, p Project, err error, postsToSlack bool) error {
	title := ""
	if apiErr, ok := apiclient.ParseError(err); ok {
		title = apiErr.Title
	}

	switch {
	case postsToSlack && httpcl.HasStatusCode(err, http.StatusUnauthorized):
		return clierrors.New("slack.connection_invalid", "Slack connection invalid",
			"The organization's connection to Slack is no longer valid, so CircleCI cannot post to Slack.").
			WithSuggestions("Reconnect Slack in the organization settings: https://app.circleci.com/settings/organization").
			WithExitCode(clierrors.ExitAPIError)
	case postsToSlack && httpcl.HasStatusCode(err, http.StatusForbidden):
		return clierrors.New("slack.cannot_post", "Cannot post to channel",
			"The CircleCI Slack app is not allowed to post in this channel.").
			WithSuggestions("Run /invite @circleci in the channel, then try again").
			WithExitCode(clierrors.ExitAPIError)
	case postsToSlack && httpcl.HasStatusCode(err, http.StatusNotFound) && strings.HasPrefix(title, channelNotFoundPrefix):
		return clierrors.New("slack.channel_not_found", "Slack channel not found",
			"Slack could not find the channel, or the CircleCI app is not in it.").
			WithSuggestions(
				"Private channels must be given by ID, e.g. --channel C0123ABCD",
				"Run /invite @circleci in the channel, then try again",
			).
			WithExitCode(clierrors.ExitNotFound)
	case httpcl.HasStatusCode(err, http.StatusNotFound):
		return notFoundErr(ctx, client, p)
	case httpcl.HasStatusCode(err, http.StatusTooManyRequests):
		return clierrors.New("slack.rate_limited", "Rate limited",
			"Slack is rate limiting requests from CircleCI.").
			WithSuggestions("Wait a minute, then try again").
			WithExitCode(clierrors.ExitAPIError)
	case httpcl.HasStatusCode(err, http.StatusBadRequest) && title != "":
		return clierrors.New("slack.invalid_input", "Invalid request",
			fmt.Sprintf("CircleCI rejected the request: %s", title)).
			WithExitCode(clierrors.ExitAPIError)
	}
	return cmdutil.APIErr(err, p.Slug, "slack.not_found", "No Slack notification settings found for project %q.")
}

func notFoundErr(ctx context.Context, client *apiclient.Client, p Project) error {
	integrations, err := client.ListSlackIntegrations(ctx, p.OrgID)
	if err == nil && len(integrations) == 0 {
		return clierrors.New("slack.not_installed", "Slack app not installed",
			"The CircleCI Slack app is not installed for this project's organization.").
			WithSuggestions("Install it from the organization settings: https://app.circleci.com/settings/organization").
			WithExitCode(clierrors.ExitNotFound)
	}
	return clierrors.New("slack.permission_denied", "Permission denied",
		fmt.Sprintf("You do not have permission to manage notifications for project %q.", p.Slug)).
		WithSuggestions("Ask an organization admin to grant you access to the project").
		WithExitCode(clierrors.ExitNotFound)
}
