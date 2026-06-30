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

	tea "charm.land/bubbletea/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/cmd/cmdauth"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configgen"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
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
}

// Run scans a repository, verifies its tests, generates a starter config when
// needed, and ensures the CLI has an authenticated CircleCI session.
func Run(ctx context.Context, dir string, opts Options) error {
	m, err := resolveMode(ctx, opts)
	if err != nil {
		return err
	}

	if m == modeSignup {
		if err := cmdauth.SignupIfNeeded(ctx, opts.NoBrowser, opts.SecureStorage, opts.ConfigPath); err != nil {
			return err
		}
		return postSignupGuidance(ctx)
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

	if err := cmdauth.SignupIfNeeded(ctx, opts.NoBrowser, opts.SecureStorage, opts.ConfigPath); err != nil {
		return err
	}

	return postSignupGuidance(ctx)
}

// postSignupGuidance offers inline project creation and prints a follow-up
// message after the user has authenticated.
//
// Errors are handled gracefully: project creation failure falls through to
// manual guidance rather than failing the onboard command.
func postSignupGuidance(ctx context.Context) error {
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

	var orgSlug string
	switch {
	case len(orgs) == 1:
		orgSlug = orgs[0].Slug
		iostream.Printf(ctx, "%s Using organization %s\n", iostream.SymbolOK(ctx), orgSlug)
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
		orgSlug = orgs[idx].Slug
	default:
		printManualGuidance(ctx)
		return nil
	}

	vcs, orgName, err := org.ParseSlug(orgSlug)
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

	proj, err := client.CreateProject(ctx, vcs, orgName, name)
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not create project: %s\n", iostream.SymbolWarn(ctx), err)
		printManualGuidance(ctx)
		return nil
	}

	iostream.Printf(ctx, "%s Project created: %s\n", iostream.SymbolOK(ctx), proj.Name)
	iostream.Printf(ctx, "  Organization: %s\n", proj.OrganizationName)
	if pipelinesURL, err := cmdutil.RunSlugURL(appURL, proj.Slug); err == nil {
		iostream.Printf(ctx, "  Pipelines: %s\n", pipelinesURL)
	}
	iostream.Printf(ctx, "\nCommit .circleci/config.yml. After your project is connected in CircleCI, pushing will start your first pipeline.\n")
	return nil
}

func printManualGuidance(ctx context.Context) {
	iostream.Printf(ctx, "\nRun 'circleci project create' to connect this repo to CircleCI.\n")
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
