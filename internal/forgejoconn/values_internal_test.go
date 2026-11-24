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

package forgejoconn

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

func TestParseHeaders(t *testing.T) {
	tooMany := make([]string, MaxRequestHeaders+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("X-H%d: v", i)
	}
	atLimit := tooMany[:MaxRequestHeaders]

	cases := []struct {
		name    string
		raw     []string
		want    map[string]string
		wantErr string
	}{
		{
			name: "none",
			raw:  nil,
			want: nil,
		},
		{
			name: "trims names and values",
			raw:  []string{"  X-Gateway :  token value "},
			want: map[string]string{"X-Gateway": "token value"},
		},
		{
			name: "a value may contain a colon",
			raw:  []string{"Authorization: Basic a:b"},
			want: map[string]string{"Authorization": "Basic a:b"},
		},
		{
			name:    "no colon",
			raw:     []string{"X-Gateway"},
			wantErr: `"X-Gateway" is not in "Name: Value" format`,
		},
		{
			name:    "no value",
			raw:     []string{"X-Gateway:  "},
			wantErr: `is not in "Name: Value" format`,
		},
		{
			name:    "no name",
			raw:     []string{": value"},
			wantErr: `is not in "Name: Value" format`,
		},
		{
			name:    "a name given twice",
			raw:     []string{"X-Gateway: a", "X-Gateway: b"},
			wantErr: "X-Gateway header is given more than once",
		},
		{
			name:    "over the limit",
			raw:     tooMany,
			wantErr: "at most 10 request headers (got: 11)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseHeaders(tc.raw)
			if tc.wantErr != "" {
				assert.Check(t, cmp.ErrorContains(err, tc.wantErr))
				return
			}
			assert.NilError(t, err)
			assert.Check(t, cmp.DeepEqual(got, tc.want))
		})
	}

	t.Run("the limit itself is allowed", func(t *testing.T) {
		got, err := ParseHeaders(atLimit)
		assert.NilError(t, err)
		assert.Check(t, cmp.Len(got, MaxRequestHeaders))
	})
}

func TestDefaultURL(t *testing.T) {
	cases := []struct {
		name string
		host string
		want string
	}{
		{
			name: "https remote",
			host: "forgejo.example.com",
			want: "https://forgejo.example.com",
		},
		{
			name: "ssh remote with a port",
			host: "forgejo.example.com:2222",
			want: "https://forgejo.example.com",
		},
		{
			name: "remote with a user",
			host: "joel@forgejo.example.com",
			want: "https://forgejo.example.com",
		},
		{
			name: "no host",
			host: "",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(defaultURL(tc.host), tc.want))
		})
	}
}

func TestServesHost(t *testing.T) {
	cases := []struct {
		name        string
		providerURL string
		host        string
		want        bool
	}{
		{
			name:        "same host",
			providerURL: "https://forgejo.example.com",
			host:        "forgejo.example.com",
			want:        true,
		},
		{
			name:        "case and trailing slash differ",
			providerURL: "https://Forgejo.Example.com/",
			host:        "forgejo.example.COM",
			want:        true,
		},
		{
			name:        "ports are ignored",
			providerURL: "https://forgejo.example.com:3000",
			host:        "forgejo.example.com:2222",
			want:        true,
		},
		{
			name:        "another instance",
			providerURL: "https://code.example.org",
			host:        "forgejo.example.com",
			want:        false,
		},
		{
			name:        "a subdomain is another instance",
			providerURL: "https://example.com",
			host:        "forgejo.example.com",
			want:        false,
		},
		{
			name:        "no provider url",
			providerURL: "",
			host:        "forgejo.example.com",
			want:        false,
		},
		{
			name:        "no host",
			providerURL: "https://forgejo.example.com",
			host:        "",
			want:        false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(ServesHost(tc.providerURL, tc.host), tc.want))
		})
	}
}

func TestFindConnection(t *testing.T) {
	t.Run("finds the forgejo connection", func(t *testing.T) {
		conn, ok := FindConnection([]apiclient.ProviderConnection{
			{ID: "1", Provider: "github_app"},
			{ID: "2", Provider: "forgejo", ProviderURL: "https://forgejo.example.com"},
		})
		assert.Check(t, ok)
		assert.Check(t, cmp.Equal(conn.ID, "2"))
	})

	t.Run("none", func(t *testing.T) {
		_, ok := FindConnection([]apiclient.ProviderConnection{{ID: "1", Provider: "github_app"}})
		assert.Check(t, !ok)
	})
}

