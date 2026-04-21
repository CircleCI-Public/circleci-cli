package artifacts_test

import (
	"context"
	"io"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/artifacts"
)

// noopClient satisfies artifacts.Client; DownloadArtifact writes fixed content.
type noopClient struct{}

func (noopClient) GetPipelineWorkflows(_ context.Context, _ string) ([]apiclient.PipelineWorkflowSummary, error) {
	return nil, nil
}
func (noopClient) GetWorkflowJobs(_ context.Context, _ string) ([]apiclient.WorkflowJob, error) {
	return nil, nil
}
func (noopClient) GetJobArtifacts(_ context.Context, _ string, _ int64) ([]apiclient.Artifact, error) {
	return nil, nil
}
func (noopClient) DownloadArtifact(_ context.Context, _ string, dst io.Writer) error {
	_, err := io.WriteString(dst, "content")
	return err
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
				assert.ErrorContains(t, err, tc.wantErr)
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
