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
			assert.Check(t, p.Short != "", "Short names the integration in progress output")
			assert.Check(t, p.Install != "", "Install is the subject of the install prompt")
			assert.Check(t, len(p.Hosts) > 0, "an integration with no hosts can never be resolved from a remote")
		})
	}
}
