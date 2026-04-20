package deploy

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

// scriptedReader replays a fixed sequence of answers and records which
// prompt+default pairs it was asked. It stands in for the interactive
// terminal prompt during tests.
type scriptedReader struct {
	answers []string
	calls   []scriptedCall
}

type scriptedCall struct {
	msg          string
	defaultValue string
}

func (s *scriptedReader) ReadStringFromUser(msg string, defaultValue string) string {
	s.calls = append(s.calls, scriptedCall{msg: msg, defaultValue: defaultValue})
	if len(s.answers) == 0 {
		return defaultValue
	}
	next := s.answers[0]
	s.answers = s.answers[1:]
	if next == "" {
		return defaultValue
	}
	return next
}

// setupTempConfig copies a testdata fixture into a fresh temp directory
// and returns the path to the copy. Tests mutate this copy freely.
func setupTempConfig(t *testing.T, fixture string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".circleci")
	require.NoError(t, os.Mkdir(cfgDir, 0o755))
	dst := filepath.Join(cfgDir, "config.yml")
	require.NoError(t, os.WriteFile(dst, src, 0o644))
	return dst
}

// runInitCmd builds the deploy init command with the supplied reader
// and captures its stdout/stderr.
func runInitCmd(t *testing.T, configPath string, reader UserInputReader) (string, error) {
	t.Helper()
	dopts := deployOpts{reader: reader}
	cmd := newInitCommand(&settings.Config{}, &dopts)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--config", configPath})

	// Root-style prerun hooks are not wired here; the init subcommand
	// does not require any beyond what cobra provides.
	parent := &cobra.Command{Use: "deploy"}
	parent.AddCommand(cmd)
	parent.SetOut(&out)
	parent.SetErr(&out)
	parent.SetArgs([]string{"init", "--config", configPath})

	err := parent.Execute()
	return out.String(), err
}

func TestInit_HappyPath(t *testing.T) {
	configPath := setupTempConfig(t, "simple-deploy.yml")
	reader := &scriptedReader{answers: []string{"api", "production"}}

	output, err := runInitCmd(t, configPath, reader)
	require.NoError(t, err, output)

	assert.Contains(t, output, "Found 1 deploy job")
	assert.Contains(t, output, "deploy-prod")
	assert.Contains(t, output, "Updated")
	assert.Contains(t, output, "https://app.circleci.com/deploys")
	assert.NotContains(t, output, "git commit -m \"Wire up CircleCI deploy markers\" && git push",
		"output should not chain commit and push on one line")

	// The config file should now contain a deploy marker step.
	patched, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(patched), "circleci run release log")
	assert.Contains(t, string(patched), "--component-name=api")
	assert.Contains(t, string(patched), "--environment-name=production")
}

func TestInit_NoDeployJobsExitsCleanly(t *testing.T) {
	configPath := setupTempConfig(t, "no-deploy-jobs.yml")
	original, err := os.ReadFile(configPath)
	require.NoError(t, err)

	reader := &scriptedReader{}
	output, err := runInitCmd(t, configPath, reader)
	require.NoError(t, err, output)

	assert.Contains(t, output, "No deploy jobs detected")
	// The config file must be untouched when nothing matched.
	after, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, string(original), string(after), "config file should not be modified when no deploy jobs are detected")
	// With no prompts needed we expect a reader that was never called.
	assert.Empty(t, reader.calls)
}

func TestInit_IdempotentOnAlreadyInstrumentedConfig(t *testing.T) {
	configPath := setupTempConfig(t, "already-instrumented.yml")
	original, err := os.ReadFile(configPath)
	require.NoError(t, err)

	reader := &scriptedReader{}
	output, err := runInitCmd(t, configPath, reader)
	require.NoError(t, err, output)

	assert.Contains(t, output, "already instrumented")
	after, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, string(original), string(after), "config should not be rewritten when everything is already instrumented")
	// No user questions should be asked either.
	assert.Empty(t, reader.calls)
}

func TestInit_PromptsPerUninferredJob(t *testing.T) {
	configPath := setupTempConfig(t, "multiple-deploys.yml")
	// Script answers: component name, then one environment per
	// job that could not be inferred. deploy-staging → staging,
	// deploy-prod → production (both inferred); release-docs
	// is not inferable so we supply "docs".
	reader := &scriptedReader{answers: []string{"api", "", "", "docs"}}

	output, err := runInitCmd(t, configPath, reader)
	require.NoError(t, err, output)

	// First prompt is always component name.
	require.NotEmpty(t, reader.calls)
	assert.Contains(t, reader.calls[0].msg, "service")

	patched, err := os.ReadFile(configPath)
	require.NoError(t, err)
	body := string(patched)
	assert.Contains(t, body, "--environment-name=staging")
	assert.Contains(t, body, "--environment-name=production")
	assert.Contains(t, body, "--environment-name=docs")
}
