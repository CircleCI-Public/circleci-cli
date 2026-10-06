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
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// newRunnerFake builds a minimal fake that responds to the v3 resource-classes endpoints.
// The handler maps filter[resource_class] values to pre-registered items; unknown resource
// classes return an empty collection. The update endpoint stores changes in-memory.
func newRunnerFake(t *testing.T, items map[string]apiclient.ResourceClass) *apiclient.Client {
	t.Helper()

	byID := make(map[string]*apiclient.ResourceClass)
	for _, rc := range items {
		rc := rc
		byID[rc.ID] = &rc
	}

	r := chi.NewMux()
	r.Get("/api/v3/runner/resource-classes", func(w http.ResponseWriter, r *http.Request) {
		resourceClass := r.URL.Query().Get("filter[resource_class]")
		rc, ok := items[resourceClass]

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
			"data": map[string]any{
				"id": rc.ID,
				"attributes": map[string]any{
					"resource_class": rc.ResourceClass,
					"description":    rc.Description,
				},
			},
		})
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return apiclient.New(apiclient.Config{
		BaseURL: srv.URL,
		Token:   "test-token",
	})
}

func TestGetResourceClass(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	seeded := apiclient.ResourceClass{
		ID:            "550e8400-e29b-41d4-a716-446655440000",
		ResourceClass: "my-ns/my-runner",
		Description:   "a runner",
	}
	client := newRunnerFake(t, map[string]apiclient.ResourceClass{
		"my-ns/my-runner": seeded,
	})

	t.Run("known resource class is returned", func(t *testing.T) {
		rc, err := client.GetResourceClass(ctx, "my-ns/my-runner")
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(*rc, seeded))
	})

	t.Run("unknown resource class returns ErrResourceClassNotFound", func(t *testing.T) {
		_, err := client.GetResourceClass(ctx, "my-ns/does-not-exist")
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

	t.Run("resolves via GetResourceClass", func(t *testing.T) {
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

func fleetJSON(id, name string, running, queued *int) map[string]any {
	attrs := map[string]any{
		"name":                name,
		"active_agents":       1,
		"idle_agents":         2,
		"disconnected_agents": 3,
		"agent_count":         6,
	}
	if running != nil {
		attrs["running_tasks"] = *running
		attrs["queued_tasks"] = *queued
	}
	return map[string]any{
		"id":         id,
		"attributes": attrs,
		"references": map[string]any{
			"resource_class": map[string]any{"id": id},
			"runner_agents": []any{map[string]any{
				"id":         "11111111-1111-4111-8111-111111111111",
				"attributes": map[string]any{"name": "agent-a"},
			}},
		},
	}
}

func TestRunnerFleets(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	const (
		idA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		idB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	)

	var queries []url.Values
	r := chi.NewMux()
	r.Get("/api/v3/runner/fleets", func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page[cursor]") == "" {
			next := "page-2"
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []any{fleetJSON(idA, "ns/a", nil, nil)},
				"page": map[string]any{"next": next},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{fleetJSON(idB, "ns/b", nil, nil)},
			"page": map[string]any{"next": nil},
		})
	})
	r.Get("/api/v3/runner/fleets/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id != idA {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "not found"})
			return
		}
		running, queued := 4, 5
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": fleetJSON(id, "ns/a", &running, &queued)})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	client := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "test-token"})

	t.Run("list follows the cursor and sends one filter", func(t *testing.T) {
		queries = nil
		fleets, err := client.ListRunnerFleetsByNamespace(ctx, "ns")
		assert.NilError(t, err)
		assert.Assert(t, cmp.Len(fleets, 2))
		assert.Check(t, cmp.Equal(fleets[0].Attributes.Name, "ns/a"))
		assert.Check(t, cmp.Equal(fleets[1].Attributes.Name, "ns/b"))
		assert.Assert(t, cmp.Len(queries, 2))
		assert.Check(t, cmp.Equal(queries[0].Get("filter[namespace]"), "ns"))
		assert.Check(t, cmp.Equal(queries[0].Get("page[limit]"), "250"))
		assert.Check(t, cmp.Equal(queries[1].Get("page[cursor]"), "page-2"))
	})

	t.Run("each list variant selects its own filter", func(t *testing.T) {
		org := uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
		rcID := uuid.MustParse(idA)
		cases := []struct {
			name, param, want string
			call              func() ([]apiclient.RunnerFleet, error)
		}{
			{"org", "filter[org_id]", org.String(), func() ([]apiclient.RunnerFleet, error) {
				return client.ListRunnerFleetsByOrg(ctx, org)
			}},
			{"resource class", "filter[resource_class]", "ns/a", func() ([]apiclient.RunnerFleet, error) {
				return client.ListRunnerFleetsByResourceClass(ctx, "ns/a")
			}},
			{"resource class id", "filter[resource_class_id]", idA, func() ([]apiclient.RunnerFleet, error) {
				return client.ListRunnerFleetsByResourceClassID(ctx, rcID)
			}},
		}
		for _, tc := range cases {
			queries = nil
			_, err := tc.call()
			assert.NilError(t, err, tc.name)
			assert.Assert(t, len(queries) > 0, tc.name)
			assert.Check(t, cmp.Equal(queries[0].Get(tc.param), tc.want), tc.name)
		}
	})

	t.Run("get decodes task counts and agents", func(t *testing.T) {
		fleet, err := client.GetRunnerFleet(ctx, uuid.MustParse(idA))
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(fleet.ID.String(), idA))
		assert.Check(t, cmp.Equal(fleet.Attributes.AgentCount, 6))
		assert.Assert(t, fleet.Attributes.RunningTasks != nil && fleet.Attributes.QueuedTasks != nil)
		assert.Check(t, cmp.Equal(*fleet.Attributes.RunningTasks, 4))
		assert.Check(t, cmp.Equal(*fleet.Attributes.QueuedTasks, 5))
		assert.Check(t, cmp.Nil(fleet.Attributes.LastTaskClaimedAt))
		assert.Assert(t, cmp.Len(fleet.References.RunnerAgents, 1))
		assert.Check(t, cmp.Equal(fleet.References.RunnerAgents[0].Attributes.Name, "agent-a"))
	})

	t.Run("get unknown fleet is a 404 error", func(t *testing.T) {
		_, err := client.GetRunnerFleet(ctx, uuid.MustParse(idB))
		assert.Check(t, httpcl.HasStatusCode(err, http.StatusNotFound))
	})
}
