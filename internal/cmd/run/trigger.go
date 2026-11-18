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

package run

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/run"
)

func newTriggerCmd() *cobra.Command {
	var (
		opts    run.TriggerOptions
		legacy  []string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "trigger",
		Short: "Trigger a new run of a pipeline",
		Long: heredoc.Doc(`
			Trigger a run of a project's pipeline; works for OAuth, GitHub App and GitLab
			projects. Same as 'circleci pipeline run'. A project with several pipelines
			needs --definition, which a terminal prompts for.

			JSON fields: ` + run.TriggerJSONFields + `
		`),
		Example: heredoc.Doc(`
			# Trigger a run on the current branch
			$ circleci run trigger

			# Trigger a named pipeline on a branch
			$ circleci run trigger --definition main-pipeline --branch main

			# Trigger with pipeline parameters
			$ circleci run trigger --param deploy_env=staging --param run_e2e=true

			# Trigger on a tag and output the run as JSON
			$ circleci run trigger --project gh/myorg/myrepo --tag v1.2.3 --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			opts.Params = append(opts.Params, legacy...)
			res, err := run.Trigger(ctx, client, opts)
			if err != nil {
				return err
			}
			return run.PrintTriggered(ctx, res, jsonOut)
		},
	}

	AddTriggerFlags(cmd, &opts)
	cmd.Flags().StringArrayVar(&legacy, "parameter", nil, "Pipeline parameter as key=value (repeatable)")
	_ = cmd.Flags().MarkDeprecated("parameter", "use --param instead")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// AddTriggerFlags registers the flags shared by `circleci run trigger` and
// `circleci pipeline run`, so the two commands accept the same input.
func AddTriggerFlags(cmd *cobra.Command, opts *run.TriggerOptions) {
	f := cmd.Flags()
	f.StringVar(&opts.ProjectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	f.StringVar(&opts.Definition, "definition", "", "Pipeline definition name; needed when the project has several")
	f.StringVar(&opts.DefinitionID, "definition-id", "", "Pipeline definition UUID, instead of --definition")
	f.StringVarP(&opts.Branch, "branch", "b", "", "Branch to run (defaults to the checkout's current branch)")
	f.StringVarP(&opts.Tag, "tag", "t", "", "Tag to run, instead of --branch")
	f.StringArrayVar(&opts.Params, "param", nil, "Pipeline parameter as key=value (repeatable)")
}
