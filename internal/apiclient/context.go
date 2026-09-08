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
	"sort"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// contextPageLimit is the page size requested from the v3 context list
// endpoints. The API accepts up to 250; 100 matches the other v3 list clients
// here and keeps a single page enough for most orgs.
const contextPageLimit = 100

// Context is a CircleCI context — a named collection of secret environment
// variables shared across pipelines in an organization.
//
// OrgID is populated by GetContext but not by ListContexts: the list response
// carries no references, since every item in it belongs to the org that was
// filtered on.
type Context struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
	OrgID     uuid.UUID
}

// ContextEnvVar is an environment variable stored in a context.
// The value is never returned by the API; TruncatedValue is the last few
// characters, enough to identify which secret is in a slot.
type ContextEnvVar struct {
	Variable       string
	TruncatedValue string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ContextID      uuid.UUID
}

// ContextRestriction is a restriction limiting which projects, groups or
// pipelines may use a context.
//
// Name is the resolved name of whatever the restriction points at — a project's
// or a group's. It is empty for expression restrictions, which have nothing to
// name, and for a subject the API could not resolve.
type ContextRestriction struct {
	ContextID       uuid.UUID
	ID              uuid.UUID
	Name            string
	RestrictionType string
	MatchPattern    string
}

// ContextDetail is a context together with everything the CLI shows for it.
//
// v2 served all of this from one endpoint; v3 splits it three ways — the
// context, its environment variables and its restrictions. GetContextDetail
// reassembles it.
type ContextDetail struct {
	ID                   uuid.UUID
	Name                 string
	CreatedAt            time.Time
	OrgID                uuid.UUID
	EnvironmentVariables []ContextEnvVar
	Restrictions         []ContextRestriction
}

// --- wire types ---

// contextV3 is a context entity in a v3 response.
type contextV3 struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"created_at"`
	} `json:"attributes"`
	References struct {
		Org *struct {
			ID uuid.UUID `json:"id"`
		} `json:"org"`
	} `json:"references"`
}

func (c contextV3) toContext() Context {
	out := Context{
		ID:        c.ID,
		Name:      c.Attributes.Name,
		CreatedAt: c.Attributes.CreatedAt,
	}
	if c.References.Org != nil {
		out.OrgID = c.References.Org.ID
	}
	return out
}

// contextEnvVarV3 is an environment-variable entity in a v3 response. It
// carries no id of its own — env vars are keyed by name within a context.
type contextEnvVarV3 struct {
	Attributes struct {
		Name           string    `json:"name"`
		TruncatedValue string    `json:"truncated_value"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
	} `json:"attributes"`
	References struct {
		Context struct {
			ID uuid.UUID `json:"id"`
		} `json:"context"`
	} `json:"references"`
}

func (e contextEnvVarV3) toEnvVar() ContextEnvVar {
	return ContextEnvVar{
		Variable:       e.Attributes.Name,
		TruncatedValue: e.Attributes.TruncatedValue,
		CreatedAt:      e.Attributes.CreatedAt,
		UpdatedAt:      e.Attributes.UpdatedAt,
		ContextID:      e.References.Context.ID,
	}
}

// contextRestrictionV3 is a restriction entity in a v3 response.
type contextRestrictionV3 struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		RestrictionType string `json:"restriction_type"`
		MatchPattern    string `json:"match_pattern"`
		// Name is the resolved name of whatever the restriction points at — a
		// project's or a group's. Absent for expression restrictions, and for a
		// subject the API could not resolve.
		Name string `json:"name"`
	} `json:"attributes"`
	References struct {
		Context struct {
			ID uuid.UUID `json:"id"`
		} `json:"context"`
	} `json:"references"`
}

func (r contextRestrictionV3) toRestriction() ContextRestriction {
	out := ContextRestriction{
		ContextID:       r.References.Context.ID,
		ID:              r.ID,
		Name:            r.Attributes.Name,
		RestrictionType: r.Attributes.RestrictionType,
		MatchPattern:    r.Attributes.MatchPattern,
	}
	return out
}

// --- contexts ---

