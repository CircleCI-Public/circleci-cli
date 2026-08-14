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
	"encoding/json"
	"net/http"
	"time"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// TriggerEventSourceRepo holds repository information for a trigger event source.
type TriggerEventSourceRepo struct {
	ExternalID string `json:"external_id"`
	FullName   string `json:"full_name,omitempty"`
}

// TriggerEventSourceWebhook holds webhook information for a trigger event source.
// Source is the source system (e.g. "vercel"); Name is the event it sends.
type TriggerEventSourceWebhook struct {
	Name   string `json:"name,omitempty"`
	Source string `json:"source,omitempty"`
}

// TriggerEventSourceSchedule holds schedule information for a trigger event source.
type TriggerEventSourceSchedule struct {
	Name           string `json:"name,omitempty"`
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

// event.type values on the v3 triggers endpoints. Every provider that is not a
// custom webhook or a schedule is VCS-backed.
const (
	eventTypeVCS      = "vcs"
	eventTypeWebhook  = "webhook"
	eventTypeSchedule = "schedule"
)

// triggerRefAttrs wraps a git ref override for the config or checkout.
type triggerRefAttrs struct {
	Ref string `json:"ref,omitempty"`
}

// triggerFilterAttrs is the event filter: which events the trigger fires on.
// Preset is the trigger-level event key shared by all of its rules; it is absent
// for a trigger whose rules match no preset.
type triggerFilterAttrs struct {
	Preset string `json:"preset,omitempty"`
}

// triggerEventAttrs is a tagged union on Type: exactly one of VCS/Webhook/Schedule
// is populated. Schedule stays raw — the CLI only reads the cron expression out of
// it, and re-encoding the rest would lose fields.
type triggerEventAttrs struct {
	Type     string              `json:"type"`
	VCS      *vcsAttrs           `json:"vcs,omitempty"`
	Webhook  *webhookAttrs       `json:"webhook,omitempty"`
	Schedule json.RawMessage     `json:"schedule,omitempty"`
	Filter   *triggerFilterAttrs `json:"filter,omitempty"`
}

// webhookAttrs describes a custom webhook trigger.
type webhookAttrs struct {
	Name   string `json:"name,omitempty"`
	Source string `json:"source,omitempty"`
}

// triggerAttrs is the attributes object of a v3 trigger entity, used for both the
// create body and the response.
type triggerAttrs struct {
	IsDisabled bool               `json:"is_disabled"`
	CreatedAt  *time.Time         `json:"created_at,omitempty"`
	Event      *triggerEventAttrs `json:"event,omitempty"`
	Config     *triggerRefAttrs   `json:"config,omitempty"`
	Checkout   *triggerRefAttrs   `json:"checkout,omitempty"`
}

// triggerRefs is the references object of a trigger entity: the owning project and
// the pipeline the trigger hangs off.
type triggerRefs struct {
	Project  entityRef `json:"project"`
	Pipeline entityRef `json:"pipeline"`
}

// triggerEntity is the data entity of GET/POST /api/v3/triggers.
type triggerEntity struct {
	ID         string       `json:"id"`
	Attributes triggerAttrs `json:"attributes"`
	References triggerRefs  `json:"references"`
}

// The create body is a narrower document than the entity: the endpoint rejects
// unknown members, so it must carry no id, no created_at, and no project
// reference (the project is derived from the pipeline, or from
// filter[project_id]).
type triggerCreateAttrs struct {
	IsDisabled bool               `json:"is_disabled"`
	Event      *triggerEventAttrs `json:"event,omitempty"`
	Config     *triggerRefAttrs   `json:"config,omitempty"`
	Checkout   *triggerRefAttrs   `json:"checkout,omitempty"`
}

type triggerCreateRefs struct {
	Pipeline entityRef `json:"pipeline"`
}

type triggerCreateData struct {
	Attributes triggerCreateAttrs `json:"attributes"`
	References triggerCreateRefs  `json:"references"`
}

// ListTriggers returns all triggers for a project's pipeline definition via
// GET /api/v3/triggers. Both filters are sent so a trigger has to belong to the
// project as well as the pipeline, matching the old project-scoped route.
func (c *Client) ListTriggers(ctx context.Context, projectID, pipelineDefinitionID string) ([]Trigger, error) {
	var resp v3List[triggerEntity]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/triggers",
		filterParam("project_id", projectID),
		filterParam("pipeline_id", pipelineDefinitionID),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}

	triggers := make([]Trigger, 0, len(resp.Data))
	for _, e := range resp.Data {
		triggers = append(triggers, e.toTrigger())
	}
	return triggers, nil
}

