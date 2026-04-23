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

package job

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/artifacts"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newArtifactsCmd() *cobra.Command {
	var (
		projectSlug string
		downloadDir string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "artifacts <job-number>",
		Short: "List or download artifacts for a job",
		Long: heredoc.Doc(`
			List or download artifacts produced by a specific job number.

			The project is inferred from the current git repository's remote
			unless overridden with --project.

			To list artifacts across a whole pipeline, use 'circleci artifacts'.

			JSON fields: job_name, job_number, path, url, node_index
		`),
		Example: heredoc.Doc(`
			# List artifacts for job number 123
			$ circleci job artifacts 123

			# Download artifacts for job 123 into ./artifacts
			$ circleci job artifacts 123 --download ./artifacts

			# Specify the project explicitly
			$ circleci job artifacts 123 --project gh/myorg/myrepo

			# Output as JSON
			$ circleci job artifacts 123 --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			err := cmdutil.RequireArgs(args, "job-number")
			if err != nil {
				return err
			}

			streams := iostream.FromCmd(cmd)
			jobNumber, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return clierrors.New("args.invalid_job_number", "Invalid job number",
					fmt.Sprintf("%q is not a valid job number.", args[0])).
					WithExitCode(clierrors.ExitBadArguments)
			}

			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}

			return runJobArtifacts(ctx, client, streams, jobNumber, projectSlug, downloadDir, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&downloadDir, "download", "d", "", "Download artifacts into this directory")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

func runJobArtifacts(ctx context.Context, client *apiclient.Client, streams iostream.Streams, jobNumber int64, projectSlug, downloadDir string, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return clierrors.New("git.detect_failed", "Could not detect project from git", err.Error()).
				WithSuggestions(
					"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
					"Or specify the project: circleci job artifacts <number> --project gh/org/repo",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		projectSlug = info.Slug
	}
	entries, err := artifacts.ForJob(ctx, client, projectSlug, jobNumber)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("artifacts.not_found", "Job not found",
				fmt.Sprintf("No job #%d found in project %q.", jobNumber, projectSlug)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return clierrors.New("api.error", "CircleCI API error", err.Error()).
			WithExitCode(clierrors.ExitAPIError)
	}

	if len(entries) == 0 {
		streams.ErrPrintln("No artifacts found.")
		return nil
	}

	if downloadDir != "" {
		sp := streams.Spinner(true, fmt.Sprintf("Downloading %d artifact(s) to %s", len(entries), downloadDir))
		dlErr := artifacts.Download(ctx, client, entries, downloadDir)
		sp.Stop()
		if dlErr != nil {
			return clierrors.New("artifacts.download_failed", "Download failed", dlErr.Error()).
				WithExitCode(clierrors.ExitGeneralError)
		}
		streams.ErrPrintf("%s Downloaded %d artifact(s)\n", streams.Symbol("✓", "OK:"), len(entries))
	}

	if jsonOut {
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	for _, e := range entries {
		streams.Println(e.Path)
	}
	return nil
}
