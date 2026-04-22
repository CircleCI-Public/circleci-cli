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

// Package artifacts implements the business logic for fetching and downloading
// CircleCI job artifacts, at either the job or pipeline level.
package artifacts

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
)

// Entry is a single artifact with the job context it came from.
type Entry struct {
	JobName   string `json:"job_name"`
	JobNumber int64  `json:"job_number"`
	Path      string `json:"path"`
	URL       string `json:"url"`
	NodeIndex int    `json:"node_index"`
}

// Client is the subset of apiclient.Client methods we need.
type Client interface {
	GetPipelineWorkflows(ctx context.Context, pipelineID string) ([]apiclient.PipelineWorkflowSummary, error)
	GetWorkflowJobs(ctx context.Context, workflowID string) ([]apiclient.WorkflowJob, error)
	GetJobArtifacts(ctx context.Context, projectSlug string, jobNumber int64) ([]apiclient.Artifact, error)
	DownloadArtifact(ctx context.Context, artifactURL string, dst io.Writer) error
}

// ForPipeline fetches all artifacts across every job in the pipeline.
// Jobs with no artifacts are silently skipped.
func ForPipeline(ctx context.Context, client Client, pipelineID string) ([]Entry, error) {
	workflows, err := client.GetPipelineWorkflows(ctx, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("fetching workflows: %w", err)
	}

	var entries []Entry
	for _, wf := range workflows {
		jobs, err := client.GetWorkflowJobs(ctx, wf.ID)
		if err != nil {
			return nil, fmt.Errorf("fetching jobs for workflow %q: %w", wf.Name, err)
		}
		for _, job := range jobs {
			if job.JobNumber == 0 || job.ProjectSlug == "" {
				continue // approval jobs and similar have no number
			}
			artifacts, err := client.GetJobArtifacts(ctx, job.ProjectSlug, job.JobNumber)
			if err != nil {
				return nil, fmt.Errorf("fetching artifacts for job %q (#%d): %w", job.Name, job.JobNumber, err)
			}
			for _, a := range artifacts {
				entries = append(entries, Entry{
					JobName:   job.Name,
					JobNumber: job.JobNumber,
					Path:      a.Path,
					URL:       a.URL,
					NodeIndex: a.NodeIndex,
				})
			}
		}
	}
	return entries, nil
}

// ForJob fetches artifacts for a single job number within a project.
func ForJob(ctx context.Context, client Client, projectSlug string, jobNumber int64) ([]Entry, error) {
	artifacts, err := client.GetJobArtifacts(ctx, projectSlug, jobNumber)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, len(artifacts))
	for i, a := range artifacts {
		entries[i] = Entry{
			JobNumber: jobNumber,
			Path:      a.Path,
			URL:       a.URL,
			NodeIndex: a.NodeIndex,
		}
	}
	return entries, nil
}

// Download fetches each entry's artifact and writes it under dir, preserving
// the artifact's Path as the relative file path.
func Download(ctx context.Context, client Client, entries []Entry, dir string) error {
	// cleanDir is the canonical form of the download root with a trailing
	// separator.  We recompute it once outside the loop rather than on every
	// iteration.
	cleanDir := filepath.Clean(dir) + string(os.PathSeparator)

	for _, e := range entries {
		dest := filepath.Join(dir, filepath.FromSlash(e.Path))

		// Path-traversal guard: artifact paths come from the CircleCI API
		// response and are not under our control.  filepath.Join resolves
		// all ".." components (via filepath.Clean), so a server-supplied
		// path such as "../../.ssh/authorized_keys" would silently escape
		// dir and produce an arbitrary write destination.  We therefore
		// require that the resolved destination still sits inside dir.
		if !strings.HasPrefix(dest, cleanDir) {
			return fmt.Errorf("artifact path %q escapes download directory", e.Path)
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("creating directory for %q: %w", e.Path, err)
		}
		f, err := os.Create(dest)
		if err != nil {
			return fmt.Errorf("creating file %q: %w", dest, err)
		}
		dlErr := client.DownloadArtifact(ctx, e.URL, f)
		closeErr := f.Close()
		if dlErr != nil {
			return fmt.Errorf("downloading %q: %w", e.Path, dlErr)
		}
		if closeErr != nil {
			return fmt.Errorf("writing %q: %w", e.Path, closeErr)
		}
	}
	return nil
}
