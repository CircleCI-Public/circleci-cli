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
	"errors"
	"fmt"
	"net/http"
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
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/org"
	"github.com/CircleCI-Public/circleci-cli/internal/projectref"
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
		// --signup may be run anywhere — it needs no repository — so the directory
		// has been through none of the validation below. repoDir yields "" unless it
		// really is a checkout, which keeps a run from a home directory out of
		// ~/.circleci/info.yml.
		return postSignupGuidance(ctx, repoDir(dir), opts)
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

	return postSignupGuidance(ctx, dir, opts)
}

// refreshConfig re-reads the config from disk when the cached copy carries no
// token.
//
// A fresh signup persists the token to the keyring (or the config file), but the
// *config.Config in ctx was loaded during root's bootstrap — before that write —
// so it still looks unauthenticated. Without this reload, LoadClient fails with
// auth.token_missing on the very run that just signed the user up, and project
// creation degrades to manual guidance.
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

// repoDir returns the root of the repository containing dir, and "" when dir is
// not inside one. A "" result disables everything that reads or writes repository
// state, which is the right behaviour when there is no repository to describe.
//
// Resolving to the root matters: run from a subdirectory, .circleci/info.yml
// belongs beside .circleci/config.yml at the top of the checkout, not wherever the
// command was invoked.
func repoDir(dir string) string {
	root, err := gitremote.RepoRootIn(dir)
	if err != nil {
		return ""
	}
	return root
}

// postSignupGuidance offers inline project creation and prints a follow-up
// message after the user has authenticated.
//
// Errors are handled gracefully: project creation failure falls through to
// manual guidance rather than failing the onboard command.
func postSignupGuidance(ctx context.Context, dir string, opts Options) error {
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

	// Resolve a project this repository is already linked to before asking for a
	// name. On a re-run the answer would be discarded, and offering one is
	// actively misleading: the recorded slug of a standalone project carries
	// opaque IDs rather than a repository name.
	var proj *apiclient.ProjectInfo
	if selectedOrg.VCSType == "circleci" {
		proj = resolveLinkedProject(ctx, client, dir, selectedOrg.ID)
	}

	if proj == nil {
		name := promptProjectName(ctx, repoNameIn(dir))
		if name == "" {
			printManualGuidance(ctx)
			return nil
		}

		if selectedOrg.VCSType != "circleci" {
			followClassicProject(ctx, client, appURL, vcs, orgName, name)
			return nil
		}

		created, err := client.CreateProject(ctx, vcs, orgName, name)
		if err != nil {
			if httpcl.HasStatusCode(err, http.StatusConflict) {
				iostream.ErrPrintf(ctx, "%s A project named %q already exists in %s.\n",
					iostream.SymbolWarn(ctx), name, selectedOrg.Slug)
				printLinkGuidance(ctx, dir, selectedOrg.Slug)
				return nil
			}
			iostream.ErrPrintf(ctx, "%s Could not create project: %s\n", iostream.SymbolWarn(ctx), err)
			printManualGuidance(ctx)
			return nil
		}
		proj = created
		iostream.Printf(ctx, "%s Project created: %s\n", iostream.SymbolOK(ctx), proj.Name)
		writeProjectRef(ctx, dir, proj)
	} else {
		iostream.Printf(ctx, "%s Using existing project: %s\n", iostream.SymbolOK(ctx), proj.Name)
	}

	iostream.Printf(ctx, "  Organization: %s\n", proj.OrganizationName)
	if pipelinesURL, err := cmdutil.RunSlugURL(appURL, proj.Slug); err == nil {
		iostream.Printf(ctx, "  Pipelines: %s\n", pipelinesURL)
	}
	// Stage the whole directory: info.yml records the project's ID, and that ID is
	// not recoverable from its name — no API maps one to the other. Committing it is
	// what lets a fresh clone, a teammate, or a later run find this project instead
	// of colliding with it.
	iostream.Printf(ctx, "\nCommit .circleci/ (config.yml and info.yml). After your project is connected in CircleCI, pushing will start your first pipeline.\n")
	return nil
}

// promptProjectName asks for the project name, offering defaultName. It returns
// "" when no name could be determined, which callers treat as "fall back to
// manual guidance".
func promptProjectName(ctx context.Context, defaultName string) string {
	if !iostream.IsInteractive(ctx) {
		return defaultName
	}
	name, err := iostream.PromptText(ctx, "Project name", defaultName)
	if err != nil {
		return ""
	}
	if name == "" {
		return defaultName
	}
	return name
}

// repoNameIn returns the repository name of the checkout at dir, or "" when there
// is no readable remote there.
func repoNameIn(dir string) string {
	if dir == "" {
		return ""
	}
	info, err := gitremote.DetectFromRemoteIn(dir)
	if err != nil {
		return ""
	}
	parts := strings.Split(info.Slug, "/")
	if len(parts) != 3 {
		return ""
	}
	return parts[2]
}