// ListContexts returns all contexts owned by orgID, via
// GET /api/v3/contexts?filter[org_id]=. Paginates automatically.
//
// A non-empty name is sent as filter[name], which the API matches upstream as a
// case-insensitive substring — so it narrows pagination rather than just the
// final page. An empty name is omitted entirely.
func (c *Client) ListContexts(ctx context.Context, orgID uuid.UUID, name string) ([]Context, error) {
	var all []Context
	cursor := ""

	for {
		var env v3List[contextV3]
		_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/contexts",
			filterParam("org_id", orgID.String()),
			filterParam("name", name),
			pageLimit(contextPageLimit),
			pageCursor(cursor),
			httpcl.JSONDecoder(&env),
		))
		if err != nil {
			return nil, err
		}
		for _, item := range env.Data {
			all = append(all, item.toContext())
		}
		if env.Page.Next == nil || *env.Page.Next == "" {
			return all, nil
		}
		cursor = *env.Page.Next
	}
}

// CreateContext creates a context owned by orgID.
func (c *Client) CreateContext(ctx context.Context, name string, orgID uuid.UUID) (*Context, error) {
	body := map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{"name": name},
			"references": map[string]any{
				"org": map[string]any{"id": orgID.String()},
			},
		},
	}
	var env v3Entity[contextV3]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/contexts",
		httpcl.Body(body),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	out := env.Data.toContext()
	// The create response echoes the org back, but fall back to what we asked
	// for so the caller always has it.
	if out.OrgID == uuid.Nil {
		out.OrgID = orgID
	}
	return &out, nil
}

// GetContext returns a context by its UUID. The response covers the context and
// its group restrictions only; use GetContextDetail for env vars and the
// remaining restrictions.
func (c *Client) GetContext(ctx context.Context, id uuid.UUID) (*Context, error) {
	var env v3Entity[contextV3]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/contexts/%s",
		httpcl.RouteParams(id),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	out := env.Data.toContext()
	return &out, nil
}

// GetContextDetail returns a context with its environment variables and all of
// its restrictions.
//
// The three reads are independent, so they run concurrently: done in series
// this would cost three round trips to answer one `context get`. The first
// failure wins and cancels the rest, which is why each goroutine writes to its
// own variable and nothing is assembled until Wait returns.
func (c *Client) GetContextDetail(ctx context.Context, id uuid.UUID) (*ContextDetail, error) {
	var (
		ctxt         *Context
		envVars      []ContextEnvVar
		restrictions []ContextRestriction
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		ctxt, err = c.GetContext(gctx, id)
		return err
	})
	g.Go(func() (err error) {
		envVars, err = c.ListContextEnvVars(gctx, id)
		return err
	})
	g.Go(func() (err error) {
		restrictions, err = c.ListContextRestrictions(gctx, id)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return contextDetail(ctxt, envVars, restrictions), nil
}

// ContextDetailFor completes a context the caller already holds, fetching only
// its environment variables and restrictions — concurrently, as above.
//
// Prefer this to GetContextDetail whenever the context is already in hand.
// Resolving a name through ListContexts yields the context itself, so
// re-fetching it by id costs a round trip for nothing. The list carries no
// references, so callers that resolved by name should set OrgID from the org
// they filtered on.
func (c *Client) ContextDetailFor(ctx context.Context, ctxt *Context) (*ContextDetail, error) {
	var (
		envVars      []ContextEnvVar
		restrictions []ContextRestriction
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		envVars, err = c.ListContextEnvVars(gctx, ctxt.ID)
		return err
	})
	g.Go(func() (err error) {
		restrictions, err = c.ListContextRestrictions(gctx, ctxt.ID)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return contextDetail(ctxt, envVars, restrictions), nil
}

// contextDetail assembles the parts, ordering the restrictions.
func contextDetail(ctxt *Context, envVars []ContextEnvVar, restrictions []ContextRestriction) *ContextDetail {
	sortRestrictions(restrictions)

	return &ContextDetail{
		ID:                   ctxt.ID,
		Name:                 ctxt.Name,
		CreatedAt:            ctxt.CreatedAt,
		OrgID:                ctxt.OrgID,
		EnvironmentVariables: envVars,
		Restrictions:         restrictions,
	}
}

// restrictionTypeOrder groups first, then projects, then expressions — the
// order the v2 API reported them in, and so the order `context get` has always
// displayed. Anything unrecognised sorts last rather than disappearing.
var restrictionTypeOrder = map[string]int{"group": 0, "project": 1, "expression": 2}

// sortRestrictions puts the merged list in a stable, type-led order.
//
// Without this the order would follow whichever source supplied each entry,
// which changes as the API moves group restrictions onto the restrictions
// endpoint. Sorting keeps a command's output identical either side of that.
func sortRestrictions(rs []ContextRestriction) {
	order := func(t string) int {
		if i, ok := restrictionTypeOrder[t]; ok {
			return i
		}
		return len(restrictionTypeOrder)
	}
	sort.SliceStable(rs, func(i, j int) bool {
		return order(rs[i].RestrictionType) < order(rs[j].RestrictionType)
	})
}

// DeleteContext deletes a context by its UUID.
func (c *Client) DeleteContext(ctx context.Context, id uuid.UUID) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3/contexts/%s",
		httpcl.RouteParams(id),
	))
	return err
}

