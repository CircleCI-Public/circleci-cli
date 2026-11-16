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

package artifacts_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/artifacts"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// noopClient satisfies artifacts.Client; DownloadArtifact writes fixed content.
type noopClient struct{}

func (noopClient) GetJobArtifactsV3(_ context.Context, _ string) ([]apiclient.Artifact, error) {
	return nil, nil
}
func (noopClient) GetJobV3(_ context.Context, _ uuid.UUID) (*apiclient.JobV3, error) {
	return &apiclient.JobV3{}, nil
}
func (noopClient) DownloadArtifact(_ context.Context, _ string, dst io.Writer) error {
	_, err := io.WriteString(dst, "content")
	return err
}

// failingClient writes part of an artifact and then fails, as an interrupted
// transfer does.
type failingClient struct{ noopClient }

func (failingClient) DownloadArtifact(_ context.Context, _ string, dst io.Writer) error {
	_, _ = io.WriteString(dst, "partial")
	return errors.New("connection reset")
}

func TestDownload_Atomic(t *testing.T) {
	entries := []artifacts.Entry{{Path: "image/device.zip", URL: "http://example.com/1"}}

	t.Run("a completed download is written under the artifact's name", func(t *testing.T) {
		dir := t.TempDir()
		err := artifacts.Download(context.Background(), noopClient{}, entries, dir)
		assert.NilError(t, err)

		got, err := os.ReadFile(filepath.Join(dir, "image", "device.zip"))
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(string(got), "content"))
		names := dirNames(t, filepath.Join(dir, "image"))
		assert.Check(t, cmp.DeepEqual(names, []string{"device.zip"}))
	})

	t.Run("a failed download leaves no file behind", func(t *testing.T) {
		dir := t.TempDir()
		err := artifacts.Download(context.Background(), failingClient{}, entries, dir)
		assert.Check(t, cmp.ErrorContains(err, "connection reset"))
		names := dirNames(t, filepath.Join(dir, "image"))
		assert.Check(t, cmp.Len(names, 0))
	})
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	des, err := os.ReadDir(dir)
	assert.NilError(t, err)
	names := make([]string, len(des))
	for i, de := range des {
		names[i] = de.Name()
	}
	return names
}

