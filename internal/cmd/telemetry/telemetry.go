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

package telemetry

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

// NewTelemetryCmd returns the "circleci telemetry" command group.
func NewTelemetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry <command>",
		Short: "Manage telemetry preferences",
		Long: heredoc.Doc(`
			Opt in or out of anonymous usage telemetry.

			Telemetry helps the CircleCI team understand how the CLI is used so we
			can improve it. No credentials, pipeline definitions, or personally
			identifiable information are ever collected.

			The following environment variables always override the stored preference:
			  CIRCLECI_NO_TELEMETRY   disable telemetry when set to any value
			  NO_ANALYTICS            disable telemetry when set to any value
			  DO_NOT_TRACK            disable telemetry when set to any value
			  CI                      disable telemetry automatically in CI environments
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(newEnableCmd())
	cmd.AddCommand(newDisableCmd())

	return cmd
}
