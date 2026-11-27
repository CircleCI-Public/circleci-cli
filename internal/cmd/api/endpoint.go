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

package api

import (
	"regexp"
	"strings"
)

// versionSegment matches the API version segment that follows "api/": v1,
// v1.1, v2, v3.
var versionSegment = regexp.MustCompile(`^v\d+(\.\d+)?$`)

// knownSegments are the literal words of CircleCI API paths. endpointTemplate
// keeps only these and replaces every other segment, so what it returns never
// holds a project, org, repo or branch name, an ID, or anything else the caller
// typed. A literal missing from this list is replaced like an ID: the template
// still shows the shape of the path, just not that word.
var knownSegments = map[string]bool{
	"approve": true, "artifacts": true, "branches": true, "cancel": true,
	"checkout-key": true, "collaborations": true, "compile-config": true,
	"compile-config-with-defaults": true, "components": true, "context": true,
	"contexts": true, "deploy": true, "deployments": true, "environment-variable": true,
	"environment-variables": true, "environments": true, "envvar": true,
	"flaky-tests": true, "insights": true, "job": true, "jobs": true, "me": true,
	"metrics": true, "organization": true, "organizations": true, "pipeline": true,
	"pipeline-definitions": true, "pipelines": true, "project": true, "projects": true,
	"restrictions": true, "rerun": true, "run": true, "runner": true, "runs": true,
	"schedule": true, "schedules": true, "search": true, "settings": true,
	"summary": true, "test-metrics": true, "tests": true, "timeseries": true,
	"token": true, "tokens": true, "trigger": true, "triggers": true, "user": true,
	"users": true, "versions": true, "webhook": true, "webhooks": true,
	"workflow": true, "workflows": true,
}

// vcsSegments start a project or org slug (gh/org/repo, bb/org).
var vcsSegments = map[string]bool{
	"gh": true, "github": true, "bb": true, "bitbucket": true, "circleci": true, "gitlab": true,
}

// endpointTemplate reduces a request path to its shape, for telemetry. IDs
// become {id} and a project or org slug becomes {slug}, so
// "api/v2/project/gh/org/repo/pipeline?branch=main" is "v2/project/{slug}/pipeline".
// An empty segment is "{empty}" and a literal null or undefined one is "{null}".
// A path with no "api/" prefix is a v3 path, as the command sends it.
func endpointTemplate(p string) string {
	p, _, _ = strings.Cut(p, "?")
	segments := strings.Split(strings.Trim(p, "/"), "/")

	version := "v3"
	if segments[0] == "api" {
		version = ""
		segments = segments[1:]
		if len(segments) > 0 && versionSegment.MatchString(segments[0]) {
			version, segments = segments[0], segments[1:]
		}
	}

	out := make([]string, 0, len(segments)+1)
	if version != "" {
		out = append(out, version)
	}
	for i := 0; i < len(segments); i++ {
		s := segments[i]
		switch {
		case s == "":
			// An ID that was empty when the path was built. Kept visible, since
			// it is a failure worth counting on its own.
			out = append(out, "{empty}")
		case s == "null" || s == "undefined":
			out = append(out, "{null}")
		case vcsSegments[s]:
			// The slug is the VCS token and up to two names after it.
			out = append(out, "{slug}")
			for n := 0; n < 2 && i+1 < len(segments) && !knownSegments[segments[i+1]]; n++ {
				i++
			}
		case knownSegments[s]:
			out = append(out, s)
		default:
			out = append(out, "{id}")
		}
	}
	return strings.Join(out, "/")
}
