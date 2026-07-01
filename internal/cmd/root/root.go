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
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/my"
	cmdnamespace "github.com/CircleCI-Public/circleci-cli/internal/cmd/namespace"
	cmdonboard "github.com/CircleCI-Public/circleci-cli/internal/cmd/onboard"
	cmdorb "github.com/CircleCI-Public/circleci-cli/internal/cmd/orb"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/pipeline"
	cmdpolicy "github.com/CircleCI-Public/circleci-cli/internal/cmd/policy"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/project"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/receivetelemetry"
	cmdrun "github.com/CircleCI-Public/circleci-cli/internal/cmd/run"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/runner"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/setting"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/setup"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/signingconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/step"
	cmdtest "github.com/CircleCI-Public/circleci-cli/internal/cmd/test"
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
	initConfig := func(cmd *cobra.Command) (func(), error) {
		if cmdutil.IsEverythingDisabled(cmd) {
			return func() {}, nil
		}

		ctx := cmd.Context()
		if cmdutil.CheckTelemetry(ctx) {
			return func() {}, nil
		}

		// Extension commands disable cobra flag parsing so the extension
		// receives its args verbatim, which leaves the root persistent flags
		// (--theme, --debug, --quiet, ...) unpopulated here. Parse them from
		// os.Args before anything below reads them.
		if cmd.DisableFlagParsing {
			extension.ParseRootFlags(cmd)
		}

		// --no-color is canonicalized into the NO_COLOR env var here, before
		// streams are built or anything renders. This is the only way to reach
		// third-party renderers (glamour, lipgloss, the bubbletea viewport pager)
		// that strip color based on NO_COLOR but know nothing about our flags.
		// Doing it once in bootstrap keeps the flag behaving exactly like the env
		// var everywhere, instead of gating each render path individually.
		if noColor, _ := cmd.Flags().GetBool("no-color"); noColor {
			_ = os.Setenv("NO_COLOR", "1")
		}

		theme, err := cmd.Flags().GetString("theme")
		if err == nil && !iostream.IsValidTheme(theme) {
			return func() {}, clierrors.New("flags.invalid_theme", "Invalid theme", "Invalid value for --theme: '"+theme+"'").
				WithSuggestions("Valid themes are: " + strings.Join(iostream.ValidThemes(), ", ")).
				WithExitCode(clierrors.ExitBadArguments)
		}

		secureStorage := cmdutil.IsSecureStorage(cmd)
		configPath := cmdutil.ConfigPath(cmd)

		// Load config before stream setup so a stored "theme" setting can act as
		// the fallback when --theme is not explicitly passed. config.Load only
		// uses ctx for its file lock timeout, so the streamless context is fine.
		cfg, err := config.Load(ctx, configPath, secureStorage)
		if err != nil {
			return func() {}, err
		}

		ctx = iostream.FromCmd(ctx, cmd, cfg.EffectiveTheme())
		ctx = cmdutil.WithVersion(ctx, version)
		ctx = cmdutil.WithConfig(ctx, cfg)

		agentName := agent.Detect()
		ctx = cmdutil.WithAgentName(ctx, agentName)

		jqFilter, _ := cmd.Flags().GetString("jq")
		ctx = iostream.WithJQFilter(ctx, jqFilter)

		// Only gather host info when telemetry will actually be sent. Skipping
		// it for telemetry-disabled commands (e.g. completion generation) avoids
		// gopsutil's `ioreg` lookup, which fails under a restricted PATH such as
		// Homebrew's sanitized completion-generation environment.
		var hostInfo *host.InfoStat
		if cfg.IsTelemetry() && !cmdutil.IsTelemetryDisabled(cmd) {
			hostInfo, err = host.InfoWithContext(ctx)
			if err != nil {
				return func() {}, err
			}
		}

		executable := executablePath("circleci")

		tc, err := telemetry.NewSender(ctx, telemetry.Config{
			Log:      cfg.IsTelemetry(),
			Send:     cfg.IsTelemetry(),
			WriteKey: telemetry.SegmentKey,
			Endpoint: os.Getenv("CIRCLE_TELEMETRY_ENDPOINT"),
			Binary:   executable,
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
			return func() {}, err
		}

		ctx = cmdutil.WithTelemetry(ctx, tc)

		cmd.SetContext(ctx)

		cleanup := func() {
			_ = tc.Close()
		}

		return cleanup, nil
	}

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
	cmd.PersistentFlags().BoolP("no-color", "", false, "disable ANSI color output (same as setting NO_COLOR)")

	cmd.PersistentFlags().BoolP("insecure-storage", "", false, "do not use the system's secure storage for storing tokens")
	_ = cmd.PersistentFlags().MarkHidden("insecure-storage")

	cmd.PersistentFlags().BoolP("skip-update-check", "", false, "skip checking for CLI updates")
	_ = cmd.PersistentFlags().MarkHidden("skip-update-check")

	cmd.AddGroup(&cobra.Group{
		ID:    "ci",
		Title: "CI commands",
	})
	cmd.AddGroup(&cobra.Group{
		ID:    "management",
		Title: "Management commands",
	})
	cmd.AddGroup(&cobra.Group{
		ID:    "user",
		Title: "User commands",
	})
	cmd.AddGroup(&cobra.Group{
		ID:    "extension",
		Title: "Extension commands",
	})

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
	cmd.AddCommand(my.NewMyCmd())
	cmd.AddCommand(pipeline.NewPipelineCmd())
	cmd.AddCommand(project.NewProjectCmd())
	cmd.AddCommand(receivetelemetry.NewReceiveTelemetryCmd())
	cmd.AddCommand(runner.NewRunnerCmd())
	cmd.AddCommand(setting.NewSettingCmd())
	cmd.AddCommand(setup.NewSetupCmd())
	cmd.AddCommand(signingconfig.NewSigningConfigCmd())
	cmd.AddCommand(step.NewStepCmd())
	cmd.AddCommand(cmdtest.NewTestCmd())
	cmd.AddCommand(workflow.NewWorkflowCmd())

	// Wire in MCP commands. ophis sets its own terse Short; override it so the
	// root command table explains what the command actually does.
	mcpCmd := ophis.Command(nil)
	mcpCmd.Short = "Run the CLI as an MCP server for AI tools"
	mcpCmd.GroupID = "user"
	cmd.AddCommand(mcpCmd)

	// Help topics
	var referenceCmd *cobra.Command
	for _, ht := range helpTopics {
		helpTopicCmd := newCmdHelpTopic(ht, initConfig)
		cmd.AddCommand(helpTopicCmd)

		// See bottom of the function for why we explicitly care about the reference cmd
		if ht.name == "reference" {
			referenceCmd = helpTopicCmd
		}
	}

	// man pages
	cmd.AddCommand(newManCmd())

	// Register extensions found in PATH. Built-in commands always win on name
	// conflicts — extensions cannot shadow them.
	builtins := map[string]bool{}
	for _, sub := range cmd.Commands() {
		builtins[sub.Name()] = true
	}

	path := os.Getenv("PATH")
	if exts := extension.FindAll(path); len(exts) > 0 {
		for _, name := range exts {
			if !builtins[name] {
				cmd.AddCommand(extension.NewCmd(name))
			}
		}
	}

	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}
		_, err := initConfig(cmd)
		return err
	}
	cmd.PersistentPostRunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		tc := cmdutil.GetTelemetry(ctx)
		_ = tc.Close()
		return nil
	}

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cleanup, err := initConfig(cmd); err == nil {
			cmdutil.RecordTelemetryNow(cmd)
			cleanup()
		}

		rootHelp(cmd, args)
	})

	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		if _, err := initConfig(cmd); err == nil {
			cmdutil.RecordTelemetryNow(cmd)
		}

		return rootUsage(cmd)
	})

	cmdutil.RecordTelemetryForSubcommands(cmd)

	if referenceCmd != nil {
		referenceCmd.Long = stringifyReference(cmd)
	}

	return cmd
}