func TestFieldForError(t *testing.T) {
	apiErr := func(status int, body string) error {
		return &httpcl.HTTPError{Method: http.MethodPost, Route: "/setup", StatusCode: status, Body: []byte(body)}
	}
	detail := func(d string) string {
		return fmt.Sprintf(`{"error":{"title":"Invalid Input","detail":%q}}`, d)
	}

	cases := []struct {
		name       string
		err        error
		wantField  setupField
		wantDetail string
	}{
		{
			name:       "source pointer wins",
			err:        apiErr(http.StatusBadRequest, `{"error":{"detail":"rejected","source":{"pointer":"/vcs/checkout_audience"}}}`),
			wantField:  fieldCheckoutAudience,
			wantDetail: "rejected",
		},
		{
			name:       "required field",
			err:        apiErr(http.StatusBadRequest, detail("vcs.owner is required.")),
			wantField:  fieldOwner,
			wantDetail: "vcs.owner is required.",
		},
		{
			name:       "old forgejo",
			err:        apiErr(http.StatusBadRequest, detail("Needs Forgejo 16 or later (found 15.0.1)")),
			wantField:  fieldURL,
			wantDetail: "Needs Forgejo 16 or later (found 15.0.1)",
		},
		{
			name:       "owner missing on the instance",
			err:        apiErr(http.StatusBadRequest, detail("owner acme does not exist on this instance")),
			wantField:  fieldOwner,
			wantDetail: "owner acme does not exist on this instance",
		},
		{
			name: "rejected checkout token",
			err: apiErr(http.StatusBadRequest, detail(
				"Forgejo rejected the checkout token: check that the integration's issuer is https://oidc.circleci.com/org/x, "+
					"that checkout_audience is its generated audience")),
			wantField: fieldCheckoutAudience,
			wantDetail: "Forgejo rejected the checkout token: check that the integration's issuer is https://oidc.circleci.com/org/x, " +
				"that checkout_audience is its generated audience",
		},
		{
			name:       "connection scope",
			err:        apiErr(http.StatusBadRequest, detail("the connection integration lacks scope read:organization")),
			wantField:  fieldConnectionAudience,
			wantDetail: "the connection integration lacks scope read:organization",
		},
		{
			name:       "request headers",
			err:        apiErr(http.StatusBadRequest, detail("vcs.request_headers: too many headers")),
			wantField:  fieldRequestHeaders,
			wantDetail: "vcs.request_headers: too many headers",
		},
		{
			name:       "a 400 naming no field",
			err:        apiErr(http.StatusBadRequest, detail("something else")),
			wantField:  "",
			wantDetail: "something else",
		},
		{
			name:       "not a 400",
			err:        apiErr(http.StatusConflict, detail("vcs.owner is already connected")),
			wantField:  "",
			wantDetail: "",
		},
		{
			name:       "not an API error",
			err:        fmt.Errorf("network down"),
			wantField:  "",
			wantDetail: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			field, d := fieldForError(tc.err)
			assert.Check(t, cmp.Equal(field, tc.wantField))
			assert.Check(t, cmp.Equal(d, tc.wantDetail))
		})
	}
}

func TestSetupError(t *testing.T) {
	apiErr := func(status int, d string) error {
		return &httpcl.HTTPError{StatusCode: status, Body: fmt.Appendf(nil, `{"error":{"title":"T","detail":%q}}`, d)}
	}

	cases := []struct {
		name        string
		err         error
		wantMessage string
		wantSuggest string
	}{
		{
			name:        "a field names its flag",
			err:         apiErr(http.StatusBadRequest, "vcs.checkout_audience is required."),
			wantMessage: "CircleCI could not connect to Forgejo: vcs.checkout_audience is required.",
			wantSuggest: "--checkout-audience",
		},
		{
			name:        "forbidden",
			err:         &httpcl.HTTPError{StatusCode: http.StatusForbidden},
			wantMessage: "needs permission to manage this CircleCI organization",
			wantSuggest: "organization admin",
		},
		{
			name:        "conflict",
			err:         &httpcl.HTTPError{StatusCode: http.StatusConflict},
			wantMessage: "acme is connected to another CircleCI organization",
			wantSuggest: "--org",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := setupError(tc.err, "acme")
			assert.Check(t, cmp.ErrorContains(got, tc.wantMessage))
			assert.Check(t, cmp.Contains(strings.Join(suggestions(t, got), "\n"), tc.wantSuggest))
		})
	}
}

func suggestions(t *testing.T, err error) []string {
	t.Helper()
	cliErr, ok := errors.AsType[*clierrors.CLIError](err)
	assert.Assert(t, ok, "not a CLIError: %v", err)
	return cliErr.Suggestions
}