// --- environment variables ---

// ListContextEnvVars returns the environment variables stored in a context.
// Values are never returned by the API; see ContextEnvVar.TruncatedValue.
func (c *Client) ListContextEnvVars(ctx context.Context, contextID uuid.UUID) ([]ContextEnvVar, error) {
	var all []ContextEnvVar
	cursor := ""

	for {
		var env v3List[contextEnvVarV3]
		_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/contexts/%s/env-vars",
			httpcl.RouteParams(contextID),
			pageLimit(contextPageLimit),
			pageCursor(cursor),
			httpcl.JSONDecoder(&env),
		))
		if err != nil {
			return nil, err
		}
		for _, item := range env.Data {
			ev := item.toEnvVar()
			// The list response omits references.context on some paths; the
			// caller asked about this context, so fill it in.
			if ev.ContextID == uuid.Nil {
				ev.ContextID = contextID
			}
			all = append(all, ev)
		}
		if env.Page.Next == nil || *env.Page.Next == "" {
			return all, nil
		}
		cursor = *env.Page.Next
	}
}

// SetContextEnvVar adds or updates an environment variable in a context.
func (c *Client) SetContextEnvVar(ctx context.Context, contextID uuid.UUID, name, value string) error {
	body := map[string]any{"name": name, "value": value}
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/contexts/%s/env-vars/set",
		httpcl.RouteParams(contextID),
		httpcl.Body(body),
	))
	return err
}

// DeleteContextEnvVar removes an environment variable from a context. The name
// is a query filter rather than a path segment, which is what the endpoint
// expects.
func (c *Client) DeleteContextEnvVar(ctx context.Context, contextID uuid.UUID, name string) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3/contexts/%s/env-vars",
		httpcl.RouteParams(contextID),
		httpcl.QueryParam("filter[name]", name),
	))
	return err
}

// --- restrictions ---

// ListContextRestrictions returns all of a context's restrictions — group,
// project and expression. It is the only source of group access; the context
// endpoints do not report it.
func (c *Client) ListContextRestrictions(ctx context.Context, contextID uuid.UUID) ([]ContextRestriction, error) {
	var env v3List[contextRestrictionV3]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/context-restrictions",
		filterParam("context_id", contextID.String()),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}

	out := make([]ContextRestriction, 0, len(env.Data))
	for _, item := range env.Data {
		r := item.toRestriction()
		if r.ContextID == uuid.Nil {
			r.ContextID = contextID
		}
		out = append(out, r)
	}
	return out, nil
}

// CreateContextRestriction adds a restriction to a context. restrictionType is
// "project", "expression" or "group"; matchPattern is a project UUID, a
// pipeline expression, or a group UUID to match. Passing the organization's own
// id as a group grants all members.
func (c *Client) CreateContextRestriction(
	ctx context.Context, contextID uuid.UUID, restrictionType, matchPattern string,
) (*ContextRestriction, error) {
	body := map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{
				"restriction_type": restrictionType,
				"match_pattern":    matchPattern,
			},
			"references": map[string]any{
				"context": map[string]any{"id": contextID.String()},
			},
		},
	}
	var env v3Entity[contextRestrictionV3]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/context-restrictions",
		httpcl.Body(body),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	out := env.Data.toRestriction()
	if out.ContextID == uuid.Nil {
		out.ContextID = contextID
	}
	return &out, nil
}

// DeleteContextRestriction removes a restriction from a context. Both ids are
// required: the restriction is addressed by path, but the endpoint also demands
// the owning context as a filter, which is what it authorises against.
func (c *Client) DeleteContextRestriction(ctx context.Context, contextID, restrictionID uuid.UUID) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3/context-restrictions/%s",
		httpcl.RouteParams(restrictionID),
		filterParam("context_id", contextID.String()),
	))
	return err
}
