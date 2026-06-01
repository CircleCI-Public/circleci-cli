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
// extension receives CIRCLECI_TOKEN, CIRCLECI_HOST, and best-effort project
// metadata via environment variables so it can call the CircleCI API without
// reimplementing authentication.
package extension

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/projectref"
)

// FindAll scans PATH for executables named "circleci-<name>" and returns the
// extension names (the part after "circleci-"). The first entry in PATH wins
// for duplicate names, matching exec.LookPath semantics.
func FindAll(path string) []string {
	seen := map[string]bool{}
	var names []string
	for _, dir := range filepath.SplitList(path) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			extName, ok := strings.CutPrefix(n, "circleci-")
			if !ok {
				continue
			}
			if runtime.GOOS == "windows" {
				extName = trimExeSufix(extName)
			}

			if extName == "" || seen[extName] {
				continue
			}
			if runtime.GOOS != "windows" {
				info, err := e.Info()
				if err != nil || info.Mode()&0o111 == 0 {
					continue
				}
			}
			seen[extName] = true
			names = append(names, extName)
		}
	}
	return names
}

var windowsExtensions = []string{
	".exe",
	".sh",
	".ps1",
}

func trimExeSufix(extName string) string {
	for _, extension := range windowsExtensions {
		ext, ok := strings.CutSuffix(extName, extension)
		if ok {
			extName = ext
			break
		}
	}
	return extName
}

// NewCmd returns a cobra command that dispatches to the circleci-<name>
// extension. DisableFlagParsing is set so the extension receives its own args
// verbatim without cobra attempting to parse them. Root persistent flags
// (--config, --insecure-storage, etc.) are parsed separately from os.Args so
// they are available for auth injection without being forwarded to the extension.
func NewCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              "Extension (circleci-" + name + ")",
		GroupID:            "extension",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			root := cmd.Root()

			// With DisableFlagParsing, cobra never calls ParseFlags, so
			// persistent flags on the root are unpopulated. We parse them
			// manually from os.Args up to the command name, keeping only
			// the args after the command name as the extension's args.
			rootArgs, extArgs := splitArgsAtCommand(name, os.Args[1:], root)
			_ = root.ParseFlags(rootArgs)

			// Some extensions do not need a CCI account, load the client and suppress
			// any errors; extensions are expected to handle any missing vars.
			client, _ := cmdutil.LoadClient(ctx)
			return Run(ctx, client, name, extArgs)
		},
	}
}

// splitArgsAtCommand scans rawArgs for the first positional argument equal to
// name, skipping past any flag-value pairs using root's persistent flag
// definitions to determine which flags consume a following value. It returns
// the args before the command name (suitable for root.ParseFlags) and the args
// after it (to forward to the extension).
func splitArgsAtCommand(name string, rawArgs []string, root *cobra.Command) (rootArgs, extArgs []string) {
	pf := root.PersistentFlags()
	skipNext := false
	for i, arg := range rawArgs {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--" {
			// Everything after -- is positional; find name there if present.
			for j := i + 1; j < len(rawArgs); j++ {
				if rawArgs[j] == name {
					return rawArgs[:j], rawArgs[j+1:]
				}
			}
			return rawArgs, nil
		}
		if strings.HasPrefix(arg, "--") && !strings.Contains(arg, "=") {
			// Long flag without =: check if it consumes the next arg as its value.
			if f := pf.Lookup(strings.TrimPrefix(arg, "--")); f != nil && f.NoOptDefVal == "" {
				skipNext = true
			}
			continue
		}
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) == 2 {
			// Short flag: same check.
			if f := pf.ShorthandLookup(arg[1:]); f != nil && f.NoOptDefVal == "" {
				skipNext = true
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			// Any other flag form (--flag=value, -flag, etc.): skip without consuming next.
			continue
		}
		// First positional arg: this is the command name.
		if arg == name {
			return rawArgs[:i], rawArgs[i+1:]
		}
		// Unexpected positional arg before the command name; stop.
		break
	}
	return rawArgs, nil
}

// ErrNotFound is returned when no circleci-<name> binary exists in PATH.
type ErrNotFound struct {
	Name string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("unknown command %q — and no extension %q found in PATH", e.Name, "circleci-"+e.Name)
}

// ErrExited is returned by Run when the extension process exits with a
// non-zero status code. The caller should exit with Code rather than printing
// an error message — the extension is responsible for its own error output.
type ErrExited struct {
	Code int
}

