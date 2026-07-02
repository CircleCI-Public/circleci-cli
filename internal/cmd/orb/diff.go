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
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newDiffCmd() *cobra.Command {
	var (
		from string
		to   string
	)

	cmd := &cobra.Command{
		Use:   "diff <ns>/<orb> --from <v1> --to <v2>",
		Short: "Show a unified diff between two orb versions",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				- ns/orb: the orb to diff, as "namespace/orb-name"
			`),
		},
		Long: heredoc.Doc(`
			Show a unified diff between two versions of an orb.

			The --from and --to versions can be semver (e.g. 1.0.0) or dev labels
			(e.g. dev:my-branch).

			Exit code is 0 regardless of whether the versions differ.
		`),
		Example: heredoc.Doc(`
			# Diff two semver versions
			$ circleci orb diff myorg/my-orb --from 1.0.0 --to 1.1.0

			# Diff a semver and a dev version
			$ circleci orb diff myorg/my-orb --from 1.0.0 --to dev:my-branch

			# Diff two dev versions
			$ circleci orb diff myorg/my-orb --from dev:branch-a --to dev:branch-b
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "ns/orb"); err != nil {
				return err
			}
			if from == "" {
				return cmdutil.RequireFlag("from")
			}
			if to == "" {
				return cmdutil.RequireFlag("to")
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runOrbDiff(ctx, client, args[0], from, to)
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "the first version (semver e.g. 1.0.0, or a dev label e.g. dev:my-branch)")
	cmd.Flags().StringVar(&to, "to", "", "the second version, in the same form as --from")

	return cmd
}

func runOrbDiff(ctx context.Context, client *apiclient.Client, orbName, v1, v2 string) error {
	ref1 := orbName + "@" + v1
	ref2 := orbName + "@" + v2

	ver1, err := client.GetOrbVersionByRef(ctx, ref1)
	if err != nil {
		return orbAPIErr(err, ref1)
	}
	ver2, err := client.GetOrbVersionByRef(ctx, ref2)
	if err != nil {
		return orbAPIErr(err, ref2)
	}

	src1, err := client.GetOrbSource(ctx, ver1.ID)
	if err != nil {
		return orbAPIErr(err, ref1)
	}
	src2, err := client.GetOrbSource(ctx, ver2.ID)
	if err != nil {
		return orbAPIErr(err, ref2)
	}

	edits := myers.ComputeEdits(span.URIFromPath(ref1), src1, src2)
	unified := gotextdiff.ToUnified(ref1, ref2, src1, edits)

	if len(unified.Hunks) == 0 {
		return nil
	}

	iostream.Print(ctx, colorizeDiff(fmt.Sprintf("%s", unified), iostream.ColorEnabled(ctx)))
	return nil
}

func colorizeDiff(diff string, color bool) string {
	if !color {
		return diff
	}
	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ "):
			lines[i] = "\033[33m" + line + "\033[0m" // yellow
		case strings.HasPrefix(line, "@@ "):
			lines[i] = "\033[36m" + line + "\033[0m" // cyan
		case strings.HasPrefix(line, "-"):
			lines[i] = "\033[31m" + line + "\033[0m" // red
		case strings.HasPrefix(line, "+"):
			lines[i] = "\033[32m" + line + "\033[0m" // green
		}
	}
	return strings.Join(lines, "\n")
}
