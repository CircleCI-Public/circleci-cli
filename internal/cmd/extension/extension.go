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
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/update"
)

const (
	// Testsuite is the official extension for running tests.
	Testsuite = "testsuite"

	// updateFetchTimeout bounds the registry call so a slow or unreachable registry
	// delays the extension by at most this long.
	updateFetchTimeout = 3 * time.Second
)

// RegisterExtensions registers extensions in the following order:
//   - Managed extensions found in the extensions directory.
//   - Official uninstalled extensions.
//   - Unmanaged extensions found in PATH.
func RegisterExtensions(rootCmd *cobra.Command) {
	assignedCommands := map[string]bool{}
	for _, sub := range rootCmd.Commands() {
		assignedCommands[sub.Name()] = true
	}

	extsDir, err := config.ExtensionsDir()
	cobra.CheckErr(err)

	store := extension.NewStore(extsDir)
	managedExts, err := store.FindAll()
	cobra.CheckErr(err)

	officialInstalled := map[string]bool{
		Testsuite: false,
	}

	for _, ext := range managedExts {
		if !assignedCommands[ext.Name] {
			if _, ok := officialInstalled[ext.Name]; ok {
				officialInstalled[ext.Name] = true
			}

			rootCmd.AddCommand(newManagedCmd(ext))
			assignedCommands[ext.Name] = true
		}
	}

	for extName, installed := range officialInstalled {
		if !installed {
			rootCmd.AddCommand(newPromptCmd(extName))
			assignedCommands[extName] = true
		}
	}

	unmanagedExts := extension.FindAllOnPATH()
	for _, ext := range unmanagedExts {
		if !assignedCommands[ext.Name] {
			rootCmd.AddCommand(newUnmanagedCmd(ext))
			assignedCommands[ext.Name] = true
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
		GroupID:            "extension",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			extArgs := ParseRootFlags(cmd)

			s := iostream.Get(ctx)
			if s.IsInteractive() {
				prompt := fmt.Sprintf("%q is not installed. Install %q now?", name, name)
				if s.Confirm(ctx, prompt) {
					ext, err := installExtension(ctx, name)
					if err != nil {
						return err
					}

					return runExtension(ctx, ext, extArgs)
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

// newManagedCmd returns a cobra command that dispatches to the circleci-<name>
// extension. DisableFlagParsing is set so the extension receives its own args
// verbatim without cobra attempting to parse them. Root persistent flags
// (--config, --insecure-storage, etc.) are parsed separately from os.Args by
// ParseRootFlags so they are available for stream setup and auth injection
// without being forwarded to the extension.
func newManagedCmd(ext extension.Manifest) *cobra.Command {
	cmd := &cobra.Command{
		Use:                ext.Name,
		Short:              "Extension (" + ext.BinaryName + ")",
		GroupID:            "extension",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			extArgs := ParseRootFlags(cmd)
			skipFlag, _ := cmd.Root().Flags().GetBool("skip-update-check")
			prompted := promptForUpdate(ctx, ext, skipFlag)
			return runExtension(ctx, prompted, extArgs)
		},
	}

	if ext.Ref != nil {
		if cmd.Annotations == nil {
			cmd.Annotations = make(map[string]string)
		}

		cmd.Annotations[extension.ReferenceAnnotation] = ext.BinaryName
	}

	return cmd
}

func runExtension(ctx context.Context, ext extension.Manifest, extArgs []string) error {
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
}

// promptForUpdate offers to install a newer version of a managed extension
// before it runs and returns the manifest to run.
func promptForUpdate(ctx context.Context, ext extension.Manifest, skip bool) extension.Manifest {
	cfg := cmdutil.GetConfig(ctx)

	if !update.ShouldCheck(ctx, cfg, ext.Version) || skip {
		return ext
	}

	statePath, err := config.StatePath()
	if err != nil {
		return ext
	}

	st, err := config.LoadState(ctx, statePath)
	if err != nil {
		return ext
	}

	if at := st.CheckedExtensionUpdateAt(ext.BinaryName); !at.IsZero() && time.Since(at) < update.CacheWindow {
		return ext
	}

	extDir, err := config.ExtensionsDir()
	if err != nil {
		return ext
	}

	store := extension.NewStore(extDir)
	m := extension.NewManager(extension.Config{
		Version: cmdutil.GetVersion(ctx),
		Agent:   cmdutil.GetAgentName(ctx),
		BaseURL: cfg.EffectiveExtensionHost(),
	})

	fetchCtx, cancel := context.WithTimeout(ctx, updateFetchTimeout)
	defer cancel()

	latest, err := m.Get(fetchCtx, ext.BinaryName)
	if err != nil {
		iostream.DebugContext(ctx, "extension update check: fetch failed",
			"extension", ext.BinaryName, "err", err)
		return ext
	}

	err = config.SaveState(ctx, statePath, func(s *config.State) error {
		s.SetCheckedExtensionUpdateAt(ext.BinaryName, time.Now())
		return nil
	})
	if err != nil {
		iostream.DebugContext(ctx, "extension update check: could not write state", "err", err)
		return ext
	}

	newer := update.IsNewer(latest.Version, ext.Version)
	iostream.DebugContext(ctx, "extension update check evaluated",
		"extension", ext.BinaryName,
		"latest", latest.Version,
		"current", ext.Version,
		"newer", newer)

	if !newer {
		return ext
	}

	update.PrintBinaryNotice(ctx, ext.BinaryName, ext.Version, latest.Version)

	if !iostream.Get(ctx).Confirm(ctx, fmt.Sprintf("Update %s %s now?", ext.BinaryName, latest.Version)) {
		return ext
	}

	iostream.ErrPrintf(ctx, "Updating %s to version %s...\n", ext.BinaryName, latest.Version)

	binary, err := m.Download(ctx, latest)
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not download %s %s: %s\n",
			iostream.SymbolWarn(ctx), ext.BinaryName, latest.Version, err)
		return ext
	}

	defer func() { _ = binary.Close() }()

	updated, err := store.Write(latest, binary)
	if err != nil {
		iostream.ErrPrintf(ctx, "%s Could not install %s %s: %s\n",
			iostream.SymbolWarn(ctx), ext.BinaryName, latest.Version, err)
		return ext
	}

	iostream.ErrPrintf(ctx, "%s Updated %s version %s\n", iostream.SymbolOK(ctx), ext.BinaryName, latest.Version)

	return updated
}

// newUnmanagedCmd returns a cobra command that dispatches to the circleci-<name>
// extension. DisableFlagParsing is set so the extension receives its own args
// verbatim without cobra attempting to parse them. Root persistent flags
// (--config, --insecure-storage, etc.) are parsed separately from os.Args by
// ParseRootFlags so they are available for stream setup and auth injection
// without being forwarded to the extension.
func newUnmanagedCmd(ext extension.Unmanaged) *cobra.Command {
	return &cobra.Command{
		Use:                ext.Name,
		Short:              "Extension (" + ext.BinaryName + ")",
		GroupID:            "extension",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		Hidden:             true,
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
