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

// Package onboarder orchestrates the local onboarding flow.
package onboarder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/cmdauth"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/configgen"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/githubapp"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/org"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
	"github.com/CircleCI-Public/circleci-cli/internal/testrunner"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

type mode int

const (
	modeScan   mode = iota // scan repo, run tests, generate config, then sign up
	modeSignup             // sign up only — no repo required
)

// Options configures the onboarding flow.
type Options struct {
	ConfigPath    string
	NoBrowser     bool
	SecureStorage bool
	Scan          bool
	Signup        bool
	// RepoID is the VCS repository external ID (the numeric GitHub repo ID) used
	// to wire up the first pipeline definition and trigger. When empty, it is
	// prompted for interactively; in non-interactive mode an empty value falls
	// back to manual guidance.
	RepoID string
}

// Run scans a repository, verifies its tests, generates a starter config when
// needed, and ensures the CLI has an authenticated CircleCI session.
func Run(ctx context.Context, dir string, opts Options) error {
	m, err := resolveMode(ctx, opts)
	if err != nil {
		return err
	}

	trackOnboard(ctx, "onboard_mode_selected", map[string]any{
		"mode": modeString(m),
	})

	if m == modeSignup {
		result, err := cmdauth.SignupIfNeeded(ctx, opts.NoBrowser, opts.SecureStorage, opts.ConfigPath)
		if err != nil {
			return err
		}
		trackOnboard(ctx, "onboard_signup", map[string]any{
			"mode":    "signup",
			"outcome": string(result.Outcome),
		})
		return postSignupGuidance(ctx, opts)
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return clierrors.New(
			"onboard.resolve_path",
			"Cannot resolve directory",
			fmt.Sprintf("Could not resolve %q to an absolute path: %s.", dir, err),
		).WithExitCode(clierrors.ExitGeneralError)
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return clierrors.New(
			"onboard.path_not_found",
			"Path not found",
			fmt.Sprintf("No directory exists at %q.", dir),
		).WithSuggestions(
			"Check the path you passed and try again",
			"Omit the argument to scan the current directory",
		).WithExitCode(clierrors.ExitBadArguments)
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return clierrors.New(
			"onboard.not_a_git_repo",
			"Not a git repository",
			fmt.Sprintf("No git repository found at %q.", dir),
		).WithSuggestions(
			"Run 'git init' in the directory, then re-run 'circleci onboard'",
			"cd to a directory containing a git repository and re-run",
		).WithExitCode(clierrors.ExitBadArguments)
	}

	if err := displayPreamble(ctx, dir); err != nil {
		return err
	}

	result, err := reposcan.NewDefaultScanner().Scan(ctx, dir)
	if err != nil {
		return clierrors.New(
			"onboard.scan_failed",
			"Repository scan failed",
			fmt.Sprintf("Could not detect the project stack: %s.", err),
		).WithSuggestions(
			"Re-run with --debug to see scan details",
			"Try again; image resolution requires network access",
		).WithExitCode(clierrors.ExitGeneralError)
	}

	if !result.IsEmpty() {
		reposcan.Render(ctx, result)
	}

	if err := testrunner.Run(ctx, dir, result); err != nil {
		return err
	}

	configPath := filepath.Join(dir, ".circleci", "config.yml")
	if _, err := os.Stat(configPath); err == nil {
		iostream.Printf(ctx, "%s Using existing config at %s\n",
			iostream.SymbolOK(ctx), configPath)
	} else if err := configgen.Generate(ctx, dir, result); err != nil {
		return err
	}

	signupResult, err := cmdauth.SignupIfNeeded(ctx, opts.NoBrowser, opts.SecureStorage, opts.ConfigPath)
	if err != nil {
		return err
	}
	trackOnboard(ctx, "onboard_signup", map[string]any{
		"mode":    "scan",
		"outcome": string(signupResult.Outcome),
	})

	return postSignupGuidance(ctx, opts)
}

