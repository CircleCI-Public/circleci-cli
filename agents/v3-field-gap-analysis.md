# V3 API Field-Level Gap Analysis

Detailed analysis of exactly what fields the CLI uses from V3 endpoints,
derived from the `main` branch source code. Pipeline/job/trigger numbers
are dropped — only UUIDs used for entity addressing.

## Status overview

The V3 migration is nearly complete. Of 13 CLI commands that hit the API,
11 are fully V3 (or V3 with a single V2 number-lookup fallback). Only
`run trigger` and `workflow rerun` remain on V2.

| Command | V3 status | V2 remnant |
|---------|-----------|------------|
| `run list` | **fully V3** | — |
| `run get` | **fully V3** | — |
| `run watch` | **V3** | `GetPipelineByNumber` for number args |
| `run trigger` | **V2** | `TriggerPipeline` — no V3 trigger endpoint |
| `run cancel` | **V3** | `GetPipelineByNumber` for number args |
| `workflow get` | **fully V3** | — |
| `workflow list` | **V3** | `GetPipelineByNumber` for number args in `resolveRunID()` |
| `workflow cancel` | **fully V3** | — |
| `workflow rerun` | **V2** | `RerunWorkflow` — no V3 rerun endpoint |
| `job get` | **fully V3** | — |
| `job output get` | **fully V3** | — |
| `job output list` | **fully V3** | — |
| `job artifact` | **fully V3** | — |

All V3 responses follow the data envelope convention:
- `data.id`, `data.attributes`, `data.references`
- `phase` + `outcome` + `current_outcome` replace V2 `status`
- `PhaseOutcomeStatus(phase, outcome, current_outcome)` maps to display strings
- No slugs in response bodies
- Cursor pagination via `page[limit]` + `page[cursor]`

---

## 1. POST /v3/runs/search — Search runs — DONE

**Used by:** `run list`, `run get` (latest on branch), `run watch` (latest on branch)

### Request body

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

`BuildRunFilter()` constructs the filter expression. `--current-branch` / `-B`
flag auto-detects the branch from git.

### Fields consumed per run

| Envelope path | Type | CLI usage |
|---------------|------|-----------|
| `data[].id` | UUID | everywhere |
| `data[].attributes.phase` | string | status display via `PhaseOutcomeStatus()` |
| `data[].attributes.outcome` | string (nullable) | status display |
| `data[].attributes.current_outcome` | string (nullable) | status display (predicted outcome for running) |
| `data[].attributes.created_at` | timestamp | display |
| `data[].attributes.vcs.branch` | string | list column, filter |
| `data[].attributes.vcs.revision` | string | list column (7-char prefix) |
| `data[].references.project.id` | UUID | project context |
| `data[].references.user.id` | UUID | trigger actor |

### `run list` table columns

| Column | V3 source |
|--------|-----------|
| `Branch` | `attributes.vcs.branch` |
| `Revision` | `attributes.vcs.revision` (truncated) |
| `ID` | `data.id` |
| `Created` | `attributes.created_at` |
| `State` | `PhaseOutcomeStatus(phase, outcome, current_outcome)` |

---

## 2. GET /v3/runs/{id} — Get a single run — DONE

**Used by:** `run get` (UUID lookup), `run watch` (poll loop), `run cancel` (resolve then cancel)

### Fields consumed

| Envelope path | Type | CLI usage |
|---------------|------|-----------|
| `data.id` | UUID | display |
| `data.attributes.phase` | string | status, terminal detection |
| `data.attributes.outcome` | string (nullable) | status |
| `data.attributes.current_outcome` | string (nullable) | status |
| `data.attributes.created_at` | timestamp | display |
| `data.attributes.vcs.branch` | string | display, watch header |
| `data.attributes.vcs.revision` | string | display, SHA matching in `watch --sha` |
| `data.attributes.errors[].type` | string | error display |
| `data.attributes.errors[].message` | string | error display |
| `data.references.project.id` | UUID | project context |
| `data.references.user.id` | UUID | actor context |

