# Remove pipeline/job number support from CLI

## Context

The V3 API will never support numbers — only UUIDs. The CLI currently accepts pipeline numbers (exposed as "run numbers") and job numbers as both input arguments and display fields. This plan removes number support in two phases: Phase 1 removes number-based *input* now (against the existing V2 API), Phase 2 converts to V3 endpoints once they exist and removes numbers from the wire entirely.

---

## Phase 1 — Remove number input (implementable now, V2 API)

### 1a. Remove pipeline-number lookups

Delete `GetPipelineByNumber` from `internal/apiclient/pipeline.go` and all call sites:

- **`run/get.go`**: remove `looksLikeNumber` helper, the number branch. `Use` becomes `get [<run-id>]`. Update Long/Example.
- **`run/cancel.go`**: remove number branch. `Use` becomes `cancel <run-id>`.
- **`run/watch.go`**: remove number branch. `Use` becomes `watch [<run-id>]`.
- **`workflow/list.go`**: delete numeric branch of `resolveRunArg`. UUID-only.

### 1b. Remove number display from run/workflow output

- **`run/list.go`**: drop `Number` from `runListEntry`, remove `#` column from table.
- **`run/get.go`**: drop `Number` from output structs, remove `#%d` prints.
- **`run/trigger.go`**: drop `Number` from output, change `"Triggered run #%d"` to use UUID.
- **`pipeline/run.go`**: drop `Number` from output, change `"Pipeline #%d triggered"` to use UUID.
- **`run/watch.go`**: remove all `Run #%d` and job `#%d` strings. Replace `failedJobLogSuggestions` — stop suggesting `circleci logs <number>`, suggest `circleci logs --last-failed` instead.
- **`workflow/get.go`**: drop `RunNumber`, job `Number`, remove `#%d` lines and Number table column.
- **`workflow/list.go`**: drop `RunNumber`, remove `Run #%d` headings.

### 1c. Remove explicit job-number entry points

Drop `<job-number>` positional args and `--job <number>` flags entirely. Keep inference modes (`--last-failed`, `--last-job`) which internally resolve via V2 numbers but never expose them.

- **`job/logs.go`**: remove `<job-number>` positional arg. `--last-failed`/`--last-job` only.
- **`job/artifacts.go`**: remove `<job-number>` positional arg. Inference or run-level scoping only.
- **`logs/logs.go`**: remove `[<job-number>]` positional arg. Same inference-only pattern.
- **`artifacts/artifacts.go`**: remove `--job` flag.

The internal plumbing still uses job numbers from `WorkflowJob.JobNumber` to call `GetJob`, `GetStepOutput`, `GetStepError`, `GetJobArtifacts`. This is an implementation detail hidden from users. Explicit job-level access by ID returns in Phase 2.

### 1d. Remove `Number` from API structs (hard cut from JSON output)

- `Pipeline.Number` — remove field
- `TriggerResponse.Number` — remove field
- `WorkflowDetail.PipelineNumber` — remove field
- `pipeline_definition.go` Number fields — remove from output

Keep `Job.Number` and `WorkflowJob.JobNumber` — still needed internally for V2 API calls but never exposed in command output or `--json`.

No `number` field in any JSON output from Phase 1 onwards.

### 1e. Update fakes and tests

- **Fakes**: remove `handleGetPipelineByNumber` route and handler from `fakes/circleci.go`.
- **Acceptance tests**: remove all "by number" test cases (`TestRunGet_ByNumber`, `TestRunWatch_ByNumber`, etc.), update assertions that check for `#NN` or `"number"` in output.
- **Golden files**: regenerate help/usage text for all affected commands.

---

## Phase 2 — Convert to V3 (self-paced, we own the endpoints)

Once V3 endpoints exist:

- Replace all V2 client methods with V3 equivalents
- Drop `Job.Number`, `WorkflowJob.JobNumber` from structs entirely
- Reintroduce `logs <job-id>` and `job artifacts <job-id>` accepting UUIDs
- Delete the v1.1 fallback (`getJobStepsV1`, `v1ProjectPath`)
- Switch `status` rendering to `phase`/`outcome`

---

## V3 API gap analysis — endpoints needed

### Runs (V2 "pipelines")

| Endpoint | Method | Path | Notes |
|---|---|---|---|
| Get run | GET | `/api/v3/runs/:id` | UUID only. `data.attributes`: phase, outcome, created_at, updated_at. `data.references`: project, trigger_actor. |
| List runs | GET | `/api/v3/runs?filter[project_id]=...` | Cursor pagination. Optional `filter[branch]=...`. No `number`. |
| Trigger run | POST | `/api/v3/runs` | Body: project_id, branch, parameters. Returns 202 with `data.id`. |
| Cancel run | POST | `/api/v3/runs/:id/cancel` | Verb-terminated POST. |

**Source**: query-service for GET (core transactional data); public-api-service for trigger/cancel (orchestration).

### Workflows

| Endpoint | Method | Path | Notes |
|---|---|---|---|
| Get workflow | GET | `/api/v3/workflows/:id` | `phase`/`outcome` not `status`. `data.references.run` has id only, no number. |
| List workflows | GET | `/api/v3/workflows?filter[run_id]=:id` | Or nested: `/api/v3/runs/:id/workflows`. Cursor pagination. |
| Workflow jobs | GET | `/api/v3/workflows/:id/jobs` | **Critical**: each job must have a UUID `id`. No `job_number`. |
| Cancel workflow | POST | `/api/v3/workflows/:id/cancel` | Verb-terminated POST. |
| Rerun workflow | POST | `/api/v3/workflows/:id/rerun` | Body: `{ "is_from_failed": true }`. |

**Source**: query-service for GET; public-api-service for cancel/rerun.

### Jobs (the hardest gap)

| Endpoint | Method | Path | Notes |
|---|---|---|---|
| Get job | GET | `/api/v3/jobs/:id` | UUID-addressable. Returns steps with output references. Replaces V2 `GET /project/{slug}/job/{number}` and the v1.1 fallback. |
| Job artifacts | GET | `/api/v3/jobs/:id/artifacts` | Replaces `GET /project/{slug}/{number}/artifacts`. |
| Job step output | GET | `/api/v3/jobs/:id/steps/:step_index/output` | **Replaces the private output API** (`/api/private/output/raw/{slug}/{number}/...`). Hardest gap — private API is fundamentally number-keyed. |
| Job step errors | GET | `/api/v3/jobs/:id/steps/:step_index/errors` | Same as above for stderr. |

**Source**: query-service for job detail; public-api-service for step output (aggregates from internal storage). Step-output endpoints may alternatively return both stdout/stderr, or use signed URLs from the job detail response.

### V3 design notes

- All paths use `:id` parsed via `request.PathID(c)` — UUID only
- No slugs in paths or response bodies
- `phase` (running, completed, cancelled) + `outcome` (success, failed, null) replaces `status`
- Datetime fields suffixed `_at`, durations `_ms`
- Boolean fields prefixed `is_`, `has_`, etc.
- References as entity-name-keyed objects: `"references": { "project": { "id": "..." }, "run": { "id": "..." } }`
- Cursor-based pagination via `page[limit]` and `page[cursor]`
- Minimal response fields — "if in doubt, leave it out"

---

## Verification (Phase 1)

1. `task check` — linter passes
2. `task test` — all tests pass (acceptance + unit)
3. Smoke test: `run list`, `run get <uuid>`, `run get` (latest), `logs --last-failed`
4. `--json` output has no `number` fields
5. `circleci run get 75` errors cleanly (invalid UUID, not "run not found")
6. `circleci logs 42` errors cleanly (no positional job-number accepted)
