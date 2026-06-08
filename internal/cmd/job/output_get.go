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
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newOutputGetCmd() *cobra.Command {
	var (
		execution int
		stepNum   int
	)

	cmd := &cobra.Command{
		Use:   "get <job-id>",
		Short: "Get the output of a job step",
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
		`),
		Example: heredoc.Doc(`
			# Get the output of step 3 in a job
			$ circleci job output get 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3

			# Get the output of step 3 from the second parallel execution
			$ circleci job output get 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3 --execution 1

			# Pipe the output to a file
			$ circleci job output get 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3 > step.log
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
			return runOutputGet(ctx, client, jobID, execution, stepNum)
		},
	}

	cmd.Flags().IntVar(&execution, "execution", 0, "Parallel execution index to read output from")
	cmd.Flags().IntVar(&stepNum, "step-num", 0, "Step number whose output to fetch (required)")

	return cmd
}

func runOutputGet(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution, stepNum int) error {
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
	_, _ = out.Write(stdout)
	_, _ = out.Write(stderr)
	return nil
}