// postSignupGuidance offers inline project creation and prints a follow-up
// message after the user has authenticated. For modern (CircleCI-native) orgs
// it continues past project creation to set up the first pipeline definition
// and trigger, so a subsequent push starts a build.
//
// Errors are handled gracefully: any failure falls through to manual guidance
// rather than failing the onboard command.
func postSignupGuidance(ctx context.Context, opts Options) error {
	ctx = refreshConfig(ctx, opts)

	client, err := cmdutil.LoadClient(ctx)
	if err != nil {
		printManualGuidance(ctx)
		return nil
	}

	appURL, _ := cmdutil.AppURL(ctx)

	orgs, err := org.Require(ctx, client)
	if err != nil {
		printManualGuidance(ctx)
		return nil
	}

	var selectedOrg apiclient.Collaboration
	switch {
	case len(orgs) == 1:
		selectedOrg = orgs[0]
		iostream.Printf(ctx, "%s Using organization %s\n", iostream.SymbolOK(ctx), selectedOrg.Slug)
	case iostream.IsInteractive(ctx):
		iostream.ErrPrintf(ctx, "\nLet's create your CircleCI project.\n\n")
		labels := make([]string, len(orgs))
		for i, c := range orgs {
			labels[i] = c.Slug
			if c.Name != "" && c.Name != c.Slug {
				labels[i] = fmt.Sprintf("%s (%s)", c.Slug, c.Name)
			}
		}
		idx, err := iostream.PromptSelect(ctx,
			"Which organization should this project belong to?", labels)
		if err != nil || idx < 0 {
			printManualGuidance(ctx)
			return nil
		}
		selectedOrg = orgs[idx]
	default:
		printManualGuidance(ctx)
		return nil
	}

	vcs, orgName, err := org.ParseSlug(selectedOrg.Slug)
	if err != nil {
		printManualGuidance(ctx)
		return nil
	}

	defaultName := gitremote.DetectRepoName()
	var name string
	if iostream.IsInteractive(ctx) {
		name, err = iostream.PromptText(ctx, "Project name", defaultName)
		if err != nil {
			printManualGuidance(ctx)
			return nil
		}
		if name == "" {
			name = defaultName
		}
		if name == "" {
			printManualGuidance(ctx)
			return nil
		}
	} else {
		name = defaultName
		if name == "" {
			printManualGuidance(ctx)
			return nil
		}
	}

	if selectedOrg.VCSType != "circleci" {
		return followClassicProject(ctx, client, appURL, vcs, orgName, name)
	}

	proj, created, err := createOrResolveProject(ctx, client, vcs, orgName, name)
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not create project: %s\n", iostream.SymbolWarn(ctx), err)
		printManualGuidance(ctx)
		return nil
	}

	if created {
		iostream.Printf(ctx, "%s Project created: %s\n", iostream.SymbolOK(ctx), proj.Name)
	} else {
		iostream.Printf(ctx, "%s Using existing project: %s\n", iostream.SymbolOK(ctx), proj.Name)
	}
	iostream.Printf(ctx, "  Organization: %s\n", proj.OrganizationName)
	if pipelinesURL, err := cmdutil.RunSlugURL(appURL, proj.Slug); err == nil {
		iostream.Printf(ctx, "  Pipelines: %s\n", pipelinesURL)
	}

	return setupFirstPipeline(ctx, client, appURL, proj, name, repoFullName(), opts.RepoID, opts.NoBrowser)
}

// refreshConfig re-reads the config from disk when the cached copy carries no
// token.
//
// A fresh signup persists the token to the keyring (or the config file), but the
// *config.Config in ctx was loaded during root's bootstrap — before that write —
// so it still looks unauthenticated. Without this reload, LoadClient fails with
// auth.token_missing on the very run that just signed the user up, and the whole
// project setup degrades to manual guidance.
//
// A reload failure is not fatal: returning the unchanged ctx degrades exactly as
// it would have anyway.
func refreshConfig(ctx context.Context, opts Options) context.Context {
	if cmdutil.GetConfig(ctx).EffectiveToken() != "" {
		return ctx
	}
	cfg, err := config.Load(ctx, opts.ConfigPath, opts.SecureStorage)
	if err != nil {
		return ctx
	}
	return cmdutil.WithConfig(ctx, cfg)
}

