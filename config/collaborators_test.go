package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/stretchr/testify/assert"
)

func TestGetOrgCollaborations(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"vcs_type":"circleci","slug":"gh/test","id":"2345"}]`)
	}))
	defer svr.Close()
	compiler := New(&settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient})

	t.Run("assert compiler has correct host", func(t *testing.T) {
		assert.Equal(t, "http://"+compiler.collaboratorRestClient.BaseURL.Host, svr.URL)
	})

	t.Run("getOrgCollaborations can parse response correctly", func(t *testing.T) {
		collabs, err := compiler.GetOrgCollaborations()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(collabs))
		assert.Equal(t, "circleci", collabs[0].VcsTye)
	})

	t.Run("can fetch orgID from a slug", func(t *testing.T) {
		expected := "1234"
		actual := GetOrgIdFromSlug("gh/test", []CollaborationResult{{OrgSlug: "gh/test", OrgId: "1234"}})
		assert.Equal(t, expected, actual)
	})

	t.Run("returns empty if no slug match", func(t *testing.T) {
		expected := ""
		actual := GetOrgIdFromSlug("gh/doesntexist", []CollaborationResult{{OrgSlug: "gh/test", OrgId: "1234"}})
		assert.Equal(t, expected, actual)
	})
}
