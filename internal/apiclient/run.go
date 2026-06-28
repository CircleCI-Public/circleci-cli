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

type runAttributesWire struct {
	Phase          string         `json:"phase"`
	Outcome        string         `json:"outcome,omitempty"`
	CurrentOutcome string         `json:"current_outcome,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	VCS            *runVCSWire    `json:"vcs,omitempty"`
	Errors         []runErrorWire `json:"errors,omitempty"`
}

type runErrorWire struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type runVCSWire struct {
	Branch        string `json:"branch"`
	Tag           string `json:"tag"`
	Revision      string `json:"revision"`
	RepositoryURL string `json:"repository_url"`
}

type runReferencesWire struct {
	// Event carries the VCS details of the event that triggered the run,
	// including the tag — which the legacy top-level attributes.vcs lacks.
	Event   runEventRefWire   `json:"event"`
	Trigger runTriggerRefWire `json:"trigger"`
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
	User struct {
		ID string `json:"id"`
	} `json:"user"`
}

type runEventRefWire struct {
	Attributes struct {
		Type   string      `json:"type"`
		Action string      `json:"action"`
		VCS    *runVCSWire `json:"vcs"`
	} `json:"attributes"`
}

type runTriggerRefWire struct {
	Attributes struct {
		EventSource struct {
			Type string `json:"type"`
		} `json:"event_source"`
	} `json:"attributes"`
}

type runWire struct {
	ID         string            `json:"id"`
	Attributes runAttributesWire `json:"attributes"`
	References runReferencesWire `json:"references"`
}

// --- V3 domain types ---

// RunError holds a config or setup error from the V3 API.
type RunError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// RunV3 holds run detail from the V3 API.
type RunV3 struct {
	ID             string     `json:"id"`
	Phase          string     `json:"phase"`
	Outcome        string     `json:"outcome,omitempty"`
	CurrentOutcome string     `json:"current_outcome,omitempty"`
	Branch         string     `json:"branch,omitempty"`
	Tag            string     `json:"tag,omitempty"`
	Revision       string     `json:"revision,omitempty"`
	RepositoryURL  string     `json:"repository_url,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ProjectID      string     `json:"project_id"`
	Errors         []RunError `json:"errors,omitempty"`
}

// Status derives a display status from phase and outcome.
func (r RunV3) Status() string {
	return PhaseOutcomeStatus(r.Phase, r.Outcome, r.CurrentOutcome)
}

func (w runWire) toRunV3() *RunV3 {
	a := w.Attributes
	r := &RunV3{
		ID:             w.ID,
		Phase:          a.Phase,
		Outcome:        a.Outcome,
		CurrentOutcome: a.CurrentOutcome,
		CreatedAt:      a.CreatedAt,
		ProjectID:      w.References.Project.ID,
	}
	// VCS details now live on the event reference, which is the only source
	// that carries the tag. Fall back to the legacy top-level attributes.vcs
	// (branch/revision only) while the API still serves it during rollout.
	if v := w.References.Event.Attributes.VCS; v != nil {
		r.Branch = v.Branch
		r.Tag = v.Tag
		r.Revision = v.Revision
	} else if a.VCS != nil {
		r.Branch = a.VCS.Branch
		r.Revision = a.VCS.Revision
	}
	// repository_url is only carried by the top-level attributes.vcs, not the
	// event reference — set it independently of the branch/tag source above.
	if a.VCS != nil {
		r.RepositoryURL = a.VCS.RepositoryURL
	}
	for _, e := range a.Errors {
		r.Errors = append(r.Errors, RunError(e))
	}
	return r
}

// GetRunV3 fetches a single run by UUID from the V3 API.
func (c *Client) GetRunV3(ctx context.Context, id string) (*RunV3, error) {
	var env v3Entity[runWire]
	if err := c.getV3(ctx, "/runs/%s", &env, routeParams(id)); err != nil {
		return nil, err
	}
	return env.Data.toRunV3(), nil
}

// RunSearchParams configures a V3 runs search request.
type RunSearchParams struct {
	ProjectIDs []string
	From       time.Time
	To         time.Time
	Filter     string
	OrderBy    string
	Limit      int
	Cursor     string
}

type runSearchRequest struct {
	Scope   runSearchScope `json:"scope"`
	Filter  string         `json:"filter"`
	OrderBy string         `json:"order_by,omitempty"`
	Page    runSearchPage  `json:"page"`
}

type runSearchScope struct {
	ProjectIDs []string `json:"project_ids"`
	From       string   `json:"from"`
	To         string   `json:"to"`
}

type runSearchPage struct {
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

// SearchRunsV3 searches for runs using the V3 search endpoint.
func (c *Client) SearchRunsV3(ctx context.Context, params RunSearchParams) ([]RunV3, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}

	body := runSearchRequest{
		Scope: runSearchScope{
			ProjectIDs: params.ProjectIDs,
			From:       params.From.Format(time.RFC3339),
			To:         params.To.Format(time.RFC3339),
		},
		Filter:  params.Filter,
		OrderBy: params.OrderBy,
		Page: runSearchPage{
			Cursor: params.Cursor,
			Limit:  params.Limit,
		},
	}

	var resp v3List[runWire]
	if err := c.postV3(ctx, "/runs/search", body, &resp); err != nil {
		return nil, err
	}

	runs := make([]RunV3, len(resp.Data))
	for i, w := range resp.Data {
		runs[i] = *w.toRunV3()
	}
	return runs, nil
}

// ListMyRunsV3 lists runs triggered by the authenticated user across all
// projects, via GET /api/v3/runs?filter[user_id]=me. limit caps the page size;
// a value <= 0 uses the server default.
func (c *Client) ListMyRunsV3(ctx context.Context, limit int) ([]RunV3, error) {
	var resp v3List[runWire]
	if err := c.getV3(ctx, "/runs", &resp,
		filterParam("user_id", "me"), pageLimit(limit)); err != nil {
		return nil, err
	}

	runs := make([]RunV3, len(resp.Data))
	for i, w := range resp.Data {
		runs[i] = *w.toRunV3()
	}
	return runs, nil
}

// BuildRunFilter constructs a filter expression for the V3 runs/search endpoint.
func BuildRunFilter(branch, status string) string {
	var parts []string
	if branch != "" {
		parts = append(parts, fmt.Sprintf("pipeline.git.branch == %q", branch))
	}
	if status != "" {
		parts = append(parts, fmt.Sprintf("pipeline.status == %q", status))
	}
	return strings.Join(parts, " and ")
}
