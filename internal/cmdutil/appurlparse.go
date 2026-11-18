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

package cmdutil

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// AppURLRef is what a CircleCI web URL identifies. Every field the URL did not
// carry is left at its zero value; a URL usually fills in more than one (a job
// page names its pipeline, workflow and job), and the resolvers pick the most
// specific field they can use.
type AppURLRef struct {
	// ProjectSlug is "vcs/org/repo" for a classic project or
	// "circleci/<org-id>/<project-id>" for a GitHub App project. Set whenever
	// the URL names the project.
	ProjectSlug string
	// PipelineNumber is the project-scoped run number from a /pipelines/ URL.
	PipelineNumber int64
	// RunID is the run UUID from a /pipeline/<uuid> URL.
	RunID uuid.UUID
	// WorkflowID is the workflow UUID from the path or the workflowId query.
	WorkflowID uuid.UUID
	// JobID is the job UUID from a /job/<uuid> path or the job query.
	JobID uuid.UUID
	// JobNumber is the project-scoped job number from a /jobs/<n> path.
	JobNumber int64
}

// errNotAppURL is the ParseAppURL error for a URL whose path is none of the
// shapes it knows.
var errNotAppURL = errors.New("the path is not a pipeline, workflow or job page")

// LooksLikeURL reports whether s is a URL rather than an id or a number, so a
// caller can fall through to its usual parsing for anything else.
func LooksLikeURL(s string) bool {
	return strings.Contains(s, "://") || strings.HasPrefix(s, "app.circleci.com/") || strings.HasPrefix(s, "circleci.com/")
}

// ParseAppURL reads the ids out of a CircleCI web URL. It accepts the shapes
// the app and the CLI produce:
//
//	/pipelines/<vcs>/<org>/<repo>/<number>
//	/pipelines/<vcs>/<org>/<repo>/<number>/workflows/<workflow-id>
//	/pipelines/<vcs>/<org>/<repo>/<number>/workflows/<workflow-id>/jobs/<job-number>
//	/pipelines/<vcs>/<org>/<repo>/<number>/details?workflowId=<id>&job=<id>
//	/pipeline/<run-id>
//	/workflow/<workflow-id>
//	/workflow/<workflow-id>/job/<job-id>
//	/jobs/<vcs>/<org>/<repo>/<job-number>
//	https://circleci.com/<vcs>/<org>/<repo>/<job-number>   (the legacy job web_url)
//
// The host is not checked, so URLs from a CircleCI server install parse too.
func ParseAppURL(raw string) (*AppURLRef, error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("not a valid URL: %w", err)
	}
	segs := splitPath(u.Path)
	ref := &AppURLRef{}
	ref.WorkflowID, _ = uuid.Parse(u.Query().Get("workflowId"))
	ref.JobID, _ = uuid.Parse(u.Query().Get("job"))

	switch {
	case len(segs) >= 5 && segs[0] == "pipelines":
		err = parsePipelinesPath(ref, segs[1:])
	case len(segs) == 2 && segs[0] == "pipeline":
		ref.RunID, err = parseUUIDSegment(segs[1], "run")
	case len(segs) >= 2 && segs[0] == "workflow":
		err = parseWorkflowPath(ref, segs[1:])
	case len(segs) == 5 && segs[0] == "jobs":
		ref.ProjectSlug = strings.Join(segs[1:4], "/")
		ref.JobNumber, err = parseNumberSegment(segs[4], "job")
	case len(segs) == 4 && isVCSSegment(segs[0]):
		// The legacy job web_url: https://circleci.com/gh/org/repo/<job-number>.
		ref.ProjectSlug = strings.Join(segs[0:3], "/")
		ref.JobNumber, err = parseNumberSegment(segs[3], "job")
	default:
		err = errNotAppURL
	}
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// parsePipelinesPath reads "<vcs>/<org>/<repo>/<number>[/workflows/<id>[/jobs/<n>]]",
// the path under /pipelines/.
func parsePipelinesPath(ref *AppURLRef, segs []string) error {
	ref.ProjectSlug = strings.Join(segs[0:3], "/")
	n, err := parseNumberSegment(segs[3], "run")
	if err != nil {
		return err
	}
	ref.PipelineNumber = n
	rest := segs[4:]
	if len(rest) < 2 || rest[0] != "workflows" {
		return nil
	}
	if ref.WorkflowID, err = parseUUIDSegment(rest[1], "workflow"); err != nil {
		return err
	}
	if len(rest) >= 4 && rest[2] == "jobs" {
		ref.JobNumber, err = parseNumberSegment(rest[3], "job")
	}
	return err
}

// parseWorkflowPath reads "<workflow-id>[/job/<job-id>]", the path under
// /workflow/ that the CLI's own URLs use.
func parseWorkflowPath(ref *AppURLRef, segs []string) error {
	var err error
	if ref.WorkflowID, err = parseUUIDSegment(segs[0], "workflow"); err != nil {
		return err
	}
	if len(segs) >= 3 && segs[1] == "job" {
		ref.JobID, err = parseUUIDSegment(segs[2], "job")
	}
	return err
}

func parseUUIDSegment(s, kind string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%q is not a %s UUID", s, kind)
	}
	return id, nil
}

func parseNumberSegment(s, kind string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%q is not a %s number", s, kind)
	}
	return n, nil
}

func splitPath(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func isVCSSegment(s string) bool {
	switch s {
	case "gh", "github", "bb", "bitbucket", "circleci":
		return true
	}
	return false
}
