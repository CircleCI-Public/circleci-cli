# V3 API Field-Level Gap Analysis (numbers dropped)

Detailed analysis of exactly what fields the CLI needs from V3 endpoints,
derived from the `next` branch source code. Assumes pipeline/job/trigger
numbers are dropped entirely.

## Progress

Key PRs closing gaps:
- **circleci-cli#1384** — migrates `run get/list/watch` to V3 runs API (GET + search)
- **circleci-cli#1383** — wires V3 jobs endpoint (`GET /v3/jobs?filter[workflow_id]`) into run/workflow commands
- **public-api-service#1010** — adds V3 step output endpoints (`GET /v3/jobs/:id/stdout`, `/stderr`)

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

## 1. GET /v3/runs — List runs for a project — DONE (cli#1384)

**Used by:** `run list`, `workflow list` (recent mode)

> **Status:** CLI now uses `POST /v3/runs/search` for listing (not GET with
> filters). `run list` calls `SearchRunsV3` with project_id + branch filter
> expression. V3 response provides `phase`, `current_outcome`, `vcs.branch`,
> `vcs.revision`, `created_at`, and `references.project`/`references.user`.
> Duration and trigger type/actor are no longer displayed — trigger section
> dropped from `run get` output.

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
| `data[].id` | UUID | everywhere | **delivered** |
| `data[].attributes.phase` | string | list display, `deriveStatus()` | **delivered** — mapped to status via `phaseOutcomeStatus()` |
| `data[].attributes.current_outcome` | string (nullable) | list display, `deriveStatus()` | **delivered** — note: field is `current_outcome` not `outcome` |
| `data[].attributes.created_at` | timestamp | display, duration calc | **delivered** |
| `data[].attributes.vcs.branch` | string | list column, filter, watch header | **delivered** — nested under `vcs`, not flat |
| `data[].attributes.vcs.revision` | string | list column (7-char prefix), SHA matching in `watch --sha` | **delivered** — nested under `vcs`, not flat |
| `data[].references.project` | `RefEntity` | display (project context) | **delivered** — id only |
| `data[].references.user` | `RefEntity` | trigger actor | **delivered** — id only, no login attribute yet |
| `data[].references.event` | `RefEntity` | trigger type | not delivered — trigger type dropped from CLI output |

### `run list` table column → source field mapping

| Column | v2 source | v3 source (actual) | Notes |
|--------|-----------|-------------------|-------|
| `#` | `Pipeline.Number` | **dropped** | |
| `Branch` | `Pipeline.VCS.Branch` or `TriggerParameters.Git.Branch` | `data[].attributes.vcs.branch` | single path now |
| `Revision` | `Pipeline.VCS.Revision` or `TriggerParameters.Git.CheckoutSHA` | `data[].attributes.vcs.revision` | truncated to 7 chars client-side |
| `ID` | `Pipeline.ID` | `data[].id` | UUID |
| `Created` | `Pipeline.CreatedAt` | `data[].attributes.created_at` | formatted client-side |
| `Duration` | computed from `PipelineWorkflowSummary.StoppedAt` | **dropped from list output** | no longer displayed in cli#1384 |
| `State` | `Pipeline.State` | `phaseOutcomeStatus(phase, current_outcome)` | **fixed** — now shows real status |

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

## 2. GET /v3/runs/{id} — Get a single run — DONE (cli#1384)

**Used by:** `run get` (UUID lookup), `run watch` (poll loop via `fetchWatchState`), `run cancel` (resolve then cancel workflows)

> **Status:** CLI now calls `GetRunV3` which hits `GET /v3/runs/{id}`.
> Returns `RunV3` domain type with `ID`, `Status` (mapped from phase/outcome),
> `Branch`, `Revision`, `CreatedAt`, `ProjectID`, `Errors[]`.
> Trigger section (type/actor) dropped from display. Number dropped.
> `run watch` uses V3 for the run, still V2 for workflows (pending section 5).

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

### `run get` display → v3 field mapping (actual, post cli#1384)

The run header section:

