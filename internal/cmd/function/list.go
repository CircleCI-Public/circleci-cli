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
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/function"
)

// descriptionMax caps the Description column; mdtable sizes a column to its
// widest cell, so one long description otherwise pads every row to match.
const descriptionMax = 72

type listEntry struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	LatestVersion string    `json:"latest_version"`
	Versions      []string  `json:"versions"`
}

func newListCmd() *cobra.Command {
	var (
		pinned  bool
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List published CircleCI functions",
		Long: heredoc.Doc(`
			List every published CircleCI function and its versions.

			JSON fields: id, name, description, latest_version, versions.
			With --pinned: alias, function, version.
		`),
		Example: heredoc.Doc(`
			# List published functions
			$ circleci function list

			# List what this repository declares
			$ circleci function list --pinned

			# Get just the names
			$ circleci function list --json --jq '.[].name'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if pinned {
				return runListPinned(ctx, configPath(cmd), jsonOut)
			}
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runList(ctx, client, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&pinned, "pinned", false, "list the functions declared in the local config")
	addConfigFlag(cmd)
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runList(ctx context.Context, client *apiclient.Client, jsonOut bool) error {
	sp := iostream.Spinner(ctx, !jsonOut, "Fetching published functions")
	functions, err := client.ListFunctions(ctx)
	sp.Stop()
	if err != nil {
		return cmdutil.APIErr(err, "published functions",
			"function.list_failed", "Could not list %s.",
			"Confirm CIRCLE_HOST points at a CircleCI installation with functions enabled")
	}

	entries := make([]listEntry, 0, len(functions))
	for _, fn := range functions {
		entry := listEntry{
			ID:            fn.ID,
			Name:          fn.Name,
			Description:   fn.Description,
			LatestVersion: fn.LatestVersion,
			Versions:      make([]string, 0, len(fn.Versions)),
		}
		for _, v := range fn.Versions {
			entry.Versions = append(entry.Versions, v.Version)
		}
		entries = append(entries, entry)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(entries) == 0 {
		iostream.ErrPrintln(ctx, "No published functions found.")
		return nil
	}

	table := mdtable.New("Name", "Latest", "Description")
	for _, e := range entries {
		table.Row(e.Name, e.LatestVersion, summarize(e.Description))
	}
	iostream.PrintMarkdown(ctx, "# Published functions\n"+table.Render())
	return nil
}

// summarize condenses a description into one table cell: its first line, with
// pipes escaped so they are not read as column separators, capped so a long
// description cannot widen every other row. Published descriptions run to
// several lines, including usage examples; --json carries the full text.
func summarize(description string) string {
	first, _, _ := strings.Cut(description, "\n")
	first = strings.TrimSpace(first)
	if rs := []rune(first); len(rs) > descriptionMax {
		first = strings.TrimSpace(string(rs[:descriptionMax])) + "…"
	}
	// Escape after truncating, so a cut can never split the escape sequence.
	return tableCell(first)
}

func runListPinned(ctx context.Context, path string, jsonOut bool) error {
	pins, err := function.ListPins(path)
	if err != nil {
		return err
	}

	if jsonOut {
		if pins == nil {
			pins = []function.Pin{}
		}
		return iostream.PrintJSON(ctx, pins)
	}

	if len(pins) == 0 {
		iostream.ErrPrintf(ctx, "No functions declared in %s.\n", path)
		return nil
	}

	table := mdtable.New("Alias", "Function", "Version")
	for _, p := range pins {
		table.Row(p.Alias, p.Function, p.Version)
	}
	iostream.PrintMarkdown(ctx, "# Pinned functions\n"+table.Render())
	return nil
}
