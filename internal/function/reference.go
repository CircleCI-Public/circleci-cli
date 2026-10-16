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

// Package function resolves published CircleCI functions.
package function

import (
	"fmt"
	"regexp"
	"strings"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
)

// DefaultOrg is the path CircleCI publishes its own functions under. A bare
// function name is resolved against it.
const DefaultOrg = "github.com/circleci-functions"

// These mirror what config processing enforces, which is the source of truth;
// checking locally only buys a faster, clearer error. Both are anchored,
// because a partial match is not a valid reference.
var (
	pathPattern    = regexp.MustCompile(`\A[a-z0-9][a-z0-9.-]*\.[a-z]{2,}(/[A-Za-z0-9._-]+){2,}\z`)
	versionPattern = regexp.MustCompile(`\Av\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?\z`)
	// pseudoPattern is the Go module pseudo-version form: a version with a
	// commit timestamp and hash, such as v0.5.2-0.20261007152818-72ffa3f7a023.
	pseudoPattern = regexp.MustCompile(`\Av\d+\.(?:0\.0-|\d+\.\d+-(?:[^+]*\.)?0\.)\d{14}-[A-Za-z0-9]+\z`)
	// commitPattern is the old publish flow's form, which suffixed a version
	// with its short commit, such as v0.5.4-2ad832a.
	commitPattern = regexp.MustCompile(`-[0-9a-f]{7}\z`)
)

// IsBuild reports whether version is a build rather than a release: a Go
// pseudo-version, or a version with a short commit suffix. The API leaves
// builds out of a function's versions unless asked.
func IsBuild(version string) bool {
	return pseudoPattern.MatchString(version) || commitPattern.MatchString(version)
}

// ValidatePath reports whether path is a function path the config accepts.
func ValidatePath(path string) error {
	if pathPattern.MatchString(path) {
		return nil
	}
	return invalidReference("Function path %q is not valid. Expected host.tld/org/name.", path)
}

// ValidateVersion reports whether version is a version the config accepts.
func ValidateVersion(version string) error {
	if versionPattern.MatchString(version) {
		return nil
	}
	return invalidReference("Function version %q is not valid. Expected a semver tag led by 'v'.", version)
}

// ExpandName resolves a user-supplied function name to the identifier the API
// lists it under. A bare name expands against DefaultOrg; anything carrying a
// path is validated and used verbatim.
func ExpandName(arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", invalidReference("A function name is required.")
	}
	if !strings.Contains(arg, "/") {
		return DefaultOrg + "/" + arg, nil
	}
	if err := ValidatePath(arg); err != nil {
		return "", err
	}
	return arg, nil
}

func invalidReference(format string, a ...any) error {
	return clierrors.New("function.invalid_reference", "Invalid function reference",
		fmt.Sprintf(format, a...)).
		WithSuggestions(
			"Name a function as host.tld/org/name, for example "+DefaultOrg+"/setup-go",
			"Run 'circleci function list' to see published functions and their versions",
		).
		WithExitCode(clierrors.ExitBadArguments)
}
