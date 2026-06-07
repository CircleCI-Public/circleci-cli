# V3 API Field-Level Gap Analysis (numbers dropped)

Detailed analysis of exactly what fields the CLI needs from V3 endpoints,
derived from the `next` branch source code. Assumes pipeline/job/trigger
numbers are dropped entirely.

All response shapes follow the V3 design rules:
- Data envelope: `data.id`, `data.attributes`, `data.references`
- `phase` + `outcome` replace `status`
- Timestamps suffixed `_at`, durations suffixed `_ms`
- Booleans prefixed `is_`, `has_`, `can_`, `should_`
- No slugs in response bodies — slugs only in `filter[slug]` query params
- References as entity-name-keyed objects with `id` + optional `attributes`
- Cursor pagination via `page[limit]` + `page[cursor]`
- Collections return `DataEntity` items (subset of single-entity response)

---

## 1. GET /v3/runs — List runs for a project

**Used by:** `run list`, `workflow list` (recent mode)

### Filter/query params

| Param | Type | Notes |
|-------|------|-------|
| `filter[project_id]` | UUID | required — identifies the project |
| `filter[slug]` | string | alternative to project_id — `{provider}/{org}/{project}` |
| `filter[branch]` | string (optional) | filter by branch |
| `page[limit]` | int | default 20, max 250 |
| `page[cursor]` | string | cursor pagination |

### Fields needed per run (as `data[]` items)

Each item is a `DataEntity` — subset of the single-run response.

| Envelope path | Type | CLI usage | V3 status |
|---------------|------|-----------|-----------|
| `data[].id` | UUID | everywhere | exists |
| `data[].attributes.phase` | string | list display, `deriveStatus()` | exists (but needs enrichment — see note) |
| `data[].attributes.outcome` | string (nullable) | list display, `deriveStatus()` | exists (but needs enrichment — see note) |
| `data[].attributes.created_at` | timestamp | display, duration calc | exists |
| `data[].attributes.branch` | string | list column, filter, watch header | **missing** |
| `data[].attributes.revision` | string | list column (7-char prefix), SHA matching in `watch --sha` | **missing** |
| `data[].references.project` | `RefEntity` | display (project context) | **missing** |
| `data[].references.event` | `RefEntity` | trigger type + actor context | **missing** |

### `run list` table column → source field mapping

| Column | v2 source | v3 source (proposed) | Notes |
|--------|-----------|---------------------|-------|
| `#` | `Pipeline.Number` | **dropped** | |
| `Branch` | `Pipeline.VCS.Branch` or `TriggerParameters.Git.Branch` | `data[].attributes.branch` | single field, no dual code path |
| `Revision` | `Pipeline.VCS.Revision` or `TriggerParameters.Git.CheckoutSHA` | `data[].attributes.revision` | truncated to 7 chars client-side |
| `ID` | `Pipeline.ID` | `data[].id` | UUID |
| `Created` | `Pipeline.CreatedAt` | `data[].attributes.created_at` | formatted client-side |
| `Duration` | computed from `PipelineWorkflowSummary.StoppedAt` | computed from workflow `ended_at` | still requires workflow fetch (see section 5) |
| `State` | `Pipeline.State` | derived from `phase` + `outcome` | see note below |

### Note on `State` column

The `State` column in `run list` today shows the raw `Pipeline.State` from v2,
which is almost always "created" — it reflects pipeline creation lifecycle,
not execution outcome. This is broken: a run that failed 5 minutes ago
still shows "created".

`run get` works around this with `deriveStatus()` which walks workflow
statuses in priority order (failed > running > on_hold > canceled > success),
but `run list` doesn't — it just shows the raw state.

For v3, if the run-level `phase` + `outcome` are computed from workflow
states (not just the pipeline creation lifecycle), both `run list` and
`run get` can use them directly. This would eliminate the need for
`deriveStatus()` and remove the per-run workflow fetch just to compute
display status. The run `phase`/`outcome` should reflect execution status:

| phase | outcome | displayed as |
|-------|---------|-------------|
| `queued` | null | queued |
| `started` | null | running |
| `started` | `failed` (current_outcome) | failing |
| `ended` | `succeeded` | success |
| `ended` | `failed` | failed |
| `ended` | `canceled` | canceled |
| `ended` | `errored` | errored |

---

## 2. GET /v3/runs/{id} — Get a single run