// createOrResolveProject creates the project, or resolves the existing one when
// creation fails because it was already created by a previous onboard run. The
// returned bool reports whether the project was newly created. Only the original
// creation error is surfaced when the project genuinely does not exist.
func createOrResolveProject(ctx context.Context, client *apiclient.Client, vcs, orgName, name string) (*apiclient.ProjectInfo, bool, error) {
	proj, err := client.CreateProject(ctx, vcs, orgName, name)
	if err == nil {
		return proj, true, nil
	}

	slug := fmt.Sprintf("%s/%s/%s", vcs, orgName, name)
	if existing, rerr := client.GetProjectInfo(ctx, slug); rerr == nil {
		return existing, false, nil
	}
	return nil, false, err
}

// repoFullName returns "owner/repo" from the git remote URL, or "" when not detectable.
func repoFullName() string {
	info, err := gitremote.DetectFromRemote()
	if err != nil {
		return ""
	}
	parts := strings.Split(info.Slug, "/")
	if len(parts) != 3 {
		return ""
	}
	return parts[1] + "/" + parts[2]
}

// setupFirstPipeline wires up a pipeline definition and an all-pushes trigger
// for a freshly created project so the first push starts a build. It uses the
// same v2 endpoints as `circleci pipeline create` and `circleci project trigger
// create`, with sensible zero-prompt defaults (config at .circleci/config.yml,
// the github_app provider, and the repo's external ID for both config and
// checkout sources).
//
// Every step degrades gracefully: the project already exists, so any failure
// prints manual guidance rather than returning an error.
func setupFirstPipeline(ctx context.Context, client *apiclient.Client, appURL string, proj *apiclient.ProjectInfo, repoName, fullName, repoID string, noBrowser bool) error {
	repoID = resolveRepoID(ctx, client, appURL, proj, fullName, repoID, noBrowser)
	if repoID == "" {
		// Without the repo's external ID we can't create the pipeline
		// definition. The project still exists; guide the user to finish setup.
		trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "skipped_no_repo_id"})
		printManualPipelineGuidance(ctx)
		return nil
	}

	def, err := ensurePipelineDefinition(ctx, client, proj.ID, repoName, repoID)
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not create pipeline definition: %s\n", iostream.SymbolWarn(ctx), err)
		trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "pipeline_definition_failed"})
		printManualPipelineGuidance(ctx)
		return nil
	}

	if err := ensureTrigger(ctx, client, proj.ID, def.ID, repoID); err != nil {
		iostream.ErrPrintf(ctx, "%s Could not create trigger: %s\n", iostream.SymbolWarn(ctx), err)
		trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "trigger_failed"})
		printManualPipelineGuidance(ctx)
		return nil
	}

	trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "created"})
	printPipelineReadyGuidance(ctx, appURL, proj.Slug)
	return nil
}

// ensurePipelineDefinition returns the pipeline definition already configured
// for the repo, or creates one. Reusing an existing definition keeps re-runs of
// onboard from creating duplicates.
func ensurePipelineDefinition(ctx context.Context, client *apiclient.Client, projectID, name, repoID string) (*apiclient.PipelineDefinition, error) {
	if defs, err := client.ListPipelineDefinitions(ctx, projectID); err == nil {
		for i := range defs {
			cs := defs[i].ConfigSource
			if cs != nil && cs.Repo != nil && cs.Repo.ExternalID == repoID {
				iostream.Printf(ctx, "%s Pipeline definition already exists: %s\n", iostream.SymbolOK(ctx), defs[i].Name)
				return &defs[i], nil
			}
		}
	}

	def, err := client.CreatePipelineDefinition(ctx, projectID, apiclient.CreatePipelineDefinitionInput{
		Name:             name,
		ConfigProvider:   "github_app",
		ConfigRepoID:     repoID,
		ConfigFilePath:   ".circleci/config.yml",
		CheckoutProvider: "github_app",
		CheckoutRepoID:   repoID,
	})
	if err != nil {
		return nil, err
	}
	iostream.Printf(ctx, "%s Pipeline definition created: %s\n", iostream.SymbolOK(ctx), def.Name)
	return def, nil
}