// CreateTriggerInput contains the fields for creating a trigger.
// Provider must be one of: github_app, github_server, github_oauth, origin,
// webhook, schedule. RepoID is the repository external ID, required by every
// VCS-backed provider; RepoFullName is additionally required by providers that
// address a repository by owner and name rather than by id.
type CreateTriggerInput struct {
	ProjectID            string
	PipelineDefinitionID string
	Provider             string
	RepoID               string
	RepoFullName         string
	EventPreset          string
	ConfigRef            string
	CheckoutRef          string
}

// CreateTrigger creates a new trigger for a project's pipeline definition via
// POST /api/v3/triggers. filter[project_id] is sent so triggers on a project's
// synthetic OAuth pipeline — whose id resolves to no stored row — can be created
// too.
func (c *Client) CreateTrigger(ctx context.Context, input CreateTriggerInput) (*Trigger, error) {
	event := &triggerEventAttrs{Type: eventTypeForProvider(input.Provider)}
	switch event.Type {
	case eventTypeWebhook:
		event.Webhook = &webhookAttrs{}
	case eventTypeSchedule:
		// A schedule's cron expression has no flag yet, so the server rejects the
		// create rather than inventing one.
	default:
		event.VCS = &vcsAttrs{
			Provider:     input.Provider,
			RepoID:       input.RepoID,
			RepoFullName: input.RepoFullName,
		}
		if input.EventPreset != "" {
			event.Filter = &triggerFilterAttrs{Preset: input.EventPreset}
		}
	}

	attrs := triggerCreateAttrs{Event: event}
	if input.ConfigRef != "" {
		attrs.Config = &triggerRefAttrs{Ref: input.ConfigRef}
	}
	if input.CheckoutRef != "" {
		attrs.Checkout = &triggerRefAttrs{Ref: input.CheckoutRef}
	}

	body := v3Entity[triggerCreateData]{Data: triggerCreateData{
		Attributes: attrs,
		References: triggerCreateRefs{Pipeline: entityRef{ID: input.PipelineDefinitionID}},
	}}

	var resp v3Entity[triggerEntity]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/triggers",
		filterParam("project_id", input.ProjectID),
		httpcl.Body(body),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	trig := resp.Data.toTrigger()
	return &trig, nil
}

// eventTypeForProvider maps a provider onto the event union's discriminator.
func eventTypeForProvider(provider string) string {
	switch provider {
	case eventTypeWebhook:
		return eventTypeWebhook
	case eventTypeSchedule:
		return eventTypeSchedule
	default:
		return eventTypeVCS
	}
}

// toTrigger flattens a v3 trigger entity into the shape the commands render. A
// non-VCS trigger reports its event type as the provider, which is the vocabulary
// --provider already uses.
func (e triggerEntity) toTrigger() Trigger {
	t := Trigger{
		ID:       e.ID,
		Disabled: e.Attributes.IsDisabled,
	}
	if e.Attributes.CreatedAt != nil {
		t.CreatedAt = *e.Attributes.CreatedAt
	}
	if e.Attributes.Config != nil {
		t.ConfigRef = e.Attributes.Config.Ref
	}
	if e.Attributes.Checkout != nil {
		t.CheckoutRef = e.Attributes.Checkout.Ref
	}

	event := e.Attributes.Event
	if event == nil {
		return t
	}
	if event.Filter != nil {
		t.EventPreset = event.Filter.Preset
	}

	switch {
	case event.Webhook != nil:
		t.EventSource = TriggerEventSource{
			Provider: eventTypeWebhook,
			Webhook:  &TriggerEventSourceWebhook{Name: event.Webhook.Name, Source: event.Webhook.Source},
		}
		// The source system is the trigger's stored name, so it is what identifies
		// the event to a reader.
		t.EventName = event.Webhook.Source
	case event.Type == eventTypeSchedule:
		t.EventSource = TriggerEventSource{
			Provider: eventTypeSchedule,
			Schedule: scheduleFromRaw(event.Schedule),
		}
		if t.EventSource.Schedule != nil {
			t.EventName = t.EventSource.Schedule.Name
		}
	case event.VCS != nil:
		t.EventSource = TriggerEventSource{Provider: event.VCS.Provider}
		if event.VCS.RepoID != "" || event.VCS.RepoFullName != "" {
			t.EventSource.Repo = &TriggerEventSourceRepo{
				ExternalID: event.VCS.RepoID,
				FullName:   event.VCS.RepoFullName,
			}
		}
	}
	return t
}

// scheduleFromRaw pulls the fields the CLI shows out of the schedule object,
// ignoring the rest. Unparseable input yields no schedule rather than an error:
// the trigger itself is still worth reporting.
func scheduleFromRaw(raw json.RawMessage) *TriggerEventSourceSchedule {
	if len(raw) == 0 {
		return nil
	}
	var s TriggerEventSourceSchedule
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	return &s
}
