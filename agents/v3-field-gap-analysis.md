# V3 API Field-Level Gap Analysis (numbers dropped)

Detailed analysis of exactly what fields the CLI needs from V3 endpoints,
derived from the `next` branch source code. Assumes pipeline/job/trigger
numbers are dropped entirely.

---

## 1. GET /v3/runs — List runs for a project

**Used by:** `run list`, `workflow list` (recent mode)

### Filter/query params needed

| Param | Type | Notes |
|-------|------|-------|
| `project_id` | UUID | resolve from slug client-side or accept slug |
| `branch` | string (optional) | filter by branch |
| `limit` | int | pagination cap |
| `page_cursor` | string | cursor pagination |

### Fields needed per run object

| Field | Type | CLI usage | V3 status |
|-------|------|-----------|-----------|
| `id` | UUID | everywhere | exists |
| `state` | string | list display, `deriveStatus()` | v3 has `phase` + `outcome` — needs mapping |
| `project_slug` | string | display, git-remote matching | missing |
| `created_at` | timestamp | display, duration calc | exists |
| `updated_at` | timestamp | `run get` display | missing |
| `branch` | string | list column, filter, watch header | missing |
| `revision` | string | list column (7-char prefix), SHA matching in `watch --sha` | missing |
| `trigger.type` | string | display (webhook/api/schedule) | missing |
| `trigger.actor.login` | string | display (who triggered) | missing |
| `errors[]` | `{type, message}` | `run get` errors section (config errors) | missing |

### `run list` table column → source field mapping

| Column | Source field | Notes |
|--------|-------------|-------|
| `#` | `Pipeline.Number` | **dropped** |
| `Branch` | `Pipeline.VCS.Branch` or `Pipeline.TriggerParameters.Git.Branch` | two code paths depending on trigger type |
| `Revision` | `Pipeline.VCS.Revision` or `Pipeline.TriggerParameters.Git.CheckoutSHA` | truncated to 7 chars client-side |
| `ID` | `Pipeline.ID` | UUID |
| `Created` | `Pipeline.CreatedAt` | formatted `2006-01-02 15:04 UTC` |
| `Duration` | computed: latest `PipelineWorkflowSummary.StoppedAt` minus `Pipeline.CreatedAt` | requires a **second API call per run** (`GetPipelineWorkflows`) |
| `State` | `Pipeline.State` | see note below |

### Note on `State` column

The `State` column in `run list` shows the raw `Pipeline.State` from v2,
which is almost always "created" — it reflects pipeline creation lifecycle,
not execution outcome. This is arguably broken today: a run that failed 5
minutes ago still shows "created".

`run get` works around this with `deriveStatus()` which walks workflow
statuses in priority order (failed > running > on_hold > canceled > success),
but `run list` doesn't — it just shows the raw state.

For v3, either:
- The run-level response includes a useful computed status (or phase/outcome
  that maps to one), so `run list` can display it without per-run workflow
  fetches, or
- `run list` fetches workflows per run (which it already does for duration)
  and derives status client-side — same as `run get` does today.

The first option is strongly preferred — it halves the API calls for `run list`
if the run response carries enough status to skip the workflow fetch.

### Note on state vs phase/outcome

The CLI uses v2's `state` (created/errored/setup-pending) for the run-level
state and derives execution status from workflow statuses via `deriveStatus()`
in `run/get.go`. V3's `phase`+`outcome` model might work but the mapping
needs defining. The CLI's `deriveStatus()` walks workflow statuses in priority
order (failed > running > on_hold > canceled > success) — this logic lives
client-side and just needs raw workflow statuses, not a pre-computed run status.

---

## 2. GET /v3/runs/{id} — Get a single run

**Used by:** `run get` (UUID lookup), `run watch` (poll loop via `fetchWatchState`), `run cancel` (resolve then cancel workflows)

Same fields as list above plus `errors[]` and `updated_at`. No additional
fields beyond what list returns if the list response is full (not a summary
projection).

### `run get` display → source field mapping

The run header section:

| Display line | Source field | Notes |
|-------------|-------------|-------|
| `ID` | `Pipeline.ID` | UUID |
| `Number` | `Pipeline.Number` | **dropped** |
| `Project` | `Pipeline.ProjectSlug` | e.g. `gh/CircleCI-Public/circleci-cli` |
| `Branch` | `Pipeline.VCS.Branch` or `Pipeline.TriggerParameters.Git.Branch` | two code paths |
| `Commit` | `Pipeline.VCS.Revision` or `Pipeline.TriggerParameters.Git.CheckoutSHA` | truncated to 7 chars |
| `Status` | **derived** via `deriveStatus()` from workflow statuses | not from `Pipeline.State` — see below |

