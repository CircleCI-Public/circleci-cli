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

package extension

import (
	"regexp"
	"strings"
)

// maxSubcommandDepth bounds how many levels SubcommandPath reports.
const maxSubcommandDepth = 3

// validSubcommandName is the shape a declared name must have to be reported.
// The reference is read back from the manifest on the user's disk, so a name is
// checked rather than trusted.
var validSubcommandName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// SubcommandPath returns the declared subcommands that args start with, joined
// by spaces — "setup guide" for args "setup guide --json". It stops at the
// first arg that is not a subcommand the reference declares at that level, so a
// flag, a value or anything undeclared is never part of the result, and returns
// "" when the first arg is not one.
func (r *Reference) SubcommandPath(args []string) string {
	if r == nil {
		return ""
	}

	var path []string
	subs := r.Subcommands
	for _, arg := range args {
		if len(path) == maxSubcommandDepth {
			break
		}
		sub, ok := findSubcommand(subs, arg)
		if !ok {
			break
		}
		path = append(path, sub.Name)
		subs = sub.Subcommands
	}
	return strings.Join(path, " ")
}

func findSubcommand(subs []ReferenceSub, name string) (ReferenceSub, bool) {
	if !validSubcommandName.MatchString(name) {
		return ReferenceSub{}, false
	}
	for _, sub := range subs {
		if sub.Name == name {
			return sub, true
		}
	}
	return ReferenceSub{}, false
}
