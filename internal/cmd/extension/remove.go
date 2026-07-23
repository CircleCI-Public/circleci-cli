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

package extension

import (
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <extension>",
		Short: "Remove an installed extension",
		Annotations: map[string]string{
			"destructiveHint": "true",
		},
		Long: heredoc.Doc(`
			Remove an installed CircleCI CLI extension.

			The extension binary and its manifest are deleted from the extension
			directory. After removal, the extension is no longer available as a
			CLI command.
		`),
		Example: heredoc.Doc(`
			# Remove using the name of the extension
			$ circleci extension remove <name>

			# Remove using the full binary name
			$ circleci extension remove circleci-<name>

			# Remove without confirmation prompt
			$ circleci extension remove <name> --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "extension"); cliErr != nil {
				return cliErr
			}

			ctx := cmd.Context()

			extDir, err := config.ExtensionsDir()
			if err != nil {
				return err
			}

			store := extension.NewStore(extDir)

			name := args[0]
			if !strings.HasPrefix(name, "circleci-") {
				name = "circleci-" + name
			}

			if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
				fmt.Sprintf("Remove extension %q? The binary and manifest will be deleted.", name),
				clierrors.New("extension.remove_aborted", "Removal aborted",
					"Extension removal was not confirmed.").
					WithExitCode(clierrors.ExitCancelled),
				clierrors.New("extension.remove_requires_force", "Removal requires --force",
					fmt.Sprintf("Removing extension %q will delete the binary and manifest.", name)).
					WithExitCode(clierrors.ExitBadArguments),
			); err != nil {
				return err
			}

			err = store.Remove(name)
			if err != nil {
				return removeCLIError(err)
			}

			iostream.Printf(ctx, "%s Removed %s\n", iostream.SymbolOK(ctx), name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func removeCLIError(err error) error {
	if invalidName, ok := errors.AsType[*extension.ErrInvalidName](err); ok {
		return clierrors.New("extension.invalid_name", "Invalid extension name", invalidName.Error()).
			WithSuggestions(
				"Extension names must start with a letter or digit and " +
					"contain only letters (a-z, A-Z), digits (0-9), hyphens (-), and underscores (_).").
			WithExitCode(clierrors.ExitBadArguments)
	}

	if notInstalled, ok := errors.AsType[*extension.ErrExtensionNotInstalled](err); ok {
		return clierrors.New("extension.not_installed", "Extension not installed", notInstalled.Error()).
			WithExitCode(clierrors.ExitNotFound)
	}

	return err
}
