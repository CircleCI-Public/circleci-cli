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

package iostream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/closer"
	"github.com/CircleCI-Public/circleci-cli/internal/jq"
	"github.com/CircleCI-Public/circleci-cli/internal/jsoncolor"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

var indentRE = regexp.MustCompile(`(?m)^`)

func Indent(s, indent string) string {
	if len(strings.TrimSpace(s)) == 0 {
		return s
	}
	return indentRE.ReplaceAllLiteralString(s, indent)
}

type jqFilterKey struct{}

func WithJQFilter(ctx context.Context, jqFilter string) context.Context {
	return context.WithValue(ctx, jqFilterKey{}, jqFilter)
}

// interactiveEnvDisabled returns true when environment variables force
// non-interactive behavior, independent of whether the streams are TTYs.
// Checked: CI (set by CI systems) and CIRCLE_NO_INTERACTIVE (explicit opt-out).
// Does NOT check TTY — call IsTerminal() for that.
func interactiveEnvDisabled() bool {
	if os.Getenv("CI") != "" {
		return true
	}
	if os.Getenv("CIRCLE_NO_INTERACTIVE") != "" {
		return true
	}
	return false
}

// colorDisabled returns true when any of the standard "no color" signals are present.
// Checked: NO_COLOR (no-color.org), CIRCLE_NO_COLOR, TERM=dumb.
// Does NOT check TTY — call IsTerminal() for that.
func colorDisabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	if os.Getenv("CIRCLE_NO_COLOR") != "" {
		return true
	}
	// TERM=dumb conventionally means "no ANSI capability", so it normally
	// disables color. CircleCI, however, sets TERM=dumb on every job while its
	// log viewer renders ANSI just fine — honoring it there would strip color
	// from all CI output. So dumb only disables color outside CircleCI; the
	// explicit NO_COLOR / CIRCLE_NO_COLOR opt-outs above still win everywhere.
	if os.Getenv("TERM") == "dumb" && !isCircleCI() {
		return true
	}
	return false
}

// isCircleCI reports whether we're running inside a CircleCI job, which sets
// CIRCLECI=true. CircleCI's log viewer renders ANSI color despite TERM=dumb.
func isCircleCI() bool {
	return os.Getenv("CIRCLECI") != ""
}

// pagerProgram interprets the PAGER environment variable.
//
//   - set is false when PAGER is unset, meaning the caller should use the
//     built-in scrollable viewport.
//   - set is true with cmd == "" when PAGER is empty or "cat" — the
//     conventional way to turn paging off, so output is printed inline.
//   - set is true with a non-empty cmd (e.g. "less", "more", "less -R") when
//     the user has chosen an external pager program to pipe output through.
func pagerProgram() (cmd string, set bool) {
	v, ok := os.LookupEnv("PAGER")
	if !ok {
		return "", false
	}
	v = strings.TrimSpace(v)
	if v == "" || v == "cat" {
		return "", true
	}
	return v, true
}

type contextKey struct{}

func fromContext(ctx context.Context) Streams {
	v := ctx.Value(contextKey{})
	if v == nil {
		return Streams{Out: io.Discard, Err: io.Discard, In: strings.NewReader(""), Quiet: true}
	}
	return v.(Streams)
}

func WithStreams(ctx context.Context, s Streams) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

// Get returns the Streams stored in ctx, or a discard-everything Streams if
// none was set. Use this when you need to pass a Streams value to a helper
// function (e.g. cmdutil.ConfirmOrForce) rather than calling the ctx-based
// package-level wrappers.
func Get(ctx context.Context) Streams {
	return fromContext(ctx)
}

// Package-level accessors for Steam functions

func IsTerminal(ctx context.Context) bool {
	return fromContext(ctx).IsTerminal()
}

func ColorEnabled(ctx context.Context) bool {
	return fromContext(ctx).ColorEnabled()
}

func IsInteractive(ctx context.Context) bool {
	return fromContext(ctx).IsInteractive()
}

