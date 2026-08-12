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

package update

import (
	"context"
	"net/http"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// proxySource fetches the latest release from GET /api/v3/tool/releases. It is
// the only Source implementation; nothing else in the package knows the wire
// format, so swapping the source is a one-file change.
type proxySource struct {
	client *apiclient.Client
}

// NewProxySource returns the Source backed by GET /api/v3/tool/releases.
func NewProxySource(client *apiclient.Client) Source {
	return proxySource{client: client}
}

// Latest maps the endpoint's status codes onto the Source contract so that a
// failed fetch does not burn the cache window (see the status mapping in the
// plan):
//
//   - 200        → the release
//   - 503/other  → error, so Check writes no state and retries next run
//   - 400        → our bug (tool name no longer matches the registry); debug-log
//     and give up quietly, never surfaced
//   - 401/403    → give up quietly; an update nag must never look like an auth
//     problem
func (s proxySource) Latest(ctx context.Context) (*ReleaseInfo, error) {
	rel, err := s.client.LatestRelease(ctx, ToolName)
	if err != nil {
		switch {
		case httpcl.HasStatusCode(err, http.StatusBadRequest):
			iostream.DebugContext(ctx, "update check: server rejected tool filter", "err", err)
			return nil, nil
		case httpcl.HasStatusCode(err, http.StatusUnauthorized, http.StatusForbidden):
			return nil, nil
		default:
			return nil, err
		}
	}
	return &ReleaseInfo{Version: rel.Version, PublishedAt: rel.PublishedAt}, nil
}
