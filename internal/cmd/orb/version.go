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
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

func newVersionCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "version <namespace>/<orb>[@<version>]/<version-id>",
		Short: "Get details of a single orb version",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				An orb version can be specified by reference or ID:
				- By reference, for example, %[1]snamespace/orb-name@1.2.3%[1]s. Without %[1]s@<version>%[1]s, the latest published version is shown
				- By ID, the orb version ID (UUID), for example, %[1]s849e7902-802f-4082-8a70-da77dcd084e3%[1]s
			`, "`"),
		},
		Long: heredoc.Doc(`
			Get the ID, version number, publish time and owning orb of a single
			orb version. Use 'circleci orb source' for the version's YAML.

			JSON fields: id, version, orb_id, orb_name, created_at
		`),
		Example: heredoc.Doc(`
			# Get a specific version by reference
			$ circleci orb version myorg/my-orb@1.2.3

			# Get the latest published version
			$ circleci orb version myorg/my-orb

			# Get a version by its ID
			$ circleci orb version 849e7902-802f-4082-8a70-da77dcd084e3

			# Print only the version ID, for use in API calls
			$ circleci orb version myorg/my-orb@1.2.3 --json --jq .id
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
			return runOrbVersion(ctx, client, args[0], jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type orbVersionOutput struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	OrbID     string `json:"orb_id"`
	OrbName   string `json:"orb_name"`
	CreatedAt string `json:"created_at"`
}

func runOrbVersion(ctx context.Context, client *apiclient.Client, ref string, jsonOut bool) error {
	var v *apiclient.OrbVersion
	var err error
	if _, parseErr := uuid.Parse(ref); parseErr == nil {
		v, err = client.GetOrbVersionByID(ctx, ref)
	} else {
		// Same default as 'orb source': no version means the latest published one.
		if !strings.Contains(ref, "@") {
			ref += "@volatile"
		}
		v, err = client.GetOrbVersionByRef(ctx, ref)
	}
	if err != nil {
		return orbAPIErr(err, ref)
	}

	out := orbVersionOutput{
		ID:        v.ID,
		Version:   v.Version,
		OrbID:     v.OrbID,
		OrbName:   v.OrbName,
		CreatedAt: v.CreatedAt,
	}
	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	md := "# Orb Version\n\n"
	md += fmt.Sprintf("- **ID:** `%s`\n", out.ID)
	md += fmt.Sprintf("- **Orb:** %s\n", out.OrbName)
	md += fmt.Sprintf("- **Version:** %s\n", out.Version)
	md += fmt.Sprintf("- **Published:** %s\n", out.CreatedAt)
	md += fmt.Sprintf("- **Orb ID:** `%s`\n", out.OrbID)

	iostream.PrintMarkdown(ctx, md)
	return nil
}