func SpinnerEnabled(ctx context.Context) bool {
	return fromContext(ctx).SpinnerEnabled()
}

func SymbolOK(ctx context.Context) string {
	return fromContext(ctx).SymbolSuccess(theme.IconOK)
}

func SymbolWarn(ctx context.Context) string {
	return fromContext(ctx).SymbolSuccess(theme.IconWarn)
}

func SymbolFail(ctx context.Context) string {
	return fromContext(ctx).SymbolSuccess(theme.IconFail)
}

func Title(ctx context.Context, strs ...string) string {
	return fromContext(ctx).Title(strs...)
}

func Print(ctx context.Context, v string) {
	fromContext(ctx).Print(v)
}

func Printf(ctx context.Context, format string, a ...any) {
	fromContext(ctx).Printf(format, a...)
}

func Println(ctx context.Context, a ...any) {
	fromContext(ctx).Println(a...)
}

func ErrPrint(ctx context.Context, s string) {
	fromContext(ctx).ErrPrint(s)
}

func ErrPrintf(ctx context.Context, format string, a ...any) {
	fromContext(ctx).ErrPrintf(format, a...)
}

func ErrPrintln(ctx context.Context, a ...any) {
	fromContext(ctx).ErrPrintln(a...)
}

// PromptSecret presents a masked text input to collect a secret value.
// Returns ("", nil) if the user cancels.
func PromptSecret(ctx context.Context, header string) (string, error) {
	return fromContext(ctx).PromptSecret(ctx, header)
}

// PromptSelect presents an interactive single-choice list to the user and
// returns the index of the selected option. Returns (-1, nil) if the user
// cancels with esc or ctrl+c.
func PromptSelect(ctx context.Context, prompt string, options []string) (int, error) {
	return fromContext(ctx).PromptSelect(ctx, prompt, options)
}

// PromptSelectDefault is like PromptSelect but pre-highlights the option at
// defaultIdx. Returns the index of the selected option, or -1 if cancelled.
func PromptSelectDefault(ctx context.Context, prompt string, options []string, defaultIdx int) (int, error) {
	return fromContext(ctx).PromptSelectDefault(ctx, prompt, options, defaultIdx)
}

// PromptThemePreview presents a split-pane theme picker with a live markdown
// preview rendered in the highlighted theme. See Streams.PromptThemePreview.
func PromptThemePreview(ctx context.Context, prompt string, labels, themes []string, defaultIdx int, sampleMarkdown string) (int, error) {
	return fromContext(ctx).PromptThemePreview(ctx, prompt, labels, themes, defaultIdx, sampleMarkdown)
}

