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

package cmdconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configaudit"
)

// auditRef is the documentation link every error from "config audit" carries.
const auditRef = "https://cli.circleci.com/reference/"

// The values of "config audit --format".
const (
	formatText  = "text"
	formatSARIF = "sarif"
)

func newAuditCmd() *cobra.Command {
	var (
		minSeverity string
		format      string
		jsonOut     bool
		online      bool
		project     string
	)

	cmd := &cobra.Command{
		Use:   "audit [<path>]",
		Short: "Check a pipeline config for supply-chain risks",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<path>%[1]s is the pipeline config file to audit, by default
				%[1]s.circleci/config.yml%[1]s. Pass %[1]s-%[1]s to read the config from stdin.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Runs offline unless --online is given; orbs are not expanded. Exits 7 when a finding
			is at or above --min-severity. Rules and suppression comments: 'circleci help config-audit'.

			JSON fields (--json): file, findings (rule_id, severity, confidence, message, fix, file, line, column, end_line, end_column, snippet, location), summary (high, medium, low, total, suppressed), online (checked, project, requests), warnings
		`),
		Example: heredoc.Doc(`
			# Audit the default config file
			$ circleci config audit

			# Audit a file and show only high-severity findings
			$ circleci config audit path/to/config.yml --min-severity high

			# Audit config from stdin and output JSON
			$ cat config.yml | circleci config audit - --json

			# Also check the project's settings (read-only API calls)
			$ circleci config audit --online
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if project != "" && !online {
				return clierrors.New("config.audit_project_without_online", "--project needs --online",
					"--project only applies with --online.").
					WithSuggestions("Add --online to check this project's settings", "Or drop --project").
					WithRef(auditRef).
					WithExitCode(clierrors.ExitBadArguments)
			}

			threshold, err := configaudit.ParseSeverity(minSeverity)
			if err != nil {
				return clierrors.New("config.audit_bad_severity", "Invalid --min-severity",
					fmt.Sprintf("--min-severity must be one of low, medium, high (got: %q)", minSeverity)).
					WithSuggestions("Use --min-severity low, medium or high").
					WithRef(auditRef).
					WithExitCode(clierrors.ExitBadArguments)
			}

			if format != formatText && format != formatSARIF {
				return clierrors.New("config.audit_bad_format", "Invalid --format",
					fmt.Sprintf("--format must be one of text, sarif (got: %q)", format)).
					WithSuggestions("Use --format text or --format sarif", "For JSON output use --json").
					WithRef(auditRef).
					WithExitCode(clierrors.ExitBadArguments)
			}

			path := cmdutil.DefaultPipelineConfigPath
			if len(args) == 1 {
				path = args[0]
			}
			src, err := readConfigInput(ctx, path)
			if err != nil {
				return err
			}
			label := path
			if path == "-" {
				label = "<stdin>"
			}

			doc, err := configaudit.Parse([]byte(src), label)
			if err != nil {
				return clierrors.New("config.audit_parse_failed", "Could not parse config", err.Error()).
					WithSuggestions(
						"Fix the YAML syntax at the line shown",
						fmt.Sprintf("Run: circleci config validate %s", path),
					).
					WithRef(auditRef).
					WithExitCode(clierrors.ExitBadArguments)
			}

			opts := configaudit.Options{MinSeverity: threshold}
			var onlineWarnings []string
			if online {
				opts.Online, onlineWarnings = fetchOnline(ctx, project, doc.Contexts())
			}
			report := configaudit.Audit(doc, opts)
			report.Warnings = append(report.Warnings, onlineWarnings...)

			switch {
			case jsonOut:
				if err := cmdutil.WriteJSON(iostream.Out(ctx), report); err != nil {
					return err
				}
			case format == formatSARIF:
				// Warnings travel inside the log, so stderr stays quiet.
				if err := configaudit.RenderSARIF(iostream.Out(ctx), report, cmdutil.GetVersion(ctx)); err != nil {
					return err
				}
			default:
				for _, w := range report.Warnings {
					iostream.ErrPrintf(ctx, "warning: %s\n", w)
				}
				if opts.Online != nil && report.Online.Checked {
					iostream.ErrPrintf(ctx, "%s\n", checkedLine(opts.Online))
				}
				if err := configaudit.RenderText(iostream.Out(ctx), report, painter(ctx)); err != nil {
					return err
				}
			}

			if len(report.Findings) > 0 {
				return clierrors.New("config.audit_findings", "Findings found",
					fmt.Sprintf("%s at or above %s in %s.",
						configaudit.CountFindings(len(report.Findings)), threshold, label)).
					WithSuggestions(
						"See each rule's fix: circleci help config-audit",
						"To accept a finding, add # circleci-audit: ignore[<rule-id>] on its line or the line above",
					).
					WithRef(auditRef).
					WithExitCode(clierrors.ExitValidationFail)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&minSeverity, "min-severity", "low", "lowest severity to report: low, medium or high")
	cmd.Flags().StringVar(&format, "format", formatText, "output format: text or sarif")
	cmd.Flags().BoolVar(&online, "online", false, "also check project, org and context settings (read-only API calls)")
	cmd.Flags().StringVar(&project, "project", "", "project slug for --online (e.g. gh/org/repo); defaults to the git remote")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmd.MarkFlagsMutuallyExclusive("json", "format")

	return cmd
}

// fetchOnline reads the settings behind --online. It never fails: a project
// that cannot be resolved, a missing token and any failed call are warnings, and
// the offline rules run regardless. The Settings are nil when no call was made.
func fetchOnline(ctx context.Context, project string, contexts []string) (*configaudit.Settings, []string) {
	slug, err := cmdutil.ResolveProjectSlug(project)
	if err != nil {
		msg := err.Error()
		var cliErr *clierrors.CLIError
		if errors.As(err, &cliErr) {
			msg = cliErr.Message
		}
		return nil, []string{fmt.Sprintf("%s; skipping --online checks, offline results only",
			strings.Join(strings.Fields(msg), " "))}
	}
	client := cmdutil.LoadClientOptionalAuth(ctx)
	if !client.Authenticated() {
		return nil, []string{"no CircleCI API token; skipping --online checks, offline results only"}
	}
	settings, warnings := configaudit.Fetch(ctx, client, slug, contexts)
	return &settings, warnings
}

// checkedLine says which project --online checked and what it read.
func checkedLine(s *configaudit.Settings) string {
	var parts []string
	if s.Proj != nil {
		parts = append(parts, "project settings")
	}
	if s.Org != nil {
		parts = append(parts, "org settings")
	}
	if n := len(s.Contexts); n == 1 {
		parts = append(parts, "1 context")
	} else if n > 1 {
		parts = append(parts, fmt.Sprintf("%d contexts", n))
	}
	return fmt.Sprintf("checked %s: %s (read-only)", s.Project, strings.Join(parts, ", "))
}

// painter colours a severity label: high as an error, medium as a warning, low
// muted.
func painter(ctx context.Context) configaudit.Painter {
	streams := iostream.Get(ctx)
	return func(sev configaudit.Severity, s string) string {
		switch sev {
		case configaudit.SeverityHigh:
			return streams.SymbolError(s)
		case configaudit.SeverityMedium:
			return streams.Warning(s)
		case configaudit.SeverityLow:
			return streams.Muted(s)
		}
		return s
	}
}
