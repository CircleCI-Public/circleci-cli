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

type tracker interface {
	Track(eventName string, props map[string]any) error
}

func RecordTelemetry(cmd *cobra.Command, telemetry tracker) {
	if isTelemetryDisabled(cmd) {
		return
	}

	if cmd.RunE == nil {
		return
	}

	currentRunE := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		runErr := currentRunE(cmd, args)

		RecordTelemetryNow(cmd, telemetry)

		return runErr
	}
}

func RecordTelemetryForSubcommands(cmd *cobra.Command, telemetry tracker) {
	for _, c := range cmd.Commands() {
		RecordTelemetry(c, telemetry)
		RecordTelemetryForSubcommands(c, telemetry)
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

func isTelemetryDisabled(cmd *cobra.Command) bool {
	return cmd.Annotations["telemetry"] == "disabled"
}

func RecordTelemetryNow(cmd *cobra.Command, telemetry tracker) {
	var flags []string
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flags = append(flags, f.Name)
	})
	slices.Sort(flags)

	_ = telemetry.Track("command_invocation",
		map[string]any{
			"command": cmd.CommandPath(),
			"flags":   strings.Join(flags, ","),
		},
	)
}
