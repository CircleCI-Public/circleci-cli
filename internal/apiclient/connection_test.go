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
package apiclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder/chirecorder"
)

const connTestOrgID = "f22b6566-597d-46d5-ba74-99ef5bb3d85c"

func TestClient_ListProviderConnections_ProviderURL(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	r := chi.NewMux()
	r.Get("/api/v3/provider/connections", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]any{
			"data": []any{map[string]any{
				"id": "conn-1",
				"attributes": map[string]any{
					"provider":     "forgejo",
					"provider_url": "https://forgejo.example.com",
					"external_id":  "7",
				},
			}},
			"page": map[string]any{"next": nil},
		})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})
	conns, err := c.ListProviderConnections(ctx, connTestOrgID)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(conns, 1))
	assert.Check(t, cmp.Equal(conns[0].Provider, "forgejo"))
	assert.Check(t, cmp.Equal(conns[0].ProviderURL, "https://forgejo.example.com"))
}

func TestClient_SetupForgejoConnection(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	var status int
	var respBody any
	rec := httprecorder.New()
	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Post("/api/v3/provider/connections/setup", func(w http.ResponseWriter, r *http.Request) {
		render.Status(r, status)
		render.JSON(w, r, respBody)
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})
	in := apiclient.ForgejoSetup{
		URL:                "https://forgejo.example.com",
		Owner:              "acme",
		ConnectionAudience: "u:1:conn",
		CheckoutAudience:   "u:1:checkout",
		RequestHeaders:     map[string]string{"X-Gateway": "secret"},
	}

	t.Run("sends the setup body and decodes the configure step", func(t *testing.T) {
		status = http.StatusCreated
		respBody = map[string]any{"data": map[string]any{
			"id": "conn-uuid",
			"attributes": map[string]any{
				"next_step": "configure",
				"owner":     map[string]any{"login": "acme", "id": "3", "type": "organization"},
				"webhook": map[string]any{
					"url":    "https://internal.circleci.com/private/soc/forgejo/webhook/conn-uuid",
					"secret": "s3cret",
					"events": []string{"push", "pull_request"},
				},
			},
		}}

		conn, err := c.SetupForgejoConnection(ctx, connTestOrgID, in)
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(conn, &apiclient.ForgejoConnection{
			ID:    "conn-uuid",
			Owner: apiclient.ConnectionOwner{Login: "acme", ID: "3", Type: "organization"},
			Webhook: apiclient.ConnectionWebhook{
				URL:    "https://internal.circleci.com/private/soc/forgejo/webhook/conn-uuid",
				Secret: "s3cret",
				Events: []string{"push", "pull_request"},
			},
		}))

		got := rec.LastRequest()
		assert.Check(t, cmp.Equal(got.URL.Query().Get("filter[org_id]"), connTestOrgID))
		assert.Assert(t, got.Body != nil)
		var body map[string]any
		err = json.Unmarshal([]byte(*got.Body), &body)
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(body, map[string]any{
			"type": "vcs",
			"vcs": map[string]any{
				"provider":            "forgejo",
				"url":                 "https://forgejo.example.com",
				"owner":               "acme",
				"connection_audience": "u:1:conn",
				"checkout_audience":   "u:1:checkout",
				"request_headers":     map[string]any{"X-Gateway": "secret"},
			},
		}))
	})

	t.Run("omits request headers when there are none", func(t *testing.T) {
		status = http.StatusCreated
		noHeaders := in
		noHeaders.RequestHeaders = nil

		_, err := c.SetupForgejoConnection(ctx, connTestOrgID, noHeaders)
		assert.NilError(t, err)
		got := rec.LastRequest()
		assert.Assert(t, got.Body != nil)
		assert.Check(t, !strings.Contains(*got.Body, "request_headers"), "body: %s", *got.Body)
	})

	t.Run("a 400 keeps the API's detail", func(t *testing.T) {
		status = http.StatusBadRequest
		respBody = map[string]any{"error": map[string]any{
			"title":  "Invalid Input",
			"detail": "vcs.checkout_audience is required.",
		}}

		_, err := c.SetupForgejoConnection(ctx, connTestOrgID, in)
		assert.Check(t, httpcl.HasStatusCode(err, http.StatusBadRequest))
		apiErr, ok := apiclient.ParseError(err)
		assert.Assert(t, ok)
		assert.Check(t, cmp.Equal(apiErr.Detail, "vcs.checkout_audience is required."))
	})

	t.Run("any step other than configure is an error", func(t *testing.T) {
		status = http.StatusOK
		respBody = map[string]any{"data": map[string]any{
			"attributes": map[string]any{"next_step": "redirect", "url": "https://example.com"},
		}}

		_, err := c.SetupForgejoConnection(ctx, connTestOrgID, in)
		assert.Check(t, cmp.ErrorContains(err, `"redirect"`))
	})
}