**Used by:** `run get` (UUID lookup), `run watch` (poll loop via `fetchWatchState`), `run cancel` (resolve then cancel workflows)

Full version of the list item — same fields plus additional detail.

### V3 response shape

```json
{
  "data": {
    "id": "951bcd16-a7bb-49f3-b6a7-8ac3a49c4587",
    "attributes": {
      "phase": "ended",
      "outcome": "succeeded",
      "branch": "dm/gap-analysis",
      "revision": "8c4978f3a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
      "created_at": "2026-06-05T09:19:51.000Z",
      "updated_at": "2026-06-05T09:21:18.000Z",
      "errors": []
    },
    "references": {
      "project": {
        "id": "770e8400-e29b-41d4-a716-446655440002"
      },
      "event": {
        "id": "880e8400-e29b-41d4-a716-446655440003",
        "attributes": {
          "type": "webhook"
        }
      },
      "user": {
        "id": "660e8400-e29b-41d4-a716-446655440001",
        "attributes": {
          "login": "danmux"
        }
      }
    }
  }
}
```

### `run get` display → v3 field mapping

The run header section:

| Display line | v2 source | v3 source |
|-------------|-----------|-----------|
| `ID` | `Pipeline.ID` | `data.id` |
| `Number` | `Pipeline.Number` | **dropped** |
| `Project` | `Pipeline.ProjectSlug` | resolved from `data.references.project.id` (or cached slug) |
| `Branch` | `Pipeline.VCS.Branch` or `TriggerParameters.Git.Branch` | `data.attributes.branch` |
| `Commit` | `Pipeline.VCS.Revision` or `TriggerParameters.Git.CheckoutSHA` | `data.attributes.revision` (truncated client-side) |
| `Status` | **derived** via `deriveStatus()` from workflow statuses | `data.attributes.phase` + `data.attributes.outcome` |

The trigger section:

| Display line | v2 source | v3 source |
|-------------|-----------|-----------|
| `Created At` | `Pipeline.CreatedAt` | `data.attributes.created_at` |
| `By` | `Pipeline.Trigger.Actor.Login` | `data.references.user.attributes.login` |
| `Type` | `Pipeline.Trigger.Type` | `data.references.event.attributes.type` |

### Note on `Status` in `run get` vs `run list`

`run get` today shows a **derived** status computed client-side from workflow
statuses via `deriveStatus()` in `run/get.go:263`. Priority order:
errored (run-level) > failed > running > on_hold > canceled > success.

If v3 run `phase`/`outcome` reflect execution state (not just pipeline
creation lifecycle), `deriveStatus()` can be replaced with a direct mapping.
Otherwise `run get` must continue fetching workflows to derive it — which
requires the `GET /v3/workflows?filter[run_id]=...` endpoint.

**Performance note:** `run watch` polls this every 5-30s (ramping interval).
Each poll currently fetches run + all workflows + all jobs. If the run
`phase`/`outcome` is authoritative, watch can skip the workflow fetch for
terminal detection.

---

## 3. POST /v3/runs — Trigger a run (MISSING)

**Used by:** `run trigger`

### Request body (data envelope)

```json
{
  "data": {
    "attributes": {
      "branch": "main",
      "parameters": {"deploy_env": "staging", "run_e2e": true}
    },
    "references": {
      "project": {"id": "770e8400-e29b-41d4-a716-446655440002"}
    }
  }
}
```

### Response (202 Accepted)

```json
{
  "data": {
    "id": "951bcd16-a7bb-49f3-b6a7-8ac3a49c4587",
    "attributes": {
      "phase": "accepted"
    }
  }
}
```

Client polls `GET /v3/runs/{id}` until `phase` transitions from `accepted`.
`Location` and `Retry-After` headers set automatically by backplane-go.

### CLI display mapping

| Display | v2 source | v3 source |
|---------|-----------|-----------|
| `Triggered run ... on {branch}` | `TriggerResponse.Number`, `TriggerResponse.ID` | `data.id` + branch from request |
| `State` | `TriggerResponse.State` | `data.attributes.phase` |

---

## 4. POST /v3/runs/search — Search runs (EXISTS)

Could cover "latest on branch" — used by `run get` (no args), `run watch`
(no args), `logs --last-failed`. Needs:

- Scope by `project_ids` + time range (`from`/`to`)
- Filter by branch
- Sort by `created_at` desc
- `page[limit]=1` for "latest" use case
- Response items must include same attributes as GET /v3/runs list items

