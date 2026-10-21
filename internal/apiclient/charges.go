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
	"time"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// Charge analyses accepted by POST /api/v3/analysis/charges. Each groups the
// matching charges by project and by the named entity.
const (
	ChargeAnalysisJob      = "charge.job"
	ChargeAnalysisWorkflow = "charge.workflow"
	ChargeAnalysisPipeline = "charge.pipeline"
)

// maxChargesPageSize is the largest page.limit the charges endpoint accepts;
// larger values are rejected with a 400 rather than clamped.
const maxChargesPageSize = 20

// ChargeSearchParams configures a SearchCharges request.
type ChargeSearchParams struct {
	// Analysis is one of the ChargeAnalysis* constants.
	Analysis string
	// ProjectIDs scopes the search; the endpoint requires at least one.
	ProjectIDs []uuid.UUID
	// From and To bound the charges by when they were recorded. Set both: the
	// endpoint otherwise looks back only 14 days.
	From time.Time
	To   time.Time
	// Filter is an expression over the charge vocabulary, e.g.
	// `pipeline.id == "<uuid>"`. Empty matches every charge in scope.
	Filter string
	// OrderBy is "<field> [asc|desc]"; empty uses the server default
	// (credits desc).
	OrderBy string
}

// Charge is one row of a charges analysis: the credits spent on one named
// entity (job, workflow or pipeline definition) in one project.
type Charge struct {
	Name          string    `json:"name"`
	Credits       int64     `json:"credits"`
	Runs          int       `json:"runs"`
	CreditsPerRun float64   `json:"credits_per_run"`
	ProjectID     uuid.UUID `json:"project_id"`
}

type chargeSearchRequest struct {
	Analysis string            `json:"analysis"`
	Scope    chargeSearchScope `json:"scope"`
	Filter   string            `json:"filter,omitempty"`
	OrderBy  string            `json:"order_by,omitempty"`
	Page     chargeSearchPage  `json:"page"`
}

type chargeSearchScope struct {
	ProjectIDs []uuid.UUID `json:"project_ids"`
	From       string      `json:"from,omitempty"`
	To         string      `json:"to,omitempty"`
}

type chargeSearchPage struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}

type chargeWire struct {
	Attributes struct {
		Name          string  `json:"name"`
		Credits       int64   `json:"credits"`
		Runs          int     `json:"runs"`
		CreditsPerRun float64 `json:"credits_per_run"`
	} `json:"attributes"`
	References struct {
		Project struct {
			ID uuid.UUID `json:"id"`
		} `json:"project"`
	} `json:"references"`
}

// SearchCharges runs a charges analysis via POST /api/v3/analysis/charges,
// following page.next until every matching row has been read. The endpoint is
// marked experimental.
func (c *Client) SearchCharges(ctx context.Context, params ChargeSearchParams) ([]Charge, error) {
	body := chargeSearchRequest{
		Analysis: params.Analysis,
		Scope: chargeSearchScope{
			ProjectIDs: params.ProjectIDs,
			From:       rfc3339OrEmpty(nonZero(params.From)),
			To:         rfc3339OrEmpty(nonZero(params.To)),
		},
		Filter:  params.Filter,
		OrderBy: params.OrderBy,
		Page:    chargeSearchPage{Limit: maxChargesPageSize},
	}

	var charges []Charge
	for {
		var resp v3List[chargeWire]
		_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/analysis/charges",
			httpcl.Body(body),
			httpcl.JSONDecoder(&resp),
		))
		if err != nil {
			return nil, err
		}
		for _, w := range resp.Data {
			charges = append(charges, Charge{
				Name:          w.Attributes.Name,
				Credits:       w.Attributes.Credits,
				Runs:          w.Attributes.Runs,
				CreditsPerRun: w.Attributes.CreditsPerRun,
				ProjectID:     w.References.Project.ID,
			})
		}
		if resp.Page.Next == nil || *resp.Page.Next == "" || len(resp.Data) == 0 {
			return charges, nil
		}
		body.Page.Cursor = *resp.Page.Next
	}
}

// nonZero returns a pointer to t, or nil for the zero time, so an unset bound
// is left out of the request rather than sent as year 1.
func nonZero(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
