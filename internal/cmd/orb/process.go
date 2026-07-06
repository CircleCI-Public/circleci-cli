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
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newProcessCmd() *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Use:   "process <path>",
		Short: "Validate and print expanded orb YAML",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				- %[1]s<path>%[1]s is the path to an orb YAML file. Pass %[1]s-%[1]s to read from stdin.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Validate an orb YAML file and print the expanded (processed) YAML.

			The processed output resolves all orb references and expands inline
			configurations. Useful to verify what CircleCI will see when the orb
			is published.

			Pass '-' as the path to read from stdin.
		`),
		Example: heredoc.Doc(`
			# Process an orb file and print expanded YAML
			$ circleci orb process orb.yml

			# Process from stdin
			$ cat orb.yml | circleci orb process -

			# Process with a specific org for private deps
			$ circleci orb process orb.yml --org gh/acme
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "path"); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			orgID, err := resolveOrgID(ctx, client, org, "circleci orb process")
			if err != nil {
				return err
			}
			return runOrbProcess(ctx, client, args[0], orgID)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Purpose: "for private orb dependencies"})

	return cmd
}

func runOrbProcess(ctx context.Context, client *apiclient.Client, path, orgID string) error {
	yaml, err := readOrbFile(path)
	if err != nil {
		return clierrors.New("orb.read_error", "Failed to read orb file",
			err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	result, err := client.ValidateOrbYAML(ctx, yaml, orgID)
	if err != nil {
		return orbAPIErr(err, path)
	}

	if !result.Valid {
		iostream.ErrPrintln(ctx, "Orb is invalid:")
		for _, e := range result.Errors {
			iostream.ErrPrintln(ctx, "  - "+e)
		}
		return clierrors.New("orb.invalid", "Orb validation failed",
			strings.Join(result.Errors, "; ")).
			WithExitCode(clierrors.ExitValidationFail)
	}

	iostream.Print(ctx, result.OutputYAML)
	return nil
}
