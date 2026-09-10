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

package runnerconfig

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
)

// maxNameLength is the agent's own limit on runner.name.
const maxNameLength = 64

// namePattern mirrors the agent's runnerNameRegexp. A name that fails it makes
// the agent exit at startup, so a generated config must never contain one.
var namePattern = regexp.MustCompile(`^[a-zA-Z0-9.()_\- ]{1,64}$`)

var disallowedNameChars = regexp.MustCompile(`[^a-zA-Z0-9.()_\- ]`)

// DefaultName derives a runner name from the local hostname, the same source
// the deb and rpm postinstall scripts use. It returns "" when there is no
// hostname or nothing survives sanitising, leaving the caller to report that
// --name is needed.
func DefaultName() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return SanitizeName(host)
}

// SanitizeName coerces s into something the agent accepts: disallowed
// characters become "-", the result is truncated to the agent's 64-character
// limit, and leading and trailing punctuation is dropped. It returns "" if
// nothing usable is left.
func SanitizeName(s string) string {
	s = disallowedNameChars.ReplaceAllString(s, "-")
	if len(s) > maxNameLength {
		s = s[:maxNameLength]
	}
	return strings.Trim(s, " -.")
}

// ValidateName reports whether name is one the agent will start on. It rejects
// the same values the agent's own validation does, so the failure surfaces here
// rather than on the runner host.
func ValidateName(name string) *clierrors.CLIError {
	invalid := func(detail string, suggestions ...string) *clierrors.CLIError {
		return clierrors.New("runner.invalid_name", "Invalid runner name", detail).
			WithSuggestions(suggestions...).
			WithExitCode(clierrors.ExitBadArguments)
	}

	switch {
	case name == "":
		return invalid("The runner name is empty, which the agent rejects at startup.",
			"Pass a name with --name <name>")
	case strings.TrimSpace(name) == "<< AGENT_NAME >>", strings.EqualFold(name, "runner_name"):
		return invalid(fmt.Sprintf("%q is a placeholder the agent rejects.", name),
			"Pass a real name with --name <name>")
	case !namePattern.MatchString(name):
		return invalid(fmt.Sprintf("%q is not a valid runner name.", name),
			"Use up to 64 letters, numbers, spaces, or the characters .()_-")
	}
	return nil
}