// PromptText presents a plain (non-secret) single-line text input via
// bubbletea. header is the bold heading above the input; placeholder is
// shown inside the empty field; defaultVal (optional) is returned when the
// user presses Enter with an empty field. Returns ("", nil) if the user
// cancels with esc or ctrl+c.
func PromptText(ctx context.Context, header, placeholder string, defaultVal ...string) (string, error) {
	dv := ""
	if len(defaultVal) > 0 {
		dv = defaultVal[0]
	}
	s := fromContext(ctx)
	p := tea.NewProgram(
		ui.NewPromptModel(header, placeholder, dv),
		tea.WithContext(ctx),
		tea.WithInput(s.In),
		tea.WithOutput(s.Err),
	)
	anyModel, err := p.Run()
	if err != nil {
		return "", err
	}
	m := anyModel.(ui.PromptModel)
	if m.Quitting() {
		return "", nil
	}
	return m.Value(), nil
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	fromContext(ctx).DebugContext(ctx, msg, args...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	fromContext(ctx).InfoContext(ctx, msg, args...)
}

func PrintJSON(ctx context.Context, v any) error {
	return fromContext(ctx).PrintJSON(ctx, v)
}

func PrintJSONFromReader(ctx context.Context, r io.Reader) error {
	return fromContext(ctx).PrintJSONFromReader(ctx, r)
}

func PrintMarkdown(ctx context.Context, md string) {
	fromContext(ctx).PrintMarkdown(md)
}

func Out(ctx context.Context) io.Writer {
	return fromContext(ctx).Out
}

func Err(ctx context.Context) io.Writer {
	return fromContext(ctx).Err
}

func In(ctx context.Context) io.Reader {
	return fromContext(ctx).In
}

func Spinner(ctx context.Context, active bool, msg string) *Spin {
	return fromContext(ctx).Spinner(active, msg)
}

// Streams bundles the I/O channels passed through every command.
// All output must go through Streams — never write to os.Stdout directly.
type Streams struct {
	Out   io.Writer // structured output (data results)
	Err   io.Writer // status messages, errors, progress
	In    io.Reader // user input for interactive prompts
	Quiet bool      // when true, ErrPrintf/ErrPrintln produce no output
	slog  *slog.Logger
	width int
	style string
}

func Testing(ctx context.Context) context.Context {
	stdin := os.Stdin
	stdout := os.Stdout
	stderr := os.Stderr
	width, style := terminalProperties(styles.DarkStyle, stdin, stdout)

	return WithStreams(ctx, Streams{
		Out:   stdout,
		Err:   stderr,
		In:    stdin,
		Quiet: false,
		style: style,
		width: width,
		slog: slog.New(log.NewWithOptions(stderr, log.Options{
			Level: log.DebugLevel,
		})),
	})
}

// FromCmd extracts Streams from a cobra.Command's Out/Err/In and reads the
// --quiet persistent flag if registered on the root command.
//
// The color theme is resolved with the --theme flag taking precedence when it
// was explicitly set; otherwise configTheme (the stored CLI setting, "" if
// none) is used, falling back to the flag's "auto" default.
func FromCmd(ctx context.Context, cmd *cobra.Command, configTheme string) context.Context {
	lvl := log.InfoLevel
	verbose, _ := cmd.Flags().GetBool("debug")
	if verbose {
		lvl = log.DebugLevel
	}
	quiet, _ := cmd.Flags().GetBool("quiet")

	stdin := cmd.InOrStdin()
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()
	width, style := terminalProperties(resolveTheme(cmd, configTheme), stdin, stdout)

	return WithStreams(ctx, Streams{
		Out:   stdout,
		Err:   stderr,
		In:    stdin,
		Quiet: quiet,
		style: style,
		width: width,
		slog: slog.New(log.NewWithOptions(stderr, log.Options{
			Level: lvl,
		})),
	})
}

// resolveTheme picks the color theme in precedence order:
//  1. an explicitly-passed --theme flag (always wins over config),
//  2. the stored config theme (configTheme; "" when none is configured),
//  3. the --theme flag's default value ("auto").
func resolveTheme(cmd *cobra.Command, configTheme string) string {
	flagTheme, _ := cmd.Flags().GetString("theme")
	if cmd.Flags().Changed("theme") {
		return flagTheme
	}
	if configTheme != "" {
		return configTheme
	}
	return flagTheme
}

// backgroundQueryable reports whether it is safe to probe the terminal for its
// background color. The probe (an OSC 11 escape) blocks until the terminal
// replies or a 2s-per-stream timeout fires, so it must only run against a real,
// interactive terminal. CI runners frequently allocate a PTY — so both streams
// report as terminals — yet nothing answers the query; gate on the same
// signals that mark a session non-interactive (CI, CIRCLE_NO_INTERACTIVE) plus
// the no-color signals, since a disabled-color session won't use the result.
func backgroundQueryable(in, out term.File) bool {
	if !term.IsTerminal(in.Fd()) || !term.IsTerminal(out.Fd()) {
		return false
	}
	return !colorDisabled() && !interactiveEnvDisabled()
}

func terminalProperties(theme string, in io.Reader, out io.Writer) (width int, style string) {
	switch theme {
	case themeAuto:
		stdIn, ok := in.(term.File)
		if !ok {
			break
		}

		stdOut, ok := out.(term.File)
		if !ok {
			break
		}

		if !backgroundQueryable(stdIn, stdOut) {
			// Detecting the background means writing an OSC 11 query to the
			// terminal and waiting for a reply. In a non-interactive context
			// (CI, NO_INTERACTIVE, no-color, or TERM=dumb) no reply ever comes,
			// so each query blocks until lipgloss's 2s timeout — and it queries
			// both stdin and stdout, so the stall is ~4s on every invocation,
			// including `--help`. CI runners that allocate a PTY hit this even
			// though the streams report as terminals. Default to the dark style
			// (lipgloss's own on-error default) without the query.
			style = styles.DarkStyle
			break
		}

		if lipgloss.HasDarkBackground(stdIn, stdOut) {
			style = styles.DarkStyle
		} else {
			style = styles.LightStyle
		}
	case ansiStyle:
		// Not a glamour built-in: a custom style using only the standard 16
		// ANSI colors so it adapts to the user's terminal palette.
		style = ansiStyle
	default:
		_, ok := styles.DefaultStyles[theme]
		if ok {
			style = theme
		} else {
			style = styles.AsciiStyle
		}
	}

	width = 120
	if f, ok := out.(term.File); ok {
		if w, _, err := term.GetSize(f.Fd()); err == nil && w > 0 {
			width = w
		}
	}
	if width > 140 {
		width = 140
	}

	return width, style
}

// IsTerminal reports whether Out is a terminal (i.e. a human is watching).
func (s Streams) IsTerminal() bool {
	if f, ok := s.Out.(*os.File); ok {
		return term.IsTerminal(f.Fd())
	}
	return false
}

// ColorEnabled reports whether color and Unicode symbols should be used.
// False when: not a TTY, NO_COLOR set, CIRCLE_NO_COLOR set, or TERM=dumb.
// The --no-color flag is honored here too: root canonicalizes it into NO_COLOR
// before streams are built, so colorDisabled() already accounts for it.
func (s Streams) ColorEnabled() bool {
	return s.IsTerminal() && !colorDisabled()
}

// IsInteractive reports whether the session can support interactive prompts.
// False when: not a TTY, CI=true (running in a CI environment),
// or CIRCLE_NO_INTERACTIVE is set.
func (s Streams) IsInteractive() bool {
	return s.IsTerminal() && !interactiveEnvDisabled()
}

// SpinnerEnabled reports whether an animated spinner should run: only in an
// interactive session with CIRCLE_SPINNER_DISABLED unset. Long-lived bubbletea
// programs that animate their own spinner (e.g. the run-get flow) should consult
// this and keep their loading placeholder static otherwise.
func (s Streams) SpinnerEnabled() bool {
	return s.IsInteractive() && !spinnerDisabled()
}

func (s Streams) SymbolSuccess(strs ...string) string {
	if !s.ColorEnabled() {
		return theme.NoColorStyle.Render(strs...)
	}

	return theme.SuccessStyle.Render(strs...)
}

func (s Streams) SymbolWarning(strs ...string) string {
	if !s.ColorEnabled() {
		return theme.NoColorStyle.Render(strs...)
	}

	return theme.WarningStyle.Render(strs...)
}

func (s Streams) SymbolError(strs ...string) string {
	if !s.ColorEnabled() {
		return theme.NoColorStyle.Render(strs...)
	}

	return theme.WarningStyle.Render(strs...)
}

func (s Streams) Title(strs ...string) string {
	if !s.ColorEnabled() {
		return theme.NoColorStyle.Render(strs...)
	}

	return theme.TitleStyle.Render(strs...)
}

// Print writes a string to Out with no newline appended.
func (s Streams) Print(v string) {
	_, _ = fmt.Fprint(s.Out, v)
}

// Printf writes a formatted string to Out.
func (s Streams) Printf(format string, a ...any) {
	_, _ = fmt.Fprintf(s.Out, format, a...)
}

// Println writes a line to Out.
func (s Streams) Println(a ...any) {
	_, _ = fmt.Fprintln(s.Out, a...)
}

// ErrPrint writes a string to Err. No-op when Quiet is true.
func (s Streams) ErrPrint(str string) {
	if s.Quiet {
		return
	}
	_, _ = fmt.Fprint(s.Err, str)
}

// ErrPrintf writes a formatted string to Err. No-op when Quiet is true.
func (s Streams) ErrPrintf(format string, a ...any) {
	if s.Quiet {
		return
	}
	_, _ = fmt.Fprintf(s.Err, format, a...)
}

// ErrPrintln writes a line to Err. No-op when Quiet is true.
func (s Streams) ErrPrintln(a ...any) {
	if s.Quiet {
		return
	}
	_, _ = fmt.Fprintln(s.Err, a...)
}

// ReadSecret returns value as-is, unless value is "-", in which case it reads
// one line from In. Use this for flags or arguments that accept sensitive
// values (tokens, passwords, secrets) to allow callers to pipe the value in
// without exposing it in shell history or process listings:
//
//	echo "mytoken" | circleci setting set token -
func (s Streams) ReadSecret(value string) (string, error) {
	if value != "-" {
		return value, nil
	}
	scanner := bufio.NewScanner(s.In)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("expected value on stdin but got EOF")
	}
	return scanner.Text(), nil
}

