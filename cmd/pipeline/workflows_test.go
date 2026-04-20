package pipeline

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pipelineapi "github.com/CircleCI-Public/circleci-cli/api/pipeline"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestPipelineWorkflowsCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/pipeline/pipeline-id/workflow", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"next_page_token": "",
			"items": []pipelineapi.Workflow{
				{
					ID:     "workflow-id",
					Name:   "ci",
					Status: "success",
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

	client, err := pipelineapi.NewPipelineRestClient(cfg)
	assert.NoError(t, err)

	opts := &pipelineOpts{
		pipelineClient: client,
	}

	noValidator := func(_ *cobra.Command, _ []string) error { return nil }

	t.Run("prints tab-separated workflows", func(t *testing.T) {
		cmd := newWorkflowsCommand(opts, noValidator)
		cmd.SetArgs([]string{"pipeline-id"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Equal(t, "workflow-id\tci\tsuccess\n", outBuf.String()+errBuf.String())
	})

	t.Run("json output", func(t *testing.T) {
		cmd := newWorkflowsCommand(opts, noValidator)
		cmd.SetArgs([]string{"pipeline-id", "--json"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)

		var got []pipelineapi.Workflow
		assert.NoError(t, json.Unmarshal([]byte(outBuf.String()+errBuf.String()), &got))
		assert.Len(t, got, 1)
		assert.Equal(t, "workflow-id", got[0].ID)
	})
}