// resolveLinkedProject returns the project recorded in .circleci/info.yml, or nil
// when this checkout has no link onboard can use.
//
// That local record is the only way to find an existing project: a CircleCI-native
// slug is "circleci/<orgID>/<projectID>" — opaque IDs, not the repository name —
// and no API maps a name to a project within an org. Resolving before creating
// keeps a re-run from attempting a create that could only conflict.
//
// A link only counts when its project belongs to orgID. A repository linked
// elsewhere — a classic VCS project being migrated to a CircleCI-native org, say —
// is ignored, so onboard sets up the organization the user actually chose. An
// unresolvable link is ignored the same way; the create path handles everything a
// link cannot supply.
func resolveLinkedProject(ctx context.Context, client *apiclient.Client, workDir, orgID string) *apiclient.ProjectInfo {
	if workDir == "" {
		return nil
	}
	info, err := projectref.Read(workDir)
	if err != nil {
		// An absent file just means "not linked". Anything else is a file the user
		// can fix, and every other command reports it rather than proceeding as if
		// the checkout were unlinked.
		if !errors.Is(err, projectref.ErrNotFound) {
			iostream.ErrPrintf(ctx, "%s Ignoring %s: %s\n",
				iostream.SymbolWarn(ctx), projectref.FilePath, err)
		}
		return nil
	}
	// Compare the recorded org before spending a request: a link to another org is
	// rejected for free. Both IDs must be present — treating two empty strings as
	// a match would reuse a foreign project while appearing to have checked.
	if orgID == "" || (info.Organization.ID != "" && info.Organization.ID != orgID) {
		return nil
	}
	proj, err := client.GetProjectInfo(ctx, info.EffectiveSlug())
	if err != nil || proj.OrganizationID != orgID {
		return nil
	}
	return proj
}

// writeProjectRef records the new project in .circleci/info.yml so a re-run of
// onboard — and every other command in this repository — can resolve it without
// asking the user for an opaque project ID.
//
// An existing file is never overwritten. It may be committed, and it may record a
// project in another organization that this run has no mandate to replace —
// `circleci project link` itself demands --force for exactly that reason.
//
// Failure is not fatal: the project exists and setup continues using the ID
// already in hand. Only the next run loses the shortcut.
func writeProjectRef(ctx context.Context, workDir string, proj *apiclient.ProjectInfo) {
	if workDir == "" {
		return
	}
	if _, err := os.Stat(projectref.Path(workDir)); err == nil {
		iostream.ErrPrintf(ctx, "%s %s already records a different project; leaving it as it is.\n",
			iostream.SymbolWarn(ctx), projectref.FilePath)
		iostream.ErrPrintf(ctx, "  To point it at %s: circleci project link --force --project %s\n",
			proj.Name, proj.Slug)
		return
	}
	err := projectref.Write(workDir, &projectref.Info{
		Organization: projectref.Organization{ID: proj.OrganizationID, Name: proj.OrganizationName},
		Project:      projectref.Project{ID: proj.ID, Slug: proj.Slug, Name: proj.Name},
	})
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not write %s: %s\n",
			iostream.SymbolWarn(ctx), projectref.FilePath, err)
		return
	}
	iostream.Printf(ctx, "%s Linked this repository to the project in %s\n",
		iostream.SymbolOK(ctx), projectref.FilePath)
}

// printLinkGuidance covers a project that exists in the organization but is not
// the one this checkout resolves to. No API maps a project name to its ID, so the
// ID has to come from the user — the same conclusion `circleci project link`
// reaches when it cannot resolve a slug on its own.
//
// The command is spelled out with the organization already filled in. --project is
// load-bearing: without it, link re-derives the slug from the git remote and a
// repository linked elsewhere lands straight back where it started. --force is
// added when .circleci/info.yml is present, since plain link refuses while it is.
func printLinkGuidance(ctx context.Context, workDir, orgSlug string) {
	args := "--project " + orgSlug + "/<projectID>"
	if _, err := os.Stat(projectref.Path(workDir)); err == nil {
		args = "--force " + args
	}
	iostream.Printf(ctx, "\nCopy that project's ID from its settings in CircleCI, then run:\n")
	iostream.Printf(ctx, "  circleci project link %s\n", args)
	iostream.Printf(ctx, "  circleci onboard\n")
}

func followClassicProject(ctx context.Context, client *apiclient.Client, appURL, vcs, orgName, repoName string) {
	if err := client.FollowProject(ctx, vcs, orgName, repoName); err != nil {
		iostream.ErrPrintf(ctx, "%s Could not connect project: %s\n", iostream.SymbolWarn(ctx), err)
		printManualGuidance(ctx)
		return
	}

	slug := fmt.Sprintf("%s/%s/%s", vcs, orgName, repoName)
	iostream.Printf(ctx, "%s Project connected: %s\n", iostream.SymbolOK(ctx), repoName)
	iostream.Printf(ctx, "  Organization: %s\n", orgName)
	if pipelinesURL, err := cmdutil.RunSlugURL(appURL, slug); err == nil {
		iostream.Printf(ctx, "  Pipelines: %s\n", pipelinesURL)
	}
	iostream.Printf(ctx, "\nCommit and push .circleci/config.yml to start your first pipeline.\n")
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