// ReadSecret reads a sensitive value from the streams in context.
// Returns value as-is unless value is "-", in which case reads one line from stdin.
func ReadSecret(ctx context.Context, value string) (string, error) {
	return fromContext(ctx).ReadSecret(value)
}

// Confirm presents a y/N confirmation prompt via bubbletea.
// Returns true only if the user presses y/Y. Returns false on n/N, esc,
// ctrl+c, enter, or any program error — the safe answer is always No.
func (s Streams) Confirm(ctx context.Context, prompt string) bool {
	p := tea.NewProgram(
		ui.NewConfirmModel(prompt),
		tea.WithContext(ctx),
		tea.WithInput(s.In),
		tea.WithOutput(s.Err),
	)
	anyModel, err := p.Run()
	if err != nil {
		return false
	}
	return anyModel.(ui.ConfirmModel).Confirmed()
}

// PromptSelect presents a bubbletea single-choice list prompt.
// Returns the selected index, or -1 if the user cancels.
func (s Streams) PromptSelect(ctx context.Context, prompt string, options []string) (int, error) {
	return s.PromptSelectDefault(ctx, prompt, options, 0)
}

// PromptSelectDefault is like PromptSelect but starts the cursor on
// defaultIdx (clamped to the options) so a default choice is pre-highlighted.
func (s Streams) PromptSelectDefault(ctx context.Context, prompt string, options []string, defaultIdx int) (int, error) {
	p := tea.NewProgram(
		ui.NewSelectModel(prompt, options).WithCursor(defaultIdx),
		tea.WithContext(ctx),
		tea.WithInput(s.In),
		tea.WithOutput(s.Err),
	)
	anyModel, err := p.Run()
	if err != nil {
		return -1, err
	}
	m := anyModel.(ui.SelectModel)
	if m.Cancelled() {
		return -1, nil
	}
	return m.Selected(), nil
}

