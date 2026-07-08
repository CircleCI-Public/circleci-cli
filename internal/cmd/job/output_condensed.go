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
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newOutputCondensedCmd() *cobra.Command {
	var (
		execution int
		stepNum   int
	)

	cmd := &cobra.Command{
		Use:   "condensed <job-id>",
		Short: "Get a step's stdout condensed to its most error-relevant lines",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				<job-id> is the UUID of the job whose step output to fetch. Job UUIDs
				are shown in the output of "circleci workflow get" and "circleci job get".
				Use --step-num to select which step's output to read. For parallel jobs,
				use --execution to choose which executor's output to read; it defaults to
				the first execution (index 0).
			`),
		},
		Long: heredoc.Doc(`
			Fetch a step's stdout filtered down to its most error-relevant lines —
			noisy or repetitive output (progress bars, download logs, passing test
			output) removed server-side. This is a much smaller payload than
			"circleci job output get", intended for feeding a failing step's
			output to an AI agent or other automation.

			The job is identified by its UUID, shown in the output of
			'circleci workflow get' and 'circleci job get'. The step number
			(--step-num) selects which step's output to fetch.

			For parallel jobs, use --execution to choose which executor's output
			to read; it defaults to the first execution (index 0).
		`),
		Example: heredoc.Doc(`
			# Get the condensed output of step 3 in a job
			$ circleci job output condensed 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3

			# Get the condensed output of step 3 from the second parallel execution
			$ circleci job output condensed 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3 --execution 1

			# Pipe the condensed output to an AI CLI tool
			$ circleci job output condensed 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3 | llm "Why did this fail?"
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

			condensed, err := client.GetJobStdoutCondensed(ctx, jobID, execution, stepNum)
			if err != nil {
				subject := fmt.Sprintf("step %d of job %s", stepNum, jobID)
				return cmdutil.APIErr(err, subject, "job.output_not_found",
					"No output found for %s.")
			}

			_, _ = iostream.Out(ctx).Write(condensed)
			return nil
		},
	}

	cmd.Flags().IntVar(&execution, "execution", 0, "Parallel execution index to read output from")
	cmd.Flags().IntVar(&stepNum, "step-num", 0, "Step number whose output to fetch (required)")

	return cmd
}
