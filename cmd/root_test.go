package cmd_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/cmd"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
)

func TestMakeCommands(t *testing.T) {
	commands := cmd.MakeCommands()
	assert.Equal(t, len(commands.Commands()), 29)
}

func TestBuildWithoutAutoUpdate(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)

	noUpdateCLI := buildCLIWithLdflags(t,
		"-X github.com/CircleCI-Public/circleci-cli/version.packageManager=homebrew",
	)

	t.Run("reports update command as unavailable", func(t *testing.T) {
		result := testhelpers.RunCLI(t, noUpdateCLI,
			[]string{"help", "--skip-update-check"},
			"HOME="+ts.Home,
			"USERPROFILE="+ts.Home,
		)
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Assert(t, matchesPattern(result.Stderr, "update", "This command is unavailable on your platform"),
			"expected stderr to contain 'update ... This command is unavailable on your platform', got: %s", result.Stderr)
	})

	t.Run("tells the user to update using their package manager", func(t *testing.T) {
		result := testhelpers.RunCLI(t, noUpdateCLI,
			[]string{"update"},
		)
		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Assert(t, strings.Contains(result.Stdout, "update is not available because this tool was installed using homebrew."),
			"stdout: %s", result.Stdout)
		assert.Assert(t, strings.Contains(result.Stdout, "Please consult the package manager's documentation on how to update the CLI."),
			"stdout: %s", result.Stdout)
	})
}

func TestBuildWithAutoUpdate(t *testing.T) {
	binary := testhelpers.BuildCLI(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{"help", "--skip-update-check"},
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, matchesPattern(result.Stderr, "update", "Update the tool to the latest version"),
		"expected stderr to contain 'update ... Update the tool to the latest version', got: %s", result.Stderr)
}

func buildCLIWithLdflags(t testing.TB, ldflags string) string {
	t.Helper()

	outPath := filepath.Join(t.TempDir(), "circleci")
	if runtime.GOOS == "windows" {
		outPath += ".exe"
	}
	buildCmd := exec.Command("go", "build", "-o", outPath, "-ldflags", ldflags, ".")
	buildCmd.Dir = testhelpers.RepoRoot()
	out, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("buildCLIWithLdflags: go build failed: %v\n%s", err, out)
	}
	return outPath
}

func matchesPattern(s string, substrs ...string) bool {
	pos := 0
	for _, sub := range substrs {
		idx := strings.Index(s[pos:], sub)
		if idx < 0 {
			return false
		}
		pos += idx + len(sub)
	}
	return true
}