| Display line | v2 source | v3 source (actual) | Status |
|-------------|-----------|-------------------|--------|
| `ID` | `Pipeline.ID` | `data.id` | **delivered** |
| `Number` | `Pipeline.Number` | — | **dropped** |
| `Project` | `Pipeline.ProjectSlug` | — | **dropped from output** — no slug in V3 response |
| `Branch` | `Pipeline.VCS.Branch` or `TriggerParameters.Git.Branch` | `data.attributes.vcs.branch` | **delivered** |
| `Commit` | `Pipeline.VCS.Revision` or `TriggerParameters.Git.CheckoutSHA` | `data.attributes.vcs.revision` | **delivered** |
| `Status` | **derived** via `deriveStatus()` | `phaseOutcomeStatus(phase, current_outcome)` | **delivered** — no longer derived from workflows |

The trigger section:

| Display line | v2 source | v3 source (actual) | Status |
|-------------|-----------|-------------------|--------|
| `Created At` | `Pipeline.CreatedAt` | `data.attributes.created_at` | **delivered** |
| `By` | `Pipeline.Trigger.Actor.Login` | — | **dropped from output** |
| `Type` | `Pipeline.Trigger.Type` | — | **dropped from output** |

### Note on `Status` — RESOLVED

`deriveStatus()` has been replaced by `phaseOutcomeStatus()` which maps
V3 `phase`/`current_outcome` directly to display strings. No more walking
workflow statuses to compute run status. Both `run list` and `run get` now
show meaningful status (success/failed/running) instead of the broken
"created" from v2.

**Performance:** `run watch` still fetches workflows + jobs each poll for
the detailed display, but terminal detection now uses the run-level
phase/outcome rather than checking all workflow statuses.

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

## 4. POST /v3/runs/search — Search runs — DONE (cli#1384)

> **Status:** CLI now uses `SearchRunsV3` for both `run list` and "latest on
> branch" lookups. The search request uses `scope.project_ids` + `scope.from`/`to`,
> `filter` expression for branch (`pipeline.git.branch == "X"`), `order_by`,
> and `page.limit`/`page.cursor`. `BuildRunFilter()` constructs the filter string.
> `run get` (no args) and `run watch` (no args) use search with `page.limit=1`
> to find the latest run on the current branch.

Request body shape (as implemented):

```json
{
  "scope": {
    "project_ids": ["proj-uuid"],
    "from": "2025-01-01T00:00:00Z",
    "to": "2026-12-31T00:00:00Z"
  },
  "filter": "pipeline.git.branch == \"main\"",
  "order_by": "",
  "page": {"cursor": "", "limit": 10}
}
```

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

## 9. GET /v3/jobs?filter[workflow_id]={id} — List jobs for a workflow — DONE (cli#1383)

**Used by:** `run get`, `run watch`, `workflow get`

> **Status:** CLI now calls `GetWorkflowJobsV3` which hits
> `GET /v3/jobs?filter[workflow_id]=<id>`. Returns `WorkflowJobV3` with
> `ID`, `Name`, `Status` (mapped from phase/outcome via `phaseOutcomeStatus`),
> `ProjectID`, `StartedAt`, `EndedAt`. V2 `GetWorkflowJobs` kept for
> artifacts/logs which still need `JobNumber` and `ProjectSlug`.
> Job `type` (build/approval) is not returned by V3 — approval jobs no longer
> distinguished in display. Job number column dropped.

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

| Envelope path | Type | CLI usage | V3 status |
|---------------|------|-----------|-----------|
| `data[].id` | UUID | job get, logs | **delivered** |
| `data[].attributes.name` | string | display in all three commands | **delivered** |
| `data[].attributes.phase` | string | `hasFailedJob()`, `failedJobNames()`, `watchFingerprint()` | **delivered** — mapped to status client-side |
| `data[].attributes.outcome` / `current_outcome` | string (nullable) | display, failure detection | **delivered** |
| `data[].attributes.type` | string | `approval` jobs rendered differently | **not delivered** — dropped from V3 response |
| `data[].attributes.started_at` | timestamp | in struct, not displayed in list | **delivered** |
| `data[].attributes.ended_at` | timestamp (nullable) | in struct, not displayed in list | **delivered** |
| `data[].references.workflow` | `RefEntity` | back-reference to parent workflow | **delivered** |
| `data[].references.project` | `RefEntity` | needed for logs V2 fallback | **delivered** |

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

### Step output (logs) — IN PROGRESS (p-a-s#1010)

