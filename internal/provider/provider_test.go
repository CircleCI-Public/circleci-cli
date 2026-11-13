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

package provider

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestForHost(t *testing.T) {
	cases := []struct {
		name string
		host string
		want string // provider name, or "" when no integration claims the host
	}{
		{name: "github", host: "github.com", want: "github_app"},
		{name: "case is ignored", host: "GitHub.com", want: "github_app"},
		{name: "port is ignored", host: "github.com:443", want: "github_app"},
		// A subdomain belongs to the same integration, so regional and enterprise
		// hosts resolve without their own registry entry.
		{name: "subdomain", host: "eu.github.com", want: "github_app"},
		{name: "subdomain of another integration", host: "eu.origin.cursor.com", want: "origin"},
		{name: "unclaimed host", host: "gitlab.com", want: ""},
		// A host that merely ends with the same letters must not match: matching on
		// a bare suffix would hand notgithub.com to the GitHub App.
		{name: "lookalike host", host: "notgithub.com", want: ""},
		{name: "empty", host: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := ForHost(tc.host)
			if tc.want == "" {
				assert.Check(t, !ok, "expected no integration for %q, got %q", tc.host, p.Name)
				return
			}
			assert.Check(t, ok, "expected an integration for %q", tc.host)
			assert.Check(t, cmp.Equal(p.Name, tc.want))
		})
	}
}

// TestRegistryIsUsable pins the fields the rest of the CLI reads off a registry
// entry. An entry missing one of them fails at runtime as empty output or a
// rejected request, so it is worth catching here.
func TestRegistryIsUsable(t *testing.T) {
	for _, p := range registry {
		t.Run(p.Name, func(t *testing.T) {
			assert.Check(t, p.Name != "", "Name is the value the API is sent")
			assert.Check(t, p.Roles != 0, "a provider with no role can never be sent")
			if p.Repo != NoRepo {
				assert.Check(t, p.RepoIDExample != "", "RepoIDExample is the repo id prompt placeholder")
			}
			onboarding := p.Short != "" || p.Install != "" || p.InstalledPath != ""
			if len(p.Hosts) == 0 {
				// Without hosts onboarding never offers the entry, so its onboarding
				// fields would be dead.
				assert.Check(t, !onboarding, "onboarding fields are set but there are no Hosts to resolve the integration from")
				return
			}
			// An onboarding integration: the connection flow reads these.
			assert.Check(t, p.Short != "", "Short names the integration in progress output")
			assert.Check(t, p.Install != "", "Install is the subject of the install prompt")
			// A missing page sends the browser to the app root after an install, which
			// reads as a finish line while the terminal is still waiting.
			assert.Check(t, p.InstalledPath != "", "InstalledPath is where an install returns the browser")
		})
	}
}

// TestNames pins the values each source field accepts, in the order help text,
// errors and pickers list them.
func TestNames(t *testing.T) {
	cases := []struct {
		name    string
		role    Role
		filters []func(Provider) bool
		want    []string
	}{
		{
			name: "config",
			role: ConfigSource,
			want: []string{"github_app", "github_server", "circleci", "origin", "forgejo"},
		},
		{
			name:    "config with repo id",
			role:    ConfigSource,
			filters: []func(Provider) bool{Provider.NeedsRepoID},
			want:    []string{"github_app", "github_server", "origin"},
		},
		{
			name:    "config with full name",
			role:    ConfigSource,
			filters: []func(Provider) bool{Provider.NeedsFullName},
			want:    []string{"origin"},
		},
		{
			name: "checkout",
			role: CheckoutSource,
			want: []string{"github_app", "github_server", "origin", "forgejo"},
		},
		{
			name:    "checkout with full name",
			role:    CheckoutSource,
			filters: []func(Provider) bool{Provider.NeedsFullName},
			want:    []string{"origin"},
		},
		{
			name: "trigger",
			role: TriggerSource,
			want: []string{"github_app", "github_server", "github_oauth", "origin", "forgejo", "webhook", "schedule"},
		},
		{
			name:    "trigger with repo id",
			role:    TriggerSource,
			filters: []func(Provider) bool{Provider.NeedsRepoID},
			want:    []string{"github_app", "github_server", "github_oauth", "origin"},
		},
		{
			name:    "trigger with full name",
			role:    TriggerSource,
			filters: []func(Provider) bool{Provider.NeedsFullName},
			want:    []string{"origin"},
		},
		{
			name:    "trigger by full name alone",
			role:    TriggerSource,
			filters: []func(Provider) bool{Provider.FullNameIsEnough},
			want:    []string{"forgejo"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.DeepEqual(Names(tc.role, tc.filters...), tc.want))
		})
	}
}

func TestLookup(t *testing.T) {
	t.Run("found in role", func(t *testing.T) {
		p, ok := Lookup("origin", CheckoutSource)
		assert.Check(t, cmp.Equal(ok, true))
		assert.Check(t, cmp.Equal(p.Name, "origin"))
		assert.Check(t, cmp.Equal(p.NeedsRepoID(), true))
		assert.Check(t, cmp.Equal(p.NeedsFullName(), true))
	})
	// Forgejo resolves a repository from either its id or its owner/name.
	t.Run("id or full name", func(t *testing.T) {
		p, ok := Lookup("forgejo", TriggerSource)
		assert.Check(t, cmp.Equal(ok, true))
		assert.Check(t, cmp.Equal(p.NeedsRepoID(), false))
		assert.Check(t, cmp.Equal(p.NeedsFullName(), false))
		assert.Check(t, cmp.Equal(p.HasRepo(), true))
		assert.Check(t, cmp.Equal(p.FullNameIsEnough(), true))
	})
	// circleci is a hosted config: valid as a config source, never as a checkout.
	t.Run("known but not in role", func(t *testing.T) {
		_, ok := Lookup("circleci", CheckoutSource)
		assert.Check(t, cmp.Equal(ok, false))
	})
	t.Run("unknown", func(t *testing.T) {
		_, ok := Lookup("gitlab", ConfigSource)
		assert.Check(t, cmp.Equal(ok, false))
	})
}
