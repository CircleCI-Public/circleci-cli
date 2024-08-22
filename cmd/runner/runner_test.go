package runner

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func Test_NewCommand(t *testing.T) {
	t.Run("Runner uses /api/v3", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Check(t, cmp.Equal(r.URL.EscapedPath(), "/api/v3/runner"))

			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"items":[]}`))
			assert.NilError(t, err)
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		cmd := NewCommand(&settings.Config{Host: server.URL, HTTPClient: &http.Client{}}, nil)
		cmd.SetArgs([]string{"instance", "ls", "my-namespace"})
		err := cmd.Execute()
		assert.NilError(t, err)
	})
}