> **Status:** V3 step output endpoints are being added in public-api-service#1010:
> - `GET /v3/jobs/:id/stdout?index=N&step_num=M` — stdout for a step task
> - `GET /v3/jobs/:id/stderr?index=N&step_num=M` — stderr for a step task
>
> These replace the private API at `/api/private/output/raw/{slug}/{number}/...`.
> Key differences from the original gap analysis proposals:
> - Uses `index` and `step_num` as required filter params (via `request.MustFilterInt`)
> - Returns raw `application/octet-stream`, not JSON — streamed directly
> - `X-Terminal: true` header indicates the step has completed (enables caching)
> - Supports `Range` header for partial reads
> - 204 No Content when output doesn't exist (not 404)
> - Depends on query#1104 for `HeadJob` (authz check by job UUID)
>
> The CLI will need a new V3 client method to call these endpoints, replacing
> the current `GetStepOutput`/`GetStepError` methods that use project slug +
> job number. The `index` and `step_num` coordinates come from the job detail
> response's steps array — same concept as today's `action.Index`/`action.Step`,
> just addressed by job UUID instead of slug+number.

---

## 11. GET /v3/projects/{id} — Get a project (EXISTS)

The CLI operates on project slugs (`gh/org/repo`). V3 uses project UUIDs
and bans slugs from response bodies and paths.

The CLI resolves slug → UUID via `GET /v3/projects?filter[slug]={provider}/{org}/{project}`
which returns a single-item collection. The UUID is then used for all
subsequent V3 calls. This can be cached per session.

---

## Endpoint status summary

| Endpoint | Status | PR | Commands affected |
|----------|--------|-----|-------------------|
| `GET /v3/runs/{id}` | **DONE** | cli#1384 | run get, run watch |
| `POST /v3/runs/search` | **DONE** | cli#1384 | run list, run get (latest), run watch (latest) |
| `GET /v3/jobs?filter[workflow_id]={id}` | **DONE** | cli#1383 | run get, run watch, workflow get |
| `GET /v3/jobs/:id/stdout` | **IN PROGRESS** | p-a-s#1010 | logs |
| `GET /v3/jobs/:id/stderr` | **IN PROGRESS** | p-a-s#1010 | logs |
| `GET /v3/workflows?filter[run_id]={id}` | **MISSING** | — | run get, run watch, run list, run cancel, workflow list (5) |
| `POST /v3/runs` | **MISSING** | — | run trigger (1) |
| `POST /v3/workflows/{id}/rerun` | **MISSING** | — | workflow rerun (1) |

### Still on V2

| CLI command | V2 dependency | Why |
|-------------|--------------|-----|
| `run cancel` | `GetPipelineByNumber`, `CancelWorkflow` | no V3 run cancel, still uses number lookup |
| `run trigger` | `TriggerPipeline` | no `POST /v3/runs` |
| `workflow cancel` | `CancelWorkflow` | V3 cancel exists but CLI not wired yet |
| `workflow rerun` | `RerunWorkflow` | no V3 rerun endpoint |
| `workflow list` | `GetPipelineWorkflows` | no V3 workflow list |
| `logs` | `GetJob`, `GetStepOutput`, `GetStepError` | needs V3 step output (p-a-s#1010) + CLI wiring |
| `artifacts` | `GetWorkflowJobs` (V2) | needs `JobNumber` and `ProjectSlug` for artifact URLs |

---

## Impact of dropping numbers — what it simplifies

- **Eliminated:** number-lookup blocker (`GET /v3/runs/by-number/...`) — not needed
- **Eliminated:** `number` field on run responses — not needed
- **Eliminated:** `job_number` field on job responses — cli#1383 drops it from display
- **Eliminated:** `pipeline_number` on workflow responses — not needed
- **Simplified:** trigger response — just returns UUID, no number allocation
- **Simplified:** "latest on branch" — search/filter by branch, no number-based convenience

## What changed vs original analysis

Fields **dropped from CLI output** rather than added to V3 (pragmatic approach):
- `project_slug` — no slug in V3 responses, dropped from display
- `trigger.type` / `trigger.actor` — trigger section dropped entirely
- `duration` — dropped from `run list` (was computed from workflow `stopped_at`)
- `job.type` — V3 doesn't return `type` (build/approval), approval jobs no longer distinguished
- `job_number` — dropped from display in all commands

Fields **delivered differently** than proposed:
- `branch`/`revision` nested under `attributes.vcs` not flat on attributes
- `outcome` field is `current_outcome` (supports predicted outcomes for running entities)
- Status mapped client-side via `phaseOutcomeStatus()` helper, not displayed as raw phase/outcome