// PromptThemePreview presents a split-pane theme picker: a select list of
// labels on the left and a live preview of sampleMarkdown rendered in the
// highlighted theme on the right. themes are the raw theme names parallel to
// labels; cursorIdx is the initially-highlighted option. Returns the selected
// index, or -1 if the user cancels.
func (s Streams) PromptThemePreview(ctx context.Context, prompt string, labels, themes []string, cursorIdx int, sampleMarkdown string) (int, error) {
	render := func(theme string, width int) string {
		out, err := s.renderMarkdownThemeAt(sampleMarkdown, theme, width)
		if err != nil {
			return sampleMarkdown
		}
		return out
	}
	p := tea.NewProgram(
		ui.NewThemePickerModel(prompt, labels, themes, render, s.ColorEnabled(), !spinnerDisabled()).WithCursor(cursorIdx),
		tea.WithContext(ctx),
		tea.WithInput(s.In),
		tea.WithOutput(s.Err),
	)
	anyModel, err := p.Run()
	if err != nil {
		return -1, err
	}
	m := anyModel.(ui.ThemePickerModel)
	if m.Cancelled() {
		return -1, nil
	}
	return m.Selected(), nil
}

// PromptSecret presents a masked text input via bubbletea to collect a secret
// value. header is displayed above the input field (e.g. "Enter value for MY_VAR").
// Returns ("", nil) if the user cancels with esc or ctrl+c.
func (s Streams) PromptSecret(ctx context.Context, header string) (string, error) {
	p := tea.NewProgram(
		ui.NewSecretModel(header),
		tea.WithContext(ctx),
		tea.WithInput(s.In),
		tea.WithOutput(s.Err),
	)
	anyModel, err := p.Run()
	if err != nil {
		return "", err
	}
	m := anyModel.(ui.SecretModel)
	if m.Quitting() {
		return "", nil
	}
	return m.Value(), nil
}

