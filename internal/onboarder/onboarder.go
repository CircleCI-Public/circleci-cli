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

	tea "charm.land/bubbletea/v2"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/cmdauth"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/configgen"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/org"
	"github.com/CircleCI-Public/circleci-cli/internal/projectref"
	"github.com/CircleCI-Public/circleci-cli/internal/provider"
	"github.com/CircleCI-Public/circleci-cli/internal/providerconn"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

type mode int

const (
	modeScan   mode = iota // generate config for a repo, then sign up
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
	// RepoID is the provider's repository ID used to wire up the first pipeline
	// definition and trigger. When empty it is resolved through the integration
	// that owns the checkout's remote; if that cannot resolve it, pipeline setup is
	// skipped with manual guidance. It is never prompted for — an opaque ID is not
	// something a user can be expected to know.
	RepoID string
}

// Run generates a starter config for a repository when it has none, and ensures
// the CLI has an authenticated CircleCI session.
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

	// A nil scan result yields the generic starter template. Onboard does not scan
	// the repository: the config only has to be valid enough for the first pipeline
	// to run, and `circleci config generate` is where stack detection belongs.
	configPath := filepath.Join(dir, ".circleci", "config.yml")
	if _, err := os.Stat(configPath); err == nil {
		iostream.Printf(ctx, "%s Using existing config at %s\n",
			iostream.SymbolOK(ctx), configPath)
	} else if err := configgen.Generate(ctx, dir, nil); err != nil {
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
// Failures up to the point where the project exists fall through to manual
// guidance and succeed; a rejected pipeline request is a real error. See
// setupFirstPipeline.
func postSignupGuidance(ctx context.Context, dir string, opts Options) error {
	ctx = refreshConfig(ctx, opts)

	client, err := cmdutil.LoadClient(ctx)
	if err != nil {
		return stopWithGuidance(ctx)
	}

	orgs, err := org.Require(ctx, client)
	if err != nil {
		return stopWithGuidance(ctx)
	}
	selectedOrg := selectOrg(ctx, orgs)
	if selectedOrg == nil {
		return stopWithGuidance(ctx)
	}

	appURL, _ := cmdutil.AppURL(ctx)

	// One read of the checkout's remote serves the suggested project name, the
	// integration that owns the repository, and the owner/repo its lookup needs.
	remote := detectRemote(dir)

	// Resolve a project this repository is already linked to before asking for a
	// name. On a re-run the answer would be discarded, and offering one is
	// actively misleading: the recorded slug of a standalone project carries
	// opaque IDs rather than a repository name.
	var proj *apiclient.ProjectInfo
	if selectedOrg.VCSType == "circleci" {
		proj = resolveLinkedProject(ctx, client, dir, selectedOrg.ID)
	}

	// A classic organization is never linked — resolveLinkedProject is only
	// consulted for CircleCI-native orgs — so it always lands here and shares the
	// name prompt and slug parse with the create path.
	if proj == nil {
		name := promptProjectName(ctx, remote.Repo)
		if name == "" {
			return stopWithGuidance(ctx)
		}
		vcs, orgName, err := org.ParseSlug(selectedOrg.Slug)
		if err != nil {
			return stopWithGuidance(ctx)
		}

		if selectedOrg.VCSType != "circleci" {
			followClassicProject(ctx, client, appURL, vcs, orgName, name)
			return nil
		}

		created, err := client.CreateProject(ctx, vcs, orgName, name)
		switch {
		case httpcl.HasStatusCode(err, http.StatusConflict):
			// The name is taken, which on a re-run is this checkout's own project from
			// an earlier attempt. Adopting it is what makes onboard resumable: the rest
			// of the flow skips whatever already exists, so continuing gets the user to
			// a working pipeline where stopping to fetch an ID by hand does not.
			proj = adoptExistingProject(ctx, client, dir, selectedOrg, name)
			if proj == nil {
				return nil
			}
		case err != nil:
			iostream.ErrPrintf(ctx, "%s Could not create project: %s\n", iostream.SymbolWarn(ctx), err)
			return stopWithGuidance(ctx)
		default:
			proj = created
			iostream.Printf(ctx, "%s Project created: %s\n", iostream.SymbolOK(ctx), proj.Name)
			writeProjectRef(ctx, dir, proj)
		}
	} else {
		iostream.Printf(ctx, "%s Using existing project: %s\n", iostream.SymbolOK(ctx), proj.Name)
	}

	pipelinesURL, _ := cmdutil.RunSlugURL(appURL, proj.Slug)
	iostream.Printf(ctx, "  Organization: %s\n", proj.OrganizationName)
	if pipelinesURL != "" {
		iostream.Printf(ctx, "  Pipelines: %s\n", pipelinesURL)
	}

	followProject(ctx, client, proj)

	return setupFirstPipeline(ctx, client, appURL, proj, remote, opts)
}

// selectOrg picks the organization to create the project in, returning nil when
// the user cannot be asked or declines.
func selectOrg(ctx context.Context, orgs []apiclient.Collaboration) *apiclient.Collaboration {
	if len(orgs) == 1 {
		iostream.Printf(ctx, "%s Using organization %s\n", iostream.SymbolOK(ctx), orgs[0].Slug)
		return &orgs[0]
	}
	if !iostream.IsInteractive(ctx) {
		return nil
	}

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
		return nil
	}
	return &orgs[idx]
}

