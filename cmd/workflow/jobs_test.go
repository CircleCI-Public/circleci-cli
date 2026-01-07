package workflow

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	workflowapi "github.com/CircleCI-Public/circleci-cli/api/workflow"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowJobsCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/workflow/workflow-id/job", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"next_page_token": "",
			"items": []workflowapi.Job{
				{
					Name:      "build",
					Status:    "success",
					JobNumber: 123,
				},
			},
		})
	}))
	defer server.Close()

	cfg := settings.Config{
		Token:      "testtoken",
		Host:       server.URL,
		HTTPClient: http.DefaultClient,
	}

	client, err := workflowapi.NewWorkflowRestClient(cfg)
	assert.NoError(t, err)

	opts := &workflowOpts{
		workflowClient: client,
	}

	noValidator := func(_ *cobra.Command, _ []string) error { return nil }

	t.Run("prints job status and number", func(t *testing.T) {
		cmd := newJobsCommand(opts, noValidator)
		cmd.SetArgs([]string{"workflow-id"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Equal(t, "build\tsuccess\t123\n", outBuf.String()+errBuf.String())
	})

	t.Run("json output", func(t *testing.T) {
		cmd := newJobsCommand(opts, noValidator)
		cmd.SetArgs([]string{"workflow-id", "--json"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)

		var got []workflowapi.Job
		assert.NoError(t, json.Unmarshal([]byte(outBuf.String()+errBuf.String()), &got))
		assert.Len(t, got, 1)
		assert.Equal(t, 123, got[0].JobNumber)
	})
}
