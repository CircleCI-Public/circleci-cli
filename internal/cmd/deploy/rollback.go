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
	"net/http"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// rollbackOptions carries the flags of `circleci deploy rollback`. Component and
// environment accept a name or a UUID: the API addresses both by ID, so a name
// is resolved against the project's components and the org's environments first.
type rollbackOptions struct {
	projectSlug  string
	componentRef string
	envRef       string
	fromVersion  string
	namespace    string
	reason       string
	params       []string
	checkoutRef  string
	configRef    string
	force        bool
	jsonOut      bool
}

// rollbackEntry is the command's output. ID is a handle to the work carrying the
// rollback out — a pipeline run or a release-agent command, told apart by
// RollbackType — so a caller has something to poll.
type rollbackEntry struct {
	ID            string `json:"id"`
	RollbackType  string `json:"rollback_type"`
	ProjectID     string `json:"project_id"`
	ComponentID   string `json:"component_id"`
	EnvironmentID string `json:"environment_id"`
}

func newRollbackCmd() *cobra.Command {
	var opts rollbackOptions

	cmd := &cobra.Command{
		Use:   "rollback <target-version>",
		Short: "Roll back a deployed component to an earlier version",
		Annotations: map[string]string{
			"destructiveHint": "true",
		},
		Long: heredoc.Doc(`
			Roll back a deployed component to an earlier version. --from must name
			the version currently deployed; omit it to use the latest deployed one.

			JSON fields: id, rollback_type, project_id, component_id, environment_id
		`),
		Example: heredoc.Doc(`
			# Roll production back to 1.2.0 (with confirmation)
			$ circleci deploy rollback 1.2.0 --component web-frontend --environment production

			# Assert the version being replaced, and skip the prompt
			$ circleci deploy rollback 1.2.0 --component web-frontend --environment production --from 1.3.0 --force

			# Roll back with a reason and a rollback-pipeline parameter
			$ circleci deploy rollback 1.2.0 --component web-frontend --environment production --reason "bad release" --param notify=true
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "target-version"); cliErr != nil {
				return cliErr
			}
			if opts.componentRef == "" {
				return cmdutil.RequireFlag("component")
			}
			if opts.envRef == "" {
				return cmdutil.RequireFlag("environment")
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runRollback(ctx, client, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVar(&opts.componentRef, "component", "", "Deploy component name or ID (required)")
	cmd.Flags().StringVar(&opts.envRef, "environment", "", "Deploy environment name or ID (required)")
	cmd.Flags().StringVar(&opts.fromVersion, "from", "", "Version being replaced; defaults to the latest deployed")
	cmd.Flags().StringVar(&opts.namespace, "namespace", "", `Namespace scoping the component (default "default")`)
	cmd.Flags().StringVar(&opts.reason, "reason", "", "Why the rollback was requested; recorded in the audit log")
	cmd.Flags().StringArrayVar(&opts.params, "param", nil, "Rollback pipeline parameter as key=value (repeatable)")
	cmd.Flags().StringVar(&opts.checkoutRef, "checkout-ref", "", "Git ref the rollback pipeline checks out")
	cmd.Flags().StringVar(&opts.configRef, "config-ref", "", "Git ref the rollback pipeline's config is read from")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "skip confirmation prompt")
	cmdutil.AddJSONFlag(cmd, &opts.jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runRollback(ctx context.Context, client *apiclient.Client, targetVersion string, opts rollbackOptions) error {
	projectSlug := opts.projectSlug
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci deploy rollback "+targetVersion+" --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	proj, err := client.GetProjectBySlug(ctx, projectSlug)
	if err != nil {
		return cmdutil.APIErr(err, projectSlug,
			"project.not_found", "No project found for %q.",
			"Run 'circleci project link' to bind this repository to a CircleCI project",
			"Check the project slug and try again",
			"Use 'circleci project list' to see followed projects")
	}

	component, err := resolveComponent(ctx, client, proj.OrgID.String(), proj.ID.String(), opts.componentRef)
	if err != nil {
		return err
	}
	environment, err := resolveEnvironment(ctx, client, proj.OrgID.String(), opts.envRef)
	if err != nil {
		return err
	}

	fromVersion := opts.fromVersion
	if fromVersion == "" {
		fromVersion, err = deployedVersion(ctx, client, component, environment)
		if err != nil {
			return err
		}
	}

	parameters, err := parseRollbackParams(opts.params)
	if err != nil {
		return err
	}

	streams := iostream.Get(ctx)
	summary := fmt.Sprintf("%s in %s from %s to %s",
		component.Attributes.Name, environment.Attributes.Name, fromVersion, targetVersion)
	if err := cmdutil.ConfirmOrForce(ctx, streams, opts.force,
		fmt.Sprintf("Roll back %s?", summary),
		clierrors.New("deploy.rollback.aborted", "Rollback aborted",
			"The rollback was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("deploy.rollback.requires_force", "Rollback requires --force",
			fmt.Sprintf("Rolling back %s changes what is deployed.", summary)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	rollback, err := client.RollbackProject(ctx, proj.ID.String(), apiclient.V3RollbackRequest{
		ComponentID:    component.ID,
		EnvironmentID:  environment.ID,
		Namespace:      opts.namespace,
		CurrentVersion: fromVersion,
		TargetVersion:  targetVersion,
		Reason:         opts.reason,
		Parameters:     parameters,
		CheckoutRef:    opts.checkoutRef,
		ConfigRef:      opts.configRef,
	})
	if err != nil {
		return rollbackErr(err, component, environment, summary)
	}

	entry := rollbackEntry{
		ID:            rollback.ID,
		RollbackType:  rollback.Attributes.RollbackType,
		ProjectID:     rollback.References.Project.ID,
		ComponentID:   rollback.References.DeployComponent.ID,
		EnvironmentID: rollback.References.DeployEnvironment.ID,
	}

	if opts.jsonOut {
		return iostream.PrintJSON(ctx, entry)
	}

	iostream.Printf(ctx, "%s Rolling back %s\n", iostream.SymbolOK(ctx), summary)
	switch entry.RollbackType {
	case apiclient.RollbackTypePipeline:
		iostream.Printf(ctx, "Pipeline run %s — follow it with: circleci run get %s\n", entry.ID, entry.ID)
	case apiclient.RollbackTypeAgent:
		iostream.Printf(ctx, "Release agent command %s — the agent in %s applies it\n",
			entry.ID, environment.Attributes.Name)
	default:
		iostream.Printf(ctx, "%s %s\n", entry.RollbackType, entry.ID)
	}
	return nil
}

// resolveComponent resolves a component name or UUID to the component itself.
// A UUID is fetched directly; a name is matched against the project's
// components, since the rollback endpoint addresses components by ID.
func resolveComponent(ctx context.Context, client *apiclient.Client, orgID, projectID, ref string) (*apiclient.V3Component, error) {
	if _, err := uuid.Parse(ref); err == nil {
		component, err := client.GetComponent(ctx, ref)
		if err != nil {
			return nil, cmdutil.APIErr(err, ref,
				"deploy.component.not_found", "No deploy component found for %q.",
				"Use 'circleci deploy component list' to see the project's components")
		}
		return component, nil
	}

	components, err := client.ListComponents(ctx, orgID, projectID, 0)
	if err != nil {
		return nil, cmdutil.APIErr(err, ref,
			"deploy.component.not_found", "No deploy components found to resolve %q against.",
			"Check that CircleCI Deploys is configured for this project")
	}
	var matches []apiclient.V3Component
	for _, c := range components {
		if c.Attributes.Name == ref {
			matches = append(matches, c)
		}
	}
	switch len(matches) {
	case 1:
		return &matches[0], nil
	case 0:
		return nil, clierrors.New("deploy.component.not_found", "Not found",
			fmt.Sprintf("No deploy component named %q in this project.", ref)).
			WithSuggestions(
				"Use 'circleci deploy component list' to see the project's components",
				"Or pass the component ID instead of its name",
			).
			WithExitCode(clierrors.ExitNotFound)
	default:
		return nil, clierrors.New("deploy.component.ambiguous", "Ambiguous component",
			fmt.Sprintf("%d deploy components in this project are named %q.", len(matches), ref)).
			WithSuggestions("Pass the component ID instead of its name: " + componentIDs(matches)).
			WithExitCode(clierrors.ExitBadArguments)
	}
}

func componentIDs(components []apiclient.V3Component) string {
	ids := make([]string, len(components))
	for i, c := range components {
		ids[i] = c.ID
	}
	return strings.Join(ids, ", ")
}

// resolveEnvironment resolves an environment name or UUID to the environment
// itself. Environments are org-scoped, so a name is matched across the org.
func resolveEnvironment(ctx context.Context, client *apiclient.Client, orgID, ref string) (*apiclient.V3Environment, error) {
	if _, err := uuid.Parse(ref); err == nil {
		environment, err := client.GetEnvironment(ctx, ref)
		if err != nil {
			return nil, cmdutil.APIErr(err, ref,
				"deploy.environment.not_found", "No deploy environment found for %q.",
				"Use 'circleci deploy environment list' to see the organization's environments")
		}
		return environment, nil
	}

	environments, err := client.ListEnvironments(ctx, orgID, 0)
	if err != nil {
		return nil, cmdutil.APIErr(err, ref,
			"deploy.environment.not_found", "No deploy environments found to resolve %q against.",
			"Check that CircleCI Deploys is configured for this organization")
	}
	for _, e := range environments {
		if e.Attributes.Name == ref {
			return &e, nil
		}
	}
	return nil, clierrors.New("deploy.environment.not_found", "Not found",
		fmt.Sprintf("No deploy environment named %q in this organization.", ref)).
		WithSuggestions(
			"Use 'circleci deploy environment list' to see the organization's environments",
			"Or pass the environment ID instead of its name",
		).
		WithExitCode(clierrors.ExitNotFound)
}

// deployedVersion reports the version the component is currently running in the
// environment, used when --from is omitted. The versions endpoint returns them
// most recently deployed first, so the first one is what is deployed there. The
// API re-checks it, so a stale answer fails the rollback rather than moving the
// wrong version.
func deployedVersion(ctx context.Context, client *apiclient.Client, component *apiclient.V3Component, environment *apiclient.V3Environment) (string, error) {
	versions, err := client.ListComponentVersions(ctx, component.ID, environment.ID, 1)
	if err != nil {
		return "", cmdutil.APIErr(err, component.Attributes.Name,
			"deploy.version.not_found", "No recorded versions found for %q.",
			"Pass the currently deployed version with --from")
	}
	if len(versions) == 0 || versions[0].Attributes.Name == "" {
		return "", clierrors.New("deploy.rollback.no_deployed_version", "No deployed version found",
			fmt.Sprintf("%s has no recorded version in %s, so there is nothing to roll back.",
				component.Attributes.Name, environment.Attributes.Name)).
			WithSuggestions(
				"Pass the currently deployed version with --from",
				fmt.Sprintf("Use 'circleci deploy version list %s' to see recorded versions", component.ID),
			).
			WithExitCode(clierrors.ExitNotFound)
	}
	return versions[0].Attributes.Name, nil
}

// parseRollbackParams converts ["key=value", ...] into the parameters map the
// rollback pipeline is run with, coercing values to bool or int where
// unambiguous so typed pipeline parameters are satisfied.
func parseRollbackParams(raw []string) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]any, len(raw))
	for _, p := range raw {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, clierrors.New("deploy.rollback.invalid_param", "Invalid parameter",
				fmt.Sprintf("%q is not in key=value format", p)).
				WithExitCode(clierrors.ExitBadArguments)
		}
		switch v {
		case "true":
			out[k] = true
		case "false":
			out[k] = false
		default:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				out[k] = n
			} else {
				out[k] = v
			}
		}
	}
	return out, nil
}

// rollbackErr maps the rollback endpoint's failures onto structured errors. A
// 409 means a command for this component instance is already being handled, and
// a 400 means the API disagrees about the versions — most often because --from
// is not what is deployed.
func rollbackErr(err error, component *apiclient.V3Component, environment *apiclient.V3Environment, summary string) error {
	if httpcl.HasStatusCode(err, http.StatusConflict) {
		return clierrors.New("deploy.rollback.in_progress", "Rollback already in progress",
			fmt.Sprintf("A command for %s in %s is already being handled.",
				component.Attributes.Name, environment.Attributes.Name)).
			WithSuggestions("Wait for the in-flight command to finish, then try again").
			WithExitCode(clierrors.ExitAPIError)
	}
	if httpcl.HasStatusCode(err, http.StatusBadRequest) {
		detail := fmt.Sprintf("CircleCI rejected the rollback of %s.", summary)
		if apiErr, ok := apiclient.ParseError(err); ok {
			detail += "\n" + apiErr.Message()
		}
		return clierrors.New("deploy.rollback.rejected", "Rollback rejected", detail).
			WithSuggestions(
				"--from must name the version currently deployed",
				fmt.Sprintf("Use 'circleci deploy version list %s --environment %s' to see recorded versions",
					component.ID, environment.ID),
			).
			WithExitCode(clierrors.ExitBadArguments)
	}
	return cmdutil.APIErr(err, summary,
		"deploy.rollback.not_found", "No deployed component instance found for %q.",
		"Check that the component is deployed to that environment",
		"Pass --namespace when the component is deployed outside the default namespace")
}
