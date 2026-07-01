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

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// TestResult is a single test's outcome as reported by a job's test metadata.
// The /api/v3/jobs/{id}/tests endpoint streams these as newline-delimited JSON
// (JSONL), one record per line.
type TestResult struct {
	Classname string  `json:"classname"` // suite/package the test belongs to
	Name      string  `json:"name"`      // test name
	Result    string  `json:"result"`    // "success", "failure", "skipped", ...
	RunTime   float64 `json:"run_time"`  // seconds
	Message   string  `json:"message"`   // failure/skip detail, empty on success
}

// StreamJobTests fetches the test metadata for a job identified by UUID,
// invoking fn for each TestResult as it is decoded from the JSONL response.
// The endpoint returns JSONL (one TestResult per line) rather than a JSON
// array, so records are handed to fn as they arrive rather than buffered.
// The returned error reports transport or decode failures, not anything fn does.
func (c *Client) StreamJobTests(ctx context.Context, jobID uuid.UUID, fn func(TestResult)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/jobs/%s/tests",
		httpcl.RouteParams(jobID),
		httpcl.JSONLDecoder(fn),
	))
	return err
}
