package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/api/collaborators"
	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	t.Run("test getCompileHost", func(t *testing.T) {
		t.Run("for Server instances", func(t *testing.T) {
			// if the host isn't equal to `https://circleci.com` then this is likely a server instance and
			// wont have the api.X.com subdomain so we should instead just respect the host for config commands
			host := GetCompileHost("test")
			assert.Equal(t, host, "test")
		})

		t.Run("for CircleCI servers", func(t *testing.T) {
			// If the host passed in is the same as the defaultHost 'https://circleci.com' - then we know this is cloud
			// and as such should use the `api.circleci.com` subdomain
			host := GetCompileHost("https://circleci.com")
			assert.Equal(t, host, "https://api.circleci.com")
		})
	})

	t.Run("test ConfigQuery", func(t *testing.T) {
		t.Run("returns the correct configCompilation response", func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"valid":true,"source-yaml":"source","output-yaml":"output","errors":[]}`)
			}))
			defer svr.Close()
			cfg := &settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient}
			apiClient := &v2APIClient{rest.NewFromConfig(cfg.Host, cfg)}
			collaboratorsClient, err := collaborators.NewCollaboratorsRestClient(*cfg)
			assert.NoError(t, err)
			compiler := New(apiClient, collaboratorsClient)

			result, err := compiler.ConfigQuery("testdata/config.yml", "1234", Parameters{}, Values{})
			assert.NoError(t, err)
			assert.Equal(t, true, result.Valid)
			assert.Equal(t, "output", result.OutputYaml)
			assert.Equal(t, "source", result.SourceYaml)
		})

		t.Run("returns error when config file could not be found", func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"valid":true,"source-yaml":"source","output-yaml":"output","errors":[]}`)
			}))
			defer svr.Close()
			cfg := &settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient}
			apiClient := &v2APIClient{rest.NewFromConfig(cfg.Host, cfg)}
			collaboratorsClient, err := collaborators.NewCollaboratorsRestClient(*cfg)
			assert.NoError(t, err)
			compiler := New(apiClient, collaboratorsClient)

			_, err = compiler.ConfigQuery("testdata/nonexistent.yml", "1234", Parameters{}, Values{})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "Could not load config file at testdata/nonexistent.yml")
		})

		// commenting this out - we have a legacy_test.go unit test that covers this behaviour
		// t.Run("handles 404 status correctly", func(t *testing.T) {
		// 	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 		w.WriteHeader(http.StatusNotFound)
		// 	}))
		// 	defer svr.Close()
		// 	compiler := New(&settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient})

		// 	_, err := compiler.ConfigQuery("testdata/config.yml", "1234", Parameters{}, Values{})
		// 	assert.Error(t, err)
		// 	assert.Contains(t, err.Error(), "this version of the CLI does not support your instance of server")
		// })

		t.Run("handles non-200 status correctly", func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer svr.Close()
			cfg := &settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient}
			apiClient := &v2APIClient{rest.NewFromConfig(cfg.Host, cfg)}
			collaboratorsClient, err := collaborators.NewCollaboratorsRestClient(*cfg)
			assert.NoError(t, err)
			compiler := New(apiClient, collaboratorsClient)

			_, err = compiler.ConfigQuery("testdata/config.yml", "1234", Parameters{}, Values{})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "config compilation request returned an error")
		})

		t.Run("server gets correct information owner ID", func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				reqBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				var req CompileConfigRequest
				err = json.Unmarshal(reqBody, &req)
				assert.NoError(t, err)
				assert.Equal(t, "1234", req.Options.OwnerID)
				assert.Equal(t, "test: test\n", req.ConfigYaml)
				fmt.Fprintf(w, `{"valid":true,"source-yaml":"source","output-yaml":"output","errors":[]}`)
			}))
			defer svr.Close()
			cfg := &settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient}
			apiClient := &v2APIClient{rest.NewFromConfig(cfg.Host, cfg)}
			collaboratorsClient, err := collaborators.NewCollaboratorsRestClient(*cfg)
			assert.NoError(t, err)
			compiler := New(apiClient, collaboratorsClient)

			resp, err := compiler.ConfigQuery("testdata/test.yml", "1234", Parameters{}, Values{})
			assert.NoError(t, err)
			assert.Equal(t, true, resp.Valid)
			assert.Equal(t, "output", resp.OutputYaml)
			assert.Equal(t, "source", resp.SourceYaml)
		})
	})
}

func TestLoadYaml(t *testing.T) {
	t.Run("tests load yaml", func(t *testing.T) {
		expected := `test: test
`
		actual, err := loadYaml("testdata/test.yml")
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("returns error for non-existent yml file", func(t *testing.T) {
		actual, err := loadYaml("testdata/non-existent.yml")
		assert.Error(t, err)
		assert.Equal(t, "", actual)
	})
}
