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

func TestPipelineRunsLatestCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/project/gh/test-org/test-repo/pipeline", r.URL.Path)
		assert.Equal(t, "main", r.URL.Query().Get("branch"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"next_page_token": "",
			"items": []pipelineapi.Pipeline{
				{
					ID:     "pipeline-id",
					Number: 456,
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

	t.Run("prints id and number", func(t *testing.T) {
		cmd := newRunsLatestCommand(opts, noValidator)
		cmd.SetArgs([]string{"gh/test-org/test-repo", "--branch", "main"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Equal(t, "pipeline-id\t456\n", outBuf.String()+errBuf.String())
	})

	t.Run("json output", func(t *testing.T) {
		cmd := newRunsLatestCommand(opts, noValidator)
		cmd.SetArgs([]string{"gh/test-org/test-repo", "--branch", "main", "--json"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)

		var got pipelineapi.Pipeline
		assert.NoError(t, json.Unmarshal([]byte(outBuf.String()+errBuf.String()), &got))
		assert.Equal(t, "pipeline-id", got.ID)
		assert.Equal(t, 456, got.Number)
	})
}