Alternatively, `GET /v3/runs?filter[project_id]=X&filter[branch]=Y&page[limit]=1`
covers this without needing search.

---

## 5. GET /v3/workflows?filter[run_id]={id} — List workflows for a run (MISSING)

**Used by:** `run get`, `run watch`, `run list` (duration calc), `run cancel`, `workflow list`

This is the single most blocking gap — 5 of 11 CLI commands need it.

### Query params

| Param | Type | Notes |
|-------|------|-------|
| `filter[run_id]` | UUID | required — the run to list workflows for |
| `page[limit]` | int | default 20, max 250 (most runs have 1-3 workflows) |
| `page[cursor]` | string | cursor pagination |

### V3 response shape

```json
{
  "data": [
    {
      "id": "aab1c2d3-e4f5-6789-abcd-ef0123456789",
      "attributes": {
        "name": "ci",
        "phase": "ended",
        "outcome": "succeeded",
        "created_at": "2026-06-05T09:19:52.000Z",
        "ended_at": "2026-06-05T09:21:18.000Z"
      },
      "references": {
        "run": {"id": "951bcd16-a7bb-49f3-b6a7-8ac3a49c4587"}
      }
    }
  ],
  "page": {
    "next": null,
    "prev": null
  }
}
```

### Fields needed per workflow

| Envelope path | Type | CLI usage |
|---------------|------|-----------|
| `data[].id` | UUID | display, pass to cancel/rerun/get-jobs |
| `data[].attributes.name` | string | display |
| `data[].attributes.phase` | string | `deriveStatus()`, `allWorkflowsDone()`, `watchFingerprint()` |
| `data[].attributes.outcome` | string (nullable) | `deriveStatus()`, display |
| `data[].attributes.created_at` | timestamp | available, not currently displayed in summary |
| `data[].attributes.ended_at` | timestamp (nullable) | `workflowDuration()` — computes run wall-clock time |
| `data[].references.run` | `RefEntity` | back-reference to parent run |

### CLI mapping

The CLI currently maps v2 workflow `status` to display text. With v3
`phase`/`outcome`, the mapping becomes:

| phase | outcome | CLI displays as |
|-------|---------|----------------|
| `queued` | null | queued |
| `started` | null | running |
| `ended` | `succeeded` | success |
| `ended` | `failed` | failed |
| `ended` | `canceled` | canceled |
| `ended` | `errored` | error |

`allWorkflowsDone()` becomes: `phase == "ended"` for all workflows.

`workflowDuration()` uses `ended_at` (was `stopped_at`).

---

## 6. GET /v3/workflows/{id} — Get a single workflow (EXISTS)

**Used by:** `workflow get`

### V3 response shape

```json
{
  "data": {
    "id": "aab1c2d3-e4f5-6789-abcd-ef0123456789",
    "attributes": {
      "name": "ci",
      "phase": "ended",
      "outcome": "succeeded",
      "created_at": "2026-06-05T09:19:52.000Z",
      "ended_at": "2026-06-05T09:21:18.000Z"
    },
    "references": {
      "run": {"id": "951bcd16-a7bb-49f3-b6a7-8ac3a49c4587"},
      "project": {"id": "770e8400-e29b-41d4-a716-446655440002"},
      "user": {
        "id": "660e8400-e29b-41d4-a716-446655440001",
        "attributes": {"login": "danmux"}
      }
    }
  }
}
```

### `workflow get` display → v3 field mapping

| Display line | v2 source | v3 source |
|-------------|-----------|-----------|
| `ID` | `WorkflowDetail.ID` | `data.id` |
| `Name` | `WorkflowDetail.Name` | `data.attributes.name` |
| `Run ID` | `WorkflowDetail.PipelineID` | `data.references.run.id` |
| `Run Number` | `WorkflowDetail.PipelineNumber` | **dropped** |
| `Project` | `WorkflowDetail.ProjectSlug` | resolved from `data.references.project.id` (or cached) |
| `Status` | `WorkflowDetail.Status` | `data.attributes.phase` + `data.attributes.outcome` |
| `Created` | `WorkflowDetail.CreatedAt` | `data.attributes.created_at` |
| `Stopped` | `WorkflowDetail.StoppedAt` | `data.attributes.ended_at` |

---

## 7. POST /v3/workflows/{id}/cancel (EXISTS)

