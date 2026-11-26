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

package extension

import (
	"context"
	"errors"
	"strings"

	goversion "github.com/hashicorp/go-version"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/extschema"
	"github.com/CircleCI-Public/circleci-cli/internal/extsocket"
)

// socketEnvVar names the variable that carries the telemetry socket path to an
// extension. The path is never logged.
const socketEnvVar = "CIRCLE_TELEMETRY_SOCKET"

// runWithTelemetry runs fn, the step that starts the extension, and records on
// the command's telemetry what the extension reported. When the extension has a
// schema and telemetry is on, it listens on a private socket first and passes
// its path to fn in the environment. A failure in any telemetry step never
// changes fn's error.
//
// env is the environment the child process receives. fn is given a copy with
// CIRCLE_TELEMETRY_SOCKET removed, or set to the CLI's own socket, so an
// inherited value never reaches the extension.
//
// An extension with no schema, and an unmanaged one (pass an empty name), gets
// no socket and the status not_supported.
func runWithTelemetry(
	ctx context.Context, name, version string, env []string, fn func(env []string) error,
) error {
	env = withoutVar(env, socketEnvVar)

	if !cmdutil.GetConfig(ctx).IsTelemetry() {
		return fn(env)
	}

	if !extschema.Has(name) {
		record(ctx, version, extschema.Result{Status: extschema.NotSupported})
		return fn(env)
	}

	ln, err := extsocket.Listen()
	if err != nil {
		record(ctx, version, extschema.Result{Status: extschema.SetupFailed})
		return fn(env)
	}
	defer func() { _ = ln.Close() }()

	runErr := fn(append(env, socketEnvVar+"="+ln.Path()))

	// A cancelled command context, such as after Ctrl+C, must not skip the read:
	// the extension may have reported before it was stopped. Collect bounds its
	// own wait, so no extra deadline is added here.
	payload, err := ln.Collect(context.WithoutCancel(ctx))
	record(ctx, version, outcome(name, payload, err))
	return runErr
}

// outcome turns what Collect returned into a Result.
func outcome(name string, payload extsocket.Payload, err error) extschema.Result {
	switch {
	case err == nil:
		return extschema.Check(name, payload.Version, payload.Attributes)
	case errors.Is(err, extsocket.ErrNoConnection):
		return extschema.Result{Status: extschema.NotSupported}
	case errors.Is(err, extsocket.ErrNoPayload):
		return extschema.Result{Status: extschema.NoPayload}
	default: // ErrUnreadable, or anything Collect did not name
		return extschema.Result{Status: extschema.ReadFailed}
	}
}

// record sets the extension version, the status and each accepted attribute on
// the command's telemetry.
func record(ctx context.Context, version string, res extschema.Result) {
	if validVersion(version) {
		cmdutil.SetTelemetryProp(ctx, "extension_version", version)
	}
	cmdutil.SetEnum(ctx, "extension_attributes_status", string(res.Status),
		"not_supported", "setup_failed", "no_payload", "accepted", "partially_accepted", "rejected", "read_failed")

	for _, a := range res.Accepted {
		switch a.Property {
		case "ext_configoptimizer_jobs_analyzed":
			setInt(a, func(v int) { cmdutil.SetInt(ctx, "ext_configoptimizer_jobs_analyzed", v) })
		case "ext_configoptimizer_jobs_with_suggestions":
			setInt(a, func(v int) { cmdutil.SetInt(ctx, "ext_configoptimizer_jobs_with_suggestions", v) })
		case "ext_configoptimizer_no_suggestion_reason":
			setString(a, func(v string) {
				cmdutil.SetEnum(ctx, "ext_configoptimizer_no_suggestion_reason", v,
					"not_enough_usage", "saving_too_small", "already_right_sized", "memory_bound",
					"pipeline_not_priced", "unsupported_jobs", "class_mismatch", "previously_rejected")
			})
		case "ext_configoptimizer_suggestions":
			setInt(a, func(v int) { cmdutil.SetInt(ctx, "ext_configoptimizer_suggestions", v) })
		case "ext_configoptimizer_validation_outcome":
			setString(a, func(v string) {
				cmdutil.SetEnum(ctx, "ext_configoptimizer_validation_outcome", v,
					"savings_confirmed", "savings_not_realized", "failed_on_suggested_hardware", "error", "inconclusive")
			})
		case "ext_configoptimizer_validation_pairs":
			setInt(a, func(v int) { cmdutil.SetInt(ctx, "ext_configoptimizer_validation_pairs", v) })
		}
	}
}

// setInt calls set with the attribute's value when it is an int.
func setInt(a extschema.Attribute, set func(int)) {
	if v, ok := a.Value.(int); ok {
		set(v)
	}
}

// setString calls set with the attribute's value when it is a string.
func setString(a extschema.Attribute, set func(string)) {
	if v, ok := a.Value.(string); ok {
		set(v)
	}
}

// validVersion reports whether v is a semantic version with all three numbers:
// 1.2.3, 1.2.3-rc.1 or 1.2.3+build. go-version also takes 1.0, v1 and 1.2.3.4,
// so the form is checked as well. The version comes from the manifest on the
// user's disk, so it is checked rather than trusted.
func validVersion(v string) bool {
	if _, err := goversion.NewSemver(v); err != nil {
		return false
	}
	if strings.HasPrefix(v, "v") {
		return false
	}
	core, _, _ := strings.Cut(v, "-")
	core, _, _ = strings.Cut(core, "+")
	return strings.Count(core, ".") == 2
}

// withoutVar returns env without the named variable. It does not change env.
func withoutVar(env []string, name string) []string {
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if key, _, _ := strings.Cut(e, "="); strings.EqualFold(key, name) {
			continue
		}
		out = append(out, e)
	}
	return out
}
