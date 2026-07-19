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

	"github.com/google/uuid"
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
	Branch        string         `json:"branch"`
	Tag           string         `json:"tag"`
	Revision      string         `json:"revision"`
	RepositoryURL string         `json:"repository_url"`
	Commit        *runCommitWire `json:"commit,omitempty"`
}

type runCommitWire struct {
	Subject string              `json:"subject"`
	URL     string              `json:"url"`
	Author  runCommitAuthorWire `json:"author"`
}

type runCommitAuthorWire struct {
	Name  string `json:"name"`
	Login string `json:"login"`
}

type runReferencesWire struct {
	// Event carries the VCS details of the event that triggered the run,
	// including the tag — which the legacy top-level attributes.vcs lacks.
	Event   runEventRefWire   `json:"event"`
	Trigger runTriggerRefWire `json:"trigger"`
	Project struct {
		ID uuid.UUID `json:"id"`
	} `json:"project"`
	User struct {
		ID uuid.UUID `json:"id"`
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
	ID         uuid.UUID         `json:"id"`
	Attributes runAttributesWire `json:"attributes"`
	References runReferencesWire `json:"references"`
}

// --- V3 domain types ---

// RunError holds a config or setup error from the V3 API.
type RunError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// RunCommit holds the commit metadata attached to a run event.
type RunCommit struct {
	Subject     string `json:"subject,omitempty"`
	URL         string `json:"url,omitempty"`
	AuthorName  string `json:"author_name,omitempty"`
	AuthorLogin string `json:"author_login,omitempty"`
}

// RunV3 holds run detail from the V3 API.
type RunV3 struct {
	ID             uuid.UUID  `json:"id"`
	Phase          string     `json:"phase"`
	Outcome        string     `json:"outcome,omitempty"`
	CurrentOutcome string     `json:"current_outcome,omitempty"`
	Branch         string     `json:"branch,omitempty"`
	Tag            string     `json:"tag,omitempty"`
	Revision       string     `json:"revision,omitempty"`
	RepositoryURL  string     `json:"repository_url,omitempty"`
	Commit         *RunCommit `json:"commit,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ProjectID      uuid.UUID  `json:"project_id"`
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
		if c := v.Commit; c != nil {
			r.Commit = &RunCommit{
				Subject:     c.Subject,
				URL:         c.URL,
				AuthorName:  c.Author.Name,
				AuthorLogin: c.Author.Login,
			}
		}
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
func (c *Client) GetRunV3(ctx context.Context, id uuid.UUID) (*RunV3, error) {
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

// maxRunsPageSize is the maximum page size accepted by the runs API endpoints.
// Requests with a larger page size are rejected with 400 "Invalid page size".
const maxRunsPageSize = 20

// paginateRunsV3 drives the common pagination loop shared by SearchRunsV3 and
// ListMyRunsV3. fetch is called once per page with the clamped page size and
// current cursor; it returns the raw response page. The loop stops when the
// response carries no next cursor, returns an empty page, or the requested
// total has been collected.
func paginateRunsV3(
	limit int,
	cursor string,
	fetch func(pageSize int, cursor string) (v3List[runWire], error),
) ([]RunV3, error) {
	if limit <= 0 {
		limit = 10
	}
	var allRuns []RunV3
	remaining := limit
	for remaining > 0 {
		pageSize := remaining
		if pageSize > maxRunsPageSize {
			pageSize = maxRunsPageSize
		}
		resp, err := fetch(pageSize, cursor)
		if err != nil {
			return nil, err
		}
		for _, w := range resp.Data {
			allRuns = append(allRuns, *w.toRunV3())
		}
		remaining -= len(resp.Data)
		if resp.Page.Next == nil || len(resp.Data) == 0 {
			break
		}
		cursor = *resp.Page.Next
	}
	return allRuns, nil
}

// SearchRunsV3 searches for runs using the V3 search endpoint.
// It paginates transparently when params.Limit exceeds maxRunsPageSize.
func (c *Client) SearchRunsV3(ctx context.Context, params RunSearchParams) ([]RunV3, error) {
	return paginateRunsV3(params.Limit, params.Cursor, func(pageSize int, cursor string) (v3List[runWire], error) {
		body := runSearchRequest{
			Scope: runSearchScope{
				ProjectIDs: params.ProjectIDs,
				From:       params.From.Format(time.RFC3339),
				To:         params.To.Format(time.RFC3339),
			},
			Filter:  params.Filter,
			OrderBy: params.OrderBy,
			Page: runSearchPage{
				Cursor: cursor,
				Limit:  pageSize,
			},
		}
		var resp v3List[runWire]
		err := c.postV3(ctx, "/runs/search", body, &resp)
		return resp, err
	})
}

// MyRunsParams configures a ListMyRunsV3 request. All fields are optional: the
// zero value lists every recent run the endpoint defaults to.
type MyRunsParams struct {
	// Limit caps the page size; a value <= 0 uses the server default.
	Limit int
	// Status narrows the list to runs with that pipeline status (e.g. "failed",
	// "on_hold"); an empty status lists every status.
	Status string
	// From and To bound the list to runs created within that window
	// (filter[from]/filter[to], RFC3339). A nil bound is omitted, letting the
	// endpoint apply its own default for that side.
	From *time.Time
	To   *time.Time
}

// ListMyRunsV3 lists runs triggered by the authenticated user across all
// projects, via GET /api/v3/runs?filter[user_id]=me. Paginates transparently
// when params.Limit exceeds maxRunsPageSize.
//
// Unlike the runs/search endpoint, this endpoint has no pipeline.status filter —
// it filters on the run's own phase and current_outcome — so the status is
// converted to those via StatusPhaseOutcome.
func (c *Client) ListMyRunsV3(ctx context.Context, params MyRunsParams) ([]RunV3, error) {
	phase, currentOutcome := StatusPhaseOutcome(params.Status)
	return paginateRunsV3(params.Limit, "", func(pageSize int, cursor string) (v3List[runWire], error) {
		var resp v3List[runWire]
		err := c.getV3(ctx, "/runs", &resp,
			filterParam("user_id", "me"),
			filterParam("phase", phase),
			filterParam("current_outcome", currentOutcome),
			filterParam("from", rfc3339OrEmpty(params.From)),
			filterParam("to", rfc3339OrEmpty(params.To)),
			pageLimit(pageSize),
			pageCursor(cursor))
		return resp, err
	})
}

// Pipeline status values, as reported by the V3 runs API and accepted by the
// pipeline.status search filter.
const (
	StatusCanceled     = "canceled"
	StatusError        = "error"
	StatusFailed       = "failed"
	StatusFailing      = "failing"
	StatusNotRun       = "not_run"
	StatusOnHold       = "on_hold"
	StatusQueued       = "queued"
	StatusRunning      = "running"
	StatusSuccess      = "success"
	StatusUnauthorized = "unauthorized"
)

// StatusPhaseOutcome maps a pipeline.status value (the tokens above, as used by
// the runs/search pipeline.status filter and the run picker's status cycle) to
// the run phase and current_outcome the my-runs list endpoint filters on
// (filter[phase], filter[current_outcome]). An empty status — or an unknown one —
// yields two empty strings, which filterParam then omits (no status filter).
//
// The pipeline status is an aggregate; a run carries phase ∈ {created, queued,
// started, ended} and current_outcome. Terminal statuses are an "ended" phase
// with the matching outcome; in-progress statuses are the "started" (or
// "queued") phase, narrowed by current_outcome only where one distinguishes them
// (a partially-failed run is "started"/"failed" → "failing"; a plainly-running
// one is just "started").
func StatusPhaseOutcome(status string) (phase, currentOutcome string) {
	switch status {
	case StatusSuccess:
		return "ended", "succeeded"
	case StatusFailed:
		return "ended", "failed"
	case StatusCanceled:
		return "ended", "canceled"
	case StatusError:
		return "ended", "errored"
	case StatusNotRun:
		return "ended", "not_run"
	case StatusUnauthorized:
		return "ended", "unauthorized"
	case StatusFailing:
		return "started", "failed"
	case StatusRunning:
		return "started", ""
	case StatusQueued:
		return "queued", ""
	default:
		return "", ""
	}
}

// rfc3339OrEmpty formats t as an RFC3339 timestamp, or returns "" for a nil
// pointer so filterParam omits the bound entirely.
func rfc3339OrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
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
