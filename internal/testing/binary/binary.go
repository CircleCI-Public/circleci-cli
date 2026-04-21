// Package binary builds the circleci CLI binary once per test run and provides
// helpers for running it in tests.
package binary

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var (
	once       sync.Once
	binaryPath string
	buildErr   error
)

// BuildBinary compiles the CLI binary once and returns its path.
// Call from TestMain; on error, the binary could not be built and tests
// should be skipped rather than failed.
func BuildBinary() (string, error) {
	once.Do(func() {
		dir, err := os.MkdirTemp("", "circleci-cli-test-*")
		if err != nil {
			buildErr = fmt.Errorf("create temp dir: %w", err)
			return
		}
		binaryPath = filepath.Join(dir, "circleci")

		// acceptance/ is one level below the module root.
		repoRoot, err := filepath.Abs(filepath.Join("..", ""))
		if err != nil {
			buildErr = fmt.Errorf("resolve repo root: %w", err)
			return
		}

		cmd := exec.Command("go", "build", "-o", binaryPath, ".")
		cmd.Dir = repoRoot
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("go build failed: %w\nstderr: %s", err, stderr.String())
			return
		}
	})
	return binaryPath, buildErr
}

// CLIResult holds the output of a CLI invocation.
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// RunCLI executes the circleci binary with the given args, env, and working directory.
func RunCLI(t *testing.T, args []string, env []string, workDir string) CLIResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = workDir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run CLI: %v", err)
		}
	}

	return CLIResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}
