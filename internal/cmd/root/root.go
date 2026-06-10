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

package root

import (
	"os"
	"strings"

	"github.com/njayp/ophis"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/agent"
	cmdapi "github.com/CircleCI-Public/circleci-cli/internal/cmd/api"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/artifacts"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/certificate"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/cmdauth"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/completion"
	cmdconfig "github.com/CircleCI-Public/circleci-cli/internal/cmd/config"
	cmdcontext "github.com/CircleCI-Public/circleci-cli/internal/cmd/context"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/deploy"
	cmddlc "github.com/CircleCI-Public/circleci-cli/internal/cmd/dlc"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/envvar"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/job"
	cmdnamespace "github.com/CircleCI-Public/circleci-cli/internal/cmd/namespace"
	cmdonboard "github.com/CircleCI-Public/circleci-cli/internal/cmd/onboard"
	cmdorb "github.com/CircleCI-Public/circleci-cli/internal/cmd/orb"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/pipeline"
	cmdpolicy "github.com/CircleCI-Public/circleci-cli/internal/cmd/policy"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/project"
	cmdrun "github.com/CircleCI-Public/circleci-cli/internal/cmd/run"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/runner"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/settings"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/signingconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/step"
	cmdversion "github.com/CircleCI-Public/circleci-cli/internal/cmd/version"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/workflow"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry"
)

