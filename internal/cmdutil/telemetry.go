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
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const telemetryPropPrefix = "telemetry_prop:"

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

	_ = tc.Track("command_invocation", props)
}
