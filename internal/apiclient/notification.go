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

package apiclient

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

type ChannelConfig struct {
	ID            uuid.UUID
	ChannelType   string
	Target        string
	ChannelName   string
	Enabled       bool
	DisableReason string
}

type NotificationPreference struct {
	ID      uuid.UUID
	Name    string
	Group   string
	Channel string
	Enabled bool
}

type NotificationIntegration struct {
	ID            uuid.UUID
	Type          string
	WorkspaceName string
	Status        string
}

type channelConfigV3 struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		ChannelType   string `json:"channel_type"`
		Target        string `json:"target"`
		ChannelName   string `json:"channel_name"`
		IsEnabled     bool   `json:"is_enabled"`
		DisableReason string `json:"disable_reason"`
	} `json:"attributes"`
}

func (c channelConfigV3) toChannelConfig() ChannelConfig {
	return ChannelConfig{
		ID:            c.ID,
		ChannelType:   c.Attributes.ChannelType,
		Target:        c.Attributes.Target,
		ChannelName:   c.Attributes.ChannelName,
		Enabled:       c.Attributes.IsEnabled,
		DisableReason: c.Attributes.DisableReason,
	}
}

type preferenceV3 struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		PreferenceName string `json:"preference_name"`
		GroupName      string `json:"group_name"`
		Channel        string `json:"channel"`
		IsEnabled      bool   `json:"is_enabled"`
	} `json:"attributes"`
}

type integrationV3 struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		Type          string `json:"type"`
		WorkspaceName string `json:"workspace_name"`
		Status        string `json:"status"`
	} `json:"attributes"`
}

func (c *Client) ListProjectChannelConfigs(ctx context.Context, orgID, projectID uuid.UUID) ([]ChannelConfig, error) {
	var all []ChannelConfig
	cursor := ""

	for {
		var env v3List[channelConfigV3]
		_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/notification/channel-configs",
			filterParam("scope", "project"),
			filterParam("org_id", orgID.String()),
			filterParam("project_id", projectID.String()),
			pageCursor(cursor),
			httpcl.JSONDecoder(&env),
		))
		if err != nil {
			return nil, err
		}
		for _, item := range env.Data {
			all = append(all, item.toChannelConfig())
		}
		if env.Page.Next == nil || *env.Page.Next == "" {
			return all, nil
		}
		cursor = *env.Page.Next
	}
}

func (c *Client) CreateProjectChannelConfig(
	ctx context.Context,
	orgID, projectID uuid.UUID,
	channelType, target string,
) (*ChannelConfig, error) {
	body := map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{
				"scope":        "project",
				"channel_type": channelType,
				"target":       target,
				"is_enabled":   true,
			},
			"references": map[string]any{
				"org":     map[string]any{"id": orgID.String()},
				"project": map[string]any{"id": projectID.String()},
			},
		},
	}
	var env v3Entity[channelConfigV3]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/notification/channel-configs",
		httpcl.Body(body),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	out := env.Data.toChannelConfig()
	return &out, nil
}

func (c *Client) SetChannelConfigEnabled(ctx context.Context, id uuid.UUID, enabled bool) (*ChannelConfig, error) {
	var env v3Entity[channelConfigV3]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/notification/channel-configs/%s/update",
		httpcl.RouteParams(id),
		httpcl.Body(map[string]any{"is_enabled": enabled}),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	out := env.Data.toChannelConfig()
	return &out, nil
}

func (c *Client) DeleteChannelConfig(ctx context.Context, id uuid.UUID) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3/notification/channel-configs/%s",
		httpcl.RouteParams(id),
	))
	return err
}

func (c *Client) ListProjectNotificationPreferences(
	ctx context.Context,
	orgID, projectID uuid.UUID,
) ([]NotificationPreference, error) {
	var env v3List[preferenceV3]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/notification/preferences",
		filterParam("scope", "project"),
		filterParam("org_id", orgID.String()),
		filterParam("project_id", projectID.String()),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	out := make([]NotificationPreference, 0, len(env.Data))
	for _, p := range env.Data {
		out = append(out, NotificationPreference{
			ID:      p.ID,
			Name:    p.Attributes.PreferenceName,
			Group:   p.Attributes.GroupName,
			Channel: p.Attributes.Channel,
			Enabled: p.Attributes.IsEnabled,
		})
	}
	return out, nil
}

func (c *Client) ListSlackIntegrations(ctx context.Context, orgID uuid.UUID) ([]NotificationIntegration, error) {
	var env v3List[integrationV3]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/notification/integrations",
		filterParam("org_id", orgID.String()),
		filterParam("type", "slack"),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	out := make([]NotificationIntegration, 0, len(env.Data))
	for _, i := range env.Data {
		out = append(out, NotificationIntegration{
			ID:            i.ID,
			Type:          i.Attributes.Type,
			WorkspaceName: i.Attributes.WorkspaceName,
			Status:        i.Attributes.Status,
		})
	}
	return out, nil
}
