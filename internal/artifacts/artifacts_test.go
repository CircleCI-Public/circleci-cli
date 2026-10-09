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
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/artifacts"
)

// noopClient satisfies artifacts.Client; DownloadArtifact writes fixed content.
type noopClient struct{}

func (noopClient) GetJobArtifactsV3(_ context.Context, _ string) ([]apiclient.Artifact, error) {
	return nil, nil
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
