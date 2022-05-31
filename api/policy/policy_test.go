package policy

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func TestClient_NewClient(t *testing.T) {
	t.Run("Host url doesn't end with slash", func(t *testing.T) {
		config := settings.Config{Host: "http://localhost:8000", RestEndpoint: "api/v1", Token: "testtoken"}
		client, err := NewClient(config)
		assert.Equal(t, client.serverUrl, "http://localhost:8000/api/v1/")
		assert.Equal(t, client.token, "testtoken")
		assert.NilError(t, err)
	})

	t.Run("Host url ends with slash", func(t *testing.T) {
		config := settings.Config{Host: "http://localhost:8000/", RestEndpoint: "api/v1", Token: "testtoken"}
		client, err := NewClient(config)
		assert.Equal(t, client.serverUrl, "http://localhost:8000/api/v1/")
		assert.Equal(t, client.token, "testtoken")
		assert.NilError(t, err)
	})

	t.Run("RestEndpoint also starts with slash", func(t *testing.T) {
		config := settings.Config{Host: "http://localhost:8000/", RestEndpoint: "/api/v1", Token: "testtoken"}
		client, err := NewClient(config)
		assert.Equal(t, client.serverUrl, "http://localhost:8000/api/v1/")
		assert.Equal(t, client.token, "testtoken")
		assert.NilError(t, err)
	})

	t.Run("RestEndpoint is empty", func(t *testing.T) {
		config := settings.Config{Host: "http://localhost:8000", RestEndpoint: "", Token: "testtoken"}
		client, err := NewClient(config)
		assert.Equal(t, client.serverUrl, "http://localhost:8000/")
		assert.Equal(t, client.token, "testtoken")
		assert.NilError(t, err)
	})

	t.Run("Illegal characters in Host", func(t *testing.T) {
		config := settings.Config{Host: "http!://localhost:8000", RestEndpoint: "/api/v1", Token: "testtoken"}
		client, err := NewClient(config)
		cmp.Equal(client, nil)
		assert.ErrorContains(t, err, "first path segment in URL cannot contain colon")
	})
}
