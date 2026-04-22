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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// colorDisabled returns true when any of the standard "no color" signals are present.
// Checked: NO_COLOR (no-color.org), CIRCLECI_NO_COLOR, TERM=dumb.
// Does NOT check TTY — call IsTerminal() for that.
func colorDisabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	if os.Getenv("CIRCLECI_NO_COLOR") != "" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return true
	}
	return false
}

// Streams bundles the I/O channels passed through every command.
// All output must go through Streams — never write to os.Stdout directly.
type Streams struct {
	Out   io.Writer // structured output (data results)
	Err   io.Writer // status messages, errors, progress
	In    io.Reader // user input for interactive prompts
	Quiet bool      // when true, ErrPrintf/ErrPrintln produce no output
}

// OS returns a Streams wired to the real os.Stdin / os.Stdout / os.Stderr.
func OS() Streams {
	return Streams{Out: os.Stdout, Err: os.Stderr, In: os.Stdin}
}

// FromCmd extracts Streams from a cobra.Command's Out/Err/In and reads the
// --quiet persistent flag if registered on the root command.
func FromCmd(cmd *cobra.Command) Streams {
	quiet, _ := cmd.Flags().GetBool("quiet")
	return Streams{Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr(), In: cmd.InOrStdin(), Quiet: quiet}
}

// Test returns a Streams backed by the provided writers with no-op stdin,
// useful in tests that don't exercise interactive prompts.
func Test(out, err io.Writer) Streams {
	return Streams{Out: out, Err: err, In: strings.NewReader("")}
}

// IsTerminal reports whether Out is a terminal (i.e. a human is watching).
func (s Streams) IsTerminal() bool {
	if f, ok := s.Out.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// ColorEnabled reports whether color and Unicode symbols should be used.
// False when: not a TTY, NO_COLOR set, CIRCLECI_NO_COLOR set, or TERM=dumb.
func (s Streams) ColorEnabled() bool {
	return s.IsTerminal() && !colorDisabled()
}

// IsInteractive reports whether the session can support interactive prompts.
// False when: not a TTY, CI=true (running in a CI environment),
// or CIRCLECI_NO_INTERACTIVE is set.
func (s Streams) IsInteractive() bool {
	if !s.IsTerminal() {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	if os.Getenv("CIRCLECI_NO_INTERACTIVE") != "" {
		return false
	}
	return true
}

// Symbol returns the Unicode symbol when color is enabled, or the ASCII
// fallback otherwise. Use this for decorative indicators like checkmarks.
//
//	streams.Symbol("✓", "ok")   →  "✓"  (TTY)  or  "ok"  (non-TTY/no-color)
func (s Streams) Symbol(unicode, ascii string) string {
	if s.ColorEnabled() {
		return unicode
	}
	return ascii
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

// Confirm prints prompt followed by " [y/N] " to stderr and reads one line
// from In. Returns true only if the user types "y" or "yes" (case-insensitive).
// Returns false on empty input, "n"/"no", EOF, or any read error — the safe
// answer is always No.
func (s Streams) Confirm(prompt string) bool {
	_, _ = fmt.Fprintf(s.Err, "%s [y/N] ", prompt)
	scanner := bufio.NewScanner(s.In)
	if !scanner.Scan() {
		return false
	}
	response := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return response == "y" || response == "yes"
}
