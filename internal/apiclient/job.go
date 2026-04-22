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
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
)

// Job holds the details of a CircleCI job including its steps.
type Job struct {
	Number    int64      `json:"job_number"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at"`
	Steps     []JobStep  `json:"steps"`
}

// JobStep is a named step within a job.
type JobStep struct {
	Name    string       `json:"name"`
	Actions []StepAction `json:"actions"`
}

// StepAction is a single action within a step, carrying the output URL.
type StepAction struct {
	Index     int        `json:"index"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	ExitCode  *int       `json:"exit_code"`
	StartedAt time.Time  `json:"start_time"`
	StoppedAt *time.Time `json:"end_time"`
	OutputURL string     `json:"output_url"`
}

// LogLine is a single line of output from a step action.
type LogLine struct {
	Type    string `json:"type"` // "out" or "err"
	Time    string `json:"time"`
	Message string `json:"message"`
}

// GetJob fetches full job details including steps and their output URLs.
// The v2 API returns job metadata but often omits step output; if steps are
// absent, GetJob transparently retries against the v1.1 API which always
// includes step output.
func (c *Client) GetJob(ctx context.Context, projectSlug string, jobNumber int64) (*Job, error) {
	path := fmt.Sprintf("/project/%s/job/%d", url.PathEscape(projectSlug), jobNumber)
	var job Job
	if err := c.get(ctx, path, &job); err != nil {
		return nil, err
	}

	if len(job.Steps) == 0 {
		if steps, err := c.getJobStepsV1(ctx, projectSlug, jobNumber); err == nil {
			job.Steps = steps
		}
	}

	return &job, nil
}

// getJobStepsV1 fetches step output from the v1.1 API, which includes output_url
// fields not present in the v2 job response.
func (c *Client) getJobStepsV1(ctx context.Context, projectSlug string, jobNumber int64) ([]JobStep, error) {
	var resp struct {
		Steps []JobStep `json:"steps"`
	}
	if err := c.getV1(ctx, v1ProjectPath(projectSlug, jobNumber), &resp); err != nil {
		return nil, err
	}
	return resp.Steps, nil
}

// v1ProjectPath converts a v2 project slug and job number into a v1.1 API path.
// v2 slugs use short VCS prefixes (gh, bb, gl); v1.1 uses full names.
func v1ProjectPath(projectSlug string, jobNumber int64) string {
	vcs, rest, ok := strings.Cut(projectSlug, "/")
	if !ok {
		return fmt.Sprintf("/project/%s/%d", url.PathEscape(projectSlug), jobNumber)
	}
	switch vcs {
	case "gh":
		vcs = "github"
	case "bb":
		vcs = "bitbucket"
	case "gl":
		vcs = "gitlab"
	}
	return fmt.Sprintf("/project/%s/%s/%d", vcs, rest, jobNumber)
}

// GetStepOutput fetches the log lines from a step action's output URL.
// The output URL is typically a pre-authenticated storage URL; the Circle-Token
// header is sent anyway for URLs that require it.
func (c *Client) GetStepOutput(ctx context.Context, outputURL string) ([]LogLine, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, outputURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Circle-Token", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.raw.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching step output: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, &httpcl.HTTPError{Method: http.MethodGet, Route: outputURL, StatusCode: resp.StatusCode}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading step output: %w", err)
	}

	if len(body) == 0 {
		return nil, nil
	}

	var lines []LogLine
	if err := json.Unmarshal(body, &lines); err != nil {
		return nil, fmt.Errorf("decoding step output: %w", err)
	}
	return lines, nil
}
