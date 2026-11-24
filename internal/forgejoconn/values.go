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
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// ProviderName is Forgejo's name in the provider registry.
const ProviderName = "forgejo"

// MaxRequestHeaders is the most request headers a connection may carry.
const MaxRequestHeaders = 10

// setupField is a setup value the user supplies and the API may reject.
type setupField string

// The fields, named as the setup request names them under vcs.
const (
	fieldURL                setupField = "url"
	fieldOwner              setupField = "owner"
	fieldConnectionAudience setupField = "connection_audience"
	fieldCheckoutAudience   setupField = "checkout_audience"
	fieldRequestHeaders     setupField = "request_headers"
)

// flag is the onboard flag that supplies f.
func (f setupField) flag() string {
	switch f {
	case fieldURL:
		return "--provider-url"
	case fieldOwner:
		return "--owner"
	case fieldConnectionAudience:
		return "--connection-audience"
	case fieldCheckoutAudience:
		return "--checkout-audience"
	case fieldRequestHeaders:
		return "--header"
	default:
		return ""
	}
}

// FindConnection returns the organization's Forgejo connection, if it has one.
func FindConnection(conns []apiclient.ProviderConnection) (apiclient.ProviderConnection, bool) {
	for _, c := range conns {
		if c.Provider == ProviderName {
			return c, true
		}
	}
	return apiclient.ProviderConnection{}, false
}

// ServesHost reports whether a connection to the instance at providerURL is the
// one a git remote on host belongs to. Ports are ignored: an ssh remote's port
// says nothing about the port the instance serves HTTP on.
func ServesHost(providerURL, host string) bool {
	u, err := url.Parse(providerURL)
	if err != nil || u.Hostname() == "" {
		return false
	}
	h := hostname(host)
	return h != "" && strings.EqualFold(u.Hostname(), h)
}

// defaultURL is the instance URL a remote on host suggests. A remote carries no
// scheme the instance can be assumed to serve, so it is always https, and no
// port, for the reason given in ServesHost.
func defaultURL(host string) string {
	h := hostname(host)
	if h == "" {
		return ""
	}
	return "https://" + h
}

// hostname strips the user and port a remote's host may carry, e.g.
// "git@forgejo.example.com:2222" is forgejo.example.com.
func hostname(host string) string {
	host = strings.TrimSpace(host)
	if i := strings.LastIndexByte(host, '@'); i >= 0 {
		host = host[i+1:]
	}
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host
}

// ParseHeaders parses -H/--header values, each "Name: Value". Names and values
// are trimmed, a header needs both, and a name is given once.
func ParseHeaders(raw []string) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > MaxRequestHeaders {
		return nil, clierrors.New("onboard.too_many_headers", "Too many request headers",
			fmt.Sprintf("A Forgejo connection takes at most %d request headers (got: %d).", MaxRequestHeaders, len(raw))).
			WithExitCode(clierrors.ExitBadArguments)
	}
	headers := make(map[string]string, len(raw))
	for _, h := range raw {
		name, value, ok := strings.Cut(h, ":")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !ok || name == "" || value == "" {
			return nil, clierrors.New("onboard.invalid_header", "Invalid header format",
				fmt.Sprintf("%q is not in \"Name: Value\" format.", h)).
				WithSuggestions(`Pass each header as --header "Name: Value"`).
				WithExitCode(clierrors.ExitBadArguments)
		}
		if _, dup := headers[name]; dup {
			return nil, clierrors.New("onboard.duplicate_header", "Duplicate request header",
				fmt.Sprintf("The %s header is given more than once.", name)).
				WithSuggestions("Pass each header once").
				WithExitCode(clierrors.ExitBadArguments)
		}
		headers[name] = value
	}
	return headers, nil
}

// detailPatterns name the field a setup error is about when it has no source
// pointer: the API names the field only in its message.
var detailPatterns = []struct {
	re    *regexp.Regexp
	field setupField
}{
	{regexp.MustCompile(`(?i)vcs\.url|url is not a forgejo|forgejo \d+ or later`), fieldURL},
	{regexp.MustCompile(`(?i)vcs\.owner|^owner `), fieldOwner},
	{regexp.MustCompile(`(?i)request_headers`), fieldRequestHeaders},
	{regexp.MustCompile(`(?i)connection_audience|connection token|connection integration`), fieldConnectionAudience},
	{regexp.MustCompile(`(?i)checkout_audience|checkout token|checkout integration`), fieldCheckoutAudience},
}

var pointerField = regexp.MustCompile(`^/vcs/([^/]+)`)

// fieldForError returns the field a rejected setup names, and the API's
// explanation. Every mistake the user can fix is a 400; any other error, or a
// 400 naming no field, returns "".
func fieldForError(err error) (setupField, string) {
	if !httpcl.HasStatusCode(err, http.StatusBadRequest) {
		return "", ""
	}
	apiErr, ok := apiclient.ParseError(err)
	if !ok {
		return "", ""
	}
	if m := pointerField.FindStringSubmatch(apiErr.Source.Pointer); m != nil {
		switch f := setupField(m[1]); f {
		case fieldURL, fieldOwner, fieldConnectionAudience, fieldCheckoutAudience, fieldRequestHeaders:
			return f, apiErr.Detail
		}
	}
	for _, p := range detailPatterns {
		if p.re.MatchString(apiErr.Detail) {
			return p.field, apiErr.Detail
		}
	}
	return "", apiErr.Detail
}

// setupError explains a setup the API refused, naming what to change.
func setupError(err error, owner string) error {
	field, detail := fieldForError(err)
	switch {
	case field != "":
		return clierrors.New("onboard.forgejo_setup_rejected", "Forgejo connection rejected",
			fmt.Sprintf("CircleCI could not connect to Forgejo: %s", sentence(detail))).
			WithSuggestions(fmt.Sprintf("Fix the value and re-run with %s <value>", field.flag())).
			WithExitCode(clierrors.ExitAPIError)
	case detail != "":
		return clierrors.New("onboard.forgejo_setup_rejected", "Forgejo connection rejected",
			fmt.Sprintf("CircleCI could not connect to Forgejo: %s", sentence(detail))).
			WithExitCode(clierrors.ExitAPIError)
	case httpcl.HasStatusCode(err, http.StatusForbidden):
		return clierrors.New("onboard.forgejo_forbidden", "Cannot connect Forgejo",
			"Connecting Forgejo needs permission to manage this CircleCI organization.").
			WithSuggestions("Ask an organization admin to run circleci onboard, or to grant you the admin role").
			WithExitCode(clierrors.ExitAPIError)
	case httpcl.HasStatusCode(err, http.StatusConflict):
		return clierrors.New("onboard.forgejo_conflict", "Forgejo already connected",
			fmt.Sprintf("This organization already has a Forgejo connection, or %s is connected to another CircleCI organization.", owner)).
			WithSuggestions(
				"Run circleci onboard --org <slug> to set the project up in the organization that has it",
				"Or remove the other connection in CircleCI, then re-run",
			).
			WithExitCode(clierrors.ExitAPIError)
	default:
		return clierrors.New("onboard.forgejo_setup_failed", "Could not connect Forgejo",
			fmt.Sprintf("Connecting Forgejo failed: %s.", err)).
			WithExitCode(clierrors.ExitAPIError)
	}
}

// sentence ends s with a period unless it already ends a sentence.
func sentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasSuffix(s, ".") || strings.HasSuffix(s, "!") || strings.HasSuffix(s, "?") {
		return s
	}
	return s + "."
}
