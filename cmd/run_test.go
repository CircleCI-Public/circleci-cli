package cmd

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

var _ = Describe("run", func() {
	var (
		tempDir    string
		pluginPath string
		config     *settings.Config
		fakeSrv    *httptest.Server
		stdout     bytes.Buffer
		stderr     bytes.Buffer
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "circleci-plugin-test")
		Expect(err).ToNot(HaveOccurred())

		// Create a test plugin
		var pluginScript string
		pluginPath = filepath.Join(tempDir, "circleci-test-plugin")
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
		err = os.WriteFile(pluginPath, []byte(pluginScript), 0755)
		Expect(err).ToNot(HaveOccurred())

		fakeSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"id": "test-project-id"}`))
			Expect(err).ToNot(HaveOccurred())
		}))

		config = &settings.Config{
			Host:   fakeSrv.URL,
			Token:  "test-token",
			Stdout: &stdout,
			Stderr: &stderr,
		}
		err = config.WithHTTPClient()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
		fakeSrv.Close()
	})

	Describe("plugin execution", func() {
		It("should find and execute a plugin in PATH", func() {
			// Add tempDir to PATH
			oldPath := os.Getenv("PATH")
			os.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)
			defer os.Setenv("PATH", oldPath)

			cmd := newRunCommand(config)
			telemetryClient := telemetry.CreateFileTelemetry(filepath.Join(tempDir, "telemetry"))
			cmd.SetContext(telemetry.NewContext(context.Background(), telemetryClient))
			cmd.SetArgs([]string{"test-plugin", "arg1", "arg2"})
			err := cmd.Execute()
			Expect(stdout.String()).To(ContainSubstring("Project ID: test-project-id"))
			Expect(stdout.String()).To(ContainSubstring("Circle Token: test-token"))
			Expect(stdout.String()).To(ContainSubstring("Circle URL: %s", fakeSrv.URL))
			Expect(stdout.String()).To(ContainSubstring("Telemetry Enabled: true"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error when plugin is not found", func() {
			cmd := newRunCommand(config)
			cmd.SetArgs([]string{"nonexistent-plugin"})
			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("plugin 'nonexistent-plugin' not found"))
		})

		It("should require at least one argument", func() {
			cmd := newRunCommand(config)
			cmd.SetArgs([]string{})
			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("requires at least 1 arg"))
		})
	})
})
