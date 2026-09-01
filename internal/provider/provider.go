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

// Package provider declares the integrations a repository can be connected to
// CircleCI through, and how to recognise one from a git remote.
//
// Adding an integration means adding one entry to the registry below. Nothing
// else in the CLI names an integration: the connection flow, repository
// resolution, and the pipeline and trigger writes all take a Provider and read
// what they need from it.
package provider

import "strings"

// Provider describes an integration onboard can connect a repository through.
type Provider struct {
	// Name is the value the API uses in provider fields and filters.
	Name string
	// Short names the integration in progress output, e.g. "GitHub App" in
	// "Waiting for GitHub App installation".
	Short string
	// Install is the subject of the install prompt, e.g. "the CircleCI GitHub
	// App" in "Install the CircleCI GitHub App to connect your repository".
	Install string
	// Hosts are the git remote hosts whose repositories belong to this
	// integration. A host matches when it equals one of these or is a subdomain
	// of it, so enterprise and regional hosts resolve without new entries.
	Hosts []string
	// InstalledPath is the app page an install returns the browser to, relative to
	// the app URL. It confirms the install and points the user back at their
	// terminal, so it names the integration the user just installed.
	InstalledPath string
}

// registry is every integration the CLI can drive. Order is significant only in
// that the first host match wins.
var registry = []Provider{
	{
		Name:          "github_app",
		Short:         "GitHub App",
		Install:       "the CircleCI GitHub App",
		Hosts:         []string{"github.com"},
		InstalledPath: "cli/github-app-installed",
	},
	{
		Name:          "origin",
		Short:         "Origin",
		Install:       "the CircleCI Origin app",
		Hosts:         []string{"origin.cursor.com"},
		InstalledPath: "cli/origin-installed",
	},
}

// ForHost returns the integration that owns repositories on host, and whether
// one was found. Matching is case-insensitive and ignores a port.
func ForHost(host string) (Provider, bool) {
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return Provider{}, false
	}

	for _, p := range registry {
		for _, h := range p.Hosts {
			if host == h || strings.HasSuffix(host, "."+h) {
				return p, true
			}
		}
	}
	return Provider{}, false
}
