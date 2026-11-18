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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
)

// TriggerOptions selects what Trigger fires. Every field is optional.
type TriggerOptions struct {
	// ProjectSlug names the project; empty resolves it from the git remote.
	ProjectSlug string
	// Definition and DefinitionID pick the pipeline by name or by UUID. With
	// neither, a project with one pipeline uses it and a project with several
	// prompts for one in a terminal.
	Definition   string
	DefinitionID string
	// Branch and Tag pick the ref, and are mutually exclusive. With neither, the
	// current git branch is used when the project is the checkout's own; failing
	// that, a terminal prompts for a branch and anything else is an error, since
	// the runs API requires a ref and has no default branch to fall back on.
	Branch string
	Tag    string
	// Params are pipeline parameters in key=value form.
	Params []string
}

// TriggerResult describes the run Trigger fired.
type TriggerResult struct {
	ID          uuid.UUID
	PipelineID  string
	ProjectID   uuid.UUID
	ProjectSlug string
	CreatedAt   time.Time
	Branch      string
	Tag         string
}

// Trigger fires a run of one of a project's pipelines via the V3 runs API. The
// project and pipeline are resolved through V3 too, and the pipelines listing
// includes a project's synthetic OAuth pipeline, so OAuth, GitHub App and
// GitLab projects all take the same path.
func Trigger(ctx context.Context, client *apiclient.Client, opts TriggerOptions) (*TriggerResult, error) {
	// Checked by value, not with MarkFlagsMutuallyExclusive: cobra counts an
	// explicitly empty flag as set, which would reject MCP calls that fill every
	// flag with "".
	if opts.Branch != "" && opts.Tag != "" {
		return nil, invalidArgs("--branch and --tag are mutually exclusive")
	}
	if opts.Definition != "" && opts.DefinitionID != "" {
		return nil, invalidArgs("--definition and --definition-id are mutually exclusive")
	}
	params, err := cmdutil.ParseParams(opts.Params)
	if err != nil {
		return nil, clierrors.New("run.trigger.invalid_param", "Invalid pipeline parameter", err.Error()).
			WithSuggestions("Parameters must be in key=value form, e.g. --param deploy_env=staging").
			WithExitCode(clierrors.ExitBadArguments)
	}

	// The checkout is read even when --project is given, so its branch can be
	// used when --project names this same project.
	git, gitErr := gitremote.Detect()
	slug := opts.ProjectSlug
	if slug == "" {
		if gitErr != nil {
			return nil, cmdutil.GitDetectErr(gitErr, "Or specify the project with --project gh/org/repo")
		}
		slug = git.Slug
	}

	// Resolved before any API call so a missing ref fails fast.
	branch, err := resolveRef(ctx, opts, git, gitErr, slug)
	if err != nil {
		return nil, err
	}

	project, err := client.GetProjectBySlug(ctx, slug)
	if err != nil {
		return nil, cmdutil.APIErr(err, slug, "project.not_found", "No project found for %q.",
			"Check the project slug and that you have access to it",
			"Run 'circleci project list' to see followed projects")
	}

	pipelineID, err := resolvePipeline(ctx, client, project.ID, opts.Definition, opts.DefinitionID)
	if err != nil {
		return nil, err
	}

	run, err := client.CreateRunV3(ctx, apiclient.CreateRunInput{
		ProjectID:  project.ID,
		PipelineID: pipelineID,
		Branch:     branch,
		Tag:        opts.Tag,
		Parameters: params,
	})
	if err != nil {
		return nil, cmdutil.APIErr(err, slug, "run.trigger.not_found",
			"The pipeline or project for %q was not found.",
			"Run 'circleci pipeline list' to see the project's pipelines")
	}

	return &TriggerResult{
		ID:          run.ID,
		PipelineID:  run.PipelineID,
		ProjectID:   project.ID,
		ProjectSlug: slug,
		CreatedAt:   run.CreatedAt,
		Branch:      branch,
		Tag:         opts.Tag,
	}, nil
}

// resolveRef returns the branch to run, or "" when opts.Tag is set. The runs
// API rejects a run with no ref, so when neither flag is given and the checkout
// is not on a branch of this project, a terminal prompts for one and anything
// else fails with the flags that would have supplied it.
func resolveRef(ctx context.Context, opts TriggerOptions, git *gitremote.ProjectInfo, gitErr error, slug string) (string, error) {
	if opts.Branch != "" || opts.Tag != "" {
		return opts.Branch, nil
	}
	if gitErr == nil && git.Slug == slug && git.Branch != "" {
		return git.Branch, nil
	}
	if !iostream.IsInteractive(ctx) {
		return "", clierrors.New("run.trigger.ref_required", "Branch or tag required",
			fmt.Sprintf("No branch to run for %q: this directory is not a checkout of it on a branch.", slug)).
			WithSuggestions("Pass --branch <name> or --tag <name>").
			WithExitCode(clierrors.ExitBadArguments)
	}
	branch, err := iostream.PromptText(ctx, "Branch to run", "e.g. main")
	if err != nil {
		return "", err
	}
	if branch = strings.TrimSpace(branch); branch == "" {
		return "", clierrors.New("run.cancelled", "Aborted", "No branch entered.").
			WithExitCode(clierrors.ExitCancelled)
	}
	return branch, nil
}

