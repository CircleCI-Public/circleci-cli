package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/stretchr/testify/assert"
)

func TestLegacyFlow(t *testing.T) {
	t.Run("tests that the compiler defaults to the graphQL resolver should the original API request fail with 404", func(t *testing.T) {
		mux := http.NewServeMux()

		mux.HandleFunc("/compile-config-with-defaults", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		mux.HandleFunc("/me/collaborations", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[{"vcs_type":"circleci","slug":"gh/test","id":"2345"}]`)
		})

		mux.HandleFunc("/graphql-unstable", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":{"buildConfig": {"valid":true,"sourceYaml":"%s","outputYaml":"%s","errors":[]}}}`, testYaml, testYaml)
		})

		svr := httptest.NewServer(mux)
		defer svr.Close()

		compiler := New(&settings.Config{
			Host:       svr.URL,
			Endpoint:   "/graphql-unstable",
			HTTPClient: http.DefaultClient,
			Token:      "",
		})
		resp, err := compiler.ConfigQuery("testdata/config.yml", "1234", Parameters{}, Values{})

		assert.Equal(t, true, resp.Valid)
		assert.NoError(t, err)
	})

	t.Run("tests that the compiler handles errors properly when returned from the graphQL endpoint", func(t *testing.T) {
		mux := http.NewServeMux()

		mux.HandleFunc("/compile-config-with-defaults", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		mux.HandleFunc("/me/collaborations", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[{"vcs_type":"circleci","slug":"gh/test","id":"2345"}]`)
		})

		mux.HandleFunc("/graphql-unstable", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":{"buildConfig":{"errors":[{"message": "failed to validate"}]}}}`)
		})

		svr := httptest.NewServer(mux)
		defer svr.Close()

		compiler := New(&settings.Config{
			Host:       svr.URL,
			Endpoint:   "/graphql-unstable",
			HTTPClient: http.DefaultClient,
			Token:      "",
		})
		_, err := compiler.ConfigQuery("testdata/config.yml", "1234", Parameters{}, Values{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to validate")
	})

	t.Run("tests that the compiler fails out completely when a non-404 is returned from the http endpoint", func(t *testing.T) {
		mux := http.NewServeMux()
		gqlHitCounter := 0

		mux.HandleFunc("/compile-config-with-defaults", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)

		})

		mux.HandleFunc("/me/collaborations", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[{"vcs_type":"circleci","slug":"gh/test","id":"2345"}]`)
		})

		mux.HandleFunc("/graphql-unstable", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":{"buildConfig":{"errors":[{"message": "failed to validate"}]}}}`)
			gqlHitCounter++
		})

		svr := httptest.NewServer(mux)
		defer svr.Close()

		compiler := New(&settings.Config{
			Host:       svr.URL,
			Endpoint:   "/graphql-unstable",
			HTTPClient: http.DefaultClient,
			Token:      "",
		})
		_, err := compiler.ConfigQuery("testdata/config.yml", "1234", Parameters{}, Values{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config compilation request returned an error:")
		assert.Equal(t, 0, gqlHitCounter)
	})
}
