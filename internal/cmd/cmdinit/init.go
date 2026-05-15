// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

// Package cmdinit implements the "circleci init" onboarding command.
package cmdinit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
	"github.com/CircleCI-Public/circleci-cli/internal/testrunner"
)

const signupURL = "https://circleci.com/signup"

type scanner interface {
	Scan(ctx context.Context, dir string) (*reposcan.Result, error)
}

// NewInitCmd returns the "circleci init" command.
func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize CircleCI for the current repository",
		Long: heredoc.Doc(`
			Walk through onboarding for the current project. circleci init will:

			  1. Scan your repo for tests
			  2. Run the tests in a Docker container using CircleCI's test framework
			  3. Generate a config file that can run your tests with CircleCI
			  4. Sign up for CircleCI to run your generated config

			Run from inside a git repository.
		`),
		Example: heredoc.Doc(`
			# Initialize CircleCI from the current repository
			$ circleci init

			# Preview the init flow help
			$ circleci init --help

			# Run init after creating a local git repository
			$ git init && circleci init
		`),
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			cmd.SetContext(ctx)

			dir, err := os.Getwd()
			if err != nil {
				return clierrors.New("init.cwd_failed", "Could not read working directory", err.Error())
			}
			if !gitremote.InsideWorkTree() {
				return clierrors.New("init.not_in_git_repo",
					"Not in a git repository",
					"You must be in a valid git repository directory to run this command.").
					WithSuggestions("Use `cd <path>` to navigate to your project, then try again.").
					WithExitCode(clierrors.ExitBadArguments)
			}

			ok := iostream.SymbolOK(ctx)
			repoName := filepath.Base(dir)

			iostream.ErrPrintf(ctx, "%s Git repository detected.\n\n", ok)
			iostream.ErrPrintf(ctx, "circleci init will:\n")
			iostream.ErrPrintf(ctx, "  • Scan your repo for tests\n")
			iostream.ErrPrintf(ctx, "  • Run the tests in a Docker container using CircleCI's test framework\n")
			iostream.ErrPrintf(ctx, "  • Generate a config file that can run your tests with CircleCI\n\n")
			iostream.ErrPrintf(ctx, "This will run in your selected repo: %s\n\n", repoName)

			iostream.ErrPrintln(ctx, "[1/3] Scanning repository")
			scan, err := newInitScanner().Scan(ctx, dir)
			if err != nil {
				return clierrors.New("init.scan_failed",
					"Repository scan failed",
					fmt.Sprintf("env-builder could not scan this repository: %v", err)).
					WithExitCode(clierrors.ExitGeneralError)
			}
			reposcan.Render(ctx, scan)

			iostream.ErrPrintln(ctx, "[2/3] Running tests in Docker")
			result := testrunner.Run(ctx, dir, scan, newInitRunner())
			testrunner.Render(ctx, result)
			switch result.Outcome {
			case testrunner.OutcomePass:
			case testrunner.OutcomeFail:
				iostream.ErrPrintln(ctx, "")
				iostream.ErrPrintln(ctx, testrunner.BuildAgentPrompt(scan, 80, result.Stdout, result.Stderr))
				return clierrors.New("init.tests_failed",
					"Tests failed",
					"The detected test suite failed in the generated CircleCI test environment.").
					WithSuggestions("Paste the agent-ready prompt above into your AI assistant to fix the failure.").
					WithExitCode(clierrors.ExitValidationFail)
			case testrunner.OutcomeError:
				suggestions := []string{"Check the runner error above and try again."}
				if errors.Is(result.Err, testrunner.ErrRunnerUnavailable) {
					suggestions = []string{"Install and start Docker, then rerun circleci init."}
				}
				return clierrors.New("init.runner_error",
					"Could not run tests",
					"circleci init could not run the detected tests through env-builder.").
					WithSuggestions(suggestions...).
					WithExitCode(clierrors.ExitGeneralError)
			}

			iostream.ErrPrintln(ctx, "[3/3] Generating config (stub)")
			iostream.ErrPrintf(ctx, "\nNext: sign up for CircleCI to run your generated config file.\n")
			iostream.ErrPrintf(ctx, "  %s\n", signupURL)
			return nil
		},
	}
}

func noArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.NoArgs(cmd, args); err != nil {
		return clierrors.New("init.unexpected_args",
			"Unexpected argument",
			err.Error()).
			WithExitCode(clierrors.ExitBadArguments)
	}
	return nil
}

func newInitScanner() scanner {
	if os.Getenv("CIRCLECI_INIT_FAKE_RUNNER") != "" {
		return fakeScanner{}
	}
	return reposcan.NewDefaultScanner()
}

func newInitRunner() testrunner.Runner {
	switch os.Getenv("CIRCLECI_INIT_FAKE_RUNNER") {
	case "pass":
		return fakeRunner{result: testrunner.RunResult{Outcome: testrunner.OutcomePass, ExitCode: 0, Stdout: "ok\n"}}
	case "fail":
		return fakeRunner{result: testrunner.RunResult{
			Outcome:  testrunner.OutcomeFail,
			ExitCode: 1,
			Stdout:   "go test ./...\n",
			Stderr:   "--- FAIL: TestExample\n    example_test.go:12: expected true\nFAIL\n",
		}}
	case "unavailable":
		return fakeRunner{buildErr: fmt.Errorf("%w: Docker is required", testrunner.ErrRunnerUnavailable)}
	default:
		return testrunner.NewDefaultRunner()
	}
}

type fakeScanner struct{}

func (fakeScanner) Scan(context.Context, string) (*reposcan.Result, error) {
	return &reposcan.Result{
		Stack:        "go",
		Image:        "cimg/go",
		ImageVersion: "1.22",
		Setup: []reposcan.SetupStep{
			{Name: "install", Command: "go mod download"},
			{Name: "test", Command: "go test ./..."},
		},
	}, nil
}

type fakeRunner struct {
	buildErr error
	result   testrunner.RunResult
}

func (r fakeRunner) Build(context.Context, string, string) error {
	return r.buildErr
}

func (r fakeRunner) Run(context.Context, string) (testrunner.RunResult, error) {
	if r.result.Outcome == "" {
		return testrunner.RunResult{Outcome: testrunner.OutcomePass, ExitCode: 0}, nil
	}
	return r.result, nil
}
