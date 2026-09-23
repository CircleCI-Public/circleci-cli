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

	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
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
		WithSuggestions(
			"Run 'circleci function list' to see published functions",
			"Functions published under the legacy circleci/<name> prefix are not listed",
		).
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
// the row and a pipe would split it.
func tableCell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(s, "|", `\|`)
}

// defaultConfigPath is where a pipeline config lives unless --config says so.
const defaultConfigPath = ".circleci/config.yml"

// addConfigFlag adds the pipeline-config path flag. It shadows the root
// --config, which names the CLI's own settings file, as config validate does.
func addConfigFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("config", "c", defaultConfigPath, "path to the pipeline config file")
}

func configPath(cmd *cobra.Command) string {
	path, _ := cmd.Flags().GetString("config")
	if path == "" {
		return defaultConfigPath
	}
	return path
}