func (s Streams) DebugContext(ctx context.Context, msg string, args ...any) {
	s.slog.DebugContext(ctx, msg, args...)
}

func (s Streams) InfoContext(ctx context.Context, msg string, args ...any) {
	s.slog.InfoContext(ctx, msg, args...)
}

func (s Streams) PrintJSON(ctx context.Context, v any) error {
	buf := bytes.Buffer{}
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return err
	}

	return s.PrintJSONFromReader(ctx, &buf)
}

func (s Streams) PrintJSONFromReader(ctx context.Context, r io.Reader) error {
	jqFilter := filterFromContext(ctx)

	indent := ""
	if s.IsTerminal() {
		indent = "  "
	}
	if jqFilter != "" {
		return jq.Evaluate(r, s.Out, jq.Options{
			Expr:     jqFilter,
			Indent:   indent,
			Colorize: s.ColorEnabled(),
		})
	}

	if s.ColorEnabled() {
		return jsoncolor.Write(s.Out, r, "  ")
	}

	_, err := io.Copy(s.Out, r)
	return err
}

func filterFromContext(ctx context.Context) string {
	v := ctx.Value(jqFilterKey{})
	jqFilter, ok := v.(string)
	if !ok {
		panic("no jq filter")
	}
	return jqFilter
}

// RenderMarkdown renders md as styled markdown when color is enabled, falling
// back to the raw string when output is not a TTY or color is disabled.
// The rendered string is returned; use PrintMarkdown to write it to Out.
func (s Streams) RenderMarkdown(md string) (string, error) {
	if !s.ColorEnabled() {
		return md, nil
	}
	return s.renderMarkdownAt(md, s.width)
}

// renderMarkdownAt renders md as styled markdown word-wrapped to width columns.
// Unlike RenderMarkdown it does not consult ColorEnabled — callers use it once
// they have already decided to produce styled output (e.g. the viewport pager).
func (s Streams) renderMarkdownAt(md string, width int) (_ string, err error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(width),
		glamour.WithTableFitContent(),
		glamour.WithEmoji(),
		glamour.WithStyles(s.styleConfig()),
		glamour.WithInlineTableLinks(true),
	)
	if err != nil {
		return md, err
	}
	defer closer.ErrorHandler(r, &err)

	return r.Render(md)
}

// renderMarkdownThemeAt renders md word-wrapped to width columns using the
// markdown style for the named theme, rather than the stream's own configured
// theme. "auto" (and any name needing terminal detection) is resolved via
// terminalProperties. Used by the interactive theme picker to preview a theme
// the user has not committed to yet.
func (s Streams) renderMarkdownThemeAt(md, theme string, width int) (_ string, err error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(width),
		glamour.WithTableFitContent(),
		glamour.WithStyles(s.styleConfigForTheme(theme)),
		glamour.WithInlineTableLinks(true),
	)
	if err != nil {
		return md, err
	}
	defer closer.ErrorHandler(r, &err)

	return r.Render(md)
}

