package project

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	projectapi "github.com/CircleCI-Public/circleci-cli/api/project"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestProjectIDCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/project/gh/test-org/test-repo", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "123e4567-e89b-12d3-a456-426614174000"})
	}))
	defer server.Close()

	cfg := settings.Config{
		Token:      "testtoken",
		Host:       server.URL,
		HTTPClient: http.DefaultClient,
	}

	client, err := projectapi.NewProjectRestClient(cfg)
	assert.NoError(t, err)

	opts := &projectOpts{
		projectClient: client,
	}

	noValidator := func(_ *cobra.Command, _ []string) error { return nil }

	cmd := newProjectIDCommand(opts, noValidator)
	cmd.SetArgs([]string{"gh/test-org/test-repo"})

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	assert.NoError(t, err)
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000\n", outBuf.String()+errBuf.String())
}
