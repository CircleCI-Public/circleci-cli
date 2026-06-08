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

// Package artifacts implements the "circleci artifacts" command.
package artifacts

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/artifacts"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// NewArtifactCmd returns the top-level "circleci artifact" command.
func NewArtifactCmd() *cobra.Command {
	var (
		downloadDir string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "artifact <job-id>",
		Short: "List or download job artifacts",
		Long: heredoc.Doc(`
			List or download artifacts produced by a CircleCI job.

			Pass the job UUID to list its artifacts. Use --download to save
			them to a local directory.

			JSON fields: path, url, node_index
		`),
		Example: heredoc.Doc(`
			# List artifacts for a job
			$ circleci artifact 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Download all artifacts into ./artifacts
			$ circleci artifact 5034460f-c7c4-4c43-9457-de07e2029e7b --download ./artifacts

			# Output as JSON for scripting
			$ circleci artifact 5034460f-c7c4-4c43-9457-de07e2029e7b --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			err := cmdutil.RequireArgs(args, "job-id")
			if err != nil {
				return err
			}

			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}

			jobID := args[0]
			entries, err := artifacts.ForJob(ctx, client, jobID)
			if err != nil {
				return cmdutil.APIErr(err, jobID, "artifacts.not_found", "No resource found for %q.")
			}

			if len(entries) == 0 {
				if !jsonOut {
					iostream.ErrPrintln(ctx, "No artifacts found.")
					return nil
				}
				entries = []artifacts.Entry{}
			}

			if downloadDir != "" {
				sp := iostream.Spinner(ctx, !jsonOut, fmt.Sprintf("Downloading %d artifact(s) to %s", len(entries), downloadDir))
				dlErr := artifacts.Download(ctx, client, entries, downloadDir)
				sp.Stop()
				if dlErr != nil {
					return clierrors.New("artifacts.download_failed", "Download failed", dlErr.Error()).
						WithExitCode(clierrors.ExitGeneralError)
				}
				iostream.ErrPrintf(ctx, "%s Downloaded %d artifact(s)\n", iostream.SymbolOK(ctx), len(entries))
			}

			if jsonOut {
				return iostream.PrintJSON(ctx, entries)
			}

			iostream.PrintMarkdown(ctx, artifacts.FormatMarkdown(entries))
			return nil
		},
	}

	cmd.Flags().StringVarP(&downloadDir, "download", "d", "", "Download artifacts into this directory")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}
