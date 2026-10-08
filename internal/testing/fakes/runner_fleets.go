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

package fakes

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// RunnerFleet is a stored runner fleet served by GET /api/v3/runner/fleets and
// /api/v3/runner/fleets/{id}. ResourceClass names a resource class registered with
// AddResourceClass: the fleet's ID and owning org come from it, as they do on the real
// server. A fleet whose resource class is not registered is never served.
//
// AgentCount is the fleet's true agent total; when zero it defaults to len(Agents), and a
// test sets it higher to model the server's capped agent list. RunningTasks and QueuedTasks
// are served by the single-fleet endpoint only, as the list omits them.
type RunnerFleet struct {
	ResourceClass      string
	ActiveAgents       int
	IdleAgents         int
	DisconnectedAgents int
	AgentCount         int
	RunningTasks       int
	QueuedTasks        int
	LastTaskClaimedAt  string // RFC 3339; empty means never claimed (served as null)
	Agents             []RunnerFleetAgent
}

// RunnerFleetAgent is an agent reference embedded in a fleet.
type RunnerFleetAgent struct {
	ID   string
	Name string
}

// AddRunnerFleet registers a fleet for a resource class.
func (f *CircleCI) AddRunnerFleet(fleet RunnerFleet) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runnerFleets = append(f.runnerFleets, fleet)
}

// SetRunnerFleetUnavailable makes both fleet endpoints answer 503 with runner-admin's
// "Unable to determine agent status." error, as they do when the Distributor is unreachable.
func (f *CircleCI) SetRunnerFleetUnavailable() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runnerFleetUnavailable = true
}

// runnerFleetEntity renders a stored fleet as its V3 entity. The task counts are included only
// when withTasks is set.
func runnerFleetEntity(fleet RunnerFleet, rc ResourceClass, withTasks bool) map[string]any {
	count := fleet.AgentCount
	if count == 0 {
		count = len(fleet.Agents)
	}
	attrs := map[string]any{
		"name":                fleet.ResourceClass,
		"active_agents":       fleet.ActiveAgents,
		"idle_agents":         fleet.IdleAgents,
		"disconnected_agents": fleet.DisconnectedAgents,
		"agent_count":         count,
	}
	if withTasks {
		attrs["running_tasks"] = fleet.RunningTasks
		attrs["queued_tasks"] = fleet.QueuedTasks
	}
	if fleet.LastTaskClaimedAt != "" {
		attrs["last_task_claimed_at"] = fleet.LastTaskClaimedAt
	} else {
		attrs["last_task_claimed_at"] = nil
	}

	agents := make([]any, len(fleet.Agents))
	for i, a := range fleet.Agents {
		agents[i] = map[string]any{
			"id":         a.ID,
			"attributes": map[string]any{"name": a.Name},
		}
	}
	return map[string]any{
		"id":         rc.ID,
		"attributes": attrs,
		"references": map[string]any{
			"resource_class": map[string]any{"id": rc.ID},
			"runner_agents":  agents,
		},
	}
}

// runnerFleetUnavailable answers runner-admin's 503 for a fleet whose agent status cannot be read.
func runnerFleetUnavailable(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusServiceUnavailable)
	render.JSON(w, r, runnerErrorBody("Unable to determine agent status."))
}

// handleListRunnerFleets serves GET /api/v3/runner/fleets. Like the real endpoint it requires
// exactly one filter and returns fleets ordered by resource class ID, but its cursor is an
// offset rather than runner-admin's opaque one.
func (f *CircleCI) handleListRunnerFleets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filters := map[string]string{}
	for _, name := range []string{"org_id", "namespace", "resource_class", "resource_class_id"} {
		if v := q.Get("filter[" + name + "]"); v != "" {
			filters[name] = v
		}
	}
	if len(filters) != 1 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, runnerErrorBody("Exactly one filter parameter is required."))
		return
	}

	f.mu.RLock()
	unavailable := f.runnerFleetUnavailable
	fleets := slices.Clone(f.runnerFleets)
	classes := slices.Clone(f.resourceClasses)
	deleted := f.deletedRCs
	orgHidden := f.hiddenRunnerOrgs[filters["org_id"]]
	f.mu.RUnlock()

	if orgHidden {
		runnerNotFound(w, r, "Organization not found.")
		return
	}
	if unavailable {
		runnerFleetUnavailable(w, r)
		return
	}

	type match struct {
		fleet RunnerFleet
		rc    ResourceClass
	}
	var matches []match
	for _, fl := range fleets {
		i := slices.IndexFunc(classes, func(rc ResourceClass) bool { return rc.ResourceClass == fl.ResourceClass })
		if i < 0 || deleted[fl.ResourceClass] {
			continue
		}
		rc := classes[i]
		switch {
		case filters["org_id"] != "" && rc.OrgID != filters["org_id"]:
			continue
		case filters["namespace"] != "" && !strings.HasPrefix(rc.ResourceClass, filters["namespace"]+"/"):
			continue
		case filters["resource_class"] != "" && rc.ResourceClass != filters["resource_class"]:
			continue
		case filters["resource_class_id"] != "" && rc.ID != filters["resource_class_id"]:
			continue
		}
		matches = append(matches, match{fl, rc})
	}

	// A single resource class that does not exist is a 404; an org or namespace with no
	// resource classes is an empty collection.
	if len(matches) == 0 && (filters["resource_class"] != "" || filters["resource_class_id"] != "") {
		runnerNotFound(w, r, "Resource class not found.")
		return
	}

	slices.SortFunc(matches, func(a, b match) int { return strings.Compare(a.rc.ID, b.rc.ID) })
	items := make([]any, len(matches))
	for i, m := range matches {
		items[i] = runnerFleetEntity(m.fleet, m.rc, false)
	}

	offset, _ := strconv.Atoi(q.Get("page[cursor]"))
	if offset > len(items) {
		offset = len(items)
	}
	page := items[offset:]
	if size, err := strconv.Atoi(q.Get("page[limit]")); err == nil && size > 0 && len(page) > size {
		page = page[:size]
	}

	body := map[string]any{"data": page}
	if next := offset + len(page); next < len(items) {
		body["page"] = map[string]any{"next": strconv.Itoa(next)}
	}
	render.JSON(w, r, body)
}

// handleGetRunnerFleet serves GET /api/v3/runner/fleets/{id}, where id is the resource class's
// ID. Filter parameters alongside the ID are a 400, as on the real endpoint.
func (f *CircleCI) handleGetRunnerFleet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	for name := range r.URL.Query() {
		if strings.HasPrefix(name, "filter[") {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, runnerErrorBody("Filter parameters cannot be combined with a direct fleet ID."))
			return
		}
	}

	f.mu.RLock()
	unavailable := f.runnerFleetUnavailable
	fleets := slices.Clone(f.runnerFleets)
	classes := slices.Clone(f.resourceClasses)
	deleted := f.deletedRCs
	f.mu.RUnlock()

	for _, rc := range classes {
		if rc.ID != id || deleted[rc.ResourceClass] {
			continue
		}
		for _, fl := range fleets {
			if fl.ResourceClass != rc.ResourceClass {
				continue
			}
			if unavailable {
				runnerFleetUnavailable(w, r)
				return
			}
			render.JSON(w, r, map[string]any{"data": runnerFleetEntity(fl, rc, true)})
			return
		}
	}
	runnerNotFound(w, r, "Resource class not found.")
}
