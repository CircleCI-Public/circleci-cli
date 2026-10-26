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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
)

func newRunCmd() *cobra.Command {
	var (
		projectSlug  string
		definitionID string
		definition   string
		branch       string
		tag          string
		params       []string
		jsonOut      bool
	)

	cmd := &cobra.Command{
		Use:     "run",
		GroupID: "ci",
		Short:   "Trigger a new pipeline run",
		Long: heredoc.Doc(`
			Trigger a pipeline run. In a terminal a missing definition or --branch/--tag
			is prompted for; in non-interactive mode both are optional.

			A skipped pipeline (e.g. a [ci skip] commit message) exits 0 and prints why.

			JSON fields: id, state, number, created_at, triggered — or triggered, message when skipped.
		`),
		Example: heredoc.Doc(`
			# Trigger a pipeline interactively — pick definition and branch from menus
			$ circleci pipeline run --project gh/myorg/myrepo

			# Trigger a definition, selected by name, on a branch non-interactively
			$ circleci pipeline run --project gh/myorg/myrepo \
			    --definition main-pipeline --branch main

			# Trigger on a tag with parameters
			$ circleci pipeline run --project gh/myorg/myrepo \
			    --definition-id 2338d0ae-5541-4bbf-88a2-55e9f7281f80 \
			    --tag v1.2.3 \
			    --param deploy_env=staging

			# Output as JSON for scripting
			$ circleci pipeline run --project gh/myorg/myrepo \
			    --definition-id 2338d0ae-5541-4bbf-88a2-55e9f7281f80 \
			    --branch main --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runRun(ctx, client, projectSlug, definitionID, definition, branch, tag, params, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&definition, "definition", "", "Pipeline definition name to run (mutually exclusive with --definition-id)")
	cmd.Flags().StringVar(&definitionID, "definition-id", "", "Pipeline definition UUID to run (prompted interactively if omitted)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch for config fetch and checkout (mutually exclusive with --tag)")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag for config fetch and checkout (mutually exclusive with --branch)")
	cmd.Flags().StringArrayVar(&params, "param", nil, "Pipeline parameter as key=value (repeatable)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type runOutput struct {
	Triggered bool   `json:"triggered"`
	ID        string `json:"id,omitempty"`
	State     string `json:"state,omitempty"`
	Number    int    `json:"number,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Message   string `json:"message,omitempty"`
}

func runRun(ctx context.Context, client *apiclient.Client, projectSlug, definitionID, definition, branch, tag string, rawParams []string, jsonOut bool) error {
	if branch != "" && tag != "" {
		return clierrors.New("pipeline.run.invalid_args", "Invalid arguments",
			"--branch and --tag are mutually exclusive").
			WithExitCode(clierrors.ExitBadArguments)
	}

	// Checked by value, not with MarkFlagsMutuallyExclusive: cobra counts an
	// explicitly empty flag as set, which would reject MCP calls that fill every
	// flag with "".
	if definition != "" && definitionID != "" {
		return clierrors.New("pipeline.run.invalid_args", "Invalid arguments",
			"--definition and --definition-id are mutually exclusive").
			WithExitCode(clierrors.ExitBadArguments)
	}

	// Parse parameters before any API call so a malformed --param fails fast.
	parameters, err := cmdutil.ParseParams(rawParams)
	if err != nil {
		return clierrors.New("pipeline.run.invalid_param", "Invalid parameter",
			err.Error()).
			WithExitCode(clierrors.ExitBadArguments)
	}

	slug, err := cmdutil.ResolveProjectSlug(projectSlug)
	if err != nil {
		return err
	}

	// Detect current git branch for interactive selection and non-interactive
	// auto-use (mirrors the behaviour of `circleci run trigger`).
	var currentGitBranch string
	if branch == "" && tag == "" {
		if info, err := gitremote.Detect(); err == nil {
			currentGitBranch = info.Branch
		}
	}

	// Fetch project info once; reused for both definition and branch prompts.
	var projInfo *apiclient.ProjectInfo
	if definition != "" {
		// A name needs the project's UUID to list its definitions, so a failure
		// here is an error rather than a skipped prompt.
		projInfo, err = client.GetProjectInfo(ctx, slug)
		if err != nil {
			return cmdutil.APIErr(err, slug,
				"pipeline.run_failed",
				"Failed to look up project %q.",
				"Check that the project slug is correct and you have access to it",
				"Run 'circleci project list' to see followed projects")
		}
		definitionID, err = resolveDefinitionName(ctx, client, projInfo.ID, definition)
		if err != nil {
			return err
		}
	} else {
		if iostream.IsInteractive(ctx) {
			projInfo, _ = client.GetProjectInfo(ctx, slug) // best-effort; nil skips prompts
		}
		definitionID, err = resolveDefinitionID(ctx, client, projInfo, definitionID)
		if err != nil {
			return err
		}
	}

	branch, err = resolveBranch(ctx, projInfo, currentGitBranch, branch, tag)
	if err != nil {
		return err
	}

	input := apiclient.TriggerPipelineRunInput{
		DefinitionID:   definitionID,
		ConfigBranch:   branch,
		ConfigTag:      tag,
		CheckoutBranch: branch,
		CheckoutTag:    tag,
		Parameters:     parameters,
	}

	result, err := client.TriggerPipelineRun(ctx, slug, input)
	if err != nil {
		return cmdutil.APIErr(err, slug,
			"pipeline.run_failed",
			"Failed to trigger pipeline run for project %q.",
			"Check that the project slug is correct and you have permission to trigger pipelines",
			"Visit: https://app.circleci.com/settings/project")
	}

	out := runOutput{
		Triggered: result.Triggered,
		Message:   result.Message,
	}
	if result.Triggered {
		out.ID = result.ID
		out.State = result.State
		out.Number = result.Number
		out.CreatedAt = result.CreatedAt.Format(time.RFC3339)
		out.Branch = branch
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	if !result.Triggered {
		iostream.Println(ctx, fmt.Sprintf("Pipeline not triggered: %s", result.Message))
		return nil
	}

	msg := fmt.Sprintf("Pipeline #%d triggered  (id: %s, state: %s)", result.Number, result.ID, result.State)
	if branch != "" {
		msg += fmt.Sprintf(" on %s", branch)
	}
	iostream.Println(ctx, msg)
	return nil
}

// resolveDefinitionName returns the ID of the project's pipeline definition
// called name. An exact match wins; failing that, names are compared ignoring
// case and surrounding whitespace. A name that matches no definition, or more
// than one, fails with the available definitions listed.
func resolveDefinitionName(ctx context.Context, client *apiclient.Client, projectID, name string) (string, error) {
	defs, err := client.ListPipelineDefinitions(ctx, projectID)
	if err != nil {
		return "", cmdutil.APIErr(err, projectID,
			"pipeline_definition.list_failed",
			"Failed to list pipeline definitions for project %q.",
			"Check that you have access to the project",
			"Run 'circleci pipeline list' to see its definitions")
	}

	var matches []apiclient.PipelineDefinition
	for _, d := range defs {
		if d.Name == name {
			matches = append(matches, d)
		}
	}
	if len(matches) == 0 {
		want := strings.TrimSpace(name)
		for _, d := range defs {
			if strings.EqualFold(strings.TrimSpace(d.Name), want) {
				matches = append(matches, d)
			}
		}
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	}

	if len(matches) > 1 {
		return "", clierrors.New("pipeline.run.ambiguous_definition", "Ambiguous pipeline definition",
			fmt.Sprintf("%d pipeline definitions are named %q:\n%s", len(matches), name, definitionList(matches))).
			WithSuggestions("Pass --definition-id <uuid> to pick one").
			WithExitCode(clierrors.ExitBadArguments)
	}

	e := clierrors.New("pipeline.run.definition_not_found", "Pipeline definition not found",
		fmt.Sprintf("No pipeline definition is named %q.", name)).
		WithExitCode(clierrors.ExitNotFound)
	if len(defs) == 0 {
		e.Message = "This project has no pipeline definitions."
		return "", e.WithSuggestions("Create one with 'circleci pipeline create'")
	}
	e.Message += "\nAvailable definitions:\n" + definitionList(defs)
	return "", e.WithSuggestions("Pass one of these names to --definition")
}

// definitionList renders one "name  (id)" line per definition.
func definitionList(defs []apiclient.PipelineDefinition) string {
	lines := make([]string, len(defs))
	for i, d := range defs {
		lines[i] = fmt.Sprintf("  %s  (%s)", d.Name, d.ID)
	}
	return strings.Join(lines, "\n")
}

// resolveDefinitionID returns definitionID as-is if provided. In interactive
// mode it lists the project's pipeline definitions and prompts for a selection.
// In non-interactive mode it returns "" so the trigger proceeds without one.
func resolveDefinitionID(ctx context.Context, client *apiclient.Client, projInfo *apiclient.ProjectInfo, definitionID string) (string, error) {
	if definitionID != "" {
		return definitionID, nil
	}
	if !iostream.IsInteractive(ctx) || projInfo == nil {
		return "", nil
	}

	defs, err := client.ListPipelineDefinitions(ctx, projInfo.ID)
	if err != nil || len(defs) == 0 {
		return "", nil
	}

	const skipOption = "None (trigger without a specific definition)"
	labels := make([]string, len(defs)+1)
	for i, d := range defs {
		label := d.Name
		if d.ID != "" {
			label += "  (" + d.ID + ")"
		}
		labels[i] = label
	}
	labels[len(defs)] = skipOption

	idx, err := iostream.PromptSelect(ctx, "Select a pipeline definition", labels)
	if err != nil {
		return "", err
	}
	if idx < 0 || labels[idx] == skipOption {
		return "", nil
	}
	return defs[idx].ID, nil
}

// resolveBranch returns branch as-is if either branch or tag is already set.
// In non-interactive mode it returns currentGitBranch (detected from the git
// repo), mirroring `circleci run trigger` behaviour. In interactive mode it
// shows a selection list: current git branch first, then the project's default
// branch (if different), then "Other..." for free-text entry.
func resolveBranch(ctx context.Context, projInfo *apiclient.ProjectInfo, currentGitBranch, branch, tag string) (string, error) {
	if branch != "" || tag != "" {
		return branch, nil
	}
	if !iostream.IsInteractive(ctx) {
		return currentGitBranch, nil
	}

	var defaultBranch string
	if projInfo != nil && projInfo.VCSInfo != nil {
		defaultBranch = projInfo.VCSInfo.DefaultBranch
	}

	// Build deduplicated option list: current branch, then project default, then Other.
	seen := map[string]bool{}
	var options []string
	for _, b := range []string{currentGitBranch, defaultBranch} {
		if b != "" && !seen[b] {
			seen[b] = true
			options = append(options, b)
		}
	}
	const otherOption = "Other..."
	options = append(options, otherOption)

	idx, err := iostream.PromptSelect(ctx, "Branch to run on", options)
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", clierrors.New("run.cancelled", "Aborted", "No branch selected.").
			WithExitCode(clierrors.ExitCancelled)
	}
	if options[idx] == otherOption {
		v, err := iostream.PromptText(ctx, "Branch name", "e.g. my-feature-branch")
		if err != nil {
			return "", err
		}
		if v == "" {
			return "", clierrors.New("run.cancelled", "Aborted", "No branch entered.").
				WithExitCode(clierrors.ExitCancelled)
		}
		return v, nil
	}
	return options[idx], nil
}
