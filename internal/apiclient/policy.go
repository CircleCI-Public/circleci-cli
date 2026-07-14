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
	"fmt"
	"time"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// PolicyBundle is a map of policy name to Rego source content.
type PolicyBundle map[string]string

// DecisionSettings controls whether policy decisions are enforced for an owner.
type DecisionSettings struct {
	Enabled bool `json:"enabled"`
}

// DecisionLogsRequest holds optional filters for GetDecisionLogs.
type DecisionLogsRequest struct {
	Status    string
	After     *time.Time
	Before    *time.Time
	Branch    string
	ProjectID string
	Offset    int
}

// CreatePolicyBundle uploads a policy bundle. When dryRun is true it performs
// a diff-only check without applying changes.
func (c *Client) CreatePolicyBundle(ctx context.Context, ownerID, policyCtx string, policies PolicyBundle, dryRun bool) (json.RawMessage, error) {
	body := map[string]any{"policies": policies}
	var out json.RawMessage
	var err error
	if dryRun {
		err = c.post(ctx, "/owner/%s/context/%s/policy-bundle", body, &out,
			queryParam("dry", "true"),
			httpcl.RouteParams(ownerID, policyCtx),
		)
	} else {
		err = c.post(ctx, "/owner/%s/context/%s/policy-bundle", body, &out,
			httpcl.RouteParams(ownerID, policyCtx),
		)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FetchPolicyBundle downloads the full bundle or a single named policy.
// Pass an empty policyName to fetch the entire bundle.
func (c *Client) FetchPolicyBundle(ctx context.Context, ownerID, policyCtx string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.get(ctx, "/owner/%s/context/%s/policy-bundle", &out,
		httpcl.RouteParams(ownerID, policyCtx),
	)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) FetchPolicyBundleWithName(ctx context.Context, ownerID, policyCtx, policyName string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.get(ctx, "/owner/%s/context/%s/policy-bundle/%s", &out,
		httpcl.RouteParams(ownerID, policyCtx, policyName),
	)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetDecisionLogs returns one page of policy decision logs. The caller is
// responsible for pagination (increment Offset until an empty slice is returned).
func (c *Client) GetDecisionLogs(ctx context.Context, ownerID, policyCtx string, req DecisionLogsRequest) ([]json.RawMessage, error) {
	opts := []func(*httpcl.Request){
		httpcl.RouteParams(ownerID, policyCtx),
		optionalQueryParam("status", req.Status),
		optionalQueryParam("branch", req.Branch),
		optionalQueryParam("project_id", req.ProjectID),
	}
	if req.After != nil {
		opts = append(opts, queryParam("after", req.After.Format(time.RFC3339)))
	}
	if req.Before != nil {
		opts = append(opts, queryParam("before", req.Before.Format(time.RFC3339)))
	}
	if req.Offset > 0 {
		opts = append(opts, queryParam("offset", fmt.Sprintf("%d", req.Offset)))
	}
	var out []json.RawMessage
	err := c.get(ctx, "/owner/%s/context/%s/decision", &out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetDecisionLog returns a single decision log by ID.
// When policyBundleOnly is true, returns only the policy bundle snapshot.
func (c *Client) GetDecisionLog(ctx context.Context, ownerID, policyCtx, decisionID string, policyBundleOnly bool) (json.RawMessage, error) {
	route := "/owner/%s/context/%s/decision/%s"
	if policyBundleOnly {
		route += "/policy-bundle"
	}
	var out json.RawMessage
	err := c.get(ctx, route, &out,
		httpcl.RouteParams(ownerID, policyCtx, decisionID),
	)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MakeDecision evaluates input against remote policies for the given owner and context.
func (c *Client) MakeDecision(ctx context.Context, ownerID, policyCtx string, input string, metadata map[string]any) (json.RawMessage, error) {
	body := map[string]any{"input": input}
	if len(metadata) > 0 {
		body["metadata"] = metadata
	}
	var out json.RawMessage
	err := c.post(ctx, "/owner/%s/context/%s/decision", body, &out,
		httpcl.RouteParams(ownerID, policyCtx),
	)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetPolicySettings retrieves whether policy enforcement is enabled.
func (c *Client) GetPolicySettings(ctx context.Context, ownerID, policyCtx string) (DecisionSettings, error) {
	var out DecisionSettings
	err := c.get(ctx, "/owner/%s/context/%s/decision/settings", &out,
		httpcl.RouteParams(ownerID, policyCtx),
	)
	if err != nil {
		return DecisionSettings{}, err
	}
	return out, nil
}

// SetPolicySettings enables or disables policy enforcement.
func (c *Client) SetPolicySettings(ctx context.Context, ownerID, policyCtx string, settings DecisionSettings) (DecisionSettings, error) {
	var out DecisionSettings
	err := c.patch(ctx, "/owner/%s/context/%s/decision/settings", settings, &out,
		httpcl.RouteParams(ownerID, policyCtx),
	)
	if err != nil {
		return DecisionSettings{}, err
	}
	return out, nil
}
