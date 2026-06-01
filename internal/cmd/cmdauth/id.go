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

package cmdauth

import (
	"runtime"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newDeviceIDCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "id",
		Short: "Show the device ID for this CLI installation",
		Long: heredoc.Doc(`
			Print the device ID for this CLI installation as "<os>:<uuid>".

			The UUID is generated on first use and stored in the config file.
			The OS prefix (e.g. darwin, linux) is added at print time so you
			can identify the machine and platform at a glance. The same UUID
			is sent with every OAuth authorization request, so you can match
			a token in the CircleCI UI back to this installation.

			JSON fields:
			  device_id  string  Stable identifier in the form <os>:<uuid>
		`),
		Example: heredoc.Doc(`
			# Print the device ID
			$ circleci auth id

			# Output as JSON
			$ circleci auth id --json

			# Use in a script
			$ DEVICE=$(circleci auth id)
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := cmdutil.GetConfig(ctx)
			id := runtime.GOOS + ":" + cfg.DeviceID().String()

			if jsonOut {
				return iostream.PrintJSON(ctx, map[string]string{"device_id": id})
			}
			iostream.Println(ctx, id)
			return nil
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	return cmd
}
