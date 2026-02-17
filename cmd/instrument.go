package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const instrumentedAnnotation = "telemetry_instrumented"

// instrumentCommands walks the full command tree and wraps every runnable
// command with automatic cli_command_started / cli_command_finished events.
// It is idempotent: calling it twice has no effect.
// Call this after building the command tree in MakeCommands.
func instrumentCommands(root *cobra.Command) {
	visitAll(root, func(cmd *cobra.Command) {
		if cmd.RunE == nil && cmd.Run == nil {
			return
		}
		// Guard against double-wrapping (e.g. if tests rebuild the tree or
		// a future refactor moves the call site).
		if cmd.Annotations[instrumentedAnnotation] == "true" {
			return
		}
		wrapCommand(cmd)
		if cmd.Annotations == nil {
			cmd.Annotations = map[string]string{}
		}
		cmd.Annotations[instrumentedAnnotation] = "true"
	})
}

// wrapCommand replaces a command's Run/RunE with a version that emits telemetry
// start and finish events, records duration, and classifies the outcome.
//
// The invocation_id is read from context rather than generated here, because
// Execute() creates it once at the top level so that args/flag validation errors
// (which fire before RunE) share the same ID as the started event.
func wrapCommand(cmd *cobra.Command) {
	origRunE := cmd.RunE
	origRun := cmd.Run

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		start := time.Now()

		invocationID, hasID := telemetry.InvocationIDFromContext(cmd.Context())
		if !hasID {
			// Fallback: generate an ID if Execute() didn't inject one (e.g. in tests
			// that build subcommands directly without going through root.Execute()).
			invocationID = uuid.NewString()
			cmd.SetContext(telemetry.WithInvocationID(cmd.Context(), invocationID))
		}

		// Policy: --help on a runnable command bypasses RunE entirely in cobra,
		// so a help invocation never reaches here. If it somehow does (e.g. a
		// command with its own help flag), we treat it as outcome "help" so
		// analysts can distinguish it from a real execution.
		if isHelpInvocation(cmd) {
			trackCommandStarted(cmd, invocationID)
			trackCommandFinished(cmd, invocationID, 0, "help", nil)
			if origRunE != nil {
				return origRunE(cmd, args)
			}
			origRun(cmd, args)
			return nil
		}

		trackCommandStarted(cmd, invocationID)

		var err error
		if origRunE != nil {
			err = origRunE(cmd, args)
		} else {
			origRun(cmd, args)
		}

		outcome := "success"
		if err != nil {
			outcome = "error"
		}
		trackCommandFinished(cmd, invocationID, time.Since(start), outcome, err)
		return err
	}
	cmd.Run = nil
}

// isHelpInvocation returns true if the command was invoked with --help.
func isHelpInvocation(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup("help")
	return f != nil && f.Changed && f.Value.String() == "true"
}

func trackCommandStarted(cmd *cobra.Command, invocationID string) {
	client, ok := telemetry.FromContext(cmd.Context())
	if !ok {
		return
	}
	_ = client.Track(telemetry.Event{
		Object: "cli_command_started",
		Action: commandPath(cmd),
		Properties: map[string]interface{}{
			"command_path":   commandPath(cmd),
			"invocation_id":  invocationID,
			"is_interactive": isStdinATTY && isStdoutATTY,
			"ci_environment": os.Getenv("CI") == trueString,
			"flags_used":     telemetry.UsedFlagNames(cmd),
		},
	})
}

// trackCommandFinished emits a cli_command_finished event.
// outcome is fully determined by the caller ("success", "error", "flag_error",
// "args_error", "help"). err is used only to populate error_type — it does not
// override outcome, so callers like setFlagErrorFunc can pass "flag_error" with
// a non-nil err and have both the specific outcome and the classified error recorded.
func trackCommandFinished(cmd *cobra.Command, invocationID string, duration time.Duration, outcome string, err error) {
	client, ok := telemetry.FromContext(cmd.Context())
	if !ok {
		return
	}

	props := map[string]interface{}{
		"command_path":   commandPath(cmd),
		"invocation_id":  invocationID,
		"duration_ms":    duration.Milliseconds(),
		"is_interactive": isStdinATTY && isStdoutATTY,
		"ci_environment": os.Getenv("CI") == trueString,
		"flags_used":     telemetry.UsedFlagNames(cmd),
	}

	if err != nil {
		errType := classifyError(err)
		if errType != "" {
			props["error_type"] = errType
		}
	}
	props["outcome"] = outcome

	_ = client.Track(telemetry.Event{
		Object:     "cli_command_finished",
		Action:     commandPath(cmd),
		Properties: props,
	})
}

// commandPath returns the full command path (e.g. "circleci project create").
func commandPath(cmd *cobra.Command) string {
	return cmd.CommandPath()
}

// classifyError maps errors to safe, non-sensitive category strings.
// Raw error messages are never sent to avoid leaking user data.
func classifyError(err error) string {
	if err == nil {
		return ""
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}

	var httpErr interface{ StatusCode() int }
	if errors.As(err, &httpErr) {
		return fmt.Sprintf("http_%d", httpErr.StatusCode())
	}

	// Inspect HTTP status codes embedded in error messages for REST errors.
	msg := err.Error()
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		if strings.Contains(msg, fmt.Sprintf("%d", code)) {
			return fmt.Sprintf("http_%d", code)
		}
	}

	return "unknown"
}
