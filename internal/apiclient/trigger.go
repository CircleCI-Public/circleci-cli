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
	"time"
)

// TriggerEventSourceRepo holds repository information for a trigger event source.
type TriggerEventSourceRepo struct {
	ExternalID string `json:"external_id"`
	FullName   string `json:"full_name,omitempty"`
}

// TriggerEventSourceWebhook holds webhook information for a trigger event source.
type TriggerEventSourceWebhook struct {
	URL    string `json:"url,omitempty"`
	Sender string `json:"sender,omitempty"`
}

// TriggerEventSourceSchedule holds schedule information for a trigger event source.
type TriggerEventSourceSchedule struct {
	CronExpression string `json:"cron_expression,omitempty"`
}

// TriggerEventSource describes the event source for a trigger.
type TriggerEventSource struct {
	Provider string                      `json:"provider"`
	Repo     *TriggerEventSourceRepo     `json:"repo,omitempty"`
	Webhook  *TriggerEventSourceWebhook  `json:"webhook,omitempty"`
	Schedule *TriggerEventSourceSchedule `json:"schedule,omitempty"`
}

// Trigger represents a CircleCI project trigger.
type Trigger struct {
	ID          string             `json:"id"`
	CreatedAt   time.Time          `json:"created_at"`
	EventName   string             `json:"event_name,omitempty"`
	EventSource TriggerEventSource `json:"event_source"`
	EventPreset string             `json:"event_preset,omitempty"`
	ConfigRef   string             `json:"config_ref,omitempty"`
	CheckoutRef string             `json:"checkout_ref,omitempty"`
	Disabled    bool               `json:"disabled"`
}

// ListTriggers returns all triggers for a project's pipeline definition.
func (c *Client) ListTriggers(ctx context.Context, projectID, pipelineDefinitionID string) ([]Trigger, error) {
	var resp struct {
		Items []Trigger `json:"items"`
	}
	err := c.get(ctx, "/projects/%s/pipeline-definitions/%s/triggers", &resp,
		routeParams(projectID, pipelineDefinitionID),
	)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateTrigger creates a new trigger for a project's pipeline definition.
// provider must be one of: github_app, github_server, github_oauth, webhook, schedule.
// repoID is the repository external ID; required for github_app, github_server, and github_oauth.
func (c *Client) CreateTrigger(ctx context.Context, projectID, pipelineDefinitionID, provider, repoID, eventPreset, configRef, checkoutRef string) (*Trigger, error) {
	eventSource := map[string]any{
		"provider": provider,
	}
	if repoID != "" {
		eventSource["repo"] = map[string]any{
			"external_id": repoID,
		}
	}
	body := map[string]any{
		"event_source": eventSource,
	}
	if eventPreset != "" {
		body["event_preset"] = eventPreset
	}
	if configRef != "" {
		body["config_ref"] = configRef
	}
	if checkoutRef != "" {
		body["checkout_ref"] = checkoutRef
	}

	var resp Trigger
	err := c.post(ctx, "/projects/%s/pipeline-definitions/%s/triggers", body, &resp,
		routeParams(projectID, pipelineDefinitionID),
	)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
