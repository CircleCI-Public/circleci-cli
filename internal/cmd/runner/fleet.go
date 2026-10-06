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
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

func newFleetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet <command>",
		Short: "View runner fleets",
		Long: heredoc.Doc(`
			View the agents and tasks of your self-hosted runner fleets.

			A fleet is the set of runner agents attached to one resource class, and shares
			that resource class's ID. This uses an experimental API: its output may change.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmdutil.AddGroup(cmd, "General commands",
		newFleetListCmd(),
	)
	cmdutil.AddGroup(cmd, "Targeted commands",
		newFleetGetCmd(),
	)

	return cmd
}

// fleetJSONFields are the fields a fleet carries in --json output.
const fleetJSONFields = "id, resource_class, active_agents, idle_agents, disconnected_agents, agent_count, last_task_claimed_at"

// fleetOutput is a fleet as listed. LastTaskClaimedAt is null for a fleet that has never claimed a task.
type fleetOutput struct {
	ID                 string  `json:"id"`
	ResourceClass      string  `json:"resource_class"`
	ActiveAgents       int     `json:"active_agents"`
	IdleAgents         int     `json:"idle_agents"`
	DisconnectedAgents int     `json:"disconnected_agents"`
	AgentCount         int     `json:"agent_count"`
	LastTaskClaimedAt  *string `json:"last_task_claimed_at"`
}

// fleetDetailOutput is a single fleet, which adds task counts and the agents attached to it.
type fleetDetailOutput struct {
	fleetOutput
	RunningTasks *int               `json:"running_tasks"`
	QueuedTasks  *int               `json:"queued_tasks"`
	Agents       []fleetAgentOutput `json:"agents"`
}

type fleetAgentOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toFleetOutput(f apiclient.RunnerFleet) fleetOutput {
	out := fleetOutput{
		ID:                 f.ID.String(),
		ResourceClass:      f.Attributes.Name,
		ActiveAgents:       f.Attributes.ActiveAgents,
		IdleAgents:         f.Attributes.IdleAgents,
		DisconnectedAgents: f.Attributes.DisconnectedAgents,
		AgentCount:         f.Attributes.AgentCount,
	}
	if t := f.Attributes.LastTaskClaimedAt; t != nil {
		s := timestamp(*t)
		out.LastTaskClaimedAt = &s
	}
	return out
}

// claimedOrNever renders a last-claimed timestamp for a human, which reads better than a blank.
func claimedOrNever(t *string) string {
	if t == nil {
		return "never"
	}
	return *t
}

// fleetUnavailableErr reports runner-admin's 503, which it answers when it cannot read the
// agent status or task counts a fleet is built from. By the time it reaches the CLI the
// request has already been retried.
func fleetUnavailableErr(err error) *clierrors.CLIError {
	detail := "CircleCI could not determine the fleet's agent status or task counts."
	if apiErr, ok := apiclient.ParseError(err); ok && apiErr.Title != "" {
		detail = apiErr.Title
	}
	return clierrors.New("runner.fleet_unavailable", "Fleet status unavailable",
		fmt.Sprintf("%s This is usually temporary.", detail)).
		WithSuggestions(
			"Try again in a few moments",
			"List connected agents without fleet totals with: circleci runner instance list",
		).
		WithExitCode(clierrors.ExitAPIError)
}

// fleetErr maps a failed fleet request to a CLI error. notFound, when set, is the more specific
// answer for a 404 on this scope; the API answers 404 for both an absent resource and one the
// caller cannot see.
func fleetErr(err error, subject string, notFound error) error {
	switch {
	case httpcl.HasStatusCode(err, http.StatusNotFound) && notFound != nil:
		return notFound
	case httpcl.HasStatusCode(err, http.StatusForbidden):
		return runnerNotEnabledErr()
	case httpcl.HasStatusCode(err, http.StatusServiceUnavailable):
		return fleetUnavailableErr(err)
	}
	return apiErr(err, subject)
}

// --- fleet list ---

func newFleetListCmd() *cobra.Command {
	var org string
	var namespace string
	var resourceClass string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List runner fleets",
		Long: heredoc.Doc(`
			List the runner fleets of an organization, a namespace, or one resource class.

			This uses an experimental API: its output may change.

			JSON fields: ` + fleetJSONFields + `
		`),
		Example: heredoc.Doc(`
			# List fleets for the org inferred from the git remote
			$ circleci runner fleet list

			# List the fleets in one namespace
			$ circleci runner fleet list --namespace my-org

			# Show the fleet of one resource class, by name or ID
			$ circleci runner fleet list --resource-class my-org/my-runner

			# Extract the resource classes with no connected agents
			$ circleci runner fleet list --json --jq '.[] | select(.agent_count == 0) | .resource_class'
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runFleetList(ctx, client, org, namespace, resourceClass, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{DefaultsToGitRemote: true})
	cmd.Flags().StringVar(&namespace, "namespace", "", "Filter by namespace (organization)")
	cmd.Flags().StringVar(&resourceClass, "resource-class", "", "Filter by resource class (namespace/name or ID)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	cmd.MarkFlagsMutuallyExclusive("org", "resource-class", "namespace")
	return cmd
}

func runFleetList(ctx context.Context, client *apiclient.Client,
	org, namespace, resourceClass string, jsonOut bool) error {

	var (
		fleets   []apiclient.RunnerFleet
		subject  string
		notFound error
		err      error
	)
	switch {
	case resourceClass != "":
		id, isID, parseErr := parseResourceClassRef(resourceClass)
		if parseErr != nil {
			return parseErr
		}
		subject = resourceClass
		notFound = resourceClassNotFoundErr(resourceClass, isID)
		if isID {
			fleets, err = client.ListRunnerFleetsByResourceClassID(ctx, id)
		} else {
			fleets, err = client.ListRunnerFleetsByResourceClass(ctx, resourceClass)
		}
	case namespace != "":
		subject = namespace
		fleets, err = client.ListRunnerFleetsByNamespace(ctx, namespace)
	default:
		var orgID uuid.UUID
		orgID, err = cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci runner fleet list")
		if err != nil {
			return err
		}
		subject = orgID.String()
		notFound = orgNotAccessibleErr(orgID)
		fleets, err = client.ListRunnerFleetsByOrg(ctx, orgID)
	}
	if err != nil {
		return fleetErr(err, subject, notFound)
	}

	out := make([]fleetOutput, len(fleets))
	for i, f := range fleets {
		out[i] = toFleetOutput(f)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		// The org is usually inferred rather than typed, so echo back only a scope the user named.
		switch {
		case resourceClass != "":
			iostream.Printf(ctx, "No runner fleets found for %s.\n", resourceClass)
		case namespace != "":
			iostream.Printf(ctx, "No runner fleets found for %s.\n", namespace)
		default:
			iostream.Printf(ctx, "No runner fleets found.\n")
		}
		return nil
	}

	table := mdtable.New("Resource Class", "Active", "Idle", "Disconnected", "Agents", "Last Task Claimed")
	for _, f := range out {
		table.Row(f.ResourceClass, fmt.Sprint(f.ActiveAgents), fmt.Sprint(f.IdleAgents),
			fmt.Sprint(f.DisconnectedAgents), fmt.Sprint(f.AgentCount), claimedOrNever(f.LastTaskClaimedAt))
	}
	iostream.PrintMarkdown(ctx, "# Runner Fleets\n"+table.Render())
	return nil
}

// --- fleet get ---

func newFleetGetCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "get <resource-class>",
		Short: "Show a runner fleet",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<resource-class>%[1]s is the runner resource class whose fleet to show,
				in the form %[1]snamespace/name%[1]s (for example, %[1]smy-org/my-runner%[1]s) or as its ID.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Show a fleet's agent counts by state, its running and queued tasks, and the
			agents attached to it. This uses an experimental API: its output may change.

			JSON fields: ` + fleetJSONFields + `, running_tasks, queued_tasks, agents (id, name)
		`),
		Example: heredoc.Doc(`
			# Show the fleet of a resource class
			$ circleci runner fleet get my-org/my-runner

			# Show it by resource class ID
			$ circleci runner fleet get 01234567-89ab-4cde-8f01-23456789abcd

			# Show how many tasks are waiting for an agent
			$ circleci runner fleet get my-org/my-runner --json --jq '.queued_tasks'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "resource-class"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runFleetGet(ctx, client, args[0], jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

func runFleetGet(ctx context.Context, client *apiclient.Client, resourceClass string, jsonOut bool) error {
	id, isID, err := parseResourceClassRef(resourceClass)
	if err != nil {
		return err
	}
	if !isID {
		// A fleet is addressed by its resource class's ID, so resolve a name to it first.
		rc, err := resolveResourceClass(ctx, client, resourceClass)
		if err != nil {
			return err
		}
		if id, err = resourceClassUUID(rc); err != nil {
			return err
		}
	}

	fleet, err := client.GetRunnerFleet(ctx, id)
	if err != nil {
		return fleetErr(err, resourceClass, resourceClassNotFoundErr(resourceClass, isID))
	}

	agents := make([]fleetAgentOutput, len(fleet.References.RunnerAgents))
	for i, a := range fleet.References.RunnerAgents {
		agents[i] = fleetAgentOutput{ID: a.ID.String(), Name: a.Attributes.Name}
	}
	out := fleetDetailOutput{
		fleetOutput:  toFleetOutput(*fleet),
		RunningTasks: fleet.Attributes.RunningTasks,
		QueuedTasks:  fleet.Attributes.QueuedTasks,
		Agents:       agents,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	md := fmt.Sprintf("# Runner Fleet\n- Resource Class: %s\n- ID: `%s`\n- Active: %d\n- Idle: %d\n- Disconnected: %d\n- Agents: %d\n",
		out.ResourceClass, out.ID, out.ActiveAgents, out.IdleAgents, out.DisconnectedAgents, out.AgentCount)
	if out.RunningTasks != nil {
		md += fmt.Sprintf("- Running Tasks: %d\n", *out.RunningTasks)
	}
	if out.QueuedTasks != nil {
		md += fmt.Sprintf("- Queued Tasks: %d\n", *out.QueuedTasks)
	}
	md += fmt.Sprintf("- Last Task Claimed: %s\n", claimedOrNever(out.LastTaskClaimedAt))

	if len(agents) > 0 {
		table := mdtable.New("Agent", "ID")
		for _, a := range agents {
			table.Row(a.Name, a.ID)
		}
		md += "\n## Agents\n" + table.Render()
	}
	if out.AgentCount > len(agents) {
		md += fmt.Sprintf("\nShowing %d of %d agents. List them all with: `circleci runner instance list --resource-class %s`\n",
			len(agents), out.AgentCount, resourceClass)
	}
	iostream.PrintMarkdown(ctx, md)
	return nil
}
