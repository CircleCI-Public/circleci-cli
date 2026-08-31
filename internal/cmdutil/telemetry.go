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

package cmdutil

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const telemetryPropPrefix = "telemetry_prop:"

// KnownIDKey names a resource-ID telemetry property tracked via TrackKnownID.
// Use one of the typed constants below rather than an ad hoc string literal —
// it doesn't stop a determined caller from constructing an arbitrary
// KnownIDKey, but it does mean the normal way to call TrackKnownID (typing
// "cmdutil.Key" and letting completion suggest a valid one) won't produce a
// typo'd, uncorrelated property name by accident.
type KnownIDKey string

// Valid KnownIDKey values, one per resource type TrackKnownID covers.
const (
	KeyRunID      KnownIDKey = "run_id"
	KeyWorkflowID KnownIDKey = "workflow_id"
	KeyJobID      KnownIDKey = "job_id"
)

type knownIDsContextKey struct{}

// knownIDCollector is the mutable sink TrackKnownID writes into. It travels
// via ctx rather than a *cobra.Command parameter specifically so that
// resolution code (runGet, workflow.Get, job.Get, ...) doesn't need cmd
// threaded through its signature just to reach telemetry — ctx is already
// there on every one of those functions.
type knownIDCollector struct {
	mu  sync.Mutex
	ids map[KnownIDKey]string
}

// WithKnownIDs seeds ctx with an empty collector for TrackKnownID. Called
// once per command execution — see root.go, alongside the rest of the
// per-invocation context setup — so it's present unconditionally and no
// individual command needs to remember to wire it up itself.
func WithKnownIDs(ctx context.Context) context.Context {
	return context.WithValue(ctx, knownIDsContextKey{}, &knownIDCollector{ids: map[KnownIDKey]string{}})
}

// TrackKnownID attaches a resource ID to the current command's
// command_invocation event under key.
//
// This is the single chokepoint every command should route a run/workflow/job
// ID through before it reaches telemetry — never call SetTelemetryProp with a
// raw CLI argument directly. id must already be "known": either parsed from
// user input and then confirmed to exist by a successful API call (e.g.
// GetRunV3), or returned directly by an API response (e.g. a search result).
// Passing an unvalidated string defeats the point of this function, and is
// exactly the "arbitrary user input in o11y" pattern this repo deliberately
// avoids elsewhere (see the CLI vs. GitHub CLI discussion this followed from).
//
// uuid.Nil is never tracked — it is never a real resource ID, only a possible
// zero value from a caller that skipped validation. A ctx that was never
// passed through WithKnownIDs (e.g. a test that doesn't need telemetry) is
// also a silent no-op, not a panic.
//
// EXPERIMENT: this measures adoption of ID-based lookups across run/workflow/
// job commands, to inform whether generic per-user API-usage tracking is worth
// building more broadly. Review by 2026-11-30; remove call sites (not this
// function — other props may still need it) once evaluated.
func TrackKnownID(ctx context.Context, key KnownIDKey, id uuid.UUID) {
	if id == uuid.Nil {
		return
	}
	c, ok := ctx.Value(knownIDsContextKey{}).(*knownIDCollector)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ids[key] = id.String()
}

// SetTelemetryProp attaches an extra property to cmd that RecordTelemetryNow
// will include in the command_invocation event.
func SetTelemetryProp(cmd *cobra.Command, key, value string) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[telemetryPropPrefix+key] = value
}

func DisableEverything(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["everything"] = "disabled"
}

func IsEverythingDisabled(cmd *cobra.Command) bool {
	return cmd.Annotations["everything"] == "disabled"
}

func RecordTelemetry(cmd *cobra.Command) {
	if IsTelemetryDisabled(cmd) {
		return
	}

	if cmd.RunE == nil {
		return
	}

	currentRunE := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		runErr := currentRunE(cmd, args)

		RecordTelemetryNow(cmd)

		return runErr
	}
}

func RecordTelemetryForSubcommands(cmd *cobra.Command) {
	for _, c := range cmd.Commands() {
		RecordTelemetry(c)
		RecordTelemetryForSubcommands(c)
	}
}

func DisableTelemetry(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["telemetry"] = "disabled"
}

func DisableTelemetryForSubcommands(cmd *cobra.Command) {
	for _, c := range cmd.Commands() {
		DisableTelemetry(c)
		DisableTelemetryForSubcommands(c)
	}
}

// IsTelemetryDisabled reports whether telemetry has been disabled for this
// specific command via DisableTelemetry (independent of the user's global
// telemetry preference).
func IsTelemetryDisabled(cmd *cobra.Command) bool {
	return cmd.Annotations["telemetry"] == "disabled"
}

func RecordTelemetryNow(cmd *cobra.Command) {
	ctx := cmd.Context()

	var flags []string
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flags = append(flags, f.Name)
	})
	slices.Sort(flags)

	tc := GetTelemetry(ctx)
	if tc == nil {
		return
	}

	props := map[string]any{
		"command": cmd.CommandPath(),
		"flags":   strings.Join(flags, ","),
	}
	for k, v := range cmd.Annotations {
		if after, ok := strings.CutPrefix(k, telemetryPropPrefix); ok {
			props[after] = v
		}
	}
	if c, ok := ctx.Value(knownIDsContextKey{}).(*knownIDCollector); ok {
		c.mu.Lock()
		for k, v := range c.ids {
			props[string(k)] = v
		}
		c.mu.Unlock()
	}

	_ = tc.Track("command_invocation", props)
}
