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

package configaudit

import (
	"regexp"
	"strings"
)

// taintedExact are pipeline values a pusher, PR author or webhook sender controls.
var taintedExact = []string{
	"pipeline.git.branch", "pipeline.git.tag",
	"pipeline.git.commit.subject", "pipeline.git.commit.body",
	"pipeline.event.github.ref", "pipeline.event.github.comment.body",
	"pipeline.event.github.sender.login",
	"pipeline.trigger_parameters.webhook.body",
	"pipeline.trigger_parameters.github_app.branch",
	"pipeline.trigger_parameters.github_app.ref",
	"pipeline.trigger_parameters.github_app.commit_title",
	"pipeline.trigger_parameters.github_app.commit_message",
	"pipeline.trigger_parameters.github_app.commit_author_name",
	"pipeline.trigger_parameters.github_app.user_name",
	"pipeline.trigger_parameters.github_app.user_username",
}

// taintedPrefixes cover families whose members are all free text.
var taintedPrefixes = []string{
	"pipeline.git.commit.author_",
	"pipeline.event.github.pull_request.",
}

// valueRefRE matches a "<< pipeline.x >>" or "<< parameters.x >>" reference.
var valueRefRE = regexp.MustCompile(`<<\s*(pipeline\.[\w.]+|parameters\.[\w-]+)\s*>>`)

// isTainted reports whether name, such as "pipeline.git.branch", is a value an
// outsider controls.
func isTainted(name string) bool {
	for _, exact := range taintedExact {
		if name == exact {
			return true
		}
	}
	for _, prefix := range taintedPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// taintedRef returns the first tainted pipeline value referenced in text.
func taintedRef(text string) (string, bool) {
	for _, m := range valueRefRE.FindAllStringSubmatch(text, -1) {
		if isTainted(m[1]) {
			return m[1], true
		}
	}
	return "", false
}