### `run get` display mapping

| Display line | V3 source |
|-------------|-----------|
| `ID` | `data.id` |
| `Branch` | `attributes.vcs.branch` |
| `Commit` | `attributes.vcs.revision` |
| `Status` | `PhaseOutcomeStatus(phase, outcome, current_outcome)` |
| `Created At` | `attributes.created_at` |
| `Errors` | `attributes.errors[]` (shown when non-empty) |

`run watch` polls this until `phase == "ended"`. The `--failfast` flag
exits early when `current_outcome == "failed"` while still running.

---

## 3. POST /v3/runs — Trigger a run — MISSING (V2)

**Used by:** `run trigger`

Still calls V2 `TriggerPipeline`. No V3 trigger endpoint exists.

### What's needed

```json
{
  "data": {
    "attributes": {
      "branch": "main",
      "parameters": {"deploy_env": "staging"}
    },
    "references": {
      "project": {"id": "proj-uuid"}
    }
  }
}
```

Response: 202 with `data.id` (UUID) and `data.attributes.phase` ("accepted").
Client polls `GET /v3/runs/{id}` until phase transitions.

---

## 4. GET /v3/workflows?filter[run_id]={id} — List workflows for a run — DONE

**Used by:** `run get`, `run watch`, `run cancel`, `workflow list`

### Fields consumed per workflow

| Envelope path | Type | CLI usage |
|---------------|------|-----------|
| `data[].id` | UUID | display, pass to cancel/rerun/get-jobs |
| `data[].attributes.name` | string | display |
| `data[].attributes.phase` | string | `allWorkflowsDone()`, status display |
| `data[].attributes.outcome` | string (nullable) | status display |
| `data[].attributes.current_outcome` | string (nullable) | status display |
| `data[].attributes.created_at` | timestamp | display |
| `data[].attributes.ended_at` | timestamp (nullable) | duration computation |
| `data[].references.run.id` | UUID | back-reference |
| `data[].references.project.id` | UUID | project context |
| `data[].references.user.id` | UUID | actor context |

`allWorkflowsDone()` checks `phase == "ended"` for all workflows.

---

## 5. GET /v3/workflows/{id} — Get a single workflow — DONE

**Used by:** `workflow get`

### `workflow get` display mapping

| Display line | V3 source |
|-------------|-----------|
| `ID` | `data.id` |
| `Name` | `attributes.name` |
| `Run ID` | `references.run.id` |
| `Status` | `PhaseOutcomeStatus(phase, outcome, current_outcome)` |
| `Created` | `attributes.created_at` |
| `Stopped` | `attributes.ended_at` |

---

## 6. POST /v3/workflows/{id}/cancel — Cancel a workflow — DONE

**Used by:** `workflow cancel`, `run cancel` (cancels each non-ended workflow)

UUID-only input, ack-only response. Uses `postV3`.

`run cancel` iterates the run's workflows via `GetRunWorkflowsV3`, cancels
those with `phase != "ended"` via this endpoint.

---

## 7. POST /v3/workflows/{id}/rerun — Rerun a workflow — MISSING (V2)

**Used by:** `workflow rerun`

Still calls V2 `POST /v2/workflow/{id}/rerun` with `{"from_failed": bool}`.
No V3 rerun endpoint exists.

---

## 8. GET /v3/jobs?filter[workflow_id]={id} — List jobs for a workflow — DONE

**Used by:** `run get`, `run watch`, `workflow get`

### Fields consumed per job

| Envelope path | Type | CLI usage |
|---------------|------|-----------|
| `data[].id` | UUID | display, pass to job get/output |
| `data[].attributes.name` | string | display |
| `data[].attributes.phase` | string | `hasFailedJob()`, `failedJobNames()`, status display |
| `data[].attributes.outcome` | string (nullable) | status display |
| `data[].attributes.current_outcome` | string (nullable) | status display |
| `data[].attributes.type` | string | build vs approval distinction |
| `data[].attributes.started_at` | timestamp | in struct |
| `data[].attributes.ended_at` | timestamp (nullable) | in struct |
| `data[].references.workflow.id` | UUID | back-reference |
| `data[].references.project.id` | UUID | project context |

