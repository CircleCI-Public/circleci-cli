package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
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

	cmd.AddCommand(newInstanceListCmd())

	return cmd
}

func newInstanceListCmd() *cobra.Command {
	var resourceClass string
	var namespace string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List connected runner instances",
		Long: heredoc.Doc(`
			List CircleCI runner instances currently connected to your organization.

			Optionally filter by resource class to see only instances of a specific type.

			The STATUS column is derived from last_connected_at:
			  online   — connected within the last 2 minutes
			  idle     — last seen 2–30 minutes ago
			  offline  — last seen more than 30 minutes ago

			JSON fields: resource_class, hostname, name, version, ip, status,
			             first_connected, last_connected, last_used
		`),
		Example: heredoc.Doc(`
			# List all connected runner instances
			$ circleci runner instance list

			# List instances for a specific resource class
			$ circleci runner instance list --resource-class my-org/my-runner

			# Output as JSON
			$ circleci runner instance list --resource-class my-org/my-runner --json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			return runInstanceList(ctx, streams, resourceClass, namespace, jsonOut)
		},
	}

	cmd.Flags().StringVar(&resourceClass, "resource-class", "", "Filter by resource class (namespace/name)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Filter by namespace (organization); defaults to git remote")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
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

func runInstanceList(ctx context.Context, streams iostream.Streams, resourceClass, namespace string, jsonOut bool) error {
	client, cliErr := cmdutil.LoadClient()
	if cliErr != nil {
		return cliErr
	}

	if resourceClass == "" && namespace == "" {
		ns, err := gitremote.DetectNamespace()
		if err != nil {
			return clierrors.New("runner.namespace_required", "Namespace required",
				"Could not detect organization namespace from git remote.").
				WithSuggestions(
					"Specify a namespace: circleci runner instance list --namespace <your-org>",
					"Or filter by resource class: circleci runner instance list --resource-class <namespace/name>",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		namespace = ns
	}

	instances, err := client.ListRunnerInstances(ctx, resourceClass, namespace)
	if err != nil {
		// 404 on instance list means no agents are connected, not that runner is unavailable.
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			instances = nil
		} else {
			return apiErr(err, resourceClass)
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
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	if len(out) == 0 {
		if resourceClass != "" {
			streams.Printf("No runner instances found for %s.\n", resourceClass)
		} else {
			streams.Printf("No runner instances found.\n")
		}
		return nil
	}

	for _, inst := range out {
		streams.Printf("%-40s  %-20s  %-7s  %s\n",
			inst.ResourceClass, inst.Hostname, inst.Status, inst.LastConnected)
	}
	return nil
}
