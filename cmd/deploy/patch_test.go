package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// copyFixture copies a file from testdata/ into a fresh temp file so the
// test can safely write to it. Returns the path to the copy.
func copyFixture(t *testing.T, fixture string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)
	dir := t.TempDir()
	dst := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(dst, src, 0o644))
	return dst
}

func TestPatchConfig_AddsMarkerStep(t *testing.T) {
	path := copyFixture(t, "simple-deploy.yml")
	_, root, err := ReadConfig(path)
	require.NoError(t, err)

	result, err := PatchConfig(root, []MarkerStep{{
		JobName:         "deploy-prod",
		ComponentName:   "api",
		EnvironmentName: "production",
	}})
	require.NoError(t, err)
	assert.Equal(t, []string{"deploy-prod"}, result.Modified)
	assert.Empty(t, result.Skipped)

	require.NoError(t, WriteConfig(path, root))

	written, err := os.ReadFile(path)
	require.NoError(t, err)
	out := string(written)
	assert.Contains(t, out, "circleci run release log")
	assert.Contains(t, out, "--component-name=api")
	assert.Contains(t, out, "--environment-name=production")
	assert.Contains(t, out, "--target-version=$CIRCLE_SHA1")
}

func TestPatchConfig_IsIdempotent(t *testing.T) {
	path := copyFixture(t, "already-instrumented.yml")
	_, root, err := ReadConfig(path)
	require.NoError(t, err)

	result, err := PatchConfig(root, []MarkerStep{{
		JobName:         "deploy-prod",
		ComponentName:   "api",
		EnvironmentName: "production",
	}})
	require.NoError(t, err)
	assert.Empty(t, result.Modified)
	assert.Equal(t, []string{"deploy-prod"}, result.Skipped)
}

func TestPatchConfig_AppliedTwiceDoesNotDuplicate(t *testing.T) {
	path := copyFixture(t, "simple-deploy.yml")
	_, root, err := ReadConfig(path)
	require.NoError(t, err)

	step := MarkerStep{JobName: "deploy-prod", ComponentName: "api", EnvironmentName: "production"}
	_, err = PatchConfig(root, []MarkerStep{step})
	require.NoError(t, err)
	require.NoError(t, WriteConfig(path, root))

	_, root2, err := ReadConfig(path)
	require.NoError(t, err)
	result, err := PatchConfig(root2, []MarkerStep{step})
	require.NoError(t, err)
	assert.Empty(t, result.Modified, "second patch should modify nothing")
	assert.Equal(t, []string{"deploy-prod"}, result.Skipped)
}

func TestPatchConfig_UnknownJobReturnsError(t *testing.T) {
	path := copyFixture(t, "simple-deploy.yml")
	_, root, err := ReadConfig(path)
	require.NoError(t, err)

	_, err = PatchConfig(root, []MarkerStep{{
		JobName:         "does-not-exist",
		ComponentName:   "api",
		EnvironmentName: "production",
	}})
	assert.Error(t, err)
}

func TestPatchConfig_MultipleJobsAreIndependent(t *testing.T) {
	path := copyFixture(t, "multiple-deploys.yml")
	_, root, err := ReadConfig(path)
	require.NoError(t, err)

	result, err := PatchConfig(root, []MarkerStep{
		{JobName: "deploy-staging", ComponentName: "api", EnvironmentName: "staging"},
		{JobName: "deploy-prod", ComponentName: "api", EnvironmentName: "production"},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"deploy-staging", "deploy-prod"}, result.Modified)

	require.NoError(t, WriteConfig(path, root))
	written, err := os.ReadFile(path)
	require.NoError(t, err)
	out := string(written)
	assert.Contains(t, out, "--environment-name=staging")
	assert.Contains(t, out, "--environment-name=production")
}