**Used by:** `workflow cancel`, `run cancel` (cancels each workflow)

No field gaps. Takes UUID, returns ack. Verb-terminated POST.

---

## 8. POST /v3/workflows/{id}/rerun (MISSING)

**Used by:** `workflow rerun`

### Request body (plain JSON — scoped action, not data envelope)

```json
{
  "is_from_failed": true
}
```

### Response

Acknowledgement — could return the new workflow as a `DataEntity` (202) or
a simple ack. The CLI currently ignores the response body beyond error
checking.

---

## 9. GET /v3/jobs?filter[workflow_id]={id} — List jobs for a workflow (MISSING)

**Used by:** `run get`, `run watch`, `workflow get`

Second most blocking gap — 3 commands need it, on the polling hot path for
`run watch`.

### Query params

| Param | Type | Notes |
|-------|------|-------|
| `filter[workflow_id]` | UUID | required — the workflow to list jobs for |
| `page[limit]` | int | default 20, max 250 |
| `page[cursor]` | string | cursor pagination |

### V3 response shape

```json
{
  "data": [
    {
      "id": "ccb1c2d3-e4f5-6789-abcd-ef0123456789",
      "attributes": {
        "name": "test-linux",
        "phase": "ended",
        "outcome": "succeeded",
        "type": "build",
        "started_at": "2026-06-05T09:20:12.000Z",
        "ended_at": "2026-06-05T09:21:18.000Z"
      },
      "references": {
        "workflow": {"id": "aab1c2d3-e4f5-6789-abcd-ef0123456789"}
      }
    }
  ],
  "page": {
    "next": null,
    "prev": null
  }
}
```

### Fields needed per job

| Envelope path | Type | CLI usage |
|---------------|------|-----------|
| `data[].id` | UUID | needed for `circleci job get <uuid>`, `circleci logs <uuid>` post-numbers |
| `data[].attributes.name` | string | display in all three commands |
| `data[].attributes.phase` | string | `hasFailedJob()`, `failedJobNames()`, `watchFingerprint()` |
| `data[].attributes.outcome` | string (nullable) | display, failure detection |
| `data[].attributes.type` | string | `approval` jobs rendered differently (no log suggestion) |
| `data[].attributes.started_at` | timestamp | in struct, not displayed in list |
| `data[].attributes.ended_at` | timestamp (nullable) | in struct, not displayed in list |
| `data[].references.workflow` | `RefEntity` | back-reference to parent workflow |

### CLI mapping for job display

The nested job lines in `run get` and `workflow get` currently show:

```
  test-linux                            success  #38835
```

Post-numbers, post-v3:

```
  test-linux                            success
```

`hasFailedJob()` becomes: `outcome == "failed"`.

`failedJobLogSuggestions()` changes from `circleci logs <number>` to
`circleci logs --last-failed` (already proposed in remove-numbers.md).

---

## 10. GET /v3/jobs/{id} — Get a single job (EXISTS)

**Used by:** `logs` command (step output fetch), proposed `job get` (step summary)

### Proposed `job get` command

Currently there's no middle ground between a one-line job entry in `workflow get`
and the full log firehose of `logs`. A `circleci job get <uuid>` command would
show step-level summary without output — fast, single request.

Proposed display:

```
# Job

- ID: ccb1c2d3-e4f5-6789-abcd-ef0123456789
- Name: test-linux
- Phase: ended
- Outcome: failed
- Started: 2026-06-05 09:20:12 UTC
- Duration: 1m14s

## Steps

  Name                          Phase    Outcome     Duration   Exit
  ────────────────────────────  ───────  ──────────  ─────────  ────
  Spin up environment           ended    succeeded   8s          0
  Checkout code                 ended    succeeded   2s          0
  Restore cache                 ended    succeeded   4s          0
  Install dependencies          ended    succeeded   22s         0
  Run tests                     ended    failed      38s         1
  Save cache                    -        -           -           -
```

### V3 response shape

