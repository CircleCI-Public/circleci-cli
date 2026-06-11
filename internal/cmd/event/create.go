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

package event

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newCreateCmd() *cobra.Command {
	var (
		projectSlug  string
		definitionID string
		branch       string
		tag          string
		params       []string
		jsonOut      bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new event (start workflows)",
		Long: heredoc.Doc(`
			Create a new trigger event for a CircleCI project — fire its
			pipeline and start the workflows it defines.

			The project is resolved from --project if provided; otherwise the
			project slug is detected from the git remote.

			In a terminal, missing --definition-id prompts you to pick from the
			project's pipeline definitions, and missing --branch/--tag prompts you
			to choose the project's default branch or enter a custom one.
			In non-interactive mode (CI, agents, scripts) both are optional and
			the event is created on the current git branch.

			With a pipeline definition (--definition-id or interactive selection),
			the event is created through the pipeline definitions API. Without
			one, the project's pipeline is fired directly on the given branch.

			Use --branch or --tag to specify which revision to use. They are
			mutually exclusive. --branch sets both the config fetch branch and the
			checkout branch; --tag sets both the config fetch tag and the checkout
			tag. --tag requires a pipeline definition.

			Pass pipeline parameters with --param key=value (repeatable). With a
			definition, values are sent as strings; without one, values are parsed
			as booleans (true/false), integers, or strings.

			When the event is skipped (e.g. due to a [ci skip] commit message)
			the command exits 0 and prints the reason to stdout.

			JSON fields (created): id, state, number, created_at, branch, triggered
			JSON fields (skipped): triggered, message
		`),
		Example: heredoc.Doc(`
			# Create an event on the current branch
			$ circleci event create

			# Create an event interactively — pick definition and branch from menus
			$ circleci event create --project gh/myorg/myrepo

			# Fire a specific pipeline definition on a branch non-interactively
			$ circleci event create --project gh/myorg/myrepo \
			    --definition-id 2338d0ae-5541-4bbf-88a2-55e9f7281f80 \
			    --branch main

			# Fire on a tag with parameters
			$ circleci event create --project gh/myorg/myrepo \
			    --definition-id 2338d0ae-5541-4bbf-88a2-55e9f7281f80 \
			    --tag v1.2.3 \
			    --param deploy_env=staging

			# Output as JSON for scripting
			$ circleci event create --branch main --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runCreate(ctx, client, projectSlug, definitionID, branch, tag, params, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&definitionID, "definition-id", "", "Pipeline definition UUID to fire (prompted interactively if omitted)")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch for config fetch and checkout (mutually exclusive with --tag)")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag for config fetch and checkout (mutually exclusive with --branch)")
	cmd.Flags().StringArrayVar(&params, "param", nil, "Pipeline parameter as key=value (repeatable)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type createOutput struct {
	Triggered bool   `json:"triggered"`
	ID        string `json:"id,omitempty"`
	State     string `json:"state,omitempty"`
	Number    int    `json:"number,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Message   string `json:"message,omitempty"`
}

func runCreate(ctx context.Context, client *apiclient.Client, projectSlug, definitionID, branch, tag string, rawParams []string, jsonOut bool) error {
	if branch != "" && tag != "" {
		return clierrors.New("event.create.invalid_args", "Invalid arguments",
			"--branch and --tag are mutually exclusive").
			WithExitCode(clierrors.ExitBadArguments)
	}

	slug, err := cmdutil.ResolveProjectSlug(projectSlug)
	if err != nil {
		return err
	}

	// Detect current git branch for interactive selection and non-interactive
	// auto-use.
	var currentGitBranch string
	if branch == "" && tag == "" {
		if info, err := gitremote.Detect(); err == nil {
			currentGitBranch = info.Branch
		}
	}

	// Fetch project info once; reused for both definition and branch prompts.
	var projInfo *apiclient.ProjectInfo
	if iostream.IsInteractive(ctx) {
		projInfo, _ = client.GetProjectInfo(ctx, slug) // best-effort; nil skips prompts
	}

	definitionID, err = resolveDefinitionID(ctx, client, projInfo, definitionID)
	if err != nil {
		return err
	}

	branch, err = resolveBranch(ctx, projInfo, currentGitBranch, branch, tag)
	if err != nil {
		return err
	}

	if definitionID == "" {
		if tag != "" {
			return clierrors.New("event.create.invalid_args", "Invalid arguments",
				"--tag requires a pipeline definition (--definition-id)").
				WithExitCode(clierrors.ExitBadArguments)
		}
		return createLegacy(ctx, client, slug, branch, rawParams, jsonOut)
	}
	return createFromDefinition(ctx, client, slug, definitionID, branch, tag, rawParams, jsonOut)
}

// createFromDefinition fires a pipeline definition via the V2 pipeline
// definitions API. Parameter values are sent as strings.
func createFromDefinition(ctx context.Context, client *apiclient.Client, slug, definitionID, branch, tag string, rawParams []string, jsonOut bool) error {
	parameters, err := parseStringParams(rawParams)
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
			"event.create_failed",
			"Failed to create an event for project %q.",
			"Check that the project slug is correct and you have permission to trigger pipelines",
			"Visit: https://app.circleci.com/settings/project")
	}

	out := createOutput{
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
		iostream.Println(ctx, fmt.Sprintf("Event not created: %s", result.Message))
		return nil
	}

	msg := fmt.Sprintf("Event #%d created  (id: %s, state: %s)", result.Number, result.ID, result.State)
	if branch != "" {
		msg += fmt.Sprintf(" on %s", branch)
	}
	iostream.Println(ctx, msg)
	return nil
}

