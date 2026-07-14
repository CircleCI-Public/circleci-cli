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
	"context"
	"errors"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <extension>",
		Short: "Install an extension",
		Long: heredoc.Doc(`
			Install a CircleCI CLI extension from the extension registry.

			The extension binary is downloaded, its SHA-256 checksum is verified
			against the release manifest, and the binary is written to the
			extension directory.

			A manifest file is written alongside the binary recording the version,
			checksum, and source URL so the extension can be upgraded or removed later.
		`),
		Example: heredoc.Doc(`
			# Install the testsuite extension
			$ circleci extension install testsuite
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "extension"); cliErr != nil {
				return cliErr
			}

			ctx := cmd.Context()
			return installExtension(ctx, args[0])
		},
	}

	return cmd
}

func installExtension(ctx context.Context, name string) error {
	extDir, err := config.ExtensionsDir()
	if err != nil {
		return err
	}

	cfg := cmdutil.GetConfig(ctx)

	m := extension.NewManager(extension.Config{
		Version:       cmdutil.GetVersion(ctx),
		Agent:         cmdutil.GetAgentName(ctx),
		ExtensionsDir: extDir,
		BaseURL:       cfg.EffectiveExtensionHost(),
	})

	n := name
	if !strings.HasPrefix(n, "circleci-") {
		n = "circleci-" + name
	}
	ext, err := m.Get(ctx, n)
	if err != nil {
		return installCLIError(err)
	}

	iostream.Printf(ctx, "Installing %s version %s...\n", ext.BinaryName, ext.Version)

	err = m.Install(ctx, ext)
	if err != nil {
		return installCLIError(err)
	}

	iostream.Printf(ctx, "%s Installed %s version %s\n", iostream.SymbolOK(ctx), ext.BinaryName, ext.Version)
	return nil
}

func installCLIError(err error) error {
	if invalidName, ok := errors.AsType[*extension.ErrInvalidName](err); ok {
		return clierrors.New("extension.invalid_name", "Invalid extension name", invalidName.Error()).
			WithSuggestions(
				"Extension names must start with a letter or digit and " +
					"contain only letters (a-z, A-Z), digits (0-9), hyphens (-), and underscores (_).").
			WithExitCode(clierrors.ExitBadArguments)
	}

	if notFound, ok := errors.AsType[*extension.ErrExtensionNotFound](err); ok {
		return clierrors.New("extension.not_found", "Extension not found", notFound.Error()).
			WithSuggestions("Check the extension name for typos.").
			WithExitCode(clierrors.ExitNotFound)
	}

	if noBinary, ok := errors.AsType[*extension.ErrNoBinaryForPlatform](err); ok {
		return clierrors.New("extension.no_binary", "No binary for this platform", noBinary.Error()).
			WithSuggestions("Check the extension's documentation for supported platforms.").
			WithExitCode(clierrors.ExitNotFound)
	}

	if downloadFailed, ok := errors.AsType[*extension.ErrDownloadFailed](err); ok {
		return clierrors.New("extension.download_error", "Extension download failed", downloadFailed.Error()).
			WithExitCode(clierrors.ExitAPIError)
	}

	if checksumMismatch, ok := errors.AsType[*extension.ErrChecksumMismatch](err); ok {
		return clierrors.New("extension.checksum_mismatch", "Extension checksum mismatch", checksumMismatch.Error()).
			WithSuggestions("Retry the installation - the file may have been corrupted in transit.").
			WithExitCode(clierrors.ExitValidationFail)
	}

	return err
}
