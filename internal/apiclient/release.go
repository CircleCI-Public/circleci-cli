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
	"net/http"
	"time"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// Release is the latest released version of a CircleCI tool.
type Release struct {
	Tool        string    // GitHub repo name the release belongs to, e.g. "circleci-cli"
	Version     string    // semver, no leading "v" (the server strips it)
	PublishedAt time.Time // release publication time, UTC
}

// ErrNoRelease is returned by LatestRelease when the server answers 200 with an
// empty data array. The handler always emits exactly one item, so this should
// be impossible; callers treat it as a fetch failure rather than "no update".
var ErrNoRelease = errors.New("no release returned")

// releaseEntity is one element of the /releases collection: only the attributes
// object carries data we use. The derived UUID id and the echoed tool are
// ignored per the endpoint contract.
type releaseEntity struct {
	Attributes struct {
		Tool        string    `json:"tool"`
		Version     string    `json:"version"`
		PublishedAt time.Time `json:"published_at"`
	} `json:"attributes"`
}

// LatestRelease returns the latest released version of the named tool via
// GET /api/v3/tool/releases?filter[tool]=<tool>.
//
// tool is the tool's GitHub repository name (e.g. "circleci-cli"). The endpoint
// models a required-filter single lookup as a one-element collection, so the
// response is unwrapped from data[0].attributes. Non-2xx statuses surface as the
// underlying *httpcl.HTTPError so callers can distinguish transient (503) from
// permanent (400/401/403) failures.
func (c *Client) LatestRelease(ctx context.Context, tool string) (*Release, error) {
	var env v3List[releaseEntity]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/tool/releases",
		filterParam("tool", tool),
		httpcl.JSONDecoder(&env),
	))
	if err != nil {
		return nil, err
	}
	if len(env.Data) == 0 {
		return nil, ErrNoRelease
	}
	a := env.Data[0].Attributes
	return &Release{Tool: a.Tool, Version: a.Version, PublishedAt: a.PublishedAt}, nil
}
