package cmd_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

// matchYAML compares two YAML documents by unmarshalling both and comparing
// the resulting structures. This replaces Gomega's MatchYAML matcher.
func matchYAML(t *testing.T, actual, expected []byte) {
	t.Helper()
	var a, e interface{}
	assert.NilError(t, yaml.Unmarshal(actual, &a), "unmarshalling actual YAML")
	assert.NilError(t, yaml.Unmarshal(expected, &e), "unmarshalling expected YAML")
	assert.DeepEqual(t, a, e)
}

func TestConfigPackTelemetry(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{"config", "pack",
			"--skip-update-check",
			filepath.Join("testdata", "hugo-pack", ".circleci"),
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	testhelpers.AssertTelemetrySubset(t, ts, []telemetry.Event{
		telemetry.CreateConfigEvent(telemetry.CommandInfo{
			Name:      "pack",
			LocalArgs: map[string]string{},
		}, nil),
	})
}

func TestConfigPackHugoOrb(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	expected := golden.Get(t, filepath.FromSlash("hugo-pack/result.yml"))

	result := testhelpers.RunCLI(t, binary,
		[]string{"config", "pack",
			"--skip-update-check",
			filepath.Join("testdata", "hugo-pack", ".circleci"),
		},
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stderr, "")
	matchYAML(t, []byte(result.Stdout), expected)
}

func TestConfigPackNestedOrbsAndLocalCommands(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	path := "nested-orbs-and-local-commands-etc"
	expected := golden.Get(t, filepath.FromSlash(fmt.Sprintf("%s/result.yml", path)))

	result := testhelpers.RunCLI(t, binary,
		[]string{"config", "pack",
			"--skip-update-check",
			filepath.Join("testdata", path, "test"),
		},
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stderr, "")
	matchYAML(t, []byte(result.Stdout), expected)
}

func TestConfigPackMyOrb(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	expected := golden.Get(t, filepath.FromSlash("myorb/result.yml"))

	result := testhelpers.RunCLI(t, binary,
		[]string{"config", "pack",
			"--skip-update-check",
			"testdata/myorb/test",
		},
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stderr, "")
	matchYAML(t, []byte(result.Stdout), expected)
}

func TestConfigPackLargeNestedRailsOrb(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	path := "test-with-large-nested-rails-orb"
	expected := golden.Get(t, filepath.FromSlash(fmt.Sprintf("%s/result.yml", path)))

	result := testhelpers.RunCLI(t, binary,
		[]string{"config", "pack",
			"--skip-update-check",
			filepath.Join("testdata", path, "test"),
		},
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stderr, "")
	matchYAML(t, []byte(result.Stdout), expected)
}

func TestConfigPackListConfigError(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	orbDir := filepath.Join(ts.Home, "myorb")
	assert.NilError(t, os.MkdirAll(orbDir, 0700))

	configPath := filepath.Join(orbDir, "config.yaml")
	assert.NilError(t, os.WriteFile(configPath, []byte(`[]`), 0600))

	expected := fmt.Sprintf("Error: Failed trying to marshal the tree to YAML : expected a map, got a `[]interface {}` which is not supported at this time for \"%s\"\n", configPath)

	result := testhelpers.RunCLI(t, binary,
		[]string{"config", "pack",
			"--skip-update-check",
			orbDir,
		},
	)

	assert.Equal(t, result.ExitCode, testhelpers.ShouldFail(),
		"stdout: %s\nstderr: %s", result.Stdout, result.Stderr)
	assert.Equal(t, result.Stderr, expected)
}

func TestConfigGenerateWithoutPath(t *testing.T) {
	binary := testhelpers.BuildCLI(t)

	cmd := exec.Command(binary, "config", "generate")
	cmd.Dir = "testdata/node"

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	assert.NilError(t, err)
	assert.Equal(t, stderr.String(), "")
	assert.Assert(t, strings.Contains(stdout.String(), "npm test"),
		"stdout: %s", stdout.String())
}

func TestConfigGenerateWithPath(t *testing.T) {
	binary := testhelpers.BuildCLI(t)

	wd, err := os.Getwd()
	assert.NilError(t, err)

	cmd := exec.Command(binary, "config", "generate", "node")
	cmd.Dir = filepath.Join(wd, "testdata")

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	assert.NilError(t, err)
	assert.Equal(t, stderr.String(), "")
	assert.Assert(t, strings.Contains(stdout.String(), "npm test"),
		"stdout: %s", stdout.String())
}
