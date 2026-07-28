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
	"github.com/CircleCI-Public/circleci-cli/internal/githubapp"
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

// allPushesPreset is the trigger event preset that starts a pipeline on every
// push, which is what onboard sets up.
const allPushesPreset = "all-pushes"

// Options configures the onboarding flow.
type Options struct {
	ConfigPath    string
	NoBrowser     bool
	SecureStorage bool
	Scan          bool
	Signup        bool
	// RepoID is the VCS repository external ID (the numeric GitHub repo ID) used
	// to wire up the first pipeline definition and trigger. When empty it is
	// resolved through the CircleCI GitHub App; if that cannot resolve it, pipeline
	// setup is skipped with manual guidance. It is never prompted for — an opaque
	// numeric ID is not something a user can be expected to know.
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
// message after the user has authenticated. For modern (CircleCI-native) orgs
// it continues past project creation to set up the first pipeline definition
// and trigger, so a subsequent push starts a build.
//
// dir is the checkout being onboarded, or "" when there is none. Every read of
// repository state — the git remote and .circleci/info.yml — is scoped to it, so
// that `circleci onboard <path>` describes the directory it was given rather than
// the one the process happens to be sitting in.
//
// Errors are handled gracefully: any failure falls through to manual guidance
// rather than failing the onboard command.
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

	// One read of the checkout's remote serves both the suggested project name and
	// the owner/repo the GitHub App lookup needs.
	remote := detectRemote(dir)

	// Resolve a project this repository is already linked to before asking for a
	// name. On a re-run the answer would be discarded, and offering one is
	// actively misleading: the recorded slug of a standalone project carries
	// opaque IDs rather than a repository name.
	var proj *apiclient.ProjectInfo
	if selectedOrg.VCSType == "circleci" {
		proj = resolveLinkedProject(ctx, client, dir, selectedOrg.ID)
	}

	if proj == nil {
		name := promptProjectName(ctx, remote.Repo)
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

	pipelinesURL, _ := cmdutil.RunSlugURL(appURL, proj.Slug)
	iostream.Printf(ctx, "  Organization: %s\n", proj.OrganizationName)
	if pipelinesURL != "" {
		iostream.Printf(ctx, "  Pipelines: %s\n", pipelinesURL)
	}

	setupFirstPipeline(ctx, client, pipelinesURL, appURL, proj, remote, opts)
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

// gitRemote is what onboard needs to know about a checkout's origin remote.
type gitRemote struct {
	// VCS is the provider segment of the slug ("gh", "bb", "gl"), or "" when the
	// remote could not be read.
	VCS string
	// Owner and Repo name the repository, e.g. "acme" and "web".
	Owner, Repo string
}

// FullName is the "owner/repo" form used as the GitHub App's repository key, or
// "" when the remote could not be read.
func (r gitRemote) FullName() string {
	if r.Owner == "" || r.Repo == "" {
		return ""
	}
	return r.Owner + "/" + r.Repo
}

// IsGitHub reports whether this is a GitHub repository — the only provider the
// CircleCI GitHub App can resolve.
func (r gitRemote) IsGitHub() bool { return r.VCS == "gh" }

// detectRemote reads the origin remote of the checkout at dir. A failure yields
// the zero value rather than an error: onboard degrades to prompting for a name
// and to manual pipeline guidance.
func detectRemote(dir string) gitRemote {
	if dir == "" {
		return gitRemote{}
	}
	info, err := gitremote.DetectFromRemoteIn(dir)
	if err != nil {
		return gitRemote{}
	}
	parts := strings.Split(info.Slug, "/")
	if len(parts) != 3 {
		return gitRemote{}
	}
	return gitRemote{VCS: parts[0], Owner: parts[1], Repo: parts[2]}
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
// unresolvable link is ignored the same way; the create path below handles
// everything a link cannot supply.
func resolveLinkedProject(ctx context.Context, client *apiclient.Client, workDir, orgID string) *apiclient.ProjectInfo {
	if workDir == "" {
		return nil
	}
	info, err := projectref.Read(workDir)
	if err != nil {
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

// setupFirstPipeline wires up a pipeline definition and an all-pushes trigger
// for a freshly created project so the first push starts a build. It uses the
// same v2 endpoints as `circleci pipeline create` and `circleci project trigger
// create`, with sensible zero-prompt defaults (config at .circleci/config.yml,
// the github_app provider, and the repo's external ID for both config and
// checkout sources).
//
// Every step degrades gracefully: the project already exists, so any failure
// prints manual guidance rather than failing the command.
func setupFirstPipeline(ctx context.Context, client *apiclient.Client, pipelinesURL, appURL string, proj *apiclient.ProjectInfo, remote gitRemote, opts Options) {
	repoID := resolveRepoID(ctx, client, pipelinesURL, appURL, proj, remote, opts)
	if repoID == "" {
		// Without the repo's external ID we can't create the pipeline
		// definition. The project still exists; guide the user to finish setup.
		trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "skipped_no_repo_id"})
		printManualPipelineGuidance(ctx)
		return
	}

	def, err := ensurePipelineDefinition(ctx, client, proj.ID, proj.Name, repoID)
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not create pipeline definition: %s\n", iostream.SymbolWarn(ctx), err)
		trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "pipeline_definition_failed"})
		printManualPipelineGuidance(ctx)
		return
	}

	if err := ensureTrigger(ctx, client, proj.ID, def.ID, repoID); err != nil {
		iostream.ErrPrintf(ctx, "%s Could not create trigger: %s\n", iostream.SymbolWarn(ctx), err)
		trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "trigger_failed"})
		printManualPipelineGuidance(ctx)
		return
	}

	trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "created"})
	printPipelineReadyGuidance(ctx)
}

