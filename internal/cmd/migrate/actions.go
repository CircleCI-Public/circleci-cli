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

package migrate

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
)

func newActionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions <workflow-file>",
		Short: "Copy a GitHub Actions workflow into .circleci/workflows/",
		Long: heredoc.Doc(`
			Copy a GitHub Actions workflow file into the .circleci/workflows/
			directory so that CircleCI can run it directly.

			The destination filename is derived from the source basename,
			prefixed with "cci-". For example:

			  .github/workflows/run_tests.yml → .circleci/workflows/cci-run_tests.yml

			The .circleci/workflows/ directory is created if it does not exist.
			An existing destination file is not overwritten unless --force is passed.
		`),
		Example: heredoc.Doc(`
			# Migrate a single workflow file
			$ circleci migrate actions .github/workflows/run_tests.yml

			# Migrate a workflow stored at a non-standard path
			$ circleci migrate actions ci/build.yml

			# Overwrite an existing destination file
			$ circleci migrate actions --force .github/workflows/run_tests.yml
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			force, _ := cmd.Flags().GetBool("force")
			return runMigrateActions(ctx, args[0], force)
		},
	}

	cmd.Flags().Bool("force", false, "Overwrite the destination file if it already exists")

	return cmd
}

func runMigrateActions(ctx context.Context, src string, force bool) error {
	basename := filepath.Base(src)
	destDir := filepath.Join(".circleci", "workflows")
	dest := filepath.Join(destDir, "cci-"+basename)
	destDisplay := path.Join(".circleci", "workflows", "cci-"+basename)

	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return clierrors.New("migrate.actions_failed", "Failed to create destination directory",
			"Could not create .circleci/workflows/: "+err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	if !force {
		if _, err := os.Stat(dest); err == nil {
			return clierrors.New("migrate.dest_exists", "Destination file already exists",
				destDisplay+" already exists.").
				WithSuggestions("Pass --force to overwrite the existing file").
				WithExitCode(clierrors.ExitBadArguments)
		}
	}

	in, err := os.Open(src) //nolint:gosec // src is a user-supplied file path — intentional
	if err != nil {
		if os.IsNotExist(err) {
			return clierrors.New("migrate.open_failed", "Workflow file not found",
				src+": file not found.").
				WithSuggestions("Check that the file path is correct and readable").
				WithExitCode(clierrors.ExitNotFound)
		}
		return clierrors.New("migrate.open_failed", "Failed to open workflow file",
			"Could not open "+src+": "+err.Error()).
			WithSuggestions("Check that the file path is correct and readable").
			WithExitCode(clierrors.ExitGeneralError)
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dest) //nolint:gosec // dest is constructed from user-supplied basename — intentional
	if err != nil {
		return clierrors.New("migrate.create_failed", "Failed to create destination file",
			"Could not create "+destDisplay+": "+err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return clierrors.New("migrate.copy_failed", "Failed to copy workflow file",
			"Could not copy to "+destDisplay+": "+err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	iostream.Printf(ctx, "Copied %s → %s\n", basename, destDisplay)
	return nil
}
