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

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/browser"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newOpenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open <job-id>",
		Short: "Open job in browser",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<job-id>%[1]s is the UUID of the job to look up. Job UUIDs are shown in
				the output of %[1]scircleci workflow get%[1]s and %[1]scircleci run get --json%[1]s.
			`, "`"),
		},
		Example: heredoc.Doc(`
			# Open job by UUID
			$ circleci job open 8e50c384-0083-43d0-bc8f-93f0db589d6b
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "job-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runOpen(ctx, client, args[0])
		},
	}

	return cmd
}

func runOpen(ctx context.Context, client *apiclient.Client, idStr string) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return err
	}

	job, err := client.GetJobV3(ctx, id)
	if err != nil {
		return cmdutil.APIErr(err, id.String(), "job.not_found", "No job found for %q.")
	}

	appURL, err := cmdutil.AppURL(ctx)
	if err != nil {
		return err
	}

	u := cmdutil.JobURL(appURL, job.WorkflowID, id)

	return browser.OpenURLOrPrint(iostream.Err(ctx), u)
}
