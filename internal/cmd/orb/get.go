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

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newGetCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "get <ns>/<orb>[@<version>]/<orb-id>",
		Short: "Get orb metadata and statistics",
		Long: heredoc.Doc(`
			Get metadata and statistics for an orb.

			Displays the orb name, namespace, privacy status, latest version,
			usage statistics for the past 30 days, associated categories, and
			all published versions.

			JSON fields: id, name, namespace, is_private, is_listed, created_at,
			             latest_version, categories, versions, stats
		`),
		Example: heredoc.Doc(`
			# Get info for an orb
			$ circleci orb get myorg/my-orb

			# Get info for a specific version
			$ circleci orb get myorg/my-orb@1.2.3

			# Output as JSON
			$ circleci orb get myorg/my-orb --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "ns/orb[@version]"); err != nil {
				return err
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runOrbInfo(ctx, client, args[0], jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type orbInfoStats struct {
	BuildCount   int64 `json:"build_count"`
	ProjectCount int64 `json:"project_count"`
	OrgCount     int64 `json:"org_count"`
}

type orbInfoVersionOutput struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	CreatedAt string `json:"created_at"`
}

type orbInfoOutput struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Namespace     string                 `json:"namespace"`
	IsPrivate     bool                   `json:"is_private"`
	IsListed      bool                   `json:"is_listed"`
	CreatedAt     string                 `json:"created_at"`
	LatestVersion string                 `json:"latest_version"`
	Categories    []orbCategoryOutput    `json:"categories"`
	Versions      []orbInfoVersionOutput `json:"versions"`
	Stats         orbInfoStats           `json:"stats"`
}

func runOrbInfo(ctx context.Context, client *apiclient.Client, ref string, jsonOut bool) error {
	var err error
	var pkg *apiclient.OrbPackage
	if id, parseErr := uuid.Parse(ref); parseErr == nil {
		pkg, err = client.GetOrbPackageByID(ctx, id)
		if err != nil {
			return orbAPIErr(err, ref)
		}
	} else {
		// Separate name from version if @ present
		name := ref
		if idx := strings.Index(ref, "@"); idx >= 0 {
			name = ref[:idx]
		}

		pkg, err = client.GetOrbPackageByName(ctx, name)
		if err != nil {
			return orbAPIErr(err, name)
		}
	}

	cats := make([]orbCategoryOutput, 0, len(pkg.Categories))
	for _, c := range pkg.Categories {
		cats = append(cats, orbCategoryOutput{ID: c.ID, Name: c.Name})
	}

	allVersions, err := client.ListOrbVersions(ctx, pkg.ID, "")
	if err != nil {
		return orbAPIErr(err, ref)
	}
	versions := make([]orbInfoVersionOutput, 0, len(allVersions))
	for _, v := range allVersions {
		versions = append(versions, orbInfoVersionOutput{
			ID:        v.ID,
			Version:   v.Version,
			CreatedAt: v.CreatedAt,
		})
	}

	if jsonOut {
		out := orbInfoOutput{
			ID:            pkg.ID,
			Name:          pkg.Name,
			Namespace:     pkg.Namespace,
			IsPrivate:     pkg.IsPrivate,
			IsListed:      pkg.IsListed,
			CreatedAt:     pkg.CreatedAt,
			LatestVersion: pkg.LatestVersion,
			Categories:    cats,
			Versions:      versions,
			Stats: orbInfoStats{
				BuildCount:   pkg.Last30DaysBuildCount,
				ProjectCount: pkg.Last30DaysProjectCount,
				OrgCount:     pkg.Last30DaysOrgCount,
			},
		}
		return iostream.PrintJSON(ctx, out)
	}

	md := "# Orb\n\n"
	md += fmt.Sprintf("- **ID:** `%s`\n", pkg.ID)
	md += fmt.Sprintf("- **Name:** %s\n", pkg.Name)
	md += fmt.Sprintf("- **Namespace:** %s\n", pkg.Namespace)
	md += fmt.Sprintf("- **Latest Version:** %s\n", pkg.LatestVersion)
	md += fmt.Sprintf("- **Created:** %s\n", pkg.CreatedAt)
	md += fmt.Sprintf("- **Private:** %s\n", fmtBool(pkg.IsPrivate))
	md += fmt.Sprintf("- **Listed:** %s\n", fmtBool(pkg.IsListed))

	md += "\n## Categories\n\n"
	if len(cats) == 0 {
		md += "_None_\n"
	} else {
		for _, c := range cats {
			md += fmt.Sprintf("- %s\n", c.Name)
		}
	}

	md += "\n## Versions\n\n"
	if len(versions) == 0 {
		md += "_No versions published_\n"
	} else {
		for _, v := range versions {
			md += fmt.Sprintf("- %s\n", v.Version)
		}
	}

	md += "\n## Last 30 Days\n\n"
	md += fmt.Sprintf("- **Builds:** %d\n", pkg.Last30DaysBuildCount)
	md += fmt.Sprintf("- **Projects:** %d\n", pkg.Last30DaysProjectCount)
	md += fmt.Sprintf("- **Orgs:** %d\n", pkg.Last30DaysOrgCount)

	iostream.PrintMarkdown(ctx, md)
	return nil
}
