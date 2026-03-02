package cmd

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

func TestPluginExecution(t *testing.T) {
	tempDir := t.TempDir()

	var pluginScript string
	pluginPath := filepath.Join(tempDir, "circleci-test-plugin")
	if runtime.GOOS == "windows" {
		pluginPath = pluginPath + ".bat"
		pluginScript = `#!/bin/bash
echo "Plugin executed"
echo "Args: %@%"
echo "Project ID: %CIRCLE_PROJECT_ID%"
echo "Circle URL: %CIRCLE_URL%"
echo "Circle Token: %CIRCLE_TOKEN%"
echo "Telemetry Enabled: %CIRCLE_TELEMETRY_ENABLED%"
exit 0
`
	} else {
		pluginScript = `#!/bin/bash
echo "Plugin executed"
echo "Args: $@"
echo "Project ID: $CIRCLE_PROJECT_ID"
echo "Circle URL: $CIRCLE_URL"
echo "Circle Token: $CIRCLE_TOKEN"
echo "Telemetry Enabled: $CIRCLE_TELEMETRY_ENABLED"
exit 0
`
	}
	err := os.WriteFile(pluginPath, []byte(pluginScript), 0755)
	assert.NilError(t, err)

	fakeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"id": "test-project-id"}`))
		assert.NilError(t, err)
	}))
	t.Cleanup(fakeSrv.Close)

	config := &settings.Config{
		Host:   fakeSrv.URL,
		Token:  "test-token",
		Stdout: new(bytes.Buffer),
		Stderr: new(bytes.Buffer),
	}
	err = config.WithHTTPClient()
	assert.NilError(t, err)

	t.Run("should find and execute a plugin in PATH", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		cfg := &settings.Config{
			Host:   config.Host,
			Token:  config.Token,
			Stdout: &stdout,
			Stderr: &stderr,
		}
		err := cfg.WithHTTPClient()
		assert.NilError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		cmd := newRunCommand(cfg)
		telemetryClient := telemetry.CreateFileTelemetry(filepath.Join(tempDir, "telemetry"))
		cmd.SetContext(telemetry.NewContext(context.Background(), telemetryClient))
		cmd.SetArgs([]string{"test-plugin", "arg1", "arg2"})
		err = cmd.Execute()
		assert.NilError(t, err)
		assert.Assert(t, strings.Contains(stdout.String(), "Project ID: test-project-id"))
		assert.Assert(t, strings.Contains(stdout.String(), "Circle Token: test-token"))
		assert.Assert(t, strings.Contains(stdout.String(), "Circle URL: "+fakeSrv.URL))
		assert.Assert(t, strings.Contains(stdout.String(), "Telemetry Enabled: true"))
	})

	t.Run("should return an error when plugin is not found", func(t *testing.T) {
		cmd := newRunCommand(config)
		cmd.SetArgs([]string{"nonexistent-plugin"})
		err := cmd.Execute()
		assert.Assert(t, err != nil)
		assert.Assert(t, strings.Contains(err.Error(), "plugin 'nonexistent-plugin' not found"))
	})

	t.Run("should require at least one argument", func(t *testing.T) {
		cmd := newRunCommand(config)
		cmd.SetArgs([]string{})
		err := cmd.Execute()
		assert.Assert(t, err != nil)
		assert.Assert(t, strings.Contains(err.Error(), "requires at least 1 arg"))
	})
}
