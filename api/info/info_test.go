package info

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"gotest.tools/v3/assert"
)

func TestOkResponse(t *testing.T) {
	token := "pluto-is-a-planet"
	id := "id"
	name := "name"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.String(), "/me/collaborations")
		assert.Equal(t, r.Header.Get("circle-token"), token)
		assert.Equal(t, r.Header.Get("Content-Type"), "application/json")
		assert.Equal(t, r.Header.Get("Accept"), "application/json")

		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(fmt.Sprintf(`[{"id": "%s", "name": "%s"}]`, id, name)))
		assert.NilError(t, err)
	}))

	defer server.Close()

	config := settings.Config{
		Host:       server.URL,
		HTTPClient: http.DefaultClient,
		Token:      token,
	}

	client, _ := NewInfoClient(config)
	orgs, err := client.GetInfo()
	organizations := *orgs

	assert.NilError(t, err)
	assert.Equal(t, len(organizations), 1)

	org := organizations[0]
	assert.Equal(t, org.ID, id)
	assert.Equal(t, org.Name, name)
}

func TestServerErrorResponse(t *testing.T) {
	token := "pluto-is-a-planet"
	message := "i-come-in-peace"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.String(), "/me/collaborations")
		assert.Equal(t, r.Header.Get("circle-token"), token)
		assert.Equal(t, r.Header.Get("Content-Type"), "application/json")
		assert.Equal(t, r.Header.Get("Accept"), "application/json")

		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(fmt.Sprintf(`{"message": "%s"}`, message)))
		assert.NilError(t, err)
	}))

	defer server.Close()

	config := settings.Config{
		Host:       server.URL,
		HTTPClient: http.DefaultClient,
		Token:      token,
	}

	client, _ := NewInfoClient(config)
	_, err := client.GetInfo()

	assert.Error(t, err, message)
}