// NewRootCmd builds the root cobra command and wires all subcommands.
func NewRootCmd(version string) *cobra.Command {
	telem := &delegatingTelemetry{}

	cmd := &cobra.Command{
		Use:           "circleci",
		Short:         "The CircleCI CLI",
		Long:          `Work with CircleCI from the command line.`,
		SilenceErrors: true, // main.go handles error printing
		SilenceUsage:  true,
	}

	cmd.Version = version
	cmd.SetVersionTemplate("circleci version {{.Version}}\n")

	cmd.PersistentFlags().StringP("config", "c", "", "path to config file (default: ~/.config/circleci/config.yml)")
	cmd.PersistentFlags().BoolP("quiet", "q", false, "suppress informational output; data on stdout is unaffected")
	cmd.PersistentFlags().BoolP("debug", "", false, "enable debug logging")
	cmd.PersistentFlags().StringP("theme", "", "auto", "set the color theme (default: auto)")
	_ = cmd.PersistentFlags().MarkHidden("theme")

	cmd.PersistentFlags().BoolP("insecure-storage", "", false, "do not use the system's secure storage for storing tokens")
	_ = cmd.PersistentFlags().MarkHidden("insecure-storage")

	cmd.AddCommand(artifacts.NewArtifactCmd())
	cmd.AddCommand(certificate.NewCertificateCmd())
	cmd.AddCommand(cmdapi.NewAPICmd())
	cmd.AddCommand(cmdauth.NewAuthCmd())
	cmd.AddCommand(cmdconfig.NewConfigCmd())
	cmd.AddCommand(cmdcontext.NewContextCmd())
	cmd.AddCommand(cmddlc.NewDLCCmd())
	cmd.AddCommand(cmdnamespace.NewNamespaceCmd())
	cmd.AddCommand(cmdonboard.NewOnboardCmd())
	cmd.AddCommand(cmdorb.NewOrbCmd())
	cmd.AddCommand(cmdpolicy.NewPolicyCmd())
	cmd.AddCommand(cmdrun.NewRunCmd())
	cmd.AddCommand(cmdversion.NewVersionCmd(version))
	cmd.AddCommand(completion.NewCompletionCmd())
	cmd.AddCommand(deploy.NewDeployCmd())
	cmd.AddCommand(envvar.NewEnvVarCmd())
	cmd.AddCommand(job.NewJobCmd())
	cmd.AddCommand(pipeline.NewPipelineCmd())
	cmd.AddCommand(project.NewProjectCmd())
	cmd.AddCommand(runner.NewRunnerCmd())
	cmd.AddCommand(settings.NewSettingsCmd())
	cmd.AddCommand(signingconfig.NewSigningConfigCmd())
	cmd.AddCommand(step.NewStepCmd())
	cmd.AddCommand(workflow.NewWorkflowCmd())

	// Wire in MCP commands
	cmd.AddCommand(ophis.Command(nil))

	// Register extensions found in PATH. Built-in commands always win on name
	// conflicts — extensions cannot shadow them.
	builtins := map[string]bool{}
	for _, sub := range cmd.Commands() {
		builtins[sub.Name()] = true
	}

	path := os.Getenv("PATH")
	if exts := extension.FindAll(path); len(exts) > 0 {
		cmd.AddGroup(&cobra.Group{ID: "extension", Title: "Extensions"})
		for _, name := range exts {
			if !builtins[name] {
				cmd.AddCommand(extension.NewCmd(name))
			}
		}
	}

	initConfig := func(cmd *cobra.Command) error {
		if theme, _ := cmd.Flags().GetString("theme"); !iostream.IsValidTheme(theme) {
			return clierrors.New("flags.invalid_theme", "Invalid theme", "Invalid value for --theme: "+theme).
				WithSuggestions("Valid themes are: " + strings.Join(iostream.ValidThemes(), ", ")).
				WithExitCode(clierrors.ExitBadArguments)
		}

		secureStorage := cmdutil.IsSecureStorage(cmd)
		configPath := cmdutil.ConfigPath(cmd)

		// Load config before stream setup so a stored "theme" setting can act as
		// the fallback when --theme is not explicitly passed. config.Load only
		// uses ctx for its file lock timeout, so the streamless context is fine.
		cfg, err := config.Load(cmd.Context(), configPath, secureStorage)
		if err != nil {
			return err
		}

		ctx := iostream.FromCmd(cmd.Context(), cmd, cfg.EffectiveTheme())
		ctx = cmdutil.WithVersion(ctx, version)
		ctx = cmdutil.WithConfig(ctx, cfg)

		agentName := agent.Detect()
		ctx = cmdutil.WithAgentName(ctx, agentName)

		// Only gather host info when telemetry will actually be sent. Skipping
		// it for telemetry-disabled commands (e.g. completion generation) avoids
		// gopsutil's `ioreg` lookup, which fails under a restricted PATH such as
		// Homebrew's sanitized completion-generation environment.
		var hostInfo *host.InfoStat
		if cfg.IsTelemetry() && !cmdutil.IsTelemetryDisabled(cmd) {
			hostInfo, err = host.InfoWithContext(ctx)
			if err != nil {
				return err
			}
		}

		tc, err := telemetry.New(ctx, telemetry.Config{
			Log:      cfg.IsTelemetry(),
			Send:     cfg.IsTelemetry(),
			WriteKey: telemetry.SegmentKey,
			Endpoint: os.Getenv("CIRCLE_TELEMETRY_ENDPOINT"),
			Metadata: telemetry.Meta{
				Version:    version,
				InstanceID: cfg.DeviceID(),
				UserID:     cfg.UserID(),
				HostInfo:   hostInfo,
				Extra: map[string]any{
					"agent":          agentName,
					"is_self_hosted": cfg.EffectiveHost() != "https://circleci.com",
					"is_tty":         iostream.IsTerminal(ctx),
				},
			},
		})
		if err != nil {
			return err
		}
		telem.Client = tc

		jqFilter, _ := cmd.Flags().GetString("jq")
		ctx = iostream.WithJQFilter(ctx, jqFilter)

		cmd.SetContext(ctx)

		return nil
	}

	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}
		return initConfig(cmd)
	}
	cmd.PersistentPostRunE = func(_ *cobra.Command, _ []string) error {
		_ = telem.Close()
		return nil
	}

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if telem.Client == nil {
			// --help flag path: pre/post run hooks don't fire, so own the full lifecycle here.
			if err := initConfig(cmd); err == nil {
				cmdutil.RecordTelemetryNow(cmd, telem.Client)
				_ = telem.Close()
			}
		} else {
			// `help` subcommand path: PersistentPreRunE already initialized, PersistentPostRunE will close.
			cmdutil.RecordTelemetryNow(cmd, telem.Client)
		}

		rootHelp(cmd, args)
	})

	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		if telem.Client == nil {
			// --help flag path: pre/post run hooks don't fire, so own the full lifecycle here.
			if err := initConfig(cmd); err == nil {
				cmdutil.RecordTelemetryNow(cmd, telem.Client)
				_ = telem.Close()
			}
		} else {
			// `help` subcommand path: PersistentPreRunE already initialized, PersistentPostRunE will close.
			cmdutil.RecordTelemetryNow(cmd, telem.Client)
		}

		return rootUsage(cmd)
	})

	cmdutil.RecordTelemetryForSubcommands(cmd, telem)

	return cmd
}

type delegatingTelemetry struct {
	*telemetry.Client
}