// ensureTrigger creates an all-pushes trigger for the pipeline definition unless
// one already exists, so re-runs of onboard don't add duplicate triggers.
func ensureTrigger(ctx context.Context, client *apiclient.Client, projectID, definitionID, repoID string) error {
	if trigs, err := client.ListTriggers(ctx, projectID, definitionID); err == nil && len(trigs) > 0 {
		iostream.Printf(ctx, "%s Trigger already exists\n", iostream.SymbolOK(ctx))
		return nil
	}

	trig, err := client.CreateTrigger(ctx, projectID, definitionID, "github_app", repoID, "all-pushes", "", "")
	if err != nil {
		return err
	}
	preset := trig.EventPreset
	if preset == "" {
		preset = "all-pushes"
	}
	iostream.Printf(ctx, "%s Trigger created: %s\n", iostream.SymbolOK(ctx), preset)
	return nil
}

// resolveRepoID determines the repo external ID for the pipeline definition and
// trigger. An explicit --repo-id wins. Otherwise it ensures the CircleCI GitHub
// App is installed for the org and matches the git remote against the app's
// accessible repositories. Any failure returns "" so the caller degrades to
// manual guidance rather than prompting for an opaque numeric ID.
func resolveRepoID(ctx context.Context, client *apiclient.Client, appURL string, proj *apiclient.ProjectInfo, fullName, repoID string, noBrowser bool) string {
	if repoID != "" {
		return repoID
	}
	if proj.OrganizationID == "" || fullName == "" {
		return ""
	}

	returnURL := appURL
	if u, err := cmdutil.RunSlugURL(appURL, proj.Slug); err == nil {
		returnURL = u
	}

	installed, err := githubapp.EnsureInstalled(ctx, client, proj.OrganizationID, returnURL, noBrowser)
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not check GitHub App installation: %s\n", iostream.SymbolWarn(ctx), err)
		return ""
	}
	if !installed {
		return ""
	}

	id, err := githubapp.ResolveRepoID(ctx, client, proj.OrganizationID, fullName)
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not list GitHub App repositories: %s\n", iostream.SymbolWarn(ctx), err)
		return ""
	}
	if id == "" {
		iostream.ErrPrintf(ctx, "%s The GitHub App can't access %s yet. Grant it access to this repository, then re-run.\n",
			iostream.SymbolWarn(ctx), fullName)
		return ""
	}

	iostream.Printf(ctx, "%s Found repository %s\n", iostream.SymbolOK(ctx), fullName)
	return id
}

// printPipelineReadyGuidance prints the happy-path next steps once the pipeline
// definition and trigger have been created.
func printPipelineReadyGuidance(ctx context.Context, appURL, slug string) {
	iostream.Printf(ctx, "\nYour project is ready! Next steps:\n")
	iostream.Printf(ctx, "  1. git add .circleci/config.yml\n")
	iostream.Printf(ctx, "  2. git commit -m \"Add CircleCI config\"\n")
	iostream.Printf(ctx, "  3. git push\n")
	iostream.Printf(ctx, "\nPushing will trigger your first pipeline.\n")
	if url, err := cmdutil.RunSlugURL(appURL, slug); err == nil {
		iostream.Printf(ctx, "%s\n", url)
	}
}

// printManualPipelineGuidance is the fallback when the pipeline definition or
// trigger could not be created. The project exists, so point the user at the
// commands that finish the job.
func printManualPipelineGuidance(ctx context.Context) {
	iostream.Printf(ctx, "\nCommit .circleci/config.yml. After your project is connected in CircleCI, pushing will start your first pipeline.\n")
	iostream.Printf(ctx, "To set up a trigger now, run 'circleci pipeline create' then 'circleci project trigger create'.\n")
}

