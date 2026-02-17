package telemetry

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// GetCommandInformation takes a cobra Command and creates a telemetry.CommandInfo.
// Only flags explicitly set by the user are included (via pflag.Visit, not VisitAll).
// Values are only sent for flags in safeValueFlags; all others receive an empty string
// to avoid leaking sensitive data (tokens, org slugs, branch names, etc.).
// The getParent parameter is retained for API compatibility but has no effect.
func GetCommandInformation(cmd *cobra.Command, _ bool) CommandInfo {
	localArgs := map[string]string{}

	// Build a set of inherited flag names so we can exclude them.
	// We only want flags defined on this command itself (local or persistent),
	// not flags inherited from parent commands (e.g. --token, --host).
	inherited := map[string]struct{}{}
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		inherited[f.Name] = struct{}{}
	})

	// cmd.Flags() is the parsed FlagSet — it reliably reflects Changed state for
	// both regular and PersistentFlags defined on this command, unlike LocalFlags()
	// which uses a lazily-built cache that may not propagate Changed correctly.
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if _, isInherited := inherited[flag.Name]; isInherited {
			return
		}
		if safeValueFlags[flag.Name] {
			localArgs[flag.Name] = flag.Value.String()
		} else {
			localArgs[flag.Name] = ""
		}
	})

	return CommandInfo{
		Name:      cmd.Name(),
		LocalArgs: localArgs,
	}
}

// UsedFlagNames returns the names of flags explicitly set by the user.
// Values are never included to avoid leaking sensitive data (tokens, org slugs, etc).
func UsedFlagNames(cmd *cobra.Command) []string {
	var names []string
	cmd.Flags().Visit(func(f *pflag.Flag) {
		names = append(names, f.Name)
	})
	return names
}

// safeValueFlags is the allowlist of flag names whose values are safe and
// analytically useful to send (booleans, known enums — never free-form strings).
var safeValueFlags = map[string]bool{
	"json":           true,
	"force":          true,
	"vcs-type":       true,
	"generate-token": true,
}

// TrackWorkflowStep emits a cli_workflow_step event for a named step within a
// multi-step command workflow. It lives in the telemetry package so that all
// subpackages (cmd/project, cmd/pipeline, cmd/trigger, etc.) can call it without
// creating import cycles back to the cmd package.
func TrackWorkflowStep(client Client, workflow, step, invocationID string, extra map[string]interface{}) {
	if client == nil || !client.Enabled() {
		return
	}

	props := map[string]interface{}{
		"workflow":      workflow,
		"step":          step,
		"invocation_id": invocationID,
	}
	for k, v := range extra {
		props[k] = v
	}

	_ = client.Track(Event{
		Object:     "cli_workflow_step",
		Action:     fmt.Sprintf("%s.%s", workflow, step),
		Properties: props,
	})
}
