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
	"io"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newValidateCmd() *cobra.Command {
	var orgID string

	cmd := &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate an orb YAML file",
		Long: heredoc.Doc(`
			Validate an orb YAML file against the CircleCI API.

			Pass '-' as the path to read from stdin.

			Use --org-id to validate against a specific organization's private
			orb dependencies.

			Exit code 7 if the orb is invalid.
		`),
		Example: heredoc.Doc(`
			# Validate an orb file
			$ circleci orb validate orb.yml

			# Validate from stdin
			$ cat orb.yml | circleci orb validate -

			# Validate with a specific org for private deps
			$ circleci orb validate orb.yml --org-id 00000000-0000-0000-0000-000000000001
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "path"); err != nil {
				return err
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runOrbValidate(ctx, client, args[0], orgID)
		},
	}

	cmd.Flags().StringVar(&orgID, "org-id", "", "organization UUID for private orb dependencies")

	return cmd
}

func readOrbFile(path string) (string, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path) //#nosec:G304 // Intentionally reading a user-provided file
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func runOrbValidate(ctx context.Context, client *apiclient.Client, path, orgID string) error {
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

	iostream.Printf(ctx, "%s Orb is valid.\n", iostream.SymbolOK(ctx))
	return nil
}