func followClassicProject(ctx context.Context, client *apiclient.Client, appURL, vcs, orgName, repoName string) error {
	if err := client.FollowProject(ctx, vcs, orgName, repoName); err != nil {
		iostream.ErrPrintf(ctx, "%s Could not connect project: %s\n", iostream.SymbolWarn(ctx), err)
		printManualGuidance(ctx)
		return nil
	}

	slug := fmt.Sprintf("%s/%s/%s", vcs, orgName, repoName)
	iostream.Printf(ctx, "%s Project connected: %s\n", iostream.SymbolOK(ctx), repoName)
	iostream.Printf(ctx, "  Organization: %s\n", orgName)
	if pipelinesURL, err := cmdutil.RunSlugURL(appURL, slug); err == nil {
		iostream.Printf(ctx, "  Pipelines: %s\n", pipelinesURL)
	}
	iostream.Printf(ctx, "\nCommit and push .circleci/config.yml to start your first pipeline.\n")
	return nil
}

func printManualGuidance(ctx context.Context) {
	iostream.Printf(ctx, "\nRun 'circleci project create' to connect this repo to CircleCI.\n")
}

func trackOnboard(ctx context.Context, event string, props map[string]any) {
	tc := cmdutil.GetTelemetry(ctx)
	if tc == nil {
		return
	}
	_ = tc.Track(event, props)
}

func modeString(m mode) string {
	switch m {
	case modeScan:
		return "scan"
	case modeSignup:
		return "signup"
	default:
		return "unknown"
	}
}

func resolveMode(ctx context.Context, opts Options) (mode, error) {
	if opts.Scan {
		return modeScan, nil
	}
	if opts.Signup {
		return modeSignup, nil
	}

	if !iostream.IsInteractive(ctx) {
		return modeScan, nil
	}

	idx, err := iostream.PromptSelect(ctx, "What would you like to do?", []string{
		"Scan this repo and generate config",
		"Sign up for CircleCI",
	})
	if err != nil {
		return 0, clierrors.New(
			"onboard.mode_prompt_failed",
			"Mode selection failed",
			err.Error(),
		).WithExitCode(clierrors.ExitGeneralError)
	}
	if idx == -1 {
		return 0, clierrors.New(
			"onboard.cancelled",
			"Onboarding cancelled",
			"No mode selected.",
		).WithExitCode(clierrors.ExitCancelled)
	}

	if idx == 1 {
		return modeSignup, nil
	}
	return modeScan, nil
}

// displayPreamble shows a confirmation gate before any work begins. The prompt
// is skipped in non-interactive sessions (no TTY, CI=true, or
// CIRCLE_NO_INTERACTIVE set), in which case the caller continues without
// user input.
func displayPreamble(ctx context.Context, dir string) error {
	if !iostream.IsInteractive(ctx) {
		return nil
	}

	model := ui.NewPreambleModel(
		"circleci onboard will:",
		dir,
		[]string{
			"Scan your repo for the language stack and tests",
			"Run your tests locally",
			"Generate a starter .circleci/config.yml",
			"Sign you up for CircleCI",
			"Create your project and connect it to GitHub",
			"Set up your first pipeline trigger",
		},
	)
	p := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(iostream.In(ctx)),
		tea.WithOutput(iostream.Err(ctx)),
	)
	final, err := p.Run()
	if err != nil {
		return clierrors.New(
			"onboard.preamble_failed",
			"Preamble prompt failed",
			err.Error(),
		).WithExitCode(clierrors.ExitGeneralError)
	}

	m := final.(ui.PreambleModel)
	if !m.Proceed() {
		return clierrors.New(
			"onboard.cancelled",
			"Onboarding cancelled",
			"Cancelled before scan started.",
		).WithExitCode(clierrors.ExitCancelled)
	}
	return nil
}
