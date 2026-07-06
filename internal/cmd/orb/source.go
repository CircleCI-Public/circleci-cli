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

package orb

import (
	"context"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newSourceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "source <namespace>/<orb>[@<version>]",
		Short: "Print the YAML source of an orb version",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				- %[1]s<namespace>/<orb>[@<version>]%[1]s is the orb to print, for example, %[1]snamespace/orb-name%[1]s.
				  Optionally append %[1]s@<version>%[1]s (for example, %[1]s@1.2.3%[1]s, %[1]s@volatile%[1]s, or %[1]s@dev:my-branch%[1]s).
				  When omitted, the latest published version is shown.
			`, "`"),
		},
		Long: heredoc.Docf(`
			Print the raw YAML source of an orb version.

			If no version is specified, the latest published version is shown.
			Specify a version with %[1]s@<version>%[1]s (e.g. @1.2.3 or @volatile for latest).
		`, "`"),
		Example: heredoc.Doc(`
			# Print source of the latest version
			$ circleci orb source myorg/my-orb

			# Print source of a specific version
			$ circleci orb source myorg/my-orb@1.2.3

			# Print source of a dev version
			$ circleci orb source myorg/my-orb@dev:my-branch

			# Save the source to a file
			$ circleci orb source myorg/my-orb@1.0.0 > orb.yml
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "ns/orb[@version]"); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runOrbSource(ctx, client, args[0])
		},
	}
}

func runOrbSource(ctx context.Context, client *apiclient.Client, ref string) error {
	// If no @ version, append @volatile
	if !strings.Contains(ref, "@") {
		ref += "@volatile"
	}

	v, err := client.GetOrbVersionByRef(ctx, ref)
	if err != nil {
		return orbAPIErr(err, ref)
	}

	src, err := client.GetOrbSource(ctx, v.ID)
	if err != nil {
		return orbAPIErr(err, ref)
	}

	iostream.Print(ctx, src)
	return nil
}
