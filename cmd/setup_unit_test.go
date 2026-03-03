package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
)

func testDummyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())
	return cmd
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	assert.NilError(t, err)

	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	f()
	assert.NilError(t, w.Close())

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	assert.NilError(t, err)
	return buf.String()
}

func TestSetupUnitNewConfigPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions test not applicable on Windows")
	}

	ts := testhelpers.WithTempSettings(t)
	token := "boondoggle"

	ts.Server.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.Path, "/api/v2/me")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":       "zomg",
			"login":      "zomg",
			"id":         "97491110-fea3-49b1-83da-ffd38ac8840c",
			"avatar_url": "https://avatars.githubusercontent.com/u/980172390812730912?v=4",
		})
	})

	opts := setupOptions{
		cfg: &settings.Config{
			FileUsed:   ts.Config,
			Host:       ts.Server.URL,
			HTTPClient: http.DefaultClient,
		},
		noPrompt: false,
		tty: setupTestUI{
			host:            ts.Server.URL,
			token:           token,
			confirmEndpoint: true,
			confirmToken:    true,
		},
		token: token,
	}
	opts.cl = graphql.NewClient(http.DefaultClient, ts.Server.URL, opts.cfg.Endpoint, token, false)

	err := setup(testDummyCmd(), opts)
	assert.NilError(t, err)

	fileInfo, err := os.Stat(ts.Config)
	assert.NilError(t, err)
	assert.Equal(t, fileInfo.Mode().Perm().String(), "-rw-------")
}

func TestSetupUnitExistingConfigPrintSuccess(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	token := "boondoggle"

	ts.Server.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.Path, "/api/v2/me")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":       "zomg",
			"login":      "zomg",
			"id":         "97491110-fea3-49b1-83da-ffd38ac8840c",
			"avatar_url": "https://avatars.githubusercontent.com/u/980172390812730912?v=4",
		})
	})

	opts := setupOptions{
		cfg: &settings.Config{
			FileUsed:   ts.Config,
			Host:       "https://example.com/graphql",
			Token:      token,
			HTTPClient: http.DefaultClient,
		},
		noPrompt: false,
		tty: setupTestUI{
			host:            ts.Server.URL,
			token:           token,
			confirmEndpoint: true,
			confirmToken:    true,
		},
		token: token,
	}
	opts.cl = graphql.NewClient(http.DefaultClient, ts.Server.URL, opts.cfg.Endpoint, token, false)

	output := captureStdout(t, func() {
		err := setup(testDummyCmd(), opts)
		assert.NilError(t, err)
	})

	assert.Assert(t, strings.Contains(output, "A CircleCI token is already set. Do you want to change it"))
	assert.Assert(t, strings.Contains(output, "CircleCI API Token"))
	assert.Assert(t, strings.Contains(output, "API token has been set."))
	assert.Assert(t, strings.Contains(output, "CircleCI Host"))
	assert.Assert(t, strings.Contains(output, "CircleCI host has been set."))
	assert.Assert(t, strings.Contains(output, "Do you want to reset the endpoint? (default: graphql-unstable)"))
	assert.Assert(t, strings.Contains(output, fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.", ts.Config)))
	assert.Assert(t, strings.Contains(output, "Hello, zomg."))

	reread, err := os.ReadFile(ts.Config)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(reread), fmt.Sprintf("host: %s", ts.Server.URL)))
	assert.Assert(t, strings.Contains(string(reread), fmt.Sprintf("token: %s", token)))
}

func TestSetupUnitExistingConfigSkipToken(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	token := "boondoggle"

	ts.Server.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.Path, "/api/v2/me")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":       "zomg",
			"login":      "zomg",
			"id":         "97491110-fea3-49b1-83da-ffd38ac8840c",
			"avatar_url": "https://avatars.githubusercontent.com/u/980172390812730912?v=4",
		})
	})

	opts := setupOptions{
		cfg: &settings.Config{
			FileUsed:   ts.Config,
			Host:       "https://example.com/graphql",
			Token:      token,
			HTTPClient: http.DefaultClient,
		},
		noPrompt: false,
		tty: setupTestUI{
			host:            ts.Server.URL,
			token:           token,
			confirmEndpoint: true,
			confirmToken:    false,
		},
		token: token,
	}
	opts.cl = graphql.NewClient(http.DefaultClient, ts.Server.URL, opts.cfg.Endpoint, token, false)

	output := captureStdout(t, func() {
		err := setup(testDummyCmd(), opts)
		assert.NilError(t, err)
	})

	assert.Assert(t, strings.Contains(output, "A CircleCI token is already set. Do you want to change it"))
	assert.Assert(t, strings.Contains(output, "CircleCI Host"))
	assert.Assert(t, strings.Contains(output, "CircleCI host has been set."))
	assert.Assert(t, strings.Contains(output, "Do you want to reset the endpoint? (default: graphql-unstable)"))
	assert.Assert(t, strings.Contains(output, fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.", ts.Config)))
	assert.Assert(t, strings.Contains(output, "Hello, zomg."))

	reread, err := os.ReadFile(ts.Config)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(reread), fmt.Sprintf("host: %s", ts.Server.URL)))
	assert.Assert(t, strings.Contains(string(reread), fmt.Sprintf("token: %s", token)))
}

func TestSetupUnitWhoamiAuthError(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	token := "boondoggle"

	ts.Server.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.Path, "/api/v2/me")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "You must log in first",
		})
	})

	opts := setupOptions{
		cfg: &settings.Config{
			FileUsed:   ts.Config,
			Host:       ts.Server.URL,
			HTTPClient: http.DefaultClient,
		},
		noPrompt: false,
		tty: setupTestUI{
			host:            ts.Server.URL,
			token:           token,
			confirmEndpoint: true,
			confirmToken:    true,
		},
		token: token,
	}
	opts.cl = graphql.NewClient(http.DefaultClient, ts.Server.URL, opts.cfg.Endpoint, token, false)

	output := captureStdout(t, func() {
		err := setup(testDummyCmd(), opts)
		assert.NilError(t, err)
	})

	assert.Assert(t, strings.Contains(output, "CircleCI API Token"))
	assert.Assert(t, strings.Contains(output, "API token has been set."))
	assert.Assert(t, strings.Contains(output, "CircleCI Host"))
	assert.Assert(t, strings.Contains(output, "CircleCI host has been set."))
	assert.Assert(t, strings.Contains(output, fmt.Sprintf("Setup complete.\nYour configuration has been saved to %s.", ts.Config)))
	assert.Assert(t, strings.Contains(output, "Unable to query our API for your profile name, please check your settings."))
}
