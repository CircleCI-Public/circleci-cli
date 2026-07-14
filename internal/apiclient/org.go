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

	"github.com/google/uuid"
)

// OrgInfo is returned by GET /api/v2/organization/{slug-or-id}.
type OrgInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	VCSType string `json:"vcs_type"`
}

// GetOrg fetches an organization by its slug or UUID.
func (c *Client) GetOrg(ctx context.Context, slugOrID string) (*OrgInfo, error) {
	var org OrgInfo
	if err := c.get(ctx, "/organization/"+slugOrID, &org); err != nil {
		return nil, err
	}
	return &org, nil
}

// CreateOrg creates a new organization. vcsType must be one of "github",
// "bitbucket", or "circleci".
func (c *Client) CreateOrg(ctx context.Context, name, vcsType string) (*OrgInfo, error) {
	body := map[string]string{"name": name, "vcs_type": vcsType}
	var org OrgInfo
	if err := c.post(ctx, "/organization", body, &org); err != nil {
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
	if err := c.getV3(ctx, "/orgs/%s/settings", &env, routeParams(orgID)); err != nil {
		return nil, err
	}
	return &env.Data.Attributes, nil
}

// UpdateOrgSettings updates org settings via POST /api/v3/orgs/:id/update-settings.
// Only the fields set in update are changed; omitted fields are left as-is.
func (c *Client) UpdateOrgSettings(ctx context.Context, orgID uuid.UUID, update OrgSettingsUpdate) (*OrgSettingsAttributes, error) {
	var env orgSettingsEnvelope
	if err := c.postV3(ctx, "/orgs/%s/update-settings", update, &env, routeParams(orgID)); err != nil {
		return nil, err
	}
	return &env.Data.Attributes, nil
}