---

## 9. GET /v3/jobs/{id} — Get a single job — DONE

**Used by:** `job get`, `job output list` (fetches step list before output)

### Fields consumed

| Envelope path | Type | CLI usage |
|---------------|------|-----------|
| `data.id` | UUID | display |
| `data.attributes.name` | string | display |
| `data.attributes.type` | string | build vs approval |
| `data.attributes.phase` | string | status display |
| `data.attributes.outcome` | string (nullable) | status display |
| `data.attributes.started_at` | timestamp | display, duration |
| `data.attributes.ended_at` | timestamp (nullable) | display, duration |
| `data.attributes.parallel_executions[]` | array | groups steps by executor index |
| `...steps[].num` | int | step number for output fetch |
| `...steps[].name` | string | display |
| `...steps[].type` | string | step type (run/builtin) |
| `...steps[].command` | string | displayed in output list |
| `...steps[].phase` | string | status display |
| `...steps[].outcome` | string (nullable) | failed indicator |
| `...steps[].exit_code` | int (nullable) | display |
| `...steps[].started_at` | timestamp | duration |
| `...steps[].ended_at` | timestamp (nullable) | duration |
| `data.references.workflow.id` | UUID | back-reference |
| `data.references.pipeline.id` | UUID | run back-reference |
| `data.references.project.id` | UUID | project context |
| `data.references.user.id` | UUID | actor context |

---

## 10. GET /v3/jobs/{id}/stdout and /stderr — Step output — DONE

**Used by:** `job output get` (single step), `job output list` (all steps)

Replaces the private API at `/api/private/output/raw/{slug}/{number}/...`.

### Query params

| Param | Type | Notes |
|-------|------|-------|
| `filter[execution]` | int | executor index (for parallel jobs) |
| `filter[step_num]` | int | step number within the execution |

### Behaviour

- Returns raw `application/octet-stream` — streamed directly
- `X-Terminal: true` header when step has completed (enables caching)
- Supports `Range` header for partial reads
- 204 No Content when output doesn't exist
- stdout and stderr fetched in parallel via `errgroup`
- Output rendered through virtual terminal (`vt.NewEmulator`) when piped

### Commands

`job output get <job-id> <step-num>` fetches stdout+stderr for a single step.

`job output list <job-id>` calls `GetJobV3` first to get the step list,
then fetches stdout+stderr for every step in the selected execution
(bounded by `maxStepOutputFetches = 8` concurrency).

---

## 11. GET /v3/jobs/{id}/artifacts — List job artifacts — DONE

**Used by:** `job artifact`

### Fields consumed per artifact

| Envelope path | Type | CLI usage |
|---------------|------|-----------|
| `data[].attributes.path` | string | display, download destination |
| `data[].attributes.url` | string | download source |
| `data[].attributes.execution` | int | display as node index |

Download uses authenticated GET to the artifact URL.

---

## 12. GET /v3/projects?filter[slug]={slug} — Resolve project — DONE

Resolves slug → UUID at session start. UUID used for all subsequent V3 calls.
Cached per session.

---

## Remaining gaps

Only two V3 endpoints are missing:

| Endpoint | Command | Priority |
|----------|---------|----------|
| `POST /v3/runs` | `run trigger` | **P1** — core use case, currently V2 |
| `POST /v3/workflows/{id}/rerun` | `workflow rerun` | **P2** — less frequent, V2 works |

### V2 number-lookup remnant

Three commands still call `GetPipelineByNumber` when the user passes a
number instead of a UUID: `run watch`, `run cancel`, `workflow list`.
This is a UX convenience, not a V3 gap — the V3 path works once the
UUID is known. Dropping number support entirely (per remove-numbers.md)
eliminates this V2 call.

