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

package runner

import (
	"context"
	"net/http"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/runnerinstall"
)

func newInitCmd() *cobra.Command {
	var (
		orgRef      string
		platform    string
		description string
		force       bool
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "init [<namespace>/<name>]",
		Short: "Set up a self-hosted runner, step by step",
		// Accepting the runner terms is irreversible from the CLI, and a resource
		// class and token are created, so MCP hosts should confirm first.
		Annotations: map[string]string{
			"destructiveHint": "true",
			"help:arguments": heredoc.Docf(`
				%[1]s<namespace>/<name>%[1]s is the resource class to create, in an existing namespace; prompted for if omitted.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Accept the runner terms if needed, create a resource class and token, then print
			install steps for your platform. Nothing is installed for you.
			JSON fields: org_id, terms_already_accepted, resource_class_id, resource_class,
			token_id, token, platform, steps, job_config, docs_url
		`),
		Example: heredoc.Doc(`
			# Walk through setup, answering prompts
			$ circleci runner init

			# Set up a Linux runner for an org
			$ circleci runner init my-org/linux-runner --platform debian --org gh/my-org

			# Capture the token and steps for automation
			$ circleci runner init my-org/ci --platform docker --force --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Everything that can fail without the API is settled first, so a bad
			// flag never leaves terms accepted or a token minted for nothing.
			target, err := resolveInitPlatform(ctx, platform, jsonOut)
			if err != nil {
				return err
			}
			resourceClass, err := resolveInitResourceClass(ctx, args, jsonOut)
			if err != nil {
				return err
			}

			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, orgRef, "circleci runner init")
			if err != nil {
				return err
			}
			label := orgLabel(orgRef, orgID, "circleci runner init")

			return runInit(ctx, client, initParams{
				orgID:         orgID,
				orgLabel:      label,
				resourceClass: resourceClass,
				description:   description,
				platform:      target,
				force:         force,
				jsonOut:       jsonOut,
			})
		},
	}

	cmdutil.AddOrgFlag(cmd, &orgRef, cmdutil.OrgFlag{DefaultsToGitRemote: true})
	cmd.Flags().StringVar(&platform, "platform", "",
		"where the agent will run: "+strings.Join(runnerinstall.Platforms(), "|")+" (prompts when unset)")
	cmd.Flags().StringVar(&description, "description", "", "description of the resource class")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "accept the runner terms without a confirmation prompt")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// resolveInitPlatform turns --platform into a platform, prompting in a terminal
// when it is unset. A --json run never prompts: its caller is a script.
func resolveInitPlatform(ctx context.Context, flag string, jsonOut bool) (runnerinstall.Platform, error) {
	if flag != "" {
		p, cliErr := runnerinstall.Parse(flag)
		if cliErr != nil {
			return "", cliErr
		}
		return p, nil
	}
	if jsonOut || !iostream.IsInteractive(ctx) {
		return "", clierrors.New("runner.platform_required", "Platform required",
			"Choose where the runner agent will be installed with --platform.").
			WithSuggestions("Use one of: " + strings.Join(runnerinstall.Platforms(), ", ")).
			WithExitCode(clierrors.ExitBadArguments)
	}

	idx, err := iostream.PromptSelect(ctx, "Where will the runner run?", runnerinstall.Labels())
	if err != nil {
		return "", err
	}
	if idx < 0 {
		return "", initCancelledErr()
	}
	return runnerinstall.Platform(runnerinstall.Platforms()[idx]), nil
}

// resolveInitResourceClass returns the namespace/name to create, prompting in a
// terminal when it was not given as an argument.
func resolveInitResourceClass(ctx context.Context, args []string, jsonOut bool) (string, error) {
	rc := ""
	switch {
	case len(args) > 0:
		rc = args[0]
	case jsonOut || !iostream.IsInteractive(ctx):
		return "", cmdutil.RequireArgs(args, "namespace/name")
	default:
		v, err := iostream.PromptText(ctx, "Resource class to create (namespace/name)", "my-org/my-runner")
		if err != nil {
			return "", err
		}
		rc = strings.TrimSpace(v)
	}

	ns, name, found := strings.Cut(rc, "/")
	if !found || ns == "" || name == "" || strings.Contains(name, "/") {
		return "", clierrors.New("runner.invalid_resource_class", "Invalid resource class",
			"The resource class must be given as namespace/name, got "+quote(rc)+".").
			WithSuggestions("Use the form namespace/name, for example my-org/my-runner").
			WithExitCode(clierrors.ExitBadArguments)
	}
	return rc, nil
}

func quote(s string) string { return `"` + s + `"` }

func initCancelledErr() *clierrors.CLIError {
	return clierrors.New("runner.init_cancelled", "Aborted", "Runner setup was cancelled.").
		WithExitCode(clierrors.ExitCancelled)
}

type initParams struct {
	orgID         uuid.UUID
	orgLabel      string
	resourceClass string
	description   string
	platform      runnerinstall.Platform
	force         bool
	jsonOut       bool
}

type initOutput struct {
	OrgID                string               `json:"org_id"`
	TermsAlreadyAccepted bool                 `json:"terms_already_accepted"`
	ResourceClassID      string               `json:"resource_class_id"`
	ResourceClass        string               `json:"resource_class"`
	TokenID              string               `json:"token_id"`
	Token                string               `json:"token"`
	Platform             string               `json:"platform"`
	Steps                []runnerinstall.Step `json:"steps"`
	JobConfig            string               `json:"job_config"`
	DocsURL              string               `json:"docs_url"`
}

func runInit(ctx context.Context, client *apiclient.Client, p initParams) error {
	alreadyAccepted, err := ensureRunnerTerms(ctx, client, p)
	if err != nil {
		return err
	}

	rc, err := client.CreateResourceClass(ctx, p.orgID, p.resourceClass, p.description)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return orgNotAccessibleErr(p.orgID)
		}
		if isAdminRequired(err) {
			return resourceClassCreateForbiddenErr(p.orgID, p.resourceClass)
		}
		return apiErr(err, p.resourceClass)
	}
	if !p.jsonOut {
		iostream.ErrPrintf(ctx, "%s Created resource class %s\n", iostream.SymbolOK(ctx), rc.ResourceClass)
	}

	// Reference the class by the ID the API returned, so a server-side
	// normalisation of the name cannot attach the token to the wrong class.
	rcID, err := uuid.Parse(rc.ID)
	if err != nil {
		return tokenGenerationErr(err, rc.ResourceClass)
	}
	tok, err := client.CreateRunnerTokenV3(ctx, rcID, defaultTokenNickname)
	if err != nil {
		return tokenGenerationErr(err, rc.ResourceClass)
	}
	if tok.Token == "" {
		return tokenValueMissingErr(rc.ResourceClass, tok.ID)
	}
	if !p.jsonOut {
		iostream.ErrPrintf(ctx, "%s Created a token for %s\n", iostream.SymbolOK(ctx), rc.ResourceClass)
	}

	guide, err := runnerinstall.Build(p.platform, runnerinstall.Options{ResourceClass: rc.ResourceClass})
	if err != nil {
		return err
	}

	out := initOutput{
		OrgID:                p.orgID.String(),
		TermsAlreadyAccepted: alreadyAccepted,
		ResourceClassID:      rc.ID,
		ResourceClass:        rc.ResourceClass,
		TokenID:              tok.ID,
		Token:                tok.Token,
		Platform:             string(guide.Platform),
		Steps:                guide.Steps,
		JobConfig:            guide.JobConfig,
		DocsURL:              guide.DocsURL,
	}
	if p.jsonOut {
		return iostream.PrintJSON(ctx, out)
	}
	printInitGuide(ctx, out, guide.Title)
	return nil
}

// ensureRunnerTerms makes sure the organization has accepted the runner terms,
// asking first when it has not, and reports whether it already had.
func ensureRunnerTerms(ctx context.Context, client *apiclient.Client, p initParams) (bool, error) {
	settings, err := client.GetOrgSettings(ctx, p.orgID)
	if err != nil {
		return false, termsErr(err, p.orgID, p.orgLabel)
	}
	if settings.RunnerTOSAccepted {
		if !p.jsonOut {
			iostream.ErrPrintf(ctx, "%s Runner terms already accepted for %s\n", iostream.SymbolOK(ctx), p.orgLabel)
		}
		return true, nil
	}
	if _, err := confirmAndAcceptTerms(ctx, client, p.orgID, p.orgLabel, p.force, p.jsonOut); err != nil {
		return false, err
	}
	if !p.jsonOut {
		iostream.ErrPrintf(ctx, "%s Accepted the self-hosted runner terms for %s\n", iostream.SymbolOK(ctx), p.orgLabel)
	}
	return false, nil
}

// printInitGuide writes the token and the numbered steps to stdout as plain
// text, so the commands copy cleanly and are never re-wrapped to the terminal.
func printInitGuide(ctx context.Context, out initOutput, title string) {
	iostream.Printf(ctx, "\nToken for %s (save this — it will not be shown again):\n%s\n", out.ResourceClass, out.Token)
	iostream.Printf(ctx, "\n%s\n", title)
	for i, s := range out.Steps {
		iostream.Printf(ctx, "\n%d. %s\n", i+1, s.Title)
		for _, c := range s.Commands {
			iostream.Printf(ctx, "   $ %s\n", strings.ReplaceAll(c, "\n", "\n     "))
		}
		if s.Note != "" {
			iostream.Printf(ctx, "   %s\n", s.Note)
		}
	}
	iostream.Printf(ctx, "\nThen run a job on it, in .circleci/config.yml:\n\n%s\n", out.JobConfig)
	iostream.Printf(ctx, "Docs: %s\n", out.DocsURL)
}
