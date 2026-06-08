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
	if os.Getenv("TERM") == "dumb" {
		return true
	}
	return false
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
func FromCmd(ctx context.Context, cmd *cobra.Command) context.Context {
	lvl := log.InfoLevel
	verbose, _ := cmd.Flags().GetBool("debug")
	if verbose {
		lvl = log.DebugLevel
	}
	theme, _ := cmd.Flags().GetString("theme")
	quiet, _ := cmd.Flags().GetBool("quiet")

	stdin := cmd.InOrStdin()
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()
	width, style := terminalProperties(theme, stdin, stdout)

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

func terminalProperties(theme string, in io.Reader, out io.Writer) (width int, style string) {
	switch theme {
	case "auto":
		// This lipgloss function doesn't seem to work on Windows
		if runtime.GOOS == "windows" {
			break
		}

		stdIn, ok := in.(term.File)
		if !ok {
			break
		}

		stdOut, ok := out.(term.File)
		if !ok {
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
func (s Streams) ColorEnabled() bool {
	return s.IsTerminal() && !colorDisabled()
}

// IsInteractive reports whether the session can support interactive prompts.
// False when: not a TTY, CI=true (running in a CI environment),
// or CIRCLE_NO_INTERACTIVE is set.
func (s Streams) IsInteractive() bool {
	if !s.IsTerminal() {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	if os.Getenv("CIRCLE_NO_INTERACTIVE") != "" {
		return false
	}
	return true
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
//	echo "mytoken" | circleci settings set token -
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
	p := tea.NewProgram(
		ui.NewSelectModel(prompt, options),
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
func (s Streams) RenderMarkdown(md string) (_ string, err error) {
	if !s.ColorEnabled() {
		return md, nil
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(s.width),
		glamour.WithTableFitContent(),
		glamour.WithStyles(s.styleConfig()),
		glamour.WithInlineTableLinks(true),
	)
	if err != nil {
		return md, err
	}
	defer closer.ErrorHandler(r, &err)

	return r.Render(md)
}

// PrintMarkdown renders md and writes the result to Out.
// Falls back to writing raw markdown on render error.
func (s Streams) PrintMarkdown(md string) {
	rendered, err := s.RenderMarkdown(md)
	if err != nil {
		_, _ = fmt.Fprint(s.Out, md)
		return
	}
	_, _ = fmt.Fprint(s.Out, rendered)
}

func (s Streams) styleConfig() ansi.StyleConfig {
	if s.style == ansiStyle {
		return ansiStyleConfig
	}
	styleConfig, ok := styles.DefaultStyles[s.style]
	if !ok {
		return styles.ASCIIStyleConfig
	}
	return *styleConfig
}
