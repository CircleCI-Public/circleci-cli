package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/stretchr/testify/assert"
)

func TestGetOrgID(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"vcs_type":"circleci","slug":"gh/test","id":"2345"}]`)
	}))
	defer svr.Close()
	compiler := New(&settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient})

	t.Run("returns the original org-id passed if it is set", func(t *testing.T) {
		expected := "1234"
		actual, err := compiler.getOrgID("1234", "")
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("returns the correct org id from org-slug", func(t *testing.T) {
		expected := "2345"
		actual, err := compiler.getOrgID("", "gh/test")
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("returns the correct org id with org-id and org-slug both set", func(t *testing.T) {
		expected := "1234"
		actual, err := compiler.getOrgID("1234", "gh/test")
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("does not return an error if org-id cannot be found", func(t *testing.T) {
		expected := ""
		actual, err := compiler.getOrgID("", "gh/doesntexist")
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

}

var testYaml = `version: 2.1\n\norbs:\n  node: circleci/node@5.0.3\n\njobs:\n  datadog-hello-world:\n    docker:\n      - image: cimg/base:stable\n    steps:\n      - run: |\n          echo \"doing something really cool\"\nworkflows:\n  datadog-hello-world:\n    jobs:\n      - datadog-hello-world\n`

func TestValidateConfig(t *testing.T) {
	t.Run("validate config works as expected", func(t *testing.T) {
		t.Run("validate config is able to send a request with no owner-id", func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				reqBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				var req CompileConfigRequest
				err = json.Unmarshal(reqBody, &req)
				assert.NoError(t, err)
				fmt.Fprintf(w, `{"valid":true,"source-yaml":"%s","output-yaml":"%s","errors":[]}`, testYaml, testYaml)
			}))
			defer svr.Close()
			compiler := New(&settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient})

			err := compiler.ValidateConfig(ValidateConfigOpts{
				ConfigPath: "testdata/config.yml",
			})
			assert.NoError(t, err)
		})

		t.Run("validate config is able to send a request with owner-id", func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				reqBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				var req CompileConfigRequest
				err = json.Unmarshal(reqBody, &req)
				assert.NoError(t, err)
				assert.Equal(t, "1234", req.Options.OwnerID)
				fmt.Fprintf(w, `{"valid":true,"source-yaml":"%s","output-yaml":"%s","errors":[]}`, testYaml, testYaml)
			}))
			defer svr.Close()
			compiler := New(&settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient})

			err := compiler.ValidateConfig(ValidateConfigOpts{
				ConfigPath: "testdata/config.yml",
				OrgID:      "1234",
			})
			assert.NoError(t, err)
		})

		t.Run("validate config is able to send a request with owner-id from slug", func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("/compile-config-with-defaults", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				reqBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				var req CompileConfigRequest
				err = json.Unmarshal(reqBody, &req)
				assert.NoError(t, err)
				assert.Equal(t, "2345", req.Options.OwnerID)
				fmt.Fprintf(w, `{"valid":true,"source-yaml":"%s","output-yaml":"%s","errors":[]}`, testYaml, testYaml)
			})

			mux.HandleFunc("/me/collaborations", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `[{"vcs_type":"circleci","slug":"gh/test","id":"2345"}]`)
			})

			svr := httptest.NewServer(mux)
			defer svr.Close()

			compiler := New(&settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient})

			err := compiler.ValidateConfig(ValidateConfigOpts{
				ConfigPath: "testdata/config.yml",
				OrgSlug:    "gh/test",
			})
			assert.NoError(t, err)
		})

		t.Run("validate config is able to send a request with no owner-id after failed collaborations lookup", func(t *testing.T) {
			mux := http.NewServeMux()

			mux.HandleFunc("/compile-config-with-defaults", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				reqBody, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				var req CompileConfigRequest
				err = json.Unmarshal(reqBody, &req)
				assert.NoError(t, err)
				assert.Equal(t, "", req.Options.OwnerID)
				fmt.Fprintf(w, `{"valid":true,"source-yaml":"%s","output-yaml":"%s","errors":[]}`, testYaml, testYaml)
			})

			mux.HandleFunc("/me/collaborations", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `[{"vcs_type":"circleci","slug":"gh/test","id":"2345"}]`)
			})

			svr := httptest.NewServer(mux)
			defer svr.Close()

			compiler := New(&settings.Config{Host: svr.URL, HTTPClient: http.DefaultClient})

			err := compiler.ValidateConfig(ValidateConfigOpts{
				ConfigPath: "testdata/config.yml",
				OrgSlug:    "gh/nonexistent",
			})
			assert.NoError(t, err)
		})
	})
}
