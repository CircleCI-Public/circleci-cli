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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder/chirecorder"
)

func TestClient_ResolveOrgID(t *testing.T) {
	ctx := iostream.Testing(context.Background())
	orgID := "f22b6566-597d-46d5-ba74-99ef5bb3d85c"

	rec := httprecorder.New()
	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Get("/api/v3/orgs", func(w http.ResponseWriter, r *http.Request) {
		data := []map[string]any{}
		if r.URL.Query().Get("filter[slug]") == "gh/acme" {
			data = append(data, map[string]any{"id": orgID})
		}
		render.JSON(w, r, map[string]any{
			"data": data,
			"page": map[string]any{"next": nil, "prev": nil},
		})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	t.Run("resolves a slug to its UUID", func(t *testing.T) {
		id, err := c.ResolveOrgID(ctx, "gh/acme")
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(id, uuid.MustParse(orgID)))

		got := rec.LastRequest()
		assert.Check(t, cmp.Equal(got.URL.Path, "/api/v3/orgs"))
		assert.Check(t, cmp.Equal(got.URL.Query().Get("filter[slug]"), "gh/acme"))
	})

	t.Run("empty result is ErrOrgNotFound", func(t *testing.T) {
		_, err := c.ResolveOrgID(ctx, "gh/nope")
		assert.ErrorIs(t, err, apiclient.ErrOrgNotFound)
	})
}