func TestDownload_PathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			// Classic traversal: "../.." resolves via filepath.Clean to a path
			// outside dir entirely.
			name:    "dotdot traversal",
			path:    "../../.ssh/authorized_keys",
			wantErr: "escapes download directory",
		},
		{
			// Traversal embedded inside a subdirectory component.
			name:    "dotdot in subdir",
			path:    "subdir/../../outside",
			wantErr: "escapes download directory",
		},
		{
			// Note: Go's filepath.Join treats a leading "/" as a string
			// component, not a root replacement, so "/etc/passwd" becomes
			// "<dir>/etc/passwd" and is safe.  This case documents that
			// behaviour explicitly so the test serves as a regression guard
			// if Go ever changes it.
			name:    "absolute path lands inside dir (Go filepath.Join semantics)",
			path:    "/etc/passwd",
			wantErr: "", // no error: resolved to <dir>/etc/passwd
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			entries := []artifacts.Entry{{Path: tc.path, URL: "http://example.com/artifact"}}
			err := artifacts.Download(context.Background(), noopClient{}, entries, dir)
			if tc.wantErr != "" {
				assert.Check(t, cmp.ErrorContains(err, tc.wantErr))
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestDownload_LegitimateNestedPath(t *testing.T) {
	dir := t.TempDir()
	entries := []artifacts.Entry{
		{Path: "coverage/index.html", URL: "http://example.com/1"},
		{Path: "results.xml", URL: "http://example.com/2"},
	}
	err := artifacts.Download(context.Background(), noopClient{}, entries, dir)
	assert.NilError(t, err)
}

// fakeForJobClient answers the artifacts and job endpoints with fixed errors
// and records whether the job endpoint was asked.
type fakeForJobClient struct {
	noopClient
	artifactsErr error
	jobErr       error
	askedJob     bool
}

func (c *fakeForJobClient) GetJobArtifactsV3(_ context.Context, _ string) ([]apiclient.Artifact, error) {
	return nil, c.artifactsErr
}

func (c *fakeForJobClient) GetJobV3(_ context.Context, _ uuid.UUID) (*apiclient.JobV3, error) {
	c.askedJob = true
	if c.jobErr != nil {
		return nil, c.jobErr
	}
	return &apiclient.JobV3{}, nil
}

func TestForJob_NotFound(t *testing.T) {
	const jobID = "cccccccc-0000-0000-0000-000000000001"
	notFound := &httpcl.HTTPError{Method: "GET", StatusCode: http.StatusNotFound}

	t.Run("a job that exists but stored nothing is an empty list", func(t *testing.T) {
		client := &fakeForJobClient{artifactsErr: notFound}
		entries, err := artifacts.ForJob(context.Background(), client, jobID)
		assert.NilError(t, err)
		assert.Check(t, cmp.Len(entries, 0))
	})

	t.Run("a job the job endpoint does not know returns its 404", func(t *testing.T) {
		jobNotFound := &httpcl.HTTPError{Method: "GET", Route: "/api/v3/jobs/%s", StatusCode: http.StatusNotFound}
		client := &fakeForJobClient{artifactsErr: notFound, jobErr: jobNotFound}
		_, err := artifacts.ForJob(context.Background(), client, jobID)
		assert.Check(t, cmp.ErrorIs(err, jobNotFound))
	})

	t.Run("an id that is not a UUID keeps the artifacts 404", func(t *testing.T) {
		client := &fakeForJobClient{artifactsErr: notFound}
		_, err := artifacts.ForJob(context.Background(), client, "not-a-uuid")
		assert.Check(t, cmp.ErrorIs(err, notFound))
		assert.Check(t, cmp.Equal(client.askedJob, false))
	})

	t.Run("other errors are returned without asking the job endpoint", func(t *testing.T) {
		serverErr := &httpcl.HTTPError{Method: "GET", StatusCode: http.StatusInternalServerError}
		client := &fakeForJobClient{artifactsErr: serverErr}
		_, err := artifacts.ForJob(context.Background(), client, jobID)
		assert.Check(t, cmp.ErrorIs(err, serverErr))
		assert.Check(t, cmp.Equal(client.askedJob, false))
	})
}

func TestFormatRunMarkdown(t *testing.T) {
	t.Run("one section per job, paths only", func(t *testing.T) {
		got := artifacts.FormatRunMarkdown([]artifacts.JobArtifacts{
			{Workflow: "build", Job: "test", Entries: []artifacts.Entry{
				{Path: "coverage/index.html", URL: "https://example.com/a", Execution: 0},
				{Path: "results.xml", URL: "https://example.com/b", Execution: 0},
			}},
			{Workflow: "build", Job: "lint", Entries: []artifacts.Entry{}},
		})
		assert.Check(t, cmp.Equal(got, "## build / test\n"+
			"- coverage/index.html\n"+
			"- results.xml\n"+
			"\n"+
			"## build / lint\n"+
			"No artifacts.\n"))
	})

	t.Run("groups a job's executions", func(t *testing.T) {
		got := artifacts.FormatRunMarkdown([]artifacts.JobArtifacts{
			{Workflow: "build", Job: "test", Entries: []artifacts.Entry{
				{Path: "t-1.xml", Execution: 1},
				{Path: "t-0.xml", Execution: 0},
			}},
		})
		assert.Check(t, cmp.Equal(got, "## build / test\n"+
			"### Execution 0\n"+
			"- t-0.xml\n"+
			"### Execution 1\n"+
			"- t-1.xml\n"))
	})

	t.Run("no jobs", func(t *testing.T) {
		assert.Check(t, cmp.Equal(artifacts.FormatRunMarkdown(nil), ""))
	})
}

func TestSafeDirName(t *testing.T) {
	for name, want := range map[string]string{
		"test":           "test",
		"build-and-test": "build-and-test",
		"a/b":            "a_b",
		`a\b`:            "a_b",
		"..":             "_",
		".":              "_",
		"":               "_",
		"../../etc":      ".._.._etc",
		"win:dows?":      "win_dows_",
	} {
		assert.Check(t, cmp.Equal(artifacts.SafeDirName(name), want), "name %q", name)
	}
}
