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

package cmdfunction

import (
	"errors"
	"fmt"
	"strings"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/function"
)

// apiErr maps a discovery API failure onto a structured error. A name the API
// does not list comes back as an empty collection rather than a 404, so the
// sentinel is checked before the shared HTTP mapping.
func apiErr(err error, subject string) error {
	if errors.Is(err, apiclient.ErrFunctionNotFound) {
		return notFoundErr(subject)
	}
	return cmdutil.APIErr(err, subject,
		"function.not_found", "No published function found for %q.",
		"Run 'circleci function list' to see published functions")
}

func notFoundErr(name string) error {
	return clierrors.New("function.not_found", "Not found",
		fmt.Sprintf("No published function found for %q.", name)).
		WithSuggestions("Run 'circleci function list' to see published functions").
		WithExitCode(clierrors.ExitNotFound)
}

// versionErr maps a failure to fetch a version the function's own references
// point at, so a 404 is not reported as the function itself being missing.
func versionErr(err error, name, version string) error {
	return cmdutil.APIErr(err, name+"@"+version,
		"function.version_not_found", "Could not fetch published version %q.",
		"Run 'circleci function get "+name+"' to see every published version")
}

func noVersionsErr(name string) error {
	return clierrors.New("function.no_versions", "No published versions",
		fmt.Sprintf("Function %q has no published versions.", name)).
		WithSuggestions("Run 'circleci function list' to see published functions and their versions").
		WithExitCode(clierrors.ExitNotFound)
}

func versionNotPublishedErr(name, version string, available []string) error {
	msg := fmt.Sprintf("Function %q has no published version %q.", name, version)
	if len(available) > 0 {
		msg += "\nPublished: " + strings.Join(available, ", ")
	}
	return clierrors.New("function.version_not_found", "Not found", msg).
		WithSuggestions(
			"Omit --version to use the latest release",
			"Run 'circleci function get "+name+"' to see every published version",
		).
		WithExitCode(clierrors.ExitNotFound)
}

// tableCell makes a value safe for one markdown table cell: a newline would end
// the row and a pipe would split it. When escape is set the value is also
// markdown-escaped, for the color path where glamour renders the table; the
// no-color path passes escape=false so its raw output stays byte-identical.
func tableCell(s string, escape bool) string {
	s = strings.Join(strings.Fields(s), " ")
	s = maybeEscape(s, escape)
	return strings.ReplaceAll(s, "|", `\|`)
}

// maybeEscape markdown-escapes free text only when escape is set. Callers pass
// iostream.ColorEnabled(ctx): glamour renders the markdown when color is on, so
// published text must be escaped; when color is off PrintMarkdown writes the
// raw string, which must stay byte-identical to what was published.
func maybeEscape(s string, escape bool) string {
	if !escape {
		return s
	}
	return escapeMarkdown(s)
}

// mdEscaper backslash-escapes the inline markdown punctuation that would
// otherwise change how published text renders: emphasis (* _), HTML tags and
// links (< > [ ]), code spans (`) and a literal backslash. glamour removes the
// backslash before each of these, so the text prints exactly as published.
// Tildes and entities are left alone — glamour keeps a backslash before either.
var mdEscaper = strings.NewReplacer(
	`\`, `\\`,
	`*`, `\*`,
	`_`, `\_`,
	`<`, `\<`,
	`>`, `\>`,
	`[`, `\[`,
	`]`, `\]`,
	"`", "\\`",
)

// escapeMarkdown makes published text render as written. Without it the
// renderer drops a <placeholder> as an HTML tag and italicizes the middle of
// snake_case_words.
func escapeMarkdown(s string) string {
	return mdEscaper.Replace(s)
}

// notDeclaredErr maps a missing functions: entry onto a structured error. Any
// other read failure is already structured by internal/function.
func notDeclaredErr(alias string, err error) error {
	if !errors.Is(err, function.ErrNotPinned) {
		return err
	}
	return clierrors.New("function.not_pinned", "Function not declared",
		fmt.Sprintf("Alias %q has no entry in the functions block of the config.", alias)).
		WithSuggestions(
			"See what is declared: circleci function list --pinned",
			"Declare it first: circleci function add <name> --as "+alias,
		).
		WithExitCode(clierrors.ExitNotFound)
}

// malformedPinErr reports an entry that is not a "<path>@<version>" reference,
// which cannot be re-pinned because the function it names is unknown.
func malformedPinErr(alias, value string) error {
	return clierrors.New("function.malformed_pin", "Function declaration is malformed",
		fmt.Sprintf("Alias %q is declared as %q, which is not a <path>@<version> reference.", alias, value)).
		WithSuggestions(
			"Edit the entry to host.tld/org/name@version, as in "+alias+": "+function.DefaultOrg+"/<name>@<version>",
			"Or delete the entry, then run: circleci function add <name> --as "+alias,
		).
		WithExitCode(clierrors.ExitValidationFail)
}
