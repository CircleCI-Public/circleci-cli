// Package testhelpers provides framework-agnostic test utilities for the
// CircleCI CLI. It replaces the legacy clitest/ package, removing all
// Gomega/gexec/ghttp dependencies in favour of os/exec, net/http/httptest,
// and gotest.tools/v3.
//
// Migration path: replace clitest imports with testhelpers imports. Each
// helper accepts a testing.TB so it works with both *testing.T and *testing.B.
// Cleanup is registered via t.Cleanup — no manual AfterEach/defer needed.
package testhelpers

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// buildOnce ensures the CLI binary is built exactly once per test run.
var (
	buildOnce    sync.Once
	builtBinary  string
	buildErr     error
	buildTempDir string
)

// CLIResult holds the captured output and exit code of a CLI invocation.
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// ShouldFail returns the expected exit code for a failed CLI invocation:
// 255 on Unix, -1 on Windows.
func ShouldFail() int {
	if runtime.GOOS == "windows" {
		return -1
	}
	return 255
}

// BuildCLI compiles the CLI binary once per test run and returns the path.
// The binary is placed in a temporary directory that persists for the
// lifetime of the test process. ldflags match the Taskfile build task.
func BuildCLI(t testing.TB) string {
	t.Helper()
	buildOnce.Do(func() {
		buildTempDir, buildErr = os.MkdirTemp("", "circleci-cli-test-bin-")
		if buildErr != nil {
			return
		}

		binaryName := "circleci"
		if runtime.GOOS == "windows" {
			binaryName = "circleci.exe"
		}
		builtBinary = filepath.Join(buildTempDir, binaryName)

		cmd := exec.Command("go", "build",
			"-o", builtBinary,
			"-ldflags=-X github.com/CircleCI-Public/circleci-cli/telemetry.SegmentEndpoint=https://api.segment.io",
			".",
		)
		// Build from the repo root — walk up from testhelpers/.
		cmd.Dir = RepoRoot()
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = &BuildError{Output: string(out), Err: err}
		}
	})
	if buildErr != nil {
		t.Fatalf("BuildCLI: %v", buildErr)
	}
	return builtBinary
}

// BuildError wraps a failed go build with its output.
type BuildError struct {
	Output string
	Err    error
}

func (e *BuildError) Error() string {
	return "go build failed: " + e.Err.Error() + "\n" + e.Output
}

func (e *BuildError) Unwrap() error {
	return e.Err
}

// RunCLI executes the CLI binary with the given arguments and optional
// environment variables. Extra env vars are appended to os.Environ().
func RunCLI(t testing.TB, binary string, args []string, env ...string) *CLIResult {
	t.Helper()

	cmd := exec.Command(binary, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &CLIResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Err:      err,
	}
}

// RepoRoot returns the repository root directory (parent of testhelpers/).
func RepoRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filename))
}
