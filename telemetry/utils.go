package telemetry

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// GetCommandInformation takes a cobra Command and creates a telemetry.CommandInfo.
// Only flags explicitly set by the user are included (via pflag.Visit, not VisitAll).
// Values are sent for all flags except those in sensitiveFlags, which are redacted.
// If getParent is true, explicitly-set flags from the parent command are also included
// (child flags take precedence over parent flags with the same name).
func GetCommandInformation(cmd *cobra.Command, getParent bool) CommandInfo {
	localArgs := map[string]string{}

	// Build a set of inherited flag names so we can exclude them.
	// We only want flags defined on this command itself (local or persistent),
	// not flags inherited from parent commands (e.g. --token, --host).
	inherited := map[string]struct{}{}
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		inherited[f.Name] = struct{}{}
	})

	// If getParent is true, collect explicitly-set flags from the parent command first.
	// These can be overwritten by flags from the child command below.
	if getParent {
		if parent := cmd.Parent(); parent != nil {
			parent.Flags().Visit(func(flag *pflag.Flag) {
				if sensitiveFlags[flag.Name] {
					localArgs[flag.Name] = redactedValue
				} else {
					localArgs[flag.Name] = flag.Value.String()
				}
			})
		}
	}

	// cmd.Flags() is the parsed FlagSet — it reliably reflects Changed state for
	// both regular and PersistentFlags defined on this command, unlike LocalFlags()
	// which uses a lazily-built cache that may not propagate Changed correctly.
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if _, isInherited := inherited[flag.Name]; isInherited {
			return
		}
		if sensitiveFlags[flag.Name] {
			localArgs[flag.Name] = redactedValue
		} else {
			localArgs[flag.Name] = flag.Value.String()
		}
	})

	return CommandInfo{
		Name:      cmd.Name(),
		LocalArgs: localArgs,
	}
}

// UsedFlagValues returns a map of flag names to values for flags explicitly set
// by the user. Sensitive flags (tokens, secrets) have their values redacted.
func UsedFlagValues(cmd *cobra.Command) map[string]string {
	flags := map[string]string{}
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if sensitiveFlags[f.Name] {
			flags[f.Name] = redactedValue
		} else {
			flags[f.Name] = f.Value.String()
		}
	})
	return flags
}

const redactedValue = "[REDACTED]"

// sensitiveFlags is the denylist of flag names whose values must never be sent
// to analytics. These contain credentials, secrets, or internal paths.
// All other flags are considered safe to send.
var sensitiveFlags = map[string]bool{
	"token":          true,
	"env-value":      true,
	"mock-telemetry": true,
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
