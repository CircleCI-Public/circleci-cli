---
name: circleci
description: Patterns for invoking the CircleCI CLI (circleci) from agents. Covers structured output,
  project and org targeting, which command replaces each REST endpoint, circleci api fallback.
---

# Reference

## Interactivity policy

`circleci` already does the right thing in non-TTY contexts: it skips the pager,
strips ANSI color, and errors out fast with a helpful message instead of
prompting (e.g. `must provide --title and --body when not running interactively`).
You don't need to defensively set `CIRCLECI_PAGER` or pass `--no-pager` (no such
flag exists).

## Parsing JSON

Human output from `circleci` is markdown-formatted. If you want structured data:

- Add `--json` for structured output.
- Run a command with `--json` once to print the data, then analyze and pick what you need.
- Use `--jq '<expr>'` for filtering without piping through a separate `jq`.

## Project and organization targeting

`circleci` infers the project from the cwd's git remotes.

Pass `--org <VCS>/<ORG>` to override the resolved CWD repo, where VCS is `gh` / `bb` / `circleci`.

## Vocabulary: the REST API's "pipeline" is a "run" here

One execution of a pipeline is a **run** here, so recent pipelines come from
`circleci run list`, not `circleci pipeline list`. `circleci pipeline` manages
pipeline *definitions*: which repo to check out and where the config YAML lives.

Every command below supports `--json` and `--jq`, so there is no output-shape
reason to reach for raw REST:

| Instead of | Use |
| --- | --- |
| `GET /api/v2/project/{slug}/pipeline` | `circleci run list [--branch <b>] [--project gh/org/repo]` |
| `GET /api/v2/pipeline/{id}` | `circleci run get <run-id>` |
| `GET /api/v2/pipeline/{id}/workflow` | `circleci workflow list <run-id>` |
| `GET /api/v2/workflow/{id}` | `circleci workflow get <workflow-id>` |
| `GET /api/v2/workflow/{id}/job` | `circleci workflow get <workflow-id> --json` (`jobs[]`) |
| `GET /api/v2/project/{slug}/job/{n}` | `circleci job get <job-id>` |
| job step output / logs | `circleci job output list <job-id>`, `circleci job output get <job-id>` |
| `GET /api/v2/project/{slug}/{n}/artifacts` | `circleci artifact <job-id>` |
| `GET /api/v2/project/{slug}/{n}/tests` | `circleci testresult list <job-id>` |
| `GET /api/v2/me` | `circleci auth me` |
| `GET /api/v2/project/{slug}/envvar` | `circleci envvar list` |
| `GET /api/v2/context`, `/environment-variable` | `circleci context list`, `circleci context secret list <ctx>` |
| `GET /api/v2/projects/{id}/pipeline-definitions` | `circleci pipeline list` |
| `GET /api/v2/projects/{id}/triggers` | `circleci project trigger list` |

Run ids accept a UUID or a run number. Workflow and job ids are UUIDs, and are
printed by `circleci run get --json` and `circleci workflow get --json`.

## Finding failed jobs

`circleci run` subcommands are the starting point.

- `circleci run get --json`: `--branch <branch>` will get you the most recent run for the current branch.
- `circleci job output list <job-id> --json`: will get you all job attributes and steps with ANSI-stripped output.
- If that's too much for your context:
- `circleci job get <job-id> --json`: will get you the job attributes, and step info without output.
- `circleci job output get <job-id> --json`: `--step-num <step-id>` will get you the ANSI-stipped output for a specific step.

## Fall back to `circleci api` only when nothing above covers it

Some data has no typed command: insights, usage exports, schedules, checkout
keys and webhooks. Reach for the escape hatch there, on `api/v2` or `api/v3`.

Don't use `api/v1.1/*`: it is the legacy API, and its common uses are already
covered. Recent builds are `circleci run list`, one build is `circleci run get`,
one job is `circleci job get`.

- REST shortcuts: `circleci api 'projects/{project-id}'` or
  `circleci api 'runs?filter[project_id]={project-id}'` - note the
  `{project-id}` placeholder is filled in for you when run from a repo
  with detected remotes; pass them literally if you want determinism.

## Authentication

- `circleci auth me` prints the active host(s), user, and which env var (if
  any) is being honored.
- `circleci auth me --json` is supported.

## Other notes

- `PAGER` is honored.
- `NO_COLOR` is honored.
