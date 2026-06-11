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
	"fmt"
	"strings"
	"time"
)

// --- V3 wire types ---

type eventAttributesWire struct {
	Phase          string           `json:"phase"`
	Outcome        string           `json:"outcome,omitempty"`
	CurrentOutcome string           `json:"current_outcome,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	VCS            *eventVCSWire    `json:"vcs,omitempty"`
	Errors         []eventErrorWire `json:"errors,omitempty"`
}

type eventErrorWire struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type eventVCSWire struct {
	Branch   string `json:"branch"`
	Revision string `json:"revision"`
}

type eventReferencesWire struct {
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
	User struct {
		ID string `json:"id"`
	} `json:"user"`
}

type eventWire struct {
	ID         string              `json:"id"`
	Attributes eventAttributesWire `json:"attributes"`
	References eventReferencesWire `json:"references"`
}

// --- V3 domain types ---

// EventError holds a config or setup error from the V3 API.
type EventError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Event is the record of a trigger firing — it groups the workflows
// produced by that firing and carries their shared VCS context and any
// pre-workflow errors.
type Event struct {
	ID             string       `json:"id"`
	Phase          string       `json:"phase"`
	Outcome        string       `json:"outcome,omitempty"`
	CurrentOutcome string       `json:"current_outcome,omitempty"`
	Branch         string       `json:"branch,omitempty"`
	Revision       string       `json:"revision,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	ProjectID      string       `json:"project_id"`
	Errors         []EventError `json:"errors,omitempty"`
}

// Status derives a display status from phase and outcome.
func (r Event) Status() string {
	return PhaseOutcomeStatus(r.Phase, r.Outcome, r.CurrentOutcome)
}

func (w eventWire) toEvent() *Event {
	a := w.Attributes
	r := &Event{
		ID:             w.ID,
		Phase:          a.Phase,
		Outcome:        a.Outcome,
		CurrentOutcome: a.CurrentOutcome,
		CreatedAt:      a.CreatedAt,
		ProjectID:      w.References.Project.ID,
	}
	if a.VCS != nil {
		r.Branch = a.VCS.Branch
		r.Revision = a.VCS.Revision
	}
	for _, e := range a.Errors {
		r.Errors = append(r.Errors, EventError(e))
	}
	return r
}

// GetEvent fetches a single event by UUID from the V3 API.
func (c *Client) GetEvent(ctx context.Context, id string) (*Event, error) {
	var env v3Entity[eventWire]
	if err := c.getV3(ctx, "/events/%s", &env, routeParams(id)); err != nil {
		return nil, err
	}
	return env.Data.toEvent(), nil
}

// EventSearchParams configures a V3 events search request.
type EventSearchParams struct {
	ProjectIDs []string
	From       time.Time
	To         time.Time
	Filter     string
	OrderBy    string
	Limit      int
	Cursor     string
}

type eventSearchRequest struct {
	Scope   eventSearchScope `json:"scope"`
	Filter  string           `json:"filter"`
	OrderBy string           `json:"order_by,omitempty"`
	Page    eventSearchPage  `json:"page"`
}

type eventSearchScope struct {
	ProjectIDs []string `json:"project_ids"`
	From       string   `json:"from"`
	To         string   `json:"to"`
}

type eventSearchPage struct {
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

// SearchEvents searches for events using the V3 search endpoint.
func (c *Client) SearchEvents(ctx context.Context, params EventSearchParams) ([]Event, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}

	body := eventSearchRequest{
		Scope: eventSearchScope{
			ProjectIDs: params.ProjectIDs,
			From:       params.From.Format(time.RFC3339),
			To:         params.To.Format(time.RFC3339),
		},
		Filter:  params.Filter,
		OrderBy: params.OrderBy,
		Page: eventSearchPage{
			Cursor: params.Cursor,
			Limit:  params.Limit,
		},
	}

	var resp v3List[eventWire]
	if err := c.postV3(ctx, "/events/search", body, &resp); err != nil {
		return nil, err
	}

	events := make([]Event, len(resp.Data))
	for i, w := range resp.Data {
		events[i] = *w.toEvent()
	}
	return events, nil
}

// BuildEventFilter constructs a filter expression for the V3 events search endpoint.
func BuildEventFilter(branch, status string) string {
	var parts []string
	if branch != "" {
		parts = append(parts, fmt.Sprintf("pipeline.git.branch == %q", branch))
	}
	if status != "" {
		parts = append(parts, fmt.Sprintf("pipeline.status == %q", status))
	}
	return strings.Join(parts, " and ")
}