### V2 dead code

`internal/apiclient/job.go` still contains V2 types and methods (`Job`,
`JobStep`, `JobAction`, `GetJob`, `v1ProjectPath`) that have no callers.
The old `GetStepOutput`/`GetStepError` private API methods are already
removed. The `internal/logs/` package is deleted.

---

## Wins from dropping slugs and numbers

Dropping project slugs and pipeline/job/trigger numbers isn't just "not needed" —
it actively removes complexity, fragility, and entire categories of bugs.

### Slug removal wins

| What's gone | Why it's a win |
|-------------|----------------|
| `ProjectSlug` on every V2 response | Slugs are VCS-provider-coupled (`gh/org/repo`), break on renames, and embed an implicit provider assumption. V3 uses project UUIDs — stable, VCS-agnostic, one canonical identifier. |
| Slug construction from git remote | V2 required inferring `gh/org/repo` from `git remote` — fragile for forks, mirrors, SSH vs HTTPS, renamed repos. V3 resolves project by UUID via a one-time lookup from slug, then everything is UUID-addressed. |
| Dual slug formats | V2 had both `gh/org/repo` and `github/org/repo` floating around. Gone. |
| Slug in step output URLs | The private output API baked `{slug}/{number}` into the path. V3 step output (`/v3/jobs/:id/stdout`) uses job UUID — no slug needed. |

### Number removal wins

| What's gone | Why it's a win |
|-------------|----------------|
| `GetPipelineByNumber` lookup | Every number-based command needed a pre-flight API call to resolve number → UUID. That's a whole extra round-trip per invocation, and the endpoint doesn't exist in V3. Gone. |
| Number allocation on trigger | V2 trigger had to allocate a project-scoped sequence number atomically — contention point at scale. V3 trigger returns a UUID; no sequence coordination. |
| `run_number` / `job_number` / `trigger_number` fields | Three separate number namespaces, each project-scoped, each requiring sequence generators. None needed — UUIDs are globally unique without coordination. |
| Number-based URL construction | Web URLs like `/pipelines/{slug}/{number}` coupled the CLI to the web app's routing scheme. UUID-based URLs are self-contained. |
| "Latest by number" ordering hack | V2 used `MAX(number)` as a proxy for "most recent" — wrong when pipelines are re-triggered or backfilled. V3 search with `created_at` ordering is correct by definition. |
| `job_number` in display columns | `workflow get` and `run get` showed job numbers that users couldn't act on (no `circleci job` commands accepted numbers). Noise removed from output. |

### Slug + number removal combined

The V2 step output path `/api/private/output/raw/{slug}/{number}/output/{taskIndex}/{stepID}`
required **both** a slug and a job number — meaning every log fetch needed two
resolved identifiers from two different namespaces. V3 step output needs one: the job UUID.
That's the single biggest simplification for the `logs` → `job output` migration.

## What changed vs original analysis

The original analysis (written against `next` branch) identified 9 missing or
partially-missing V3 endpoints. Since then:

- **Workflows by run** (was P0 blocker) — delivered, cli#1387
- **Workflow cancel** (was "exists but not wired") — wired to V3, uses `postV3`
- **Job get with step detail** (was proposed) — delivered, cli#1382
- **Step output** (was "in progress") — delivered and CLI wired, replaces private API entirely
- **Job artifacts** — migrated to V3, no slug/number needed
- **`logs` command** — deleted, replaced by `job output get` and `job output list`
- **Job `type`** (was "not delivered") — now present in V3 wire types
- **`PhaseOutcomeStatus`** — now 3-arg exported function (phase, outcome, current_outcome)
- **Raw phase/outcome/current_outcome** — stored on domain types instead of pre-mapping to status
- **Event rename** — reverted, everything back to "run" terminology
- **`--current-branch` flag** — added to `run list`
- **`--failfast` flag** — added to `run watch`
- **V2 dead code** — `GetStepOutput`/`GetStepError`, `internal/logs/` package removed
