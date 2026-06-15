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

package receivetelemetry

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry/receiver"
)

// NewReceiveTelemetryCmd builds the hidden `receive-telemetry` command. The CLI
// re-execs itself with this subcommand to deliver buffered telemetry events out
// of process (see internal/telemetry/delegate.go): the parent serializes the
// events as JSON to this command's stdin, and this command forwards them to
// Segment. Telemetry is disabled here so receiving telemetry never emits its own.
func NewReceiveTelemetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "receive-telemetry",
		Short:        "Receive telemetry events and forward them to Segment",
		SilenceUsage: true,
		Hidden:       true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return receiver.Receive(cmd.InOrStdin())
		},
	}
	cmdutil.DisableEverything(cmd)
	return cmd
}