// styleConfigForTheme resolves a theme name to its glamour style config. Unlike
// styleConfig (which uses the stream's already-resolved style), it accepts any
// valid theme name and resolves "auto" against the terminal background, so a
// preview matches what the theme would actually produce.
func (s Streams) styleConfigForTheme(theme string) ansi.StyleConfig {
	_, style := terminalProperties(theme, s.In, s.Out)
	if sc, ok := themeStyles[style]; ok {
		return sc
	}
	return styles.ASCIIStyleConfig
}

// PrintMarkdown renders md and writes the result to Out. When Out is an
// interactive terminal and the rendered output is taller than the screen, it
// is shown in a scrollable full-screen viewport instead. Falls back to writing
// raw markdown on render error.
func (s Streams) PrintMarkdown(md string) {
	rendered, err := s.RenderMarkdown(md)
	if err != nil {
		_, _ = fmt.Fprint(s.Out, md)
		return
	}
	if s.pageMarkdown(md, rendered) {
		return
	}
	_, _ = fmt.Fprint(s.Out, rendered)
}

// pageMarkdown displays the rendered markdown through a pager when Out is an
// interactive terminal. An explicit PAGER program is piped the output; with no
// PAGER set, content taller than the screen is shown in the built-in
// scrollable viewport. It returns true when it has handled the output, in which
// case the caller must not also print inline. Any failure (non-interactive,
// pager disabled, size unknown, content fits, program error) returns false so
// the caller prints normally.
func (s Streams) pageMarkdown(md, rendered string) bool {
	if !s.IsInteractive() {
		return false
	}
	if os.Getenv("CIRCLE_NO_PAGER") != "" {
		return false
	}
	if cmd, set := pagerProgram(); set {
		// An explicit PAGER overrides the built-in viewport: "" disables
		// paging (print inline); anything else is run as an external pager.
		if cmd == "" {
			return false
		}
		return s.runPager(cmd, rendered)
	}

	out, ok := s.Out.(term.File)
	if !ok {
		return false
	}
	width, height, err := term.GetSize(out.Fd())
	if err != nil || width <= 0 || height <= 0 {
		return false
	}
	// Nothing to page if it already fits on screen (leaving room for the footer).
	if lipgloss.Height(rendered) <= height-ui.MarkdownViewportFooterHeight {
		return false
	}

	model := ui.NewMarkdownViewportModel(func(w int) string {
		r, err := s.renderMarkdownAt(md, w)
		if err != nil {
			return rendered
		}
		return r
	})
	p := tea.NewProgram(model,
		tea.WithInput(s.In),
		tea.WithOutput(s.Out),
	)
	_, err = p.Run()
	return err == nil
}

// runPager pipes content through the external pager command line (e.g. "less"
// or "less -R"), connecting the pager to the user's terminal. It returns true
// when the pager ran successfully; on any error (pager not found, exec failed)
// it returns false so the caller falls back to printing inline.
//
// When LESS is not already set we default it to "FRX" — the same defaults git
// uses — so colored output is shown raw (R), the pager quits if the content
// fits on one screen (F), and the screen is not cleared on exit (X).
//
// cmdline is run through the platform shell so PAGER may carry flags (e.g.
// "less -R"): sh on Unix, cmd on Windows (where "more" is built in and "less"
// works if installed).
func (s Streams) runPager(cmdline, content string) bool {
	cmd := pagerCommand(cmdline)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = s.Out
	cmd.Stderr = s.Err
	if _, ok := os.LookupEnv("LESS"); !ok {
		cmd.Env = append(os.Environ(), "LESS=FRX")
	}
	return cmd.Run() == nil
}

// pagerCommand builds the exec.Cmd that runs cmdline through the platform shell.
func pagerCommand(cmdline string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", cmdline) //nolint:gosec // user-controlled PAGER, by design
	}
	return exec.Command("sh", "-c", cmdline) //nolint:gosec // user-controlled PAGER, by design
}

func (s Streams) styleConfig() ansi.StyleConfig {
	if sc, ok := themeStyles[s.style]; ok {
		return sc
	}
	return styles.ASCIIStyleConfig
}
