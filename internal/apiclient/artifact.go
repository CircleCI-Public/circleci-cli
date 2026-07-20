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
	"io"
	"net/http"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// Artifact is a file produced by a CircleCI job.
type Artifact struct {
	Path      string `json:"path"`
	URL       string `json:"url"`
	NodeIndex int    `json:"node_index"`
}

// --- V3 wire types ---

type artifactAttributesWire struct {
	Path      string `json:"path"`
	URL       string `json:"url"`
	Execution int    `json:"execution"`
}

type artifactWire struct {
	ID         string                 `json:"id"`
	Attributes artifactAttributesWire `json:"attributes"`
}

// GetJobArtifactsV3 returns the artifacts for a job identified by UUID,
// using the V3 API.
func (c *Client) GetJobArtifactsV3(ctx context.Context, jobID string) ([]Artifact, error) {
	var resp v3List[artifactWire]
	err := c.getV3(ctx, "/jobs/%s/artifacts", &resp,
		routeParams(jobID),
	)
	if err != nil {
		return nil, err
	}
	artifacts := make([]Artifact, len(resp.Data))
	for i, w := range resp.Data {
		artifacts[i] = Artifact{
			Path:      w.Attributes.Path,
			URL:       w.Attributes.URL,
			NodeIndex: w.Attributes.Execution,
		}
	}
	return artifacts, nil
}

// DownloadArtifact fetches an artifact URL (authenticated) and writes its
// contents to dst. The URL is a full absolute URL, not a base-relative path.
func (c *Client) DownloadArtifact(ctx context.Context, artifactURL string, dst io.Writer) error {
	_, err := c.raw.Call(ctx, httpcl.NewRequest(http.MethodGet, artifactURL,
		httpcl.CopyDecoder(dst),
	))
	return err
}
