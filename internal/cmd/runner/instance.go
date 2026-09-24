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
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

func newInstanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instance <command>",
		Short: "Manage runner instances",
		Long: heredoc.Doc(`
			View CircleCI runner instances connected to your organization.

			Instances are live runner agents currently connected to CircleCI.
		`),
	}

	cmdutil.AddGroup(cmd, "General commands",
		newInstanceListCmd(),
	)

	return cmd
}

func newInstanceListCmd() *cobra.Command {
	var org string
	var resourceClass string
	var namespace string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List connected runner instances",
		Long: heredoc.Doc(`
			List CircleCI runner instances currently connected to your organization.

			STATUS is derived from last_connected_at (online under 2 minutes, idle
			under 30, offline beyond). BUSY says whether the instance holds a task.
			Hostname, IP and last-used time are not reported by the agents API.

			JSON fields: id, resource_class, resource_class_id, name, version, status, is_busy, first_connected_at, last_connected_at
		`),
		Example: heredoc.Doc(`
			# List connected instances for the org inferred from the git remote
			$ circleci runner instance list

			# List instances for a specific organization (slug or UUID)
			$ circleci runner instance list --org gh/my-org

			# List instances for a specific resource class
			$ circleci runner instance list --resource-class my-org/my-runner

			# Extract just the names
			$ circleci runner instance list --json --jq '.[].name'
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runInstanceList(ctx, client, org, resourceClass, namespace, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{DefaultsToGitRemote: true})
	cmd.Flags().StringVar(&resourceClass, "resource-class", "", "Filter by resource class (namespace/name)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Filter by namespace (organization)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	cmd.MarkFlagsMutuallyExclusive("org", "resource-class", "namespace")
	return cmd
}

type instanceOutput struct {
	ID               string `json:"id"`
	ResourceClass    string `json:"resource_class"`
	ResourceClassID  string `json:"resource_class_id"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	Status           string `json:"status"`
	IsBusy           bool   `json:"is_busy"`
	FirstConnectedAt string `json:"first_connected_at"`
	LastConnectedAt  string `json:"last_connected_at"`
}

// instanceStatus derives a liveness status from last_connected_at. It is independent of is_busy,
// which says whether a connected instance holds a task rather than whether it is still there.
func instanceStatus(lastConnected time.Time) string {
	if lastConnected.IsZero() {
		return "unknown"
	}
	switch age := time.Since(lastConnected); {
	case age < 2*time.Minute:
		return "online"
	case age < 30*time.Minute:
		return "idle"
	default:
		return "offline"
	}
}

// timestamp renders an API timestamp, leaving an absent one empty rather than printing the zero time.
func timestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func runInstanceList(ctx context.Context, client *apiclient.Client,
	org, resourceClass, namespace string, jsonOut bool) error {

	var (
		agents  []apiclient.RunnerAgent
		subject string
		err     error
	)
	switch {
	case resourceClass != "":
		subject = resourceClass
		agents, err = client.ListRunnerAgentsByResourceClass(ctx, resourceClass)
	case namespace != "":
		subject = namespace
		agents, err = client.ListRunnerAgentsByNamespace(ctx, namespace)
	default:
		var orgID uuid.UUID
		orgID, err = cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci runner instance list")
		if err != nil {
			return err
		}
		subject = orgID.String()
		agents, err = client.ListRunnerAgentsByOrg(ctx, orgID)
	}
	if err != nil {
		return apiErr(err, subject)
	}

	out := make([]instanceOutput, len(agents))
	for i, a := range agents {
		out[i] = instanceOutput{
			ID:               a.ID.String(),
			ResourceClass:    a.References.ResourceClass.Attributes.ResourceClass,
			ResourceClassID:  a.References.ResourceClass.ID.String(),
			Name:             a.Attributes.Name,
			Version:          a.Attributes.Version,
			Status:           instanceStatus(a.Attributes.LastConnectedAt),
			IsBusy:           a.Attributes.IsBusy,
			FirstConnectedAt: timestamp(a.Attributes.FirstConnectedAt),
			LastConnectedAt:  timestamp(a.Attributes.LastConnectedAt),
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		// The org is usually inferred rather than typed, so echo back only a scope the user named.
		switch {
		case resourceClass != "":
			iostream.Printf(ctx, "No runner instances found for %s.\n", resourceClass)
		case namespace != "":
			iostream.Printf(ctx, "No runner instances found for %s.\n", namespace)
		default:
			iostream.Printf(ctx, "No runner instances found.\n")
		}
		return nil
	}

	table := mdtable.New("Resource Class", "Name", "Status", "Busy", "Last Connected")
	for _, inst := range out {
		table.Row(inst.ResourceClass, inst.Name, inst.Status, yesNo(inst.IsBusy), inst.LastConnectedAt)
	}
	iostream.PrintMarkdown(ctx, "# Runner Instances\n"+table.Render())
	return nil
}