// createLegacy fires the project's pipeline directly on a branch via the
// legacy V2 trigger endpoint. Parameter values are coerced to booleans or
// integers where unambiguous, since this endpoint expects typed values.
func createLegacy(ctx context.Context, client *apiclient.Client, slug, branch string, rawParams []string, jsonOut bool) error {
	parsedParams, err := parseParams(rawParams)
	if err != nil {
		return clierrors.New("event.create.invalid_param", "Invalid pipeline parameter",
			err.Error()).
			WithSuggestions("Parameters must be in key=value form, e.g. --param deploy_env=staging").
			WithExitCode(clierrors.ExitBadArguments)
	}

	resp, err := client.TriggerPipeline(ctx, slug, branch, parsedParams)
	if err != nil {
		return apiErr(err, slug)
	}

	if jsonOut {
		out := createOutput{
			Triggered: true,
			ID:        resp.ID,
			State:     resp.State,
			Number:    int(resp.Number),
			CreatedAt: resp.CreatedAt.Format(time.RFC3339),
			Branch:    branch,
		}
		return iostream.PrintJSON(ctx, out)
	}

	msg := fmt.Sprintf("Event #%d created (%s)", resp.Number, resp.ID)
	if branch != "" {
		msg += " on " + branch
	}
	iostream.Println(ctx, msg)
	return nil
}

// resolveDefinitionID returns definitionID as-is if provided. In interactive
// mode it lists the project's pipeline definitions and prompts for a selection.
// In non-interactive mode it returns "" so the event is created without one.
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

	const skipOption = "None (fire without a specific definition)"
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
// repo). In interactive mode it shows a selection list: current git branch
// first, then the project's default branch (if different), then "Other..."
// for free-text entry.
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

	idx, err := iostream.PromptSelect(ctx, "Branch to fire on", options)
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", clierrors.New("event.create_aborted", "Aborted", "No branch selected.").
			WithExitCode(clierrors.ExitCancelled)
	}
	if options[idx] == otherOption {
		v, err := iostream.PromptText(ctx, "Branch name", "e.g. my-feature-branch")
		if err != nil {
			return "", err
		}
		if v == "" {
			return "", clierrors.New("event.create_aborted", "Aborted", "No branch entered.").
				WithExitCode(clierrors.ExitCancelled)
		}
		return v, nil
	}
	return options[idx], nil
}

// parseStringParams converts ["key=value", ...] into a map of string values
// for the pipeline definitions API.
func parseStringParams(raw []string) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]any, len(raw))
	for _, p := range raw {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, clierrors.New("event.create.invalid_param", "Invalid pipeline parameter",
				fmt.Sprintf("%q is not in key=value format", p)).
				WithExitCode(clierrors.ExitBadArguments)
		}
		out[k] = v
	}
	return out, nil
}

// parseParams converts ["key=value", ...] into a map, coercing values to bool
// or int where unambiguous.
func parseParams(params []string) (map[string]any, error) {
	if len(params) == 0 {
		return nil, nil
	}
	result := make(map[string]any, len(params))
	for _, p := range params {
		k, v, found := strings.Cut(p, "=")
		if !found || k == "" {
			return nil, fmt.Errorf("%q is not valid: expected key=value", p)
		}
		switch v {
		case "true":
			result[k] = true
		case "false":
			result[k] = false
		default:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				result[k] = n
			} else {
				result[k] = v
			}
		}
	}
	return result, nil
}