The trigger section:

| Display line | Source field | Notes |
|-------------|-------------|-------|
| `Created At` | `Pipeline.CreatedAt` | formatted `2006-01-02 15:04:05 UTC` |
| `By` | `Pipeline.Trigger.Actor.Login` | username string |
| `Type` | `Pipeline.Trigger.Type` | webhook, api, schedule, etc. |

The workflows section (nested, per workflow):

| Display field | Source field | Notes |
|--------------|-------------|-------|
| Workflow name | `PipelineWorkflowSummary.Name` | section heading |
| `Status` | `PipelineWorkflowSummary.Status` | workflow-level status |

Per job within each workflow:

| Display field | Source field | Notes |
|--------------|-------------|-------|
| Job name | `WorkflowJob.Name` | left-aligned |
| Job status | `WorkflowJob.Status` | e.g. success, failed |
| Job number | `WorkflowJob.JobNumber` | **dropped** — displayed as `#38835` today |

### Note on `Status` in `run get` vs `run list`

`run get` shows a **derived** status that is computed client-side from workflow
statuses via `deriveStatus()` in `run/get.go:263`. Priority order:
errored (run-level) > failed > running > on_hold > canceled > success.

This is different from `run list` which shows the raw `Pipeline.State` (always
"created"). The derived status is what users actually care about — "success",
"failed", "running", etc.

For v3: if the run response carries a useful top-level status (or phase/outcome
that maps cleanly), both commands can use it directly. Otherwise `run get`
will continue deriving it from workflows — which requires the
`GET /v3/runs/{id}/workflows` endpoint to exist.

**Performance note:** `run watch` polls this every 5-30s (ramping interval).
Each poll fetches run + all workflows + all jobs. If the response is heavy,
consider whether a lightweight status-only variant would help.

---

## 3. POST /v3/runs — Trigger a run (MISSING)

**Used by:** `run trigger`

### Request body

| Field | Type | Notes |
|-------|------|-------|
| `project_slug` or `project_id` | string/UUID | identify the project |
| `branch` | string | branch to run on |
| `parameters` | `map[string]any` | pipeline parameters (bool/int/string values) |

### Response fields needed

| Field | Type | CLI usage |
|-------|------|-----------|
| `id` | UUID | display, could chain to `run watch` |
| `state` | string | display |
| `created_at` | timestamp | display |

---

## 4. POST /v3/runs/search — Search runs (EXISTS)

Could cover "latest on branch" — used by `run get` (no args), `run watch`
(no args), `logs --last-failed`. Needs:

- Filter by `project_id` + `branch`
- Sort by `created_at` desc
- `limit=1` for "latest" use case
- Response must include same fields as GET /v3/runs

Alternatively, GET /v3/runs with `branch` filter + `limit=1` covers this
without needing search.

---

## 5. GET /v3/runs/{id}/workflows — List workflows for a run (MISSING)

**Used by:** `run get`, `run watch`, `run list` (duration calc), `run cancel`, `workflow list`

This is the single most blocking gap — 5 of 11 CLI commands need it.

### Fields needed per workflow

| Field | Type | CLI usage |
|-------|------|-----------|
| `id` | UUID | display, pass to cancel/rerun/get-jobs |
| `name` | string | display |
| `status` | string | display, `deriveStatus()`, `allWorkflowsDone()`, `watchFingerprint()` |
| `created_at` | timestamp | available in struct, not currently displayed in summary |
| `stopped_at` | timestamp (nullable) | `workflowDuration()` — computes run wall-clock time |

---

## 6. GET /v3/workflows/{id} — Get a single workflow (EXISTS)

**Used by:** `workflow get`

### Fields needed

| Field | Type | CLI usage | V3 status |
|-------|------|-----------|-----------|
| `id` | UUID | display | exists |
| `name` | string | display | exists |
| `status` | string | display | exists |
| `pipeline_id` | UUID | display as `run_id` | unknown |
| `project_slug` | string | display | unknown |
| `started_by` | string | in struct, not displayed | unknown |
| `created_at` | timestamp | display | exists |
| `stopped_at` | timestamp (nullable) | display | unknown |

---

## 7. POST /v3/workflows/{id}/cancel (EXISTS)

**Used by:** `workflow cancel`, `run cancel` (cancels each workflow)

No field gaps. Takes UUID, returns ack.

---

## 8. POST /v3/workflows/{id}/rerun (MISSING)

**Used by:** `workflow rerun`

### Request body