```json
{
  "data": {
    "id": "ccb1c2d3-e4f5-6789-abcd-ef0123456789",
    "attributes": {
      "name": "test-linux",
      "phase": "ended",
      "outcome": "failed",
      "started_at": "2026-06-05T09:20:12.000Z",
      "ended_at": "2026-06-05T09:21:26.000Z",
      "steps": [
        {
          "name": "Spin up environment",
          "phase": "ended",
          "outcome": "succeeded",
          "started_at": "2026-06-05T09:20:12.000Z",
          "ended_at": "2026-06-05T09:20:20.000Z",
          "exit_code": 0
        },
        {
          "name": "Run tests",
          "phase": "ended",
          "outcome": "failed",
          "started_at": "2026-06-05T09:20:48.000Z",
          "ended_at": "2026-06-05T09:21:26.000Z",
          "exit_code": 1
        }
      ]
    },
    "references": {
      "workflow": {"id": "aab1c2d3-e4f5-6789-abcd-ef0123456789"},
      "project": {"id": "770e8400-e29b-41d4-a716-446655440002"}
    }
  }
}
```

### Fields needed

| Envelope path | Type | Usage | V3 status |
|---------------|------|-------|-----------|
| `data.id` | UUID | identity | exists |
| `data.attributes.name` | string | display | unknown |
| `data.attributes.phase` | string | display | unknown |
| `data.attributes.outcome` | string (nullable) | display | unknown |
| `data.attributes.started_at` | timestamp | display, duration calc | unknown |
| `data.attributes.ended_at` | timestamp (nullable) | display, duration calc | unknown |
| `data.attributes.steps[]` | array | step summary and log fetching | unknown |
| `data.attributes.steps[].name` | string | step header display | unknown |
| `data.attributes.steps[].phase` | string | display | unknown |
| `data.attributes.steps[].outcome` | string (nullable) | display (failed indicator) | unknown |
| `data.attributes.steps[].started_at` | timestamp | duration calc | unknown |
| `data.attributes.steps[].ended_at` | timestamp (nullable) | duration calc | unknown |
| `data.attributes.steps[].exit_code` | int (nullable) | display | unknown |
| `data.references.workflow` | `RefEntity` | back-reference | unknown |
| `data.references.project` | `RefEntity` | needed for step output URL construction | unknown |

### Step output (logs)

Step output is not part of the job detail response — it's a separate fetch.
The current private API at `/api/private/output/raw/{slug}/{number}/output/{taskIndex}/{stepID}`
is keyed on job number and project slug. Post-numbers, this needs a UUID-based
alternative. Options:

1. **New V3 endpoint:** `GET /v3/jobs/{id}/steps/{index}/output` — cleanest
2. **Signed URLs in job response:** each step includes an `output_url` that
   the CLI fetches directly — avoids a second API hop
3. **Inline output:** steps include output in the response — impractical for
   large jobs

Option 1 or 2 preferred. For parallel jobs (multiple tasks per step), the
endpoint should accept a `filter[task_index]` or return all tasks' output
concatenated.

---

## 11. GET /v3/projects/{id} — Get a project (EXISTS)

The CLI operates on project slugs (`gh/org/repo`). V3 uses project UUIDs
and bans slugs from response bodies and paths.

The CLI resolves slug → UUID via `GET /v3/projects?filter[slug]={provider}/{org}/{project}`
which returns a single-item collection. The UUID is then used for all
subsequent V3 calls. This can be cached per session.

---

## Missing endpoints ranked by blast radius

| Priority | Endpoint | Commands blocked |
|----------|----------|------------------|
| **P0** | `GET /v3/workflows?filter[run_id]={id}` | run get, run watch, run list, run cancel, workflow list (5) |
| **P0** | `GET /v3/jobs?filter[workflow_id]={id}` | run get, run watch, workflow get (3) |
| **P1** | `POST /v3/runs` | run trigger (1, but core use case) |
| **P1** | `POST /v3/workflows/{id}/rerun` | workflow rerun (1) |
| **P2** | `GET /v3/jobs/{id}/steps/{index}/output` | logs (1, replaces private output API) |
| **P2** | Run data enrichment (branch/revision/trigger on existing endpoints) | run list, run get, run watch, workflow list (4) |

---

## Impact of dropping numbers — what it simplifies

- **Eliminated:** number-lookup blocker (`GET /v3/runs/by-number/...`) — not needed
- **Eliminated:** `number` field on run responses — not needed
- **Eliminated:** `job_number` field on job responses — not needed
- **Eliminated:** `pipeline_number` on workflow responses — not needed
- **Simplified:** trigger response — just returns UUID, no number allocation
- **Simplified:** "latest on branch" — search/filter by branch, no number-based convenience
- **Unchanged:** all 4 missing endpoints, all data enrichment gaps, step output dependency
