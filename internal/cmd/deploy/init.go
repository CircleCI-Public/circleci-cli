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

package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/deployinit"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newInitCmd() *cobra.Command {
	var (
		configPath string
		component  string
		defEnv     string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Instrument your config for deploy tracking",
		Long: heredoc.Doc(`
			Set up deploy markers in .circleci/config.yml.

			Scans the config for deploy-like jobs (names containing deploy,
			release, publish, or ship), prompts for your service name and
			target environment, then adds a log step to each deploy job.

			The command is idempotent: running it twice on the same config
			makes no duplicate changes.

			Works offline — no API calls required.
		`),
		Example: heredoc.Doc(`
			# Interactive setup in the current repo
			$ circleci deploy init

			# Skip the service-name prompt
			$ circleci deploy init --component api

			# Fully non-interactive (supply both answers as flags)
			$ circleci deploy init --component api --environment production

			# Use a non-default config location
			$ circleci deploy init --pipeline-config path/to/config.yml
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return runInit(ctx, configPath, component, defEnv)
		},
	}

	cmd.Flags().StringVar(&configPath, "pipeline-config", ".circleci/config.yml", "Path to CircleCI pipeline config file")
	cmd.Flags().StringVar(&component, "component", "", "Service/component name (skips prompt)")
	cmd.Flags().StringVar(&defEnv, "environment", "", "Default environment for jobs whose target can't be inferred (skips prompt)")

	return cmd
}

func runInit(ctx context.Context, configPath, component, defEnv string) error {
	iostream.Printf(ctx, "Scanning %s...\n\n", configPath)

	result, err := deployinit.Detect(configPath)
	if err != nil {
		return clierrors.New("deploy.init_scan_failed", "Could not scan config",
			err.Error()).
			WithSuggestions(
				"Check that the file exists and is valid YAML",
				"Run: circleci config validate",
			).
			WithExitCode(clierrors.ExitGeneralError)
	}

	if len(result.Jobs) == 0 {
		iostream.ErrPrintln(ctx, "No deploy jobs found in "+configPath+".")
		iostream.ErrPrintln(ctx, "")
		iostream.ErrPrintln(ctx, "Add a job with a name like deploy, release, publish, or ship,")
		iostream.ErrPrintln(ctx, "then run this command again.")
		return clierrors.New("deploy.init_no_jobs", "No deploy jobs found",
			"No jobs with deploy-like names were detected in the config.").
			WithExitCode(clierrors.ExitNotFound)
	}

	iostream.Printf(ctx, "Found %d deploy job(s):\n", len(result.Jobs))
	for _, j := range result.Jobs {
		branch := j.Branch
		if branch == "" {
			branch = "any"
		}
		iostream.Printf(ctx, "  %-30s runs on: %s\n", j.Name, branch)
	}
	iostream.Printf(ctx, "\n")

	// Determine component name.
	if component == "" {
		defaultComponent := repoName()
		if iostream.IsInteractive(ctx) {
			val, err := iostream.PromptText(ctx, "What's this service called?", defaultComponent)
			if err != nil {
				return clierrors.New("deploy.init_cancelled", "Cancelled", "Prompt was cancelled.").
					WithExitCode(clierrors.ExitCancelled)
			}
			if val == "" {
				val = defaultComponent
			}
			component = val
		} else {
			component = defaultComponent
		}
	}

	// Determine environment for each job — infer where possible, ask once for the rest.
	needsPrompt := false
	for _, j := range result.Jobs {
		if j.InferredEnv == "" {
			needsPrompt = true
			break
		}
	}
	if needsPrompt && defEnv == "" {
		if iostream.IsInteractive(ctx) {
			val, err := iostream.PromptText(ctx, "What environment do your deploy jobs target?", "production")
			if err != nil {
				return clierrors.New("deploy.init_cancelled", "Cancelled", "Prompt was cancelled.").
					WithExitCode(clierrors.ExitCancelled)
			}
			if val == "" {
				val = "production"
			}
			defEnv = val
		} else {
			defEnv = "production"
		}
	}

	// Build patch jobs list.
	patchJobs := make([]deployinit.PatchJob, len(result.Jobs))
	for i, j := range result.Jobs {
		env := j.InferredEnv
		if env == "" {
			env = defEnv
		}
		patchJobs[i] = deployinit.PatchJob{Name: j.Name, Environment: env}
	}

	iostream.Printf(ctx, "Updating %s...\n\n", configPath)

	changed, err := deployinit.Patch(configPath, deployinit.PatchInput{
		Component: component,
		Jobs:      patchJobs,
	}, result.HasDeploys)
	if err != nil {
		return clierrors.New("deploy.init_patch_failed", "Could not patch config",
			err.Error()).
			WithSuggestions("Check that the file is writable and is valid YAML").
			WithExitCode(clierrors.ExitGeneralError)
	}

	if !changed {
		iostream.Printf(ctx, "Already configured — no changes needed.\n")
		return nil
	}

	iostream.Printf(ctx, "Done. Your next deploy will appear at:\n")
	iostream.Printf(ctx, "→ %s\n\n", deploysURL())
	iostream.Printf(ctx, "Commit and push %s to activate.\n", filepath.Base(configPath))
	return nil
}

// repoName returns the current directory's base name as a default component name.
func repoName() string {
	info, err := gitremote.Detect()
	if err != nil {
		return "my-service"
	}
	parts := strings.Split(info.Slug, "/")
	if len(parts) == 3 {
		return parts[2]
	}
	return "my-service"
}

// deploysURL returns the org deploys dashboard URL.
func deploysURL() string {
	info, err := gitremote.Detect()
	if err != nil {
		return "https://app.circleci.com/deploys"
	}
	parts := strings.Split(info.Slug, "/")
	if len(parts) != 3 {
		return "https://app.circleci.com/deploys"
	}
	provider := providerName(parts[0])
	org := parts[1]
	return fmt.Sprintf("https://app.circleci.com/deploys/%s/%s", provider, org)
}

func providerName(slug string) string {
	switch slug {
	case "gh":
		return "github"
	case "bb":
		return "bitbucket"
	case "gl":
		return "gitlab"
	default:
		return slug
	}
}
