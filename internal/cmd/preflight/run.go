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

package preflight

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/preflight"
)

// defaultRunWait is how long to wait for the push to start a run.
// CIRCLE_SHA_WAIT_MS overrides it for testing, as it does for run watch.
const defaultRunWait = 2 * time.Minute

func runWait() time.Duration {
	if ms := os.Getenv("CIRCLE_SHA_WAIT_MS"); ms != "" {
		if n, err := strconv.Atoi(ms); err == nil {
			return time.Duration(n) * time.Millisecond
		}
	}
	return defaultRunWait
}

// fixedID returns the preflight ID set by CIRCLE_PREFLIGHT_ID, a test override
// that makes the branch name predictable. Zero means generate one.
func fixedID() uuid.UUID {
	id, _ := uuid.Parse(os.Getenv("CIRCLE_PREFLIGHT_ID"))
	return id
}

type runOptions struct {
	projectSlug string
	remote      string
	noWatch     bool
	noFailFast  bool
	noCleanup   bool
	interval    time.Duration
	timeout     time.Duration
	jsonOut     bool
}

func newRunCmd() *cobra.Command {
	var opts runOptions

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Push the working tree to a temporary branch and watch its run",
		Long: heredoc.Docf(`
			Push the working tree to a %[1]s%[2]s<id>%[1]s branch and watch the run it starts,
			reporting each failed job's steps and tests as it fails. See %[1]scircleci help preflight-guide%[1]s.

			--json streams one event per line, by type: branch_pushed (preflight_id, branch,
			commit, base, changes), run_started (run_id, url), job_failed (job.id/name/workflow/url/
			failed_steps[]/failed_tests[]), run_summary (run_id, outcome, failed_jobs[], ...)
		`, "`", preflight.BranchPrefix),
		Example: heredoc.Doc(`
			# Run local changes through CI, stopping at the first failure
			$ circleci preflight run

			# Stream events for a coding agent
			$ circleci preflight run --json

			# Push and return once the run starts; clean up later
			$ circleci preflight run --no-watch
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPreflight(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.projectSlug, "project", "", "project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&opts.remote, "remote", preflight.DefaultRemote, "git remote to push the branch to")
	cmd.Flags().BoolVar(&opts.noWatch, "no-watch", false, "return once the run starts, keeping the branch")
	cmd.Flags().BoolVar(&opts.noFailFast, "no-failfast", false, "wait for the whole run instead of stopping at the first failed job")
	cmd.Flags().BoolVar(&opts.noCleanup, "no-cleanup", false, "keep the branch, and the run going, when the preflight stops")
	cmd.Flags().DurationVar(&opts.interval, "interval", 5*time.Second, "how often to poll the run")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 30*time.Minute, "maximum time to watch the run")
	cmdutil.AddJSONFlag(cmd, &opts.jsonOut)
	cmd.MarkFlagsMutuallyExclusive("no-watch", "no-failfast")

	return cmd
}

func runPreflight(ctx context.Context, opts runOptions) error {
	if opts.interval <= 0 {
		return badDuration("--interval", opts.interval)
	}
	if opts.timeout <= 0 {
		return badDuration("--timeout", opts.timeout)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return noRepoErr(err)
	}
	repo, err := preflight.OpenRepo(ctx, cwd, opts.remote)
	if err != nil {
		return noRepoErr(err)
	}
	slug, err := cmdutil.ResolveProjectSlug(opts.projectSlug)
	if err != nil {
		return err
	}
	client, err := cmdutil.LoadClient(ctx)
	if err != nil {
		return err
	}
	appURL, err := cmdutil.AppURL(ctx)
	if err != nil {
		return clierrors.New("config.invalid_host", "Invalid CircleCI host", err.Error()).
			WithExitCode(clierrors.ExitBadArguments)
	}

	text := &textObserver{ctx: ctx}
	var observer preflight.Observer = text
	if opts.jsonOut {
		observer = &jsonObserver{ctx: ctx}
	}

	summary, err := preflight.Run(ctx, preflight.Options{
		Client:      client,
		Repo:        repo,
		ProjectSlug: slug,
		ID:          fixedID(),
		Watch:       !opts.noWatch,
		FailFast:    !opts.noFailFast,
		Cleanup:     !opts.noCleanup,
		Interval:    opts.interval,
		Timeout:     opts.timeout,
		RunWait:     runWait(),
		RunURL:      func(id uuid.UUID) string { return cmdutil.RunURL(appURL, id) },
		JobURL:      func(wf, job uuid.UUID) string { return cmdutil.JobURL(appURL, wf, job) },
		Observer:    observer,
	})
	if err != nil {
		return runErr(err, slug, summary)
	}
	if summary == nil {
		if !opts.jsonOut {
			text.kept()
		}
		return nil
	}
	return outcomeErr(*summary)
}

// runErr converts what stopped a preflight into a CLIError.
func runErr(err error, slug string, summary *preflight.RunSummaryEvent) error {
	var gitE *preflight.GitError
	var noRun *preflight.ErrNoRun
	switch {
	case errors.Is(err, context.Canceled):
		return clierrors.New("preflight.interrupted", "Preflight interrupted",
			"Stopped before the run completed.").
			WithExitCode(clierrors.ExitCancelled)
	case errors.Is(err, preflight.ErrTimedOut):
		msg := "The run did not complete in time."
		if summary != nil {
			msg = fmt.Sprintf("Run %s did not complete in time.", summary.RunID)
		}
		return clierrors.New("preflight.timeout", "Preflight timed out", msg).
			WithSuggestions("Raise the limit with --timeout, e.g. --timeout 1h").
			WithExitCode(clierrors.ExitTimeout)
	case errors.Is(err, preflight.ErrNoCommits):
		return clierrors.New("preflight.no_commits", "No commits to build on",
			"The preflight commit is a child of HEAD, and this repository has no commits yet.").
			WithSuggestions("Make an initial commit, then run the preflight again").
			WithExitCode(clierrors.ExitBadArguments)
	case errors.As(err, &noRun):
		return clierrors.New("preflight.no_run", "No run started",
			fmt.Sprintf("Pushed %s, but no run started for it within %s.", noRun.Branch, noRun.Waited)).
			WithSuggestions(
				"Check that the project's triggers build pushes to every branch, including "+preflight.BranchPrefix+"*",
				"Check the project on CircleCI: circleci project open",
			).
			WithExitCode(clierrors.ExitNotFound)
	case errors.As(err, &gitE):
		return gitErr(err, "Git command failed")
	default:
		return projectErr(err, slug)
	}
}

// outcomeErr maps how the watched run ended onto the exit status.
func outcomeErr(s preflight.RunSummaryEvent) error {
	switch s.Outcome {
	case preflight.OutcomeSucceeded:
		return nil
	case preflight.OutcomeCanceled:
		return clierrors.New("preflight.run_cancelled", "Run cancelled",
			fmt.Sprintf("Run %s was cancelled.", s.RunID)).
			WithExitCode(clierrors.ExitCancelled)
	}

	suggestions := []string{
		fmt.Sprintf("Get the failed steps' output: circleci run get %s --failure-report", s.RunID),
	}
	exitCode := clierrors.ExitGeneralError
	if hasConfigError(s) && len(s.FailedJobs) == 0 {
		exitCode = clierrors.ExitValidationFail
		suggestions = []string{"Validate the config locally: circleci config validate"}
	}
	return clierrors.New("preflight.failed", "Preflight failed", failureMessage(s)).
		WithSuggestions(suggestions...).
		WithExitCode(exitCode)
}

func failureMessage(s preflight.RunSummaryEvent) string {
	msg := fmt.Sprintf("Run %s failed.", s.RunID)
	if !s.Ended {
		msg = fmt.Sprintf("Run %s has %d failed job(s).", s.RunID, len(s.FailedJobs))
	}
	for _, e := range s.Errors {
		msg += fmt.Sprintf("\n\n%s error: %s", e.Type, e.Message)
	}
	return msg
}

func hasConfigError(s preflight.RunSummaryEvent) bool {
	for _, e := range s.Errors {
		if e.Type == "config" {
			return true
		}
	}
	return false
}

func badDuration(flag string, d time.Duration) error {
	return clierrors.New("preflight.invalid_duration", "Invalid duration",
		fmt.Sprintf("%s must be positive (got: %s).", flag, d)).
		WithExitCode(clierrors.ExitBadArguments)
}