// stopWithGuidance prints how to connect the repository by hand and ends the run
// successfully. Everything up to the point where the project exists is recoverable
// this way; once a request has been made and rejected, setupFirstPipeline returns a
// real error instead.
func stopWithGuidance(ctx context.Context) error {
	printManualGuidance(ctx)
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

// detectRemote reads the origin remote of the checkout at dir. A failure yields
// the zero value rather than an error: onboard degrades to prompting for a name
// and to manual pipeline guidance.
//
// The host is what matters here rather than a project slug: the integration is
// resolved from it, and slugs exist only for the hosts CircleCI has a VCS segment
// for.
func detectRemote(dir string) gitremote.RemoteRef {
	if dir == "" {
		return gitremote.RemoteRef{}
	}
	ref, err := gitremote.DetectRemoteRefIn(dir)
	if err != nil {
		return gitremote.RemoteRef{}
	}
	return ref
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

// adoptExistingProject resolves the project that already holds name in the
// organization, records it for this checkout, and returns it so the flow can carry
// on into pipeline setup. A nil result means the caller should stop, with the
// reason already reported.
//
// The name is the only handle available: a CircleCI-native project's slug carries
// opaque IDs, so it cannot be built from what the user typed. Its UUIDs can be
// turned back into a slug though, which is what hydrates the rest of the record.
func adoptExistingProject(
	ctx context.Context,
	client *apiclient.Client,
	workDir string,
	selectedOrg *apiclient.Collaboration,
	name string,
) *apiclient.ProjectInfo {
	// Only a project this organization holds can be adopted. When it cannot be
	// resolved — the name belongs to an organization the caller cannot read, say —
	// the manual route is still the way through, so it is reported as before rather
	// than guessed at.
	existing, err := client.GetProjectByName(ctx, selectedOrg.ID, name)
	if err != nil {
		reportUnresolvedConflict(ctx, workDir, selectedOrg, name)
		return nil
	}

	proj, err := client.GetProjectInfo(ctx, projectref.SlugFor(existing.OrgID.String(), existing.ID.String()))
	if err != nil {
		reportUnresolvedConflict(ctx, workDir, selectedOrg, name)
		return nil
	}

	iostream.Printf(ctx, "%s Using existing project: %s\n", iostream.SymbolOK(ctx), proj.Name)
	writeProjectRef(ctx, workDir, proj)
	return proj
}

// reportUnresolvedConflict explains a name collision onboard could not resolve and
// points at the command that fixes it. The project exists but this checkout cannot
// be pointed at it without its ID.
func reportUnresolvedConflict(
	ctx context.Context, workDir string, selectedOrg *apiclient.Collaboration, name string,
) {
	iostream.ErrPrintf(ctx, "%s A project named %q already exists in %s.\n",
		iostream.SymbolWarn(ctx), name, selectedOrg.Slug)
	printLinkGuidance(ctx, workDir, selectedOrg.Slug)
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

// setupFirstPipeline wires up a pipeline definition and an all-pushes trigger
// for a freshly created project so the first push starts a build. It uses the
// same endpoints as `circleci pipeline create` and `circleci project trigger
// create`, with sensible zero-prompt defaults (config at .circleci/config.yml, the
// integration that owns the checkout's remote, and the repo's ID for both config
// and checkout sources).
//
// A missing prerequisite — the organization not connected to that integration, the
// repository not granted to it, a remote no integration claims — means nothing was
// attempted, so onboard prints the next step and succeeds. A request the API
// rejected is a real error: it leaves the project half-configured, and a definition
// with no trigger will never build.
func setupFirstPipeline(ctx context.Context, client *apiclient.Client, appURL string, proj *apiclient.ProjectInfo, remote gitremote.RemoteRef, opts Options) error {
	repoID, p := resolveRepoID(ctx, client, appURL, proj, remote, opts)
	if repoID == "" {
		// No external ID, so there is nothing to attach a definition to.
		trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "skipped_no_repo_id"})
		printManualPipelineGuidance(ctx)
		return nil
	}

	def, err := ensurePipelineDefinition(ctx, client, p, proj.ID, proj.Name, repoID, remote.FullName())
	if err != nil {
		trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "pipeline_definition_failed"})
		return clierrors.New("onboard.pipeline_definition_failed",
			"Could not set up the pipeline definition",
			fmt.Sprintf("The project was created, but its pipeline definition could not be set up: %s.", err)).
			WithSuggestions(
				"Run 'circleci pipeline create' to finish setting up the pipeline",
				"Then commit and push .circleci/ to start your first pipeline",
			).
			WithExitCode(clierrors.ExitAPIError)
	}

	if err := ensureTrigger(ctx, client, p, proj.ID, def.ID, repoID, remote.FullName()); err != nil {
		trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "trigger_failed"})
		return clierrors.New("onboard.trigger_failed",
			"Could not set up the trigger",
			fmt.Sprintf("The pipeline definition was created, but its trigger could not be: %s.", err)).
			WithSuggestions(
				"Run 'circleci project trigger create' to finish setting up the trigger",
				"Until a trigger exists, pushing will not start a pipeline",
			).
			WithExitCode(clierrors.ExitAPIError)
	}

	trackOnboard(ctx, "onboard_project_setup", map[string]any{"outcome": "created"})
	printPipelineReadyGuidance(ctx)
	return nil
}