// ensurePipelineDefinition returns the pipeline definition already configured
// for the repo, or creates one. Reusing an existing definition keeps re-runs of
// onboard from creating duplicates.
func ensurePipelineDefinition(ctx context.Context, client *apiclient.Client, projectID, name, repoID string) (*apiclient.PipelineDefinition, error) {
	defs, err := client.ListPipelineDefinitions(ctx, projectID)
	if err != nil {
		// Creating blindly after a failed lookup risks a duplicate definition, so
		// report the lookup failure instead of guessing.
		return nil, err
	}
	for i := range defs {
		cs := defs[i].ConfigSource
		if cs != nil && cs.Repo != nil && cs.Repo.ExternalID == repoID {
			iostream.Printf(ctx, "%s Pipeline definition already exists: %s\n", iostream.SymbolOK(ctx), defs[i].Name)
			return &defs[i], nil
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
//
// Only an all-pushes trigger counts. A definition carrying just a schedule or
// webhook trigger would otherwise be reported as ready, and the push onboard tells
// the user to make would build nothing.
func ensureTrigger(ctx context.Context, client *apiclient.Client, projectID, definitionID, repoID string) error {
	trigs, err := client.ListTriggers(ctx, projectID, definitionID)
	if err != nil {
		return err
	}
	for _, t := range trigs {
		if t.EventPreset == allPushesPreset {
			iostream.Printf(ctx, "%s Trigger already exists: %s\n", iostream.SymbolOK(ctx), t.EventPreset)
			return nil
		}
	}

	trig, err := client.CreateTrigger(ctx, projectID, definitionID, "github_app", repoID, allPushesPreset, "", "")
	if err != nil {
		return err
	}
	preset := trig.EventPreset
	if preset == "" {
		preset = allPushesPreset
	}
	iostream.Printf(ctx, "%s Trigger created: %s\n", iostream.SymbolOK(ctx), preset)
	return nil
}

// resolveRepoID determines the repo external ID for the pipeline definition and
// trigger. An explicit --repo-id wins. Otherwise it ensures the CircleCI GitHub
// App is installed for the org and matches the git remote against the app's
// accessible repositories. Any failure returns "" so the caller degrades to
// manual guidance rather than prompting for an opaque numeric ID.
func resolveRepoID(ctx context.Context, client *apiclient.Client, pipelinesURL, appURL string, proj *apiclient.ProjectInfo, remote gitRemote, opts Options) string {
	if opts.RepoID != "" {
		return opts.RepoID
	}
	if proj.OrganizationID == "" {
		return ""
	}
	fullName := remote.FullName()
	if fullName == "" {
		iostream.ErrPrintf(ctx, "%s Could not read this repository's git remote, so the pipeline was not set up.\n",
			iostream.SymbolWarn(ctx))
		return ""
	}
	// The GitHub App resolves GitHub repositories only. Sending a GitLab or
	// Bitbucket remote through it produces an install prompt that cannot help and
	// a "grant access to this repository" message that can never be satisfied.
	if !remote.IsGitHub() {
		iostream.ErrPrintf(ctx, "%s %s is not a GitHub repository, so its ID cannot be looked up automatically.\n",
			iostream.SymbolWarn(ctx), fullName)
		iostream.ErrPrintf(ctx, "  Re-run with --repo-id <id> to set up the pipeline.\n")
		return ""
	}

	returnURL := pipelinesURL
	if returnURL == "" {
		returnURL = appURL
	}

	installed, err := githubapp.EnsureInstalled(ctx, client, proj.OrganizationID, returnURL, opts.NoBrowser)
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
// definition and trigger have been created. The pipelines URL is not repeated
// here; the caller has already printed it alongside the project details.
func printPipelineReadyGuidance(ctx context.Context) {
	// Stage the whole directory rather than just config.yml: info.yml records the
	// project's ID, and that ID is not recoverable from its name — no API maps one
	// to the other. Committing it is what lets a fresh clone, a teammate, or a
	// later onboard run find this project instead of colliding with it.
	iostream.Printf(ctx, "\nYour project is ready! Next steps:\n")
	iostream.Printf(ctx, "  1. git add .circleci/\n")
	iostream.Printf(ctx, "  2. git commit -m \"Add CircleCI config\"\n")
	iostream.Printf(ctx, "  3. git push\n")
	iostream.Printf(ctx, "\nPushing will trigger your first pipeline.\n")
}

// printManualPipelineGuidance is the fallback when the pipeline definition or
// trigger could not be created. The project exists, so point the user at the
// commands that finish the job.
func printManualPipelineGuidance(ctx context.Context) {
	iostream.Printf(ctx, "\nCommit .circleci/config.yml. After your project is connected in CircleCI, pushing will start your first pipeline.\n")
	iostream.Printf(ctx, "To set up a trigger now, run 'circleci pipeline create' then 'circleci project trigger create'.\n")
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
