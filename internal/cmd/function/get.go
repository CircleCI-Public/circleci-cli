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

package cmdfunction

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/function"
)

type getEntry struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Version       string    `json:"version"`
	LatestVersion string    `json:"latest_version"`
	Versions      []string  `json:"versions"`
}

func newGetCmd() *cobra.Command {
	var (
		version string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "get <name>",
		Aliases: []string{"show"},
		Short:   "Show a function's versions",
		Long: heredoc.Docf(`
			Show a function's published versions.

			A bare name resolves against %[1]sgithub.com/circleci-functions%[1]s.

			JSON fields: id, name, description, version, latest_version,
			versions.
		`, "`"),
		Example: heredoc.Doc(`
			# Show a function at its latest version
			$ circleci function get setup-go

			# Show a specific version, or a function published elsewhere
			$ circleci function get github.com/myorg/my-fn --version v1.0.0

			# List the published versions
			$ circleci function get setup-go --json --jq '.versions[]'
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runGet(ctx, client, args[0], version, jsonOut)
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "version to show (default: the latest release)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runGet(ctx context.Context, client *apiclient.Client, arg, version string, jsonOut bool) error {
	name, err := function.ExpandName(arg)
	if err != nil {
		return err
	}
	if version != "" {
		if err := function.ValidateVersion(version); err != nil {
			return err
		}
	}

	sp := iostream.Spinner(ctx, !jsonOut, fmt.Sprintf("Fetching %s", name))
	fn, err := client.GetFunctionByName(ctx, name)
	if err != nil {
		sp.Stop()
		return apiErr(err, name)
	}

	wanted := version
	if wanted == "" {
		if fn.LatestVersion == "" {
			sp.Stop()
			return noVersionsErr(name)
		}
		wanted = fn.LatestVersion
	}

	versions := make([]string, 0, len(fn.Versions))
	for _, v := range fn.Versions {
		versions = append(versions, v.Version)
	}

	ref, ok := fn.Find(wanted)
	sp.Stop()
	if !ok {
		return versionNotPublishedErr(name, wanted, versions)
	}

	entry := getEntry{
		ID:            fn.ID,
		Name:          fn.Name,
		Description:   fn.Description,
		Version:       ref.Version,
		LatestVersion: fn.LatestVersion,
		Versions:      versions,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entry)
	}

	printFunction(ctx, entry)
	return nil
}

func printFunction(ctx context.Context, e getEntry) {
	md := fmt.Sprintf("# %s\n\n", e.Name)
	if e.Description != "" {
		md += e.Description + "\n\n"
	}
	md += fmt.Sprintf("**Version:** %s\n**Latest:** %s\n", e.Version, e.LatestVersion)

	if len(e.Versions) > 0 {
		table := mdtable.New("Version")
		for _, v := range e.Versions {
			table.Row(v)
		}
		md += "\n## Published versions\n" + table.Render()
	}

	iostream.PrintMarkdown(ctx, md)
}