// ensurePipelineDefinition returns the pipeline definition already configured
// for the repo, or creates one. Reusing an existing definition keeps re-runs of
// onboard from creating duplicates.
func ensurePipelineDefinition(
	ctx context.Context,
	client *apiclient.Client,
	p provider.Provider,
	projectID, name, repoID, repoFullName string,
) (*apiclient.PipelineDefinition, error) {
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
		Name:                 name,
		ConfigProvider:       p.Name,
		ConfigRepoID:         repoID,
		ConfigRepoFullName:   repoFullName,
		ConfigFilePath:       ".circleci/config.yml",
		CheckoutProvider:     p.Name,
		CheckoutRepoID:       repoID,
		CheckoutRepoFullName: repoFullName,
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
func ensureTrigger(
	ctx context.Context,
	client *apiclient.Client,
	p provider.Provider,
	projectID, definitionID, repoID, repoFullName string,
) error {
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

	trig, err := client.CreateTrigger(ctx, apiclient.CreateTriggerInput{
		ProjectID:            projectID,
		PipelineDefinitionID: definitionID,
		Provider:             p.Name,
		RepoID:               repoID,
		RepoFullName:         repoFullName,
		EventPreset:          allPushesPreset,
	})
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
// trigger, and returns it with the integration that owns the checkout's remote so
// the caller can write both without resolving it again. An explicit --repo-id wins. Otherwise it ensures the org is connected
// to the integration that owns the checkout's remote, then matches the remote
// against the repositories that connection can reach. Any failure returns "" so
// the caller degrades to manual guidance rather than prompting for an opaque ID.
func resolveRepoID(
	ctx context.Context,
	client *apiclient.Client,
	appURL string,
	proj *apiclient.ProjectInfo,
	remote gitremote.RemoteRef,
	opts Options,
) (string, provider.Provider) {
	p, hasProvider := provider.ForHost(remote.Host)
	if opts.RepoID != "" {
		return opts.RepoID, p
	}
	if proj.OrganizationID == "" {
		return "", p
	}
	fullName := remote.FullName()
	if fullName == "" {
		iostream.ErrPrintf(ctx, "%s Could not read this repository's git remote, so the pipeline was not set up.\n",
			iostream.SymbolWarn(ctx))
		return "", p
	}
	// A host no integration claims cannot be resolved: sending it through one
	// produces an install prompt that cannot help and a "grant access to this
	// repository" message that can never be satisfied.
	if !hasProvider {
		iostream.ErrPrintf(ctx, "%s %s is not on a provider CircleCI can look up, so its ID cannot be resolved automatically.\n",
			iostream.SymbolWarn(ctx), fullName)
		iostream.ErrPrintf(ctx, "  Re-run with --repo-id <id> to set up the pipeline.\n")
		return "", p
	}

	// Where the provider returns the browser after an install. Landing on the
	// project's own page reads as a finish line, in the browser, while the rest of
	// setup is still waiting back in the terminal, so each integration has a page
	// that says so.
	returnURL := cmdutil.InstalledURL(appURL, p.InstalledPath)

	installed, err := providerconn.EnsureConnected(ctx, client, p, proj.OrganizationID, returnURL, opts.NoBrowser)
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not check the %s installation: %s\n", iostream.SymbolWarn(ctx), p.Short, err)
		return "", p
	}
	if !installed {
		return "", p
	}

	id, err := providerconn.ResolveRepoID(ctx, client, p, proj.OrganizationID, fullName)
	if errors.Is(err, providerconn.ErrTooManyRepositories) {
		iostream.ErrPrintf(ctx, "%s This organization has too many repositories to find %s automatically.\n",
			iostream.SymbolWarn(ctx), fullName)
		iostream.ErrPrintf(ctx, "  Re-run with --repo-id <id> to set up the pipeline.\n")
		return "", p
	}
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not list %s repositories: %s\n", iostream.SymbolWarn(ctx), p.Short, err)
		return "", p
	}
	if id == "" {
		iostream.ErrPrintf(ctx, "%s %s can't access %s yet. Grant it access to this repository, then re-run.\n",
			iostream.SymbolWarn(ctx), p.Short, fullName)
		return "", p
	}

	iostream.Printf(ctx, "%s Found repository %s\n", iostream.SymbolOK(ctx), fullName)
	return id, p
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

// followProject follows the project on the CircleCI-native path, so the first
// pipeline the user is about to push reaches them: a follower is who the
// notifications for a run go to, and onboard's whole promise is that the next push
// builds. followClassicProject does the same for a classic organization, where the
// slug's segments are the org and repository names rather than opaque IDs.
//
// A failure is reported and the run continues. Following decides who hears about a
// pipeline, not whether the project has one, so it must not stand between the user
// and a working trigger. It is idempotent, which is what makes it safe on the
// re-run and adopt-existing paths, where the project may already be followed —
// possibly by a teammate who created it.
func followProject(ctx context.Context, client *apiclient.Client, proj *apiclient.ProjectInfo) {
	vcs, orgSegment, projectSegment, err := cmdutil.ParseSlug(proj.Slug)
	if err != nil {
		trackOnboard(ctx, "onboard_project_follow", map[string]any{"outcome": "skipped_bad_slug"})
		return
	}

	if err := client.FollowProject(ctx, vcs, orgSegment, projectSegment); err != nil {
		iostream.ErrPrintf(ctx,
			"%s Could not follow the project, so you may not be notified about its pipelines: %s\n",
			iostream.SymbolWarn(ctx), err)
		trackOnboard(ctx, "onboard_project_follow", map[string]any{"outcome": "failed"})
		return
	}

	iostream.Printf(ctx, "%s Following %s\n", iostream.SymbolOK(ctx), proj.Name)
	trackOnboard(ctx, "onboard_project_follow", map[string]any{"outcome": "followed"})
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
		"Set up this repo on CircleCI",
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
			"Generate a starter .circleci/config.yml",
			"Sign you up for CircleCI",
			"Create your project and connect your repository",
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
			"Cancelled before setup started.",
		).WithExitCode(clierrors.ExitCancelled)
	}
	return nil
}
