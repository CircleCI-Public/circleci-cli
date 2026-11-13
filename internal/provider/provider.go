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

// Package provider declares every provider value the CLI sends the API in a
// config, checkout or trigger source, and, for the integrations onboarding can
// connect, how to recognise one from a git remote.
//
// Adding a provider means adding one entry to the registry below. The provider
// table in the triggers help topic is prose and is kept up to date by hand.
package provider

import "strings"

// Role is a source field a provider may be sent in. A provider can hold several.
type Role uint8

const (
	// ConfigSource providers can supply a pipeline definition's config.
	ConfigSource Role = 1 << iota
	// CheckoutSource providers can supply the code a pipeline checks out.
	CheckoutSource
	// TriggerSource providers can be the event source of a trigger.
	TriggerSource
)

// RepoKey is how the API identifies a provider's repository.
type RepoKey uint8

const (
	// NoRepo providers have no repository.
	NoRepo RepoKey = iota
	// RepoID providers identify a repository by its external id alone.
	RepoID
	// RepoIDAndFullName providers need the owner/name as well as the external id.
	// Origin exposes no lookup by id, so the full name is what resolves the
	// repository at all; the API rejects a write without it and rejects a pair that
	// names two different repositories.
	RepoIDAndFullName
)

// Provider describes a value the API accepts in a source's provider field.
type Provider struct {
	// Name is the value the API uses in provider fields and filters.
	Name string
	// Roles are the source fields Name may be sent in.
	Roles Role
	// Repo is how the API identifies a repository of this provider.
	Repo RepoKey
	// RepoIDExample is a sample external repository id in the provider's shape,
	// shown as a prompt placeholder. Set for every provider with a repository.
	RepoIDExample string

	// The fields below are set only for integrations onboarding can connect.

	// Short names the integration in progress output, e.g. "GitHub App" in
	// "Waiting for GitHub App installation".
	Short string
	// Install is the subject of the install prompt, e.g. "the CircleCI GitHub
	// App" in "Install the CircleCI GitHub App to connect your repository".
	Install string
	// Hosts are the git remote hosts whose repositories belong to this
	// integration. A host matches when it equals one of these or is a subdomain
	// of it, so enterprise and regional hosts resolve without new entries. A
	// provider with no hosts is never resolved from a remote.
	Hosts []string
	// InstalledPath is the app page an install returns the browser to, relative to
	// the app URL. It confirms the install and points the user back at their
	// terminal, so it names the integration the user just installed.
	InstalledPath string
}

// Has reports whether p may be sent in the source field r.
func (p Provider) Has(r Role) bool { return p.Roles&r != 0 }

// NeedsRepoID reports whether a source of p must name a repository external id.
func (p Provider) NeedsRepoID() bool { return p.Repo == RepoID || p.Repo == RepoIDAndFullName }

// NeedsFullName reports whether a source of p must name the repository's
// owner/name as well as its id.
func (p Provider) NeedsFullName() bool { return p.Repo == RepoIDAndFullName }

// registry is every provider the CLI can send. Order is significant: it is the
// order providers are listed in help text, errors and pickers, and the first
// host match wins in ForHost.
var registry = []Provider{
	{
		Name:          "github_app",
		Roles:         ConfigSource | CheckoutSource | TriggerSource,
		Repo:          RepoID,
		RepoIDExample: "e.g. 123456789",
		Short:         "GitHub App",
		Install:       "the CircleCI GitHub App",
		Hosts:         []string{"github.com"},
		InstalledPath: "cli/github-app-installed",
	},
	{
		Name:          "github_server",
		Roles:         ConfigSource | CheckoutSource | TriggerSource,
		Repo:          RepoID,
		RepoIDExample: "e.g. 123456789",
	},
	{
		Name:          "github_oauth",
		Roles:         TriggerSource,
		Repo:          RepoID,
		RepoIDExample: "e.g. 123456789",
	},
	{
		// A config hosted by CircleCI rather than in a repository.
		Name:  "circleci",
		Roles: ConfigSource,
		Repo:  NoRepo,
	},
	{
		Name:          "origin",
		Roles:         ConfigSource | CheckoutSource | TriggerSource,
		Repo:          RepoIDAndFullName,
		RepoIDExample: "e.g. repo_01abc",
		Short:         "Origin",
		Install:       "the CircleCI Origin app",
		Hosts:         []string{"origin.cursor.com"},
		InstalledPath: "cli/origin-installed",
	},
	{
		Name:  "webhook",
		Roles: TriggerSource,
		Repo:  NoRepo,
	},
	{
		Name:  "schedule",
		Roles: TriggerSource,
		Repo:  NoRepo,
	},
}

// Lookup returns the provider named name that may be sent in the source field
// r, and whether there is one.
func Lookup(name string, r Role) (Provider, bool) {
	for _, p := range registry {
		if p.Name == name && p.Has(r) {
			return p, true
		}
	}
	return Provider{}, false
}

// Names returns, in registry order, the names of the providers that may be sent
// in the source field r and satisfy every filter.
func Names(r Role, filters ...func(Provider) bool) []string {
	var names []string
next:
	for _, p := range registry {
		if !p.Has(r) {
			continue
		}
		for _, f := range filters {
			if !f(p) {
				continue next
			}
		}
		names = append(names, p.Name)
	}
	return names
}

// List is Names joined for help text and error suggestions, e.g.
// "github_app, github_server, origin".
func List(r Role, filters ...func(Provider) bool) string {
	return strings.Join(Names(r, filters...), ", ")
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
