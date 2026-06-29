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

package workflow

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/browser"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newOpenCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "open <workflow-id>",
		Short: "Open workflow in browser",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				<workflow-id> is the UUID of the workflow to look up. Workflow IDs are
				shown in the output of "circleci run get".
			`),
		},
		Example: heredoc.Doc(`
			# Open workflow details
			$ circleci workflow open 5034460f-c7c4-4c43-9457-de07e2029e7b
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "workflow-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			return runOpen(ctx, args[0])
		},
	}

	return cmd
}

func runOpen(ctx context.Context, idStr string) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return err
	}

	appURL, err := cmdutil.AppURL(ctx)
	if err != nil {
		return err
	}

	u := cmdutil.WorkflowURL(appURL, id)

	return browser.OpenURLOrPrint(iostream.Err(ctx), u)
}