| Field | Type | Notes |
|-------|------|-------|
| `from_failed` | bool | `true` = rerun only failed jobs; `false` = full rerun |

### Response

Acknowledgement (message string).

---

## 9. GET /v3/workflows/{id}/jobs — List jobs for a workflow (MISSING)

**Used by:** `run get`, `run watch`, `workflow get`

Second most blocking gap — 3 commands need it, on the polling hot path for
`run watch`.

### Fields needed per job

| Field | Type | CLI usage |
|-------|------|-----------|
| `id` | UUID | needed for `circleci logs <job-uuid>` post-numbers |
| `name` | string | display in all three commands |
| `status` | string | display, `hasFailedJob()`, `failedJobNames()`, `watchFingerprint()` |
| `type` | string | `approval` jobs rendered differently (no log suggestion) |
| `started_at` | timestamp | in struct, not displayed in list |
| `stopped_at` | timestamp (nullable) | in struct, not displayed in list |

---

## 10. GET /v3/jobs/{id} — Get a single job (EXISTS)

**Used by:** `logs` command (fetches job detail + step output)

### Fields needed

| Field | Type | CLI usage | V3 status |
|-------|------|-----------|-----------|
| `id` | UUID | identity | exists |
| `name` | string | not displayed in logs but useful | unknown |
| `status` | string | not used by logs | unknown |
| `project_slug` | string | **critical** — used to construct step output URLs | unknown |
| `started_at` | timestamp | not used by logs | unknown |
| `stopped_at` | timestamp (nullable) | not used by logs | unknown |
| `steps[]` | array | **critical** — the actual log content | unknown | 
| `steps[].name` | string | step header display | unknown |
| `steps[].actions[].index` | int | passed to step output fetch | unknown |
| `steps[].actions[].step` | int | passed to step output fetch | unknown |
| `steps[].actions[].name` | string | display | unknown |
| `steps[].actions[].status` | string | display (failed indicator) | unknown |
| `steps[].actions[].exit_code` | int (nullable) | display | unknown |
| `steps[].actions[].start_time` | timestamp | display | unknown |
| `steps[].actions[].end_time` | timestamp (nullable) | display | unknown |

### Hidden dependency: step output API

The step output is currently fetched via a private API at
`/api/private/output/raw/{slug}/{job_number}/output/{taskIndex}/{stepID}`.
Dropping job numbers means this endpoint needs a UUID-based alternative — or
the job response needs to embed output inline / provide pre-signed output
URLs. This is a dependency not visible in the v2 public API surface.

---

## 11. GET /v3/projects/{id} — Get a project (EXISTS)

The CLI operates on project slugs (`gh/org/repo`). V3 uses project UUIDs.
Either:

- V3 endpoints accept slugs alongside UUIDs, or
- The CLI resolves slug to UUID once via this endpoint and caches it

**Minimum fields needed:** `id` (UUID), `slug` (string).

---

## Target V3 response shapes

### Run object

```json
{
  "id": "uuid",
  "state": "created|errored|...",
  "project_id": "uuid",
  "project_slug": "gh/org/repo",
  "created_at": "2026-...",
  "updated_at": "2026-...",
  "branch": "main",
  "revision": "abc1234def5678...",
  "trigger": {
    "type": "webhook|api|schedule",
    "actor": { "login": "danmux" }
  },
  "errors": [{"type": "config", "message": "..."}]
}
```

### Workflow object (in list-for-run response)

```json
{
  "id": "uuid",
  "name": "build-and-test",
  "status": "running|success|failed|error|on_hold|canceled|...",
  "created_at": "2026-...",
  "stopped_at": "2026-..." | null
}
```

### Job object (in list-for-workflow response)

```json
{
  "id": "uuid",
  "name": "test",
  "status": "running|success|failed|queued|not_run|...",
  "type": "build|approval",
  "started_at": "2026-...",
  "stopped_at": "2026-..." | null
}
```

### Trigger response

```json
{
  "id": "uuid",
  "state": "created",
  "created_at": "2026-..."
}
```

---

## Missing endpoints ranked by blast radius

| Priority | Endpoint | Commands blocked |
|----------|----------|------------------|
| **P0** | `GET /v3/runs/{id}/workflows` | run get, run watch, run list, run cancel, workflow list (5) |
| **P0** | `GET /v3/workflows/{id}/jobs` | run get, run watch, workflow get (3) |
| **P1** | `POST /v3/runs` | run trigger (1, but core use case) |
| **P1** | `POST /v3/workflows/{id}/rerun` | workflow rerun (1) |
| **P2** | UUID-based step output API | logs (1, but private API dependency) |
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
