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

package org

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// ErrNotMember is returned by Details when orgID is not among the
// organizations the authenticated user belongs to.
var ErrNotMember = errors.New("not a member of this organization")

// Info is what `circleci org get` reports about one organization.
type Info struct {
	ID   uuid.UUID
	Name string
	// VCS is the provider backing the org; empty for a standalone org.
	VCS string
	// Admin is nil when the server does not support the admin check.
	Admin *bool
}

// Details gathers the organization's identity and the caller's admin status.
//
// The identity comes from the caller's org list, and admin status from whether
// the org is among those the caller can manage (GET /api/v3/orgs with
// filter[can_manage]). A server without that filter — an older CircleCI
// server install — rejects it with 400 Bad Request; Admin is then left nil
// rather than failing the whole lookup. Any other error (5xx, network,
// cancellation) is returned.
func Details(ctx context.Context, client *apiclient.Client, orgID uuid.UUID) (*Info, error) {
	orgs, err := client.ListOrgs(ctx)
	if err != nil {
		return nil, err
	}
	info := &Info{ID: orgID}
	found := false
	for _, o := range orgs {
		if o.ID == orgID {
			info.Name, info.VCS, found = o.Name, o.VCS, true
			break
		}
	}
	if !found {
		return nil, ErrNotMember
	}

	manageable, err := client.ListManageableOrgs(ctx)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusBadRequest) {
			return info, nil
		}
		return nil, err
	}
	admin := false
	for _, o := range manageable {
		if o.ID == orgID {
			admin = true
			break
		}
	}
	info.Admin = &admin
	return info, nil
}