func invalidArgs(msg string) error {
	return clierrors.New("run.trigger.invalid_args", "Invalid arguments", msg).
		WithExitCode(clierrors.ExitBadArguments)
}

// resolvePipeline returns the UUID of the pipeline to run. An explicit UUID is
// used as given. Otherwise the project's pipelines are listed and one is picked
// by name, by being the only one, or by a prompt in a terminal.
func resolvePipeline(ctx context.Context, client *apiclient.Client, projectID uuid.UUID, name, id string) (string, error) {
	if id != "" {
		return id, nil
	}

	defs, err := client.ListPipelineDefinitions(ctx, projectID.String())
	if err != nil {
		return "", cmdutil.APIErr(err, projectID.String(),
			"pipeline_definition.list_failed",
			"Failed to list pipeline definitions for project %q.",
			"Check that you have access to the project")
	}
	if len(defs) == 0 {
		return "", clierrors.New("run.trigger.no_definitions", "No pipeline definitions",
			"This project has no pipeline definitions.").
			WithSuggestions("Create one with 'circleci pipeline create'").
			WithExitCode(clierrors.ExitNotFound)
	}

	if name != "" {
		return pipelineByName(defs, name)
	}
	if len(defs) == 1 {
		return defs[0].ID, nil
	}

	if !iostream.IsInteractive(ctx) {
		return "", clierrors.New("run.trigger.definition_required", "Pipeline definition required",
			fmt.Sprintf("This project has %d pipeline definitions:\n%s", len(defs), definitionList(defs))).
			WithSuggestions("Pass one of these names to --definition, or a UUID to --definition-id").
			WithExitCode(clierrors.ExitBadArguments)
	}
	labels := make([]string, len(defs))
	for i, d := range defs {
		labels[i] = fmt.Sprintf("%s  (%s)", d.Name, d.ID)
	}
	idx, err := iostream.PromptSelect(ctx, "Select a pipeline definition", labels)
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", clierrors.New("run.cancelled", "Aborted", "No pipeline definition selected.").
			WithExitCode(clierrors.ExitCancelled)
	}
	return defs[idx].ID, nil
}

// pipelineByName returns the ID of the definition called name. An exact match
// wins; failing that, names are compared ignoring case and surrounding
// whitespace. A name that matches none, or more than one, fails with the
// available definitions listed.
func pipelineByName(defs []apiclient.PipelineDefinition, name string) (string, error) {
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
	switch {
	case len(matches) == 1:
		return matches[0].ID, nil
	case len(matches) > 1:
		return "", clierrors.New("run.trigger.ambiguous_definition", "Ambiguous pipeline definition",
			fmt.Sprintf("%d pipeline definitions are named %q:\n%s", len(matches), name, definitionList(matches))).
			WithSuggestions("Pass --definition-id <uuid> to pick one").
			WithExitCode(clierrors.ExitBadArguments)
	}
	return "", clierrors.New("run.trigger.definition_not_found", "Pipeline definition not found",
		fmt.Sprintf("No pipeline definition is named %q.\nAvailable definitions:\n%s", name, definitionList(defs))).
		WithSuggestions("Pass one of these names to --definition").
		WithExitCode(clierrors.ExitNotFound)
}

// definitionList renders one "name  (id)" line per definition.
func definitionList(defs []apiclient.PipelineDefinition) string {
	lines := make([]string, len(defs))
	for i, d := range defs {
		lines[i] = fmt.Sprintf("  %s  (%s)", d.Name, d.ID)
	}
	return strings.Join(lines, "\n")
}

// triggerJSON is the --json shape of a triggered run, shared by
// `circleci run trigger` and `circleci pipeline run`.
type triggerJSON struct {
	ID         string `json:"id"`
	PipelineID string `json:"pipeline_id"`
	ProjectID  string `json:"project_id"`
	CreatedAt  string `json:"created_at,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Tag        string `json:"tag,omitempty"`
}

// TriggerJSONFields lists the --json fields PrintTriggered emits, for help text.
const TriggerJSONFields = "id, pipeline_id, project_id, created_at, branch, tag"

// PrintTriggered reports a triggered run: the JSON document on stdout with
// jsonOut, otherwise a one-line summary on stdout and the command to follow the
// run on stderr.
func PrintTriggered(ctx context.Context, res *TriggerResult, jsonOut bool) error {
	if jsonOut {
		return iostream.PrintJSON(ctx, triggerJSON{
			ID:         res.ID.String(),
			PipelineID: res.PipelineID,
			ProjectID:  res.ProjectID.String(),
			CreatedAt:  cmdutil.JSONTime(res.CreatedAt),
			Branch:     res.Branch,
			Tag:        res.Tag,
		})
	}

	msg := fmt.Sprintf("Triggered run %s for %s", res.ID, res.ProjectSlug)
	if res.Tag != "" {
		msg += " at tag " + res.Tag
	} else {
		msg += " on " + res.Branch
	}
	iostream.Println(ctx, msg)
	iostream.ErrPrintf(ctx, "Watch it with: circleci run watch %s\n", res.ID)
	return nil
}
