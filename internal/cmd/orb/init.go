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

package orb

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/orbinit"
	"github.com/CircleCI-Public/circleci-cli/internal/pack"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

const publishingContextName = "orb-publishing"

type orbInitOpts struct {
	path         string
	private      bool
	org          string
	templateOnly bool
	skipGit      bool
	branch       string
	remote       string
}

func newInitCmd() *cobra.Command {
	opts := orbInitOpts{}

	cmd := &cobra.Command{
		Use:   "init <path>",
		Short: "Initialize a new orb project",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				- %[1]s<path>%[1]s is the directory to scaffold the orb project into. It is
				  created if it does not exist.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Scaffold a new orb project from the CircleCI orb template.

			The template is downloaded from CircleCI-Public/Orb-Template and
			extracted into <path>. By default the command then walks you through
			an automated setup: it reserves the namespace and orb, assigns
			categories, sets up an 'orb-publishing' context, initializes a local
			git repository, and publishes a dev:alpha version.

			Use --template-only to download the template without any setup, or
			--skip-git to run the registry setup without initializing a git repo.

			Note: once published, orbs cannot be deleted.
		`),
		Example: heredoc.Doc(`
			# Interactive setup
			$ circleci orb init ./my-orb

			# Just download the template, no setup
			$ circleci orb init ./my-orb --template-only

			# Non-interactive: create a private orb under an org, skip git
			$ circleci orb init ./my-orb --private --org gh/acme --skip-git
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "path"); err != nil {
				return err
			}
			opts.path = args[0]
			return runOrbInit(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.private, "private", false, "initialize a private orb")
	cmdutil.AddOrgFlag(cmd, &opts.org, cmdutil.OrgFlag{Purpose: "to own the orb namespace and publishing context"})
	cmd.Flags().BoolVar(&opts.templateOnly, "template-only", false, "download the template only; skip all setup")
	cmd.Flags().BoolVar(&opts.skipGit, "skip-git", false, "skip local git repository setup")
	cmd.Flags().StringVar(&opts.branch, "branch", "main", "primary git branch to track")
	cmd.Flags().StringVar(&opts.remote, "remote", "", "remote git repository URL (required for git setup when non-interactive)")

	return cmd
}

func runOrbInit(ctx context.Context, opts orbInitOpts) error {
	iostream.Printf(ctx, "Note: once published, orbs cannot be deleted.\n")

	if iostream.IsInteractive(ctx) {
		return runOrbInitInteractive(ctx, opts)
	}
	return runOrbInitFlagged(ctx, opts)
}

// runOrbInitFlagged runs the non-interactive path: every decision comes from
// flags, no prompts fire, and a missing required value produces a clear error.
func runOrbInitFlagged(ctx context.Context, opts orbInitOpts) error {
	iostream.Printf(ctx, "Downloading orb project template into %s\n", opts.path)
	if err := orbinit.FetchTemplate(ctx, opts.path); err != nil {
		return orbInitDownloadErr(err)
	}
	if opts.private {
		if err := orbinit.RemovePrivateLicense(opts.path); err != nil {
			return orbInitLicenseErr(err)
		}
	}
	if opts.templateOnly {
		iostream.Printf(ctx, "%s Orb template extracted to %s\n", iostream.SymbolOK(ctx), opts.path)
		return nil
	}

	client, err := cmdutil.LoadClient(ctx)
	if err != nil {
		return err
	}

	if opts.org == "" {
		return clierrors.New("orb.init_org_required", "Organization required",
			"Pass --org <vcs>/<org> to run orb init non-interactively.").
			WithExitCode(clierrors.ExitBadArguments)
	}
	_, owner, ok := splitOrgSlug(opts.org)
	if !ok {
		return clierrors.New("orb.init_invalid_org", "Invalid organization slug",
			"Expected <vcs>/<org> (e.g. gh/acme), got: "+opts.org).
			WithExitCode(clierrors.ExitBadArguments)
	}

	return applyOrbInitSetup(ctx, client, opts.path, orbInitDecisions{
		private:   opts.private,
		orgSlug:   opts.org,
		namespace: owner,
		orbName:   filepath.Base(opts.path),
		gitSetup:  !opts.skipGit,
		branch:    opts.branch,
		remote:    opts.remote,
	}, false)
}

