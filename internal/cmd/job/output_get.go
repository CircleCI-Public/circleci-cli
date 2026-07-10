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
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/termrender"
)

func newOutputGetCmd() *cobra.Command {
	var (
		execution int
		stepNum   int
		stripANSI bool
		condensed bool
	)

	cmd := &cobra.Command{
		Use:   "get <job-id>",
		Short: "Get the output of a job step",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<job-id>%[1]s is the UUID of the job whose step output to fetch. Job UUIDs
				are shown in the output of %[1]scircleci workflow get%[1]s and %[1]scircleci job get%[1]s.
				Use %[1]s--step-num%[1]s to select which step's output to read.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Fetch the raw stdout and stderr of a single step within a job and
			print it to the terminal.

			The job is identified by its UUID, shown in the output of
			'circleci workflow get' and 'circleci job get'. The step number
			(--step-num) selects which step's output to fetch.

			For parallel jobs, use --execution to choose which executor's output
			to read; it defaults to the first execution (index 0).

			Stdout and stderr are fetched in parallel and printed together,
			stdout first.

			By default, when writing to a terminal the raw output is passed
			through unchanged, and when redirected to a file or pipe it is
			rendered down to plain text: ANSI escapes are removed and progress
			redraws (carriage returns and cursor movement, e.g. Docker pulls)
			are collapsed to their final state.

			Use --strip-ansi to force this rendering even on a terminal, or
			--strip-ansi=false to pass the raw output through even when piped.

			Use --condensed to fetch a filtered version of stdout only, with
			noisy or repetitive lines removed server-side. This produces a much
			smaller payload suitable for feeding to an AI tool. When --condensed
			is set, output is always rendered through the plain-text renderer
			regardless of the terminal or --strip-ansi.

			Note: --condensed is experimental and subject to change.
		`),
		Example: heredoc.Doc(`
			# Get the output of step 3 in a job
			$ circleci job output get 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3

			# Get the output of step 3 from the second parallel execution
			$ circleci job output get 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3 --execution 1

			# Pipe the output to a file
			$ circleci job output get 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3 > step.log

			# Pipe condensed output to an AI CLI tool
			$ circleci job output get 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3 --condensed | llm "Why did this fail?"
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "job-id"); cliErr != nil {
				return cliErr
			}
			if !cmd.Flags().Changed("step-num") {
				return cmdutil.RequireFlag("step-num")
			}
			jobID, err := uuid.Parse(args[0])
			if err != nil {
				return clierrors.New("args.invalid_job_id", "Invalid job ID",
					fmt.Sprintf("%q is not a valid job UUID.", args[0])).
					WithExitCode(clierrors.ExitBadArguments)
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			// --strip-ansi is tri-state: when the user sets it explicitly we
			// honour that value; otherwise default to rendering whenever the
			// output is not going to a terminal.
			if condensed {
				return runOutputGetCondensed(ctx, client, jobID, execution, stepNum)
			}
			strip := !iostream.IsTerminal(ctx)
			if cmd.Flags().Changed("strip-ansi") {
				strip = stripANSI
			}
			return runOutputGet(ctx, client, jobID, execution, stepNum, strip)
		},
	}

	cmd.Flags().IntVar(&execution, "execution", 0, "Parallel execution index to read output from")
	cmd.Flags().IntVar(&stepNum, "step-num", 0, "Step number whose output to fetch (required)")
	cmd.Flags().BoolVar(&stripANSI, "strip-ansi", false, "Force (or with =false, disable) ANSI stripping; defaults to stripping only when not a terminal")
	cmd.Flags().BoolVar(&condensed, "condensed", false, "Fetch error-relevant lines only, filtered server-side (experimental)")

	return cmd
}

// OutputGet renders a job step's stdout/stderr exactly as "circleci job output
// get" does. It is exported so interactive callers (e.g. "circleci run get")
// can reuse the same output without duplicating the fetch/render logic.
func OutputGet(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution, stepNum int, strip bool) error {
	return runOutputGet(ctx, client, jobID, execution, stepNum, strip)
}

func runOutputGet(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution, stepNum int, strip bool) error {
	var stdout, stderr []byte

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		stdout, err = client.GetJobStdout(gctx, jobID, execution, stepNum)
		return err
	})
	g.Go(func() (err error) {
		stderr, err = client.GetJobStderr(gctx, jobID, execution, stepNum)
		return err
	})
	if err := g.Wait(); err != nil {
		subject := fmt.Sprintf("step %d of job %s", stepNum, jobID)
		return cmdutil.APIErr(err, subject, "job.output_not_found",
			"No output found for %s.")
	}

	out := iostream.Out(ctx)
	writeOutput(out, stdout, strip)
	writeOutput(out, stderr, strip)
	return nil
}

func writeOutput(out io.Writer, b []byte, strip bool) {
	if !strip {
		// Write the raw bytes directly to avoid a string copy.
		_, _ = out.Write(b)
		return
	}
	// Render the captured stream down to plain text: strip ANSI styling and
	// collapse cursor/carriage-return redraws (Docker pulls, progress bars) to
	// the final state a human would have seen. See internal/termrender.
	_ = termrender.Render(out, bytes.NewReader(b))
}

func runOutputGetCondensed(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution, stepNum int) error {
	condensed, err := client.GetJobStdoutCondensed(ctx, jobID, execution, stepNum)
	if err != nil {
		subject := fmt.Sprintf("step %d of job %s", stepNum, jobID)
		return cmdutil.APIErr(err, subject, "job.output_not_found",
			"No output found for %s.")
	}
	_ = termrender.Render(iostream.Out(ctx), bytes.NewReader(condensed))
	return nil
}
