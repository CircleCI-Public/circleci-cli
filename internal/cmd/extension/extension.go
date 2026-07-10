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

// Package extension implements the circleci plugin mechanism.
//
// Any executable named "circleci-<name>" found in PATH is treated as an
// extension and can be invoked transparently as "circleci <name>". The
// extension receives CIRCLE_TOKEN, CIRCLE_HOST, and best-effort project
// metadata via environment variables so it can call the CircleCI API without
// reimplementing authentication.
package extension

import (
	"errors"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

const (
	// Testsuite is the official extension for running tests.
	Testsuite = "testsuite"
)

func RegisterExtensions(rootCmd *cobra.Command) {
	// Register extensions found in PATH. Built-in commands always win on name
	// conflicts — extensions cannot shadow them.
	builtins := map[string]bool{}
	for _, sub := range rootCmd.Commands() {
		builtins[sub.Name()] = true
	}

	extsDir, err := config.ExtensionsDir()
	cobra.CheckErr(err)

	exts, err := extension.FindAll(extsDir)
	cobra.CheckErr(err)

	officialInstalled := map[string]bool{
		Testsuite: false,
	}

	for _, ext := range exts {
		if !builtins[ext.Name] {
			if _, ok := officialInstalled[ext.Name]; ok {
				officialInstalled[ext.Name] = true
			}

			rootCmd.AddCommand(newCmd(ext))
		}
	}

	for extName, installed := range officialInstalled {
		if !installed {
			rootCmd.AddCommand(newPromptCmd(extName))
		}
	}
}

func newPromptCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: "Extension (circleci-" + name + ")",
		Long: heredoc.Doc(fmt.Sprintf(`
			The CircleCI %q extension is not installed by default.
			
			Install it with 'circleci extension install %s'.
		`, name, name)),
		GroupID: "extension",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			s := iostream.Get(ctx)
			if s.IsInteractive() {
				prompt := fmt.Sprintf("%q is not installed. Install %q now?", name, name)
				if s.Confirm(ctx, prompt) {
					return installExtension(ctx, name)
				}
			}

			return clierrors.New("extension.not_installed", "Extension not installed",
				fmt.Sprintf("extension %q is not installed", name)).
				WithSuggestions(fmt.Sprintf("Install with: 'circleci extension install %s'", name)).
				WithExitCode(clierrors.ExitCancelled)
		},
	}
}

func NewExtensionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "extension <command>",
		Short:   "Manage CLI extensions",
		GroupID: "extension",
		Long: heredoc.Doc(`
			Manage CircleCI CLI extensions.

			Extensions are binaries named circleci-<name> that add new
			commands to the CLI. Once installed, an extension is invoked transparently
			as 'circleci <name>'.

			Use 'circleci extension install <name>' to fetch an extension from the CircleCI
			extension registry and verify its checksum before installing it.
		`),
		RunE:               cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmdutil.AddGroup(cmd, "Targeted commands",
		newInstallCmd(),
		newRemoveCmd(),
	)

	return cmd
}

// newCmd returns a cobra command that dispatches to the circleci-<name>
// extension. DisableFlagParsing is set so the extension receives its own args
// verbatim without cobra attempting to parse them. Root persistent flags
// (--config, --insecure-storage, etc.) are parsed separately from os.Args by
// ParseRootFlags so they are available for stream setup and auth injection
// without being forwarded to the extension.
func newCmd(ext extension.Manifest) *cobra.Command {
	return &cobra.Command{
		Use:                ext.Name,
		Short:              "Extension (" + ext.BinaryName + ")",
		GroupID:            "extension",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			extArgs := ParseRootFlags(cmd)

			// Some extensions do not need a CCI account, load the client and suppress
			// any errors; extensions are expected to handle any missing vars.
			client, _ := cmdutil.LoadClient(ctx)

			err := ext.Run(ctx, client, extArgs)
			if err != nil {
				if exitErr, ok := errors.AsType[*extension.ErrExited](err); ok {
					return exitErr
				}

				return clierrors.New(
					"extension.exec_failed",
					"Extension failed",
					fmt.Sprintf("extension %q could not be executed: %s", ext.BinaryName, err),
				)
			}

			return nil
		},
	}
}

// ParseRootFlags populates the root command's persistent flags from os.Args
// for a command that sets DisableFlagParsing. Cobra never calls ParseFlags for
// such commands, so without this --theme, --debug, --quiet, --config, etc.
// would still hold their defaults when the root PersistentPreRunE sets up
// streams and loads config.
//
// Parsing is non-interspersed, so it stops at the first positional argument
// (the extension name); flags after it belong to the extension and are
// returned verbatim along with any other trailing args.
func ParseRootFlags(cmd *cobra.Command) (extArgs []string) {
	fs := pflag.NewFlagSet(cmd.Name(), pflag.ContinueOnError)
	fs.ParseErrorsAllowlist.UnknownFlags = true
	fs.SetInterspersed(false)
	// AddFlagSet shares the underlying *Flag values, so parsing the scratch
	// set populates the root persistent flags directly.
	fs.AddFlagSet(cmd.Root().PersistentFlags())
	_ = fs.Parse(os.Args[1:])

	// fs.Args() holds the extension name followed by its args.
	if args := fs.Args(); len(args) > 1 {
		return args[1:]
	}
	return nil
}