// runOrbInitInteractive drives the interactive setup through the bubbletea
// OrbInitFlowModel, then applies the gathered decisions. The flow performs the
// three gating operations (template download, orb-existence check, category
// list) via callbacks; the client is loaded lazily so a template-only run needs
// no authentication.
func runOrbInitInteractive(ctx context.Context, opts orbInitOpts) error {
	var (
		client    *apiclient.Client
		clientErr error
		loaded    bool
	)
	getClient := func() (*apiclient.Client, error) {
		if !loaded {
			client, clientErr = cmdutil.LoadClient(ctx)
			loaded = true
		}
		return client, clientErr
	}

	model := ui.NewOrbInitFlow(ctx, ui.OrbInitFlowOptions{
		Path:         opts.path,
		Private:      opts.private,
		TemplateOnly: opts.templateOnly,
		OrgSlug:      opts.org,
		SkipGit:      opts.skipGit,
		Branch:       opts.branch,
		Remote:       opts.remote,
		Color:        iostream.ColorEnabled(ctx),
		Animate:      iostream.SpinnerEnabled(ctx),
		Download: func(ctx context.Context, private bool) error {
			if err := orbinit.FetchTemplate(ctx, opts.path); err != nil {
				return orbInitDownloadErr(err)
			}
			if private {
				if err := orbinit.RemovePrivateLicense(opts.path); err != nil {
					return orbInitLicenseErr(err)
				}
			}
			return nil
		},
		GetOrb: func(ctx context.Context, fullName string) (bool, error) {
			cl, err := getClient()
			if err != nil {
				return false, err
			}
			if _, err := cl.GetOrbPackageByName(ctx, fullName); err != nil {
				if errors.Is(err, apiclient.ErrOrbNotFound) {
					return false, nil
				}
				return false, orbAPIErr(err, fullName)
			}
			return true, nil
		},
		ListCategories: func(ctx context.Context) ([]ui.OrbInitCategory, error) {
			cl, err := getClient()
			if err != nil {
				return nil, err
			}
			categories, err := cl.ListOrbCategories(ctx)
			if err != nil {
				return nil, orbAPIErr(err, "categories")
			}
			out := make([]ui.OrbInitCategory, 0, len(categories))
			for _, c := range categories {
				out = append(out, ui.OrbInitCategory{ID: c.ID, Name: c.Name})
			}
			return out, nil
		},
	})

	final, err := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(iostream.In(ctx)),
		tea.WithOutput(iostream.Err(ctx)),
	).Run()
	if err != nil {
		return clierrors.New("orb.init_prompt_failed", "Failed to run orb init prompt", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	res := final.(ui.OrbInitFlowModel).Result()

	switch {
	case res.Cancelled:
		return clierrors.New("orb.init_cancelled", "Cancelled", "Orb init was cancelled.").
			WithExitCode(clierrors.ExitCancelled)
	case res.Err != nil:
		// Callbacks already wrap failures in a structured CLI error.
		return res.Err
	}

	// The template was downloaded and extracted by the flow's Download callback.
	if res.TemplateOnly {
		iostream.Printf(ctx, "%s Orb template extracted to %s\n", iostream.SymbolOK(ctx), opts.path)
		return nil
	}

	cl, err := getClient()
	if err != nil {
		return err
	}

	categories := make([]*apiclient.OrbCategory, 0, len(res.Categories))
	for _, c := range res.Categories {
		categories = append(categories, &apiclient.OrbCategory{ID: c.ID, Name: c.Name})
	}
	return applyOrbInitSetup(ctx, cl, opts.path, orbInitDecisions{
		private:      res.Private,
		orgSlug:      res.OrgSlug,
		namespace:    res.Namespace,
		orbName:      res.OrbName,
		categories:   categories,
		setupContext: res.SetupContext,
		gitSetup:     res.GitSetup,
		branch:       res.Branch,
		remote:       res.Remote,
	}, true)
}

// orbInitDecisions is the fully-resolved set of choices needed for the automated
// setup, gathered either from flags (non-interactive) or the interactive flow.
// applyOrbInitSetup consumes it and never prompts.
type orbInitDecisions struct {
	private      bool
	orgSlug      string
	namespace    string
	orbName      string
	categories   []*apiclient.OrbCategory
	setupContext bool
	gitSetup     bool
	branch       string
	remote       string
}

// applyOrbInitSetup performs the automated setup — namespace, orb, categories,
// publishing context, and optional git project + dev publish — from already
// resolved decisions. interactive controls only the post-push "have you pushed?"
// pause in the git path; no other prompts occur here.
func applyOrbInitSetup(ctx context.Context, client *apiclient.Client, path string, d orbInitDecisions, interactive bool) error {
	// The context owner slug and project follow need the vcs/owner slug;
	// namespace creation needs the org UUID (resolved lazily below).
	vcsType, owner, ok := splitOrgSlug(d.orgSlug)
	if !ok {
		return clierrors.New("orb.init_invalid_org", "Invalid organization slug",
			"Expected <vcs>/<org> (e.g. gh/acme), got: "+d.orgSlug).
			WithExitCode(clierrors.ExitBadArguments)
	}

	// Namespace: create if it does not exist.
	ns, err := client.GetNamespace(ctx, d.namespace)
	if err != nil {
		if !errors.Is(err, apiclient.ErrNamespaceNotFound) {
			return orbAPIErr(err, d.namespace)
		}
		orgID, resolveErr := cmdutil.ResolveOrgSlugOrID(ctx, client, d.orgSlug, "circleci orb init")
		if resolveErr != nil {
			return resolveErr
		}
		iostream.Printf(ctx, "Namespace %q does not exist, creating it...\n", d.namespace)
		ns, err = client.CreateNamespace(ctx, apiclient.CreateNamespaceRequest{
			Name:  d.namespace,
			OrgID: orgID.String(),
		})
		if err != nil {
			return orbAPIErr(err, d.namespace)
		}
	}

	fullName := d.namespace + "/" + d.orbName
	existing, err := client.GetOrbPackageByName(ctx, fullName)
	orbExists := err == nil
	if err != nil && !errors.Is(err, apiclient.ErrOrbNotFound) {
		return orbAPIErr(err, fullName)
	}

	// Publishing context.
	if d.setupContext {
		setupPublishingContext(ctx, client, d.orgSlug)
	}

	// Ensure the orb exists in the registry and capture its ID.
	orbID := ""
	if orbExists {
		orbID = existing.ID
	} else {
		pkg, createErr := client.CreateOrbPackage(ctx, apiclient.CreateOrbPackageRequest{
			Name:        fullName,
			NamespaceID: ns.ID,
			IsPrivate:   d.private,
		})
		if createErr != nil {
			return orbAPIErr(createErr, fullName)
		}
		orbID = pkg.ID
		iostream.Printf(ctx, "%s Created orb %q\n", iostream.SymbolOK(ctx), fullName)
	}

	for _, cat := range d.categories {
		if err := client.AddOrbToCategory(ctx, orbID, cat.ID); err != nil {
			return orbAPIErr(err, cat.Name)
		}
	}

	if !d.gitSetup {
		return finalizeOrbInit(ctx, d.private, d.namespace, d.orbName, "")
	}

	return applyOrbInitGit(ctx, client, path, d, interactive, orbInitGitContext{
		orbID:     orbID,
		fullName:  fullName,
		namespace: d.namespace,
		orbName:   d.orbName,
		vcsType:   vcsType,
		owner:     owner,
	})
}

type orbInitGitContext struct {
	orbID     string
	fullName  string
	namespace string
	orbName   string
	vcsType   string
	owner     string
}

func applyOrbInitGit(ctx context.Context, client *apiclient.Client, path string, d orbInitDecisions, interactive bool, g orbInitGitContext) error {
	if d.remote == "" {
		return clierrors.New("orb.init_remote_required", "Remote repository required",
			"Pass --remote <url> to set up git non-interactively, or use --skip-git.").
			WithExitCode(clierrors.ExitBadArguments)
	}

	projectName := orbinit.ProjectNameFromRemote(d.remote)

	// Rewrite the template placeholders now that we know the project details.
	if err := orbinit.ApplyTemplate(path, projectName, g.owner, g.orbName, g.namespace); err != nil {
		return clierrors.New("orb.init_template_apply_failed", "Could not update template files",
			err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.Printf(ctx, "Setting up your orb...\n")
	_, w, err := orbinit.InitRepo(path, d.remote, d.branch)
	if err != nil {
		return clierrors.New("orb.init_git_failed", "Could not initialize git repository",
			err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	// Pack the orb source and publish a dev:alpha version.
	packed, err := pack.Pack(filepath.Join(path, "src"))
	if err != nil {
		return clierrors.New("orb.init_pack_failed", "Could not pack orb source",
			err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	if _, err := client.PublishOrbVersion(ctx, apiclient.PublishOrbVersionRequest{
		OrbID:   g.orbID,
		YAML:    packed,
		Version: "dev:alpha",
	}); err != nil {
		return orbAPIErr(err, g.fullName)
	}
	iostream.Printf(ctx, "%s Published %s@dev:alpha\n", iostream.SymbolOK(ctx), g.fullName)

	iostream.Printf(ctx, "\nAn initial commit has been created. Push it with:\n")
	iostream.Printf(ctx, "  git branch -M %s\n", d.branch)
	iostream.Printf(ctx, "  git push origin %s\n", d.branch)

	if interactive {
		iostream.Get(ctx).Confirm(ctx, "I have pushed to my git repository using the above commands")
	}

	if err := client.FollowProject(ctx, g.vcsType, g.owner, projectName); err != nil {
		iostream.Printf(ctx, "%s Could not follow the project automatically: %s\n", iostream.SymbolWarn(ctx), err)
	} else {
		iostream.Printf(ctx, "%s Project followed on CircleCI\n", iostream.SymbolOK(ctx))
	}

	if err := orbinit.CheckoutAlpha(w); err != nil {
		return clierrors.New("orb.init_checkout_failed", "Could not create the alpha branch",
			err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	return finalizeOrbInit(ctx, d.private, g.namespace, g.orbName, projectName)
}

// orbInitDownloadErr wraps a template-download failure in a structured error.
func orbInitDownloadErr(err error) error {
	return clierrors.New("orb.init_template_download_failed", "Could not download orb template",
		err.Error()).
		WithSuggestions("Check your network connection and try again").
		WithExitCode(clierrors.ExitGeneralError)
}

// orbInitLicenseErr wraps a LICENSE-removal failure in a structured error.
func orbInitLicenseErr(err error) error {
	return clierrors.New("orb.init_license_removal_failed", "Could not remove template LICENSE",
		err.Error()).
		WithExitCode(clierrors.ExitGeneralError)
}

// setupPublishingContext creates (or reuses) the orb-publishing context and
// stores the current API token as CIRCLE_TOKEN. Failures are surfaced as
// warnings so they do not abort the whole init.
func setupPublishingContext(ctx context.Context, client *apiclient.Client, orgSlug string) {
	ctxt, err := client.CreateContext(ctx, publishingContextName, orgSlug)
	if err != nil {
		// The context may already exist; try to look it up.
		existing, lerr := client.ListContexts(ctx, orgSlug, publishingContextName)
		if lerr != nil || len(existing) == 0 {
			iostream.Printf(ctx, "%s Could not set up the publishing context: %s\n", iostream.SymbolWarn(ctx), err)
			return
		}
		c := existing[0]
		ctxt = &c
		iostream.Printf(ctx, "Context %q already exists, continuing\n", publishingContextName)
	}

	token := cmdutil.GetConfig(ctx).EffectiveToken()
	if _, err := client.SetContextEnvVar(ctx, ctxt.ID.String(), "CIRCLE_TOKEN", token); err != nil {
		iostream.Printf(ctx, "%s Could not set CIRCLE_TOKEN on the publishing context: %s\n", iostream.SymbolWarn(ctx), err)
	}
}

func finalizeOrbInit(ctx context.Context, private bool, namespace, orbName, projectName string) error {
	if projectName != "" {
		iostream.Printf(ctx, "%s Your orb project is set up. You are now on the alpha branch.\n", iostream.SymbolOK(ctx))
	} else {
		iostream.Printf(ctx, "%s Orb %s/%s is ready.\n", iostream.SymbolOK(ctx), namespace, orbName)
	}
	if !private {
		iostream.Printf(ctx, "Once the first version is published it will appear at: https://circleci.com/developer/orbs/orb/%s/%s\n", namespace, orbName)
		iostream.Printf(ctx, "Orb authoring guide: https://circleci.com/docs/orbs/author/orb-author/\n")
	}
	return nil
}

// splitOrgSlug splits "vcs/org" into its parts. It returns ok=false if the
// slug is not in that form.
func splitOrgSlug(slug string) (vcs, org string, ok bool) {
	parts := strings.SplitN(slug, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
