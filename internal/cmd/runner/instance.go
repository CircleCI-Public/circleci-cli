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
	"net/http"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
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

			The organization is inferred from the current git repository's remote
			unless overridden with --org, which accepts an org slug (e.g. gh/myorg)
			or an org UUID. Optionally filter by resource class to see only
			instances of a specific type.

			The STATUS column is derived from last_connected_at:
			  online   — connected within the last 2 minutes
			  idle     — last seen 2–30 minutes ago
			  offline  — last seen more than 30 minutes ago

			JSON fields: resource_class, hostname, name, version, ip, status,
			             first_connected, last_connected, last_used
		`),
		Example: heredoc.Doc(`
			# List connected instances for the org inferred from the git remote
			$ circleci runner instance list

			# List instances for a specific organization (slug or UUID)
			$ circleci runner instance list --org gh/my-org

			# List instances for a specific resource class
			$ circleci runner instance list --resource-class my-org/my-runner

			# Output as JSON
			$ circleci runner instance list --org gh/my-org --json
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
	return cmd
}

type instanceOutput struct {
	ResourceClass  string `json:"resource_class"`
	Hostname       string `json:"hostname"`
	Name           string `json:"name"`
	Version        string `json:"version"`
	IP             string `json:"ip"`
	Status         string `json:"status"`
	FirstConnected string `json:"first_connected"`
	LastConnected  string `json:"last_connected"`
	LastUsed       string `json:"last_used"`
}

// instanceStatus derives a human-readable liveness status from last_connected_at.
// The CircleCI runner API does not expose an explicit status field.
func instanceStatus(lastConnectedAt string) string {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02T15:04:05Z",
	}
	var t time.Time
	for _, f := range formats {
		if parsed, err := time.Parse(f, lastConnectedAt); err == nil {
			t = parsed
			break
		}
	}
	if t.IsZero() {
		return "unknown"
	}
	age := time.Since(t)
	switch {
	case age < 2*time.Minute:
		return "online"
	case age < 30*time.Minute:
		return "idle"
	default:
		return "offline"
	}
}

func runInstanceList(ctx context.Context, client *apiclient.Client, org, resourceClass, namespace string, jsonOut bool) error {
	// List by org UUID when --org (slug or UUID) is given or can be inferred
	// from the git remote. When only a resource-class or namespace filter is
	// supplied, keep the legacy filter-based listing and skip org resolution.
	var (
		instances []apiclient.RunnerInstance
		subject   = resourceClass
		err       error
	)
	if org != "" || (resourceClass == "" && namespace == "") {
		var orgID uuid.UUID
		orgID, err = cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci runner instance list")
		if err != nil {
			return err
		}
		subject = orgID.String()
		instances, err = client.ListRunnerInstancesByOrg(ctx, orgID)
	} else {
		instances, err = client.ListRunnerInstances(ctx, resourceClass, namespace)
	}
	if err != nil {
		// 404 on instance list means no agents are connected, not that runner is unavailable.
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			instances = nil
		} else {
			return apiErr(err, subject)
		}
	}

	out := make([]instanceOutput, len(instances))
	for i, inst := range instances {
		out[i] = instanceOutput{
			ResourceClass:  inst.ResourceClass,
			Hostname:       inst.Hostname,
			Name:           inst.Name,
			Version:        inst.Version,
			IP:             inst.IP,
			Status:         instanceStatus(inst.LastConnected),
			FirstConnected: inst.FirstConnected,
			LastConnected:  inst.LastConnected,
			LastUsed:       inst.LastUsed,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		if resourceClass != "" {
			iostream.Printf(ctx, "No runner instances found for %s.\n", resourceClass)
		} else {
			iostream.Printf(ctx, "No runner instances found.\n")
		}
		return nil
	}

	table := mdtable.New("Resource Class", "Hostname", "Status", "Last Connected")
	for _, inst := range out {
		table.Row(inst.ResourceClass, inst.Hostname, inst.Status, inst.LastConnected)
	}
	iostream.PrintMarkdown(ctx, "# Runner Instances\n"+table.Render())
	return nil
}
