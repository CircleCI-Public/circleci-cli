package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"gotest.tools/v3/assert"
)

func TestAPIClient(t *testing.T) {
	t.Run("detectCompilerVersion", func(t *testing.T) {
		t.Run("when the route returns a 404 tells that the version is v1", func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
				fmt.Fprintf(w, "Invalid input")
			}))
			url, err := url.Parse(svr.URL)
			assert.NilError(t, err)

			restClient := rest.New(url, "token", http.DefaultClient)
			version, err := detectAPIClientVersion(restClient)
			assert.NilError(t, err)
			assert.Equal(t, version, v1_string)
		})

		t.Run("on other cases return v2", func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
				fmt.Fprintf(w, "Invalid input")
			}))
			url, err := url.Parse(svr.URL)
			assert.NilError(t, err)

			restClient := rest.New(url, "token", http.DefaultClient)
			version, err := detectAPIClientVersion(restClient)
			assert.NilError(t, err)
			assert.Equal(t, version, v2_string)
		})
	})
}
