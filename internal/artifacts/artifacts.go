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
// CircleCI job artifacts.
package artifacts

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// Entry is a single artifact produced by a job.
type Entry struct {
	Path      string `json:"path"`
	URL       string `json:"url"`
	NodeIndex int    `json:"node_index"`
}

// Client is the subset of apiclient.Client methods we need.
type Client interface {
	GetJobArtifactsV3(ctx context.Context, jobID string) ([]apiclient.Artifact, error)
	DownloadArtifact(ctx context.Context, artifactURL string, dst io.Writer) error
}

// ForJob fetches artifacts for a job identified by UUID.
func ForJob(ctx context.Context, client Client, jobID string) ([]Entry, error) {
	arts, err := client.GetJobArtifactsV3(ctx, jobID)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, len(arts))
	for i, a := range arts {
		entries[i] = Entry{
			Path:      a.Path,
			URL:       a.URL,
			NodeIndex: a.NodeIndex,
		}
	}
	return entries, nil
}

// ExecDir returns the zero-padded directory name for an execution index.
func ExecDir(idx int) string {
	return fmt.Sprintf("exec-%04d", idx)
}

func hasMultipleExecutions(entries []Entry) bool {
	if len(entries) == 0 {
		return false
	}
	first := entries[0].NodeIndex
	for _, e := range entries[1:] {
		if e.NodeIndex != first {
			return true
		}
	}
	return false
}

// FormatMarkdown returns a markdown string listing the artifacts.
// When all entries share the same execution index the output is a flat list.
// When multiple executions are present, entries are grouped under
// ## Execution N headings.
func FormatMarkdown(entries []Entry) string {
	var md strings.Builder
	md.WriteString("# Artifacts\n")

	if !hasMultipleExecutions(entries) {
		for _, e := range entries {
			fmt.Fprintf(&md, "- %s\n", e.Path)
		}
		return md.String()
	}

	seen := map[int]struct{}{}
	for _, e := range entries {
		seen[e.NodeIndex] = struct{}{}
	}
	idxs := make([]int, 0, len(seen))
	for idx := range seen {
		idxs = append(idxs, idx)
	}
	sort.Ints(idxs)

	for _, idx := range idxs {
		fmt.Fprintf(&md, "## Execution %d\n", idx)
		for _, e := range entries {
			if e.NodeIndex == idx {
				fmt.Fprintf(&md, "- %s\n", e.Path)
			}
		}
	}
	return md.String()
}

// Download fetches each entry's artifact and writes it under dir, preserving
// the artifact's Path as the relative file path. When entries span multiple
// execution indices, each execution's artifacts are placed under a
// subdirectory named by the index (e.g. dir/0/path, dir/3/path).
func Download(ctx context.Context, client Client, entries []Entry, dir string) error {
	multiExec := hasMultipleExecutions(entries)

	// cleanDir is the canonical form of the download root with a trailing
	// separator.  We recompute it once outside the loop rather than on every
	// iteration.
	cleanDir := filepath.Clean(dir) + string(os.PathSeparator)

	for _, e := range entries {
		base := dir
		if multiExec {
			base = filepath.Join(dir, ExecDir(e.NodeIndex))
		}
		dest := filepath.Join(base, filepath.FromSlash(e.Path))

		// Path-traversal guard: artifact paths come from the CircleCI API
		// response and are not under our control.  filepath.Join resolves
		// all ".." components (via filepath.Clean), so a server-supplied
		// path such as "../../.ssh/authorized_keys" would silently escape
		// dir and produce an arbitrary write destination.  We therefore
		// require that the resolved destination still sits inside dir.
		if !strings.HasPrefix(dest, cleanDir) {
			return fmt.Errorf("artifact path %q escapes download directory", e.Path)
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil { //#nosec:G301 // 0o755 is appropriate for artifact download directories
			return fmt.Errorf("creating directory for %q: %w", e.Path, err)
		}
		f, err := os.Create(dest) //#nosec:G304 // dest is validated above via HasPrefix check against the clean download dir
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
