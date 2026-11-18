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

package pipeline

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	cmdrun "github.com/CircleCI-Public/circleci-cli/internal/cmd/run"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/run"
)

func newRunCmd() *cobra.Command {
	var (
		opts    run.TriggerOptions
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "run",
		GroupID: "ci",
		Short:   "Trigger a new run of a pipeline",
		Long: heredoc.Doc(`
			Trigger a run of a project's pipeline; works for OAuth, GitHub App and GitLab
			projects. Same as 'circleci run trigger'. A project with several pipelines
			needs --definition, which a terminal prompts for.

			JSON fields: ` + run.TriggerJSONFields + `
		`),
		Example: heredoc.Doc(`
			# Trigger a run on the current branch, picking the pipeline from a menu
			$ circleci pipeline run

			# Trigger a pipeline, selected by name, on a branch
			$ circleci pipeline run --project gh/myorg/myrepo \
			    --definition main-pipeline --branch main

			# Trigger on a tag with parameters
			$ circleci pipeline run --project gh/myorg/myrepo \
			    --definition-id 2338d0ae-5541-4bbf-88a2-55e9f7281f80 \
			    --tag v1.2.3 --param deploy_env=staging

			# Output as JSON for scripting
			$ circleci pipeline run --definition main-pipeline --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			res, err := run.Trigger(ctx, client, opts)
			if err != nil {
				return err
			}
			return run.PrintTriggered(ctx, res, jsonOut)
		},
	}

	cmdrun.AddTriggerFlags(cmd, &opts)
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}
