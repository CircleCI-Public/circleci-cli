# Rename "run" to "event"

## Context

The V3 API is dropping the run entity (design doc:
circleci/public-api-service#1023). Its responsibilities move to the
**event** — the record of a trigger firing, which groups workflows and
carries VCS context, parameters, and pre-workflow errors.

The CLI adopts this vocabulary now as a **straight rename**: the `run`
command group becomes `event`. No restructuring — workflow stays as it
is, command shapes stay as they are.

**Vocabulary:**
- **Pipeline** = definition only (config source + checkout). Never an
  execution instance.
- **Event** = an execution instance (what V3 currently calls "run",
  V2 calls "pipeline").
- **Workflow / Job** = unchanged.

---

## Command surface

### `circleci run` → `circleci event`

| Old | New | Notes |
|---|---|---|
| `run list` | `event list` | rename only |
| `run get` | `event get` | rename only |
| `run trigger` | `event create` | matches `POST /events`; same flags/behavior |
| `run cancel` | `event cancel` | rename only |
| `run watch` | `event watch` | rename only |
| `run open` | `event open` | rename only |

No `run` alias kept — `next` is an unreleased rewrite.

### `pipeline run` → removed, folds into `event create`

Firing is not a pipeline operation (the pipeline owns config location
only). `event create` absorbs both trigger paths:

- `event create --definition <id|name>` — definitions API
  (`TriggerPipelineRun`), with `--branch/--tag/--parameter` — the
  current `pipeline run` behavior.
- `event create` (no definition) — legacy V2 project trigger
  (`TriggerPipeline`) on the current branch — the current
  `run trigger` behavior.

`pipeline create` / `pipeline list` stay untouched.

### `circleci workflow` — terminology only

Commands and shapes unchanged. Only:
- JSON field `run_id` → `event_id` (`workflow list` recent mode,
  `workflow get`)
- Help text / display: "run" → "event" (e.g. `workflow get` shows
  `Event: <uuid>` instead of `Run: <uuid>`)
- `resolveRunID` → `resolveEventID`

---

## Internal renames

### `internal/apiclient/run.go` → `event.go`

| Old | New |
|---|---|
| `RunV3` | `Event` |
| `RunError` | `EventError` |
| `GetRunV3` | `GetEvent` |
| `SearchRunsV3` | `SearchEvents` |
| `RunSearchParams` | `EventSearchParams` |
| `BuildRunFilter` | `BuildEventFilter` |

Wire paths **unchanged** (`GET /runs/:id`, `POST /runs/search`) — the
events API is still a proposal. Comment each call with its target
endpoint (`GET /events/:id`, `POST /events/search`) so the flip is
mechanical when the server ships.

### `internal/apiclient/workflow.go`

- `WorkflowV3.RunID` → `EventID`
- `GetRunWorkflowsV3` → `GetEventWorkflows` (wire stays
  `GET /workflows?filter run_id=`; comment target `event_id=`)

### `internal/run/` → `internal/event/`

`Cancel(ctx, client, eventID)` — logic unchanged, `ErrNothingToCancel`
message wording updated.

### `internal/cmd/run/` → `internal/cmd/event/`

All files move; package `run` → `event`. Throughout:
- `Use`/`Short`/`Long`/`Example` text: run → event
- Error codes: `run.not_found` → `event.not_found`, `run.timeout` →
  `event.timeout`, `run.interrupted`, `run.cancel_*`, `run.sha_not_found`
  etc. all get the `event.` prefix
- User-facing messages ("Watching run %s" → "Watching event %s", etc.)
- `trigger.go` → `create.go`, absorbing `pipeline run` logic per above

### Registration

- `internal/cmd/root/root.go`: `cmdrun.NewRunCmd()` →
  `cmdevent.NewEventCmd()`
- `internal/cmd/pipeline/pipeline.go`: drop `newRunCmd()` registration;
  delete `internal/cmd/pipeline/run.go`

Numbers still resolve via V2 `GetPipelineByNumber` (`event get 75`,
`event cancel 75`, `event watch 75`) — terminology change only.

---

## Tests and docs

1. `acceptance/run_test.go` → `acceptance/event_test.go`; update all
   invocations (`run list` → `event list`, ...), assertions on output
   text and error codes
2. `acceptance/pipeline_test.go`: `pipeline run` tests move to
   `event create --definition` coverage in event_test.go
3. `acceptance/workflow_test.go`: `run_id` → `event_id` JSON
   assertions, display text
4. CLAUDE.md: package structure (`internal/cmd/event/`,
   `internal/event/`), command references
5. Repo-wide grep for "run" as an execution noun in help text and
   error messages (careful: "running", "rerun", cobra's `RunE` are fine)

---

## Key files

**Rename (git mv):**
- `internal/cmd/run/` → `internal/cmd/event/` (run.go → event.go,
  trigger.go → create.go, rest keep names)
- `internal/apiclient/run.go` → `internal/apiclient/event.go`
- `internal/run/` → `internal/event/`
- `acceptance/run_test.go` → `acceptance/event_test.go`

**Modify:**
- `internal/apiclient/workflow.go` — EventID, GetEventWorkflows
- `internal/cmd/workflow/{list,get,workflow}.go` — event_id, help text
- `internal/cmd/root/root.go` — registration
- `internal/cmd/pipeline/pipeline.go` — drop run subcommand
- `acceptance/{workflow,pipeline}_test.go`
- `CLAUDE.md`

**Delete:**
- `internal/cmd/pipeline/run.go` (logic absorbed by event create)

---

## Branch

Work on `dm/drop-run-concept` (based on `next`) in worktree
`.claude/worktrees/drop-run-concept`. Update the committed plan file
`tasks/drop-run-concept.md` to match this version.

---

## Verification

- `task check` and `task test` pass
- `circleci event list|get|create|cancel|watch|open` all work; output
  identical to the old run commands modulo wording
- `event create --definition` covers old `pipeline run` behavior;
  `event create` covers old `run trigger`
- `workflow list --json` / `workflow get --json` emit `event_id`
- `circleci run` and `circleci pipeline run` → unknown command
- No help text uses "run" as a noun for an execution
