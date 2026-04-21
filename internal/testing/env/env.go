// Package env provides a test environment builder for acceptance tests.
package env

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestEnv holds the environment configuration for a single test run.
type TestEnv struct {
	// HomeDir is the temp home directory for this test.
	HomeDir string
	// CircleCIURL overrides the CircleCI API base URL (for fake servers).
	CircleCIURL string
	// Token is the CircleCI API token injected via environment variable.
	Token string
	// Extra holds additional environment variables.
	Extra map[string]string
}

// New creates a TestEnv with an isolated temp home directory.
func New(t *testing.T) *TestEnv {
	t.Helper()
	home := t.TempDir()
	return &TestEnv{
		HomeDir: home,
		Extra:   map[string]string{},
	}
}

// Environ returns the environment slice suitable for exec.Cmd.Env.
// It includes a minimal safe environment (PATH, no inherited HOME).
func (e *TestEnv) Environ() []string {
	env := []string{
		"HOME=" + e.HomeDir,
		"XDG_CONFIG_HOME=" + filepath.Join(e.HomeDir, ".config"),
		"PATH=" + os.Getenv("PATH"),
		"NO_COLOR=1", // deterministic output in tests
	}
	if e.Token != "" {
		env = append(env, "CIRCLECI_TOKEN="+e.Token)
	}
	if e.CircleCIURL != "" {
		// The fake server URL is injected via a dedicated env var that
		// the API client reads in test builds.
		env = append(env, "CIRCLECI_HOST="+e.CircleCIURL)
	}
	for k, v := range e.Extra {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
