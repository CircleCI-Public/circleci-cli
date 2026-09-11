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
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// OrgInfo is returned by POST /api/v2/organization.
type OrgInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	VCSType string `json:"vcs_type"`
}

// ErrOrgNotFound is returned by ResolveOrgID when no org matches the slug.
var ErrOrgNotFound = errors.New("organization not found")

// Organization is an organization the authenticated user belongs to, as
// returned by GET /api/v3/orgs.
//
// The collection carries no slug — see ListOrgs.
type Organization struct {
	ID   uuid.UUID
	Name string
	// VCS is the version control provider backing the org ("github",
	// "bitbucket"). Empty for a standalone CircleCI org, which has no provider.
	VCS string
}

// orgEntity is a single entry in the GET /api/v3/orgs response.
type orgEntity struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		Name string `json:"name"`
		// VCS is null for a standalone CircleCI org.
		VCS *struct {
			Provider string `json:"provider"`
		} `json:"vcs"`
	} `json:"attributes"`
}

func (e orgEntity) toOrganization() Organization {
	org := Organization{ID: e.ID, Name: e.Attributes.Name}
	if e.Attributes.VCS != nil {
		org.VCS = e.Attributes.VCS.Provider
	}
	return org
}

// orgPageLimit is the largest page[limit] GET /api/v3/orgs accepts; a bigger
// value is rejected with a 400 rather than clamped.
const orgPageLimit = 50

// ListOrgs returns every organization the authenticated user belongs to, via
// GET /api/v3/orgs. Paginates automatically.
//
// The response carries no slug, and one cannot be derived: only a VCS-backed
// org has a slug at all, so an org is identified here by its UUID.
func (c *Client) ListOrgs(ctx context.Context) ([]Organization, error) {
	var all []Organization
	cursor := ""

	for {
		var env v3List[orgEntity]
		_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/orgs",
			pageLimit(orgPageLimit),
			pageCursor(cursor),
			httpcl.JSONDecoder(&env),
		))
		if err != nil {
			return nil, err
		}
		for _, e := range env.Data {
			all = append(all, e.toOrganization())
		}
		if env.Page.Next == nil || *env.Page.Next == "" {
			return all, nil
		}
		cursor = *env.Page.Next
	}
}

// ResolveOrgID resolves an organization slug (e.g. "gh/acme") to its UUID via
// GET /api/v3/orgs?filter[slug]=<slug>.
//
// The endpoint is a collection: a slug matching no org returns an empty list
// (not a 404), which is surfaced as ErrOrgNotFound.
func (c *Client) ResolveOrgID(ctx context.Context, slug string) (uuid.UUID, error) {
	var env v3List[orgEntity]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/orgs",
		filterParam("slug", slug),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return uuid.Nil, err
	}
	if len(env.Data) == 0 {
		return uuid.Nil, fmt.Errorf("%w: %q", ErrOrgNotFound, slug)
	}
	return env.Data[0].ID, nil
}

// CreateOrg creates a new organization. vcsType must be one of "github",
// "bitbucket", or "circleci".
func (c *Client) CreateOrg(ctx context.Context, name, vcsType string) (*OrgInfo, error) {
	body := map[string]string{"name": name, "vcs_type": vcsType}
	var org OrgInfo
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v2/organization",
		httpcl.Body(body),
		httpcl.JSONDecoder(&org),
	))
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// OrgSettingsAttributes is the "attributes" object returned by
// GET /api/v3/orgs/:id/settings and POST /api/v3/orgs/:id/update-settings.
type OrgSettingsAttributes struct {
	RunnerTOSAccepted                   bool `json:"is_runner_terms_of_service_accepted"`
	AIErrorSummarization                bool `json:"enable_ai_error_summarization"`
	AIAgents                            bool `json:"enable_ai_agents"`
	UnversionedConfig                   bool `json:"enable_unversioned_config"`
	CertifiedPublicOrbs                 bool `json:"enable_certified_public_orbs"`
	ChunkIPRanges                       bool `json:"enable_chunk_ip_ranges"`
	MinorAIFeatures                     bool `json:"enable_minor_ai_features"`
	PrivateOrbs                         bool `json:"enable_private_orbs"`
	UncertifiedPublicOrbs               bool `json:"enable_uncertified_public_orbs"`
	BitbucketWorkspaceMemberIsOrgMember bool `json:"is_bitbucket_workspace_member_org_member"`
	UserCheckoutKeysDisabled            bool `json:"is_user_checkout_keys_disabled"`
	DisableRunning                      bool `json:"is_running_disabled"`
	ImageBrownouts                      bool `json:"enable_image_brownouts"`
	ContextGroupRestrictionRequired     bool `json:"is_context_group_restriction_required"`
	ResourceClassBrownouts              bool `json:"enable_resource_class_brownouts"`
}

// OrgSettingsUpdate is the body for POST /api/v3/orgs/:id/update-settings.
// Only non-nil fields are sent; omitting a field leaves that setting unchanged.
type OrgSettingsUpdate struct {
	RunnerTOSAccepted                   *bool `json:"is_runner_terms_of_service_accepted,omitempty"`
	AIErrorSummarization                *bool `json:"enable_ai_error_summarization,omitempty"`
	AIAgents                            *bool `json:"enable_ai_agents,omitempty"`
	UnversionedConfig                   *bool `json:"enable_unversioned_config,omitempty"`
	CertifiedPublicOrbs                 *bool `json:"enable_certified_public_orbs,omitempty"`
	ChunkIPRanges                       *bool `json:"enable_chunk_ip_ranges,omitempty"`
	MinorAIFeatures                     *bool `json:"enable_minor_ai_features,omitempty"`
	PrivateOrbs                         *bool `json:"enable_private_orbs,omitempty"`
	UncertifiedPublicOrbs               *bool `json:"enable_uncertified_public_orbs,omitempty"`
	BitbucketWorkspaceMemberIsOrgMember *bool `json:"is_bitbucket_workspace_member_org_member,omitempty"`
	UserCheckoutKeysDisabled            *bool `json:"is_user_checkout_keys_disabled,omitempty"`
	DisableRunning                      *bool `json:"is_running_disabled,omitempty"`
	ImageBrownouts                      *bool `json:"enable_image_brownouts,omitempty"`
	ContextGroupRestrictionRequired     *bool `json:"is_context_group_restriction_required,omitempty"`
	ResourceClassBrownouts              *bool `json:"enable_resource_class_brownouts,omitempty"`
}

type orgSettingsEnvelope struct {
	Data struct {
		Attributes OrgSettingsAttributes `json:"attributes"`
	} `json:"data"`
}

// GetOrgSettings returns settings for an organization via GET /api/v3/orgs/:id/settings.
func (c *Client) GetOrgSettings(ctx context.Context, orgID uuid.UUID) (*OrgSettingsAttributes, error) {
	var env orgSettingsEnvelope
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/orgs/%s/settings",
		httpcl.RouteParams(orgID),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	return &env.Data.Attributes, nil
}

// UpdateOrgSettings updates org settings via POST /api/v3/orgs/:id/update-settings.
// Only the fields set in update are changed; omitted fields are left as-is.
func (c *Client) UpdateOrgSettings(ctx context.Context, orgID uuid.UUID, update OrgSettingsUpdate) (*OrgSettingsAttributes, error) {
	var env orgSettingsEnvelope
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/orgs/%s/update-settings",
		httpcl.RouteParams(orgID),
		httpcl.Body(update),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	return &env.Data.Attributes, nil
}