func (e *ErrExited) Error() string {
	return fmt.Sprintf("extension exited with code %d", e.Code)
}

// Run looks up circleci-<name> in PATH and execs it with args, injecting
// CircleCI environment variables. configPath is the --config flag value
// (empty means use the default XDG path). The current process is replaced
// by the extension via syscall exec on Unix; on Windows the extension is
// run as a child process and its exit code is propagated.
//
// If no matching binary is found, ErrNotFound is returned and the caller
// should show the original "unknown command" error instead.
func Run(ctx context.Context, client *apiclient.Client, name string, args []string) error {
	binary := "circleci-" + name
	path, err := exec.LookPath(binary)
	if err != nil {
		return &ErrNotFound{Name: name}
	}

	env := buildEnv(ctx, client)

	cmd := exec.CommandContext(ctx, path, args...) //#nosec:G204,G702 // path comes from LookPath, args are user-supplied CLI args for the extension
	cmd.Stdin = iostream.In(ctx)
	cmd.Stdout = iostream.Out(ctx)
	cmd.Stderr = iostream.Err(ctx)
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if cmd.ProcessState != nil {
			return &ErrExited{Code: cmd.ProcessState.ExitCode()}
		}
		return clierrors.New(
			"extension.exec_failed",
			"Extension failed",
			fmt.Sprintf("extension %q could not be executed: %s", binary, err),
		)
	}
	return nil
}

// buildEnv constructs the environment for the extension process. It starts
// from the current process environment and overlays CIRCLECI_* variables so
// extensions can call the CircleCI API without reimplementing auth.
func buildEnv(ctx context.Context, client *apiclient.Client) []string {
	env := os.Environ()

	cfg := cmdutil.GetConfig(ctx)

	type kv struct{ key, val string }
	overlays := []kv{
		{"CIRCLECI_TOKEN", cfg.EffectiveToken()},
		{"CIRCLECI_HOST", cfg.EffectiveHost()},
		{"CIRCLECI_TELEMETRY_ENABLED", fmt.Sprintf("%t", cfg.IsTelemetry())},
	}

	// Best-effort: inject project metadata from git remote. Failures are
	// silently ignored — the extension is responsible for handling missing vars.
	if info, err := gitremote.Detect(); err == nil {
		parts := strings.SplitN(info.Slug, "/", 3)
		if len(parts) == 3 {
			overlays = append(overlays,
				kv{"CIRCLECI_VCS_TYPE", vcsLong(parts[0])},
				kv{"CIRCLECI_PROJECT_USERNAME", parts[1]},
				kv{"CIRCLECI_PROJECT_REPONAME", parts[2]},
			)
		}
		if info.Branch != "" {
			overlays = append(overlays, kv{"CIRCLECI_BRANCH", info.Branch})
		}

		if id := resolveProjectID(ctx, client, cfg, info.Slug); id != "" {
			overlays = append(overlays, kv{"CIRCLECI_PROJECT_ID", id})
		}
	}

	for _, o := range overlays {
		if o.val == "" {
			continue
		}
		prefix := o.key + "="
		replaced := false
		for i, e := range env {
			if strings.HasPrefix(e, prefix) {
				env[i] = prefix + o.val
				replaced = true
				break
			}
		}
		if !replaced {
			env = append(env, prefix+o.val)
		}
	}
	return env
}

// resolveProjectID returns the CircleCI project UUID for the current checkout,
// preferring the locally recorded value in .circleci/info.yml and falling back
// to an API lookup keyed by slug. Returns "" if no project ID can be determined.
func resolveProjectID(ctx context.Context, client *apiclient.Client, cfg *config.Config, slug string) string {
	if cwd, err := os.Getwd(); err == nil {
		if ref, err := projectref.Read(cwd); err == nil && ref.Project.ID != "" {
			return ref.Project.ID
		}
	}

	token := cfg.EffectiveToken()
	if token == "" || slug == "" || client == nil {
		return ""
	}

	info, err := client.GetProjectInfo(ctx, slug)
	if err != nil {
		return ""
	}

	return info.ID
}

func vcsLong(short string) string {
	switch short {
	case "gh":
		return "github"
	case "bb":
		return "bitbucket"
	case "gl":
		return "gitlab"
	default:
		return short
	}
}
