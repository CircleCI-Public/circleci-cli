// settings.go provides temporary home directory and config file setup for
// CLI integration tests, replacing clitest.WithTempSettings/TempSettings.
//
// Migration path: replace clitest.WithTempSettings() with
// testhelpers.WithTempSettings(t) and use t.Cleanup instead of
// AfterEach/defer.
package testhelpers

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

// TempSettings holds paths to temporary config files and a test server,
// replacing the clitest.TempSettings struct.
type TempSettings struct {
	Home              string
	Config            string
	TelemetryFile     string
	UpdateFile        string
	TelemetryDestPath string
	Server            *TestServer
}

// WithTempSettings creates a temporary home directory with a .circleci
// subdirectory containing cli.yml, telemetry.yml (disabled, answered), and
// update_check.yml. A TestServer is also started. All resources are
// cleaned up via t.Cleanup.
func WithTempSettings(t testing.TB) *TempSettings {
	t.Helper()

	home, err := os.MkdirTemp("", "circleci-cli-test-")
	if err != nil {
		t.Fatalf("WithTempSettings: creating temp dir: %v", err)
	}

	settingsDir := filepath.Join(home, ".circleci")
	if err := os.Mkdir(settingsDir, 0700); err != nil {
		t.Fatalf("WithTempSettings: creating .circleci dir: %v", err)
	}

	configPath := filepath.Join(settingsDir, "cli.yml")
	if err := os.WriteFile(configPath, []byte(""), 0600); err != nil {
		t.Fatalf("WithTempSettings: creating cli.yml: %v", err)
	}

	telContent, err := yaml.Marshal(settings.TelemetrySettings{
		IsEnabled:         false,
		HasAnsweredPrompt: true,
	})
	if err != nil {
		t.Fatalf("WithTempSettings: marshalling telemetry settings: %v", err)
	}
	telPath := filepath.Join(settingsDir, "telemetry.yml")
	if err := os.WriteFile(telPath, telContent, 0600); err != nil {
		t.Fatalf("WithTempSettings: creating telemetry.yml: %v", err)
	}

	updatePath := filepath.Join(settingsDir, "update_check.yml")
	if err := os.WriteFile(updatePath, []byte(""), 0600); err != nil {
		t.Fatalf("WithTempSettings: creating update_check.yml: %v", err)
	}

	telemetryDestPath := filepath.Join(home, "telemetry-content")

	server := NewTestServer(t)

	ts := &TempSettings{
		Home:              home,
		Config:            configPath,
		TelemetryFile:     telPath,
		UpdateFile:        updatePath,
		TelemetryDestPath: telemetryDestPath,
		Server:            server,
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(home)
	})

	return ts
}

// WriteConfig writes the given content to the cli.yml config file.
func (ts *TempSettings) WriteConfig(t testing.TB, content string) {
	t.Helper()
	if err := os.WriteFile(ts.Config, []byte(content), 0600); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
}

// EnvForCLI returns an environment variable slice suitable for passing to
// RunCLI. It sets HOME to the temp directory and disables update checks.
func EnvForCLI(t testing.TB, s *TempSettings) []string {
	t.Helper()
	return []string{
		"HOME=" + s.Home,
		"CIRCLECI_CLI_SKIP_UPDATE_CHECK=true",
		"CIRCLECI_CLI_HOST=" + s.Server.URL,
	}
}
