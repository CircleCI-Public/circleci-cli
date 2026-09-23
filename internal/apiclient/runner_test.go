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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// newRunnerFake builds a minimal fake that responds to the v3 resource-classes endpoints.
// The handler maps filter[slug] values to pre-registered items; unknown slugs return an
// empty collection. The update endpoint stores changes in-memory.
func newRunnerFake(t *testing.T, items map[string]apiclient.ResourceClass) *apiclient.Client {
	t.Helper()

	byID := make(map[string]*apiclient.ResourceClass)
	for _, rc := range items {
		rc := rc
		byID[rc.ID] = &rc
	}

	r := chi.NewMux()
	r.Get("/api/v3/runner/resource-classes", func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("filter[slug]")
		rc, ok := items[slug]

		var data []any
		if ok {
			data = []any{map[string]any{
				"id": rc.ID,
				"attributes": map[string]any{
					"resource_class": rc.ResourceClass,
					"description":    rc.Description,
				},
			}}
		} else {
			data = []any{}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	})
	r.Post("/api/v3/runner/resource-classes/{id}/update", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		rc, ok := byID[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "not found"})
			return
		}
		var body struct {
			Description string `json:"description"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		rc.Description = body.Description
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":             rc.ID,
			"resource_class": rc.ResourceClass,
			"description":    rc.Description,
		})
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return apiclient.New(apiclient.Config{
		BaseURL: srv.URL,
		Token:   "test-token",
	})
}

func TestGetResourceClassBySlug(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	seeded := apiclient.ResourceClass{
		ID:            "550e8400-e29b-41d4-a716-446655440000",
		ResourceClass: "my-ns/my-runner",
		Description:   "a runner",
	}
	client := newRunnerFake(t, map[string]apiclient.ResourceClass{
		"my-ns/my-runner": seeded,
	})

	t.Run("known slug returns the resource class", func(t *testing.T) {
		rc, err := client.GetResourceClassBySlug(ctx, "my-ns/my-runner")
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(*rc, seeded))
	})

	t.Run("unknown slug returns ErrResourceClassNotFound", func(t *testing.T) {
		_, err := client.GetResourceClassBySlug(ctx, "my-ns/does-not-exist")
		assert.Check(t, errors.Is(err, apiclient.ErrResourceClassNotFound))
	})
}

func TestResourceClassByName(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	seeded := apiclient.ResourceClass{
		ID:            "550e8400-e29b-41d4-a716-446655440001",
		ResourceClass: "acme/fast",
		Description:   "",
	}
	client := newRunnerFake(t, map[string]apiclient.ResourceClass{
		"acme/fast": seeded,
	})

	t.Run("resolves via GetResourceClassBySlug", func(t *testing.T) {
		rc, err := client.ResourceClassByName(ctx, "acme/fast")
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(*rc, seeded))
	})

	t.Run("missing slash returns ErrResourceClassNotFound", func(t *testing.T) {
		_, err := client.ResourceClassByName(ctx, "no-slash")
		assert.Check(t, errors.Is(err, apiclient.ErrResourceClassNotFound))
	})

	t.Run("unknown name returns ErrResourceClassNotFound", func(t *testing.T) {
		_, err := client.ResourceClassByName(ctx, "acme/unknown")
		assert.Check(t, errors.Is(err, apiclient.ErrResourceClassNotFound))
	})
}

func TestUpdateResourceClass(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	seeded := apiclient.ResourceClass{
		ID:            "550e8400-e29b-41d4-a716-446655440002",
		ResourceClass: "my-ns/my-runner",
		Description:   "original description",
	}
	client := newRunnerFake(t, map[string]apiclient.ResourceClass{
		"my-ns/my-runner": seeded,
	})

	id, err := uuid.Parse(seeded.ID)
	assert.NilError(t, err)

	t.Run("updates description", func(t *testing.T) {
		rc, err := client.UpdateResourceClass(ctx, id, "updated description")
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(rc.ID, seeded.ID))
		assert.Check(t, cmp.Equal(rc.ResourceClass, seeded.ResourceClass))
		assert.Check(t, cmp.Equal(rc.Description, "updated description"))
	})

	t.Run("unknown id returns error", func(t *testing.T) {
		unknownID := uuid.MustParse("00000000-0000-0000-0000-000000000000")
		_, err := client.UpdateResourceClass(ctx, unknownID, "new desc")
		assert.Check(t, err != nil)
	})
}
