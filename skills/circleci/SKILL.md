---
name: circleci
description: Patterns for invoking the CircleCI CLI (circleci) from agents. Covers authentication,
  structured output, project and org targeting, which command covers each v3 endpoint, circleci api
  fallback.
---

# Reference

## Interactivity policy

`circleci` already does the right thing in non-TTY contexts: it skips the pager,
strips ANSI color, and errors out fast with a helpful message instead of
prompting (e.g. `must provide --title and --body when not running interactively`).
You don't need to defensively set `CIRCLECI_PAGER` or pass `--no-pager` (no such
flag exists).

## Authentication

`circleci auth me` tells you whether there is a usable token; it fails with
`No CircleCI API token found` when there is not.

To authenticate, run `circleci auth login`. Don't ask the user for an API token
— the OAuth flow needs no secret from them, and pasting tokens around is worse
for them than a browser round-trip.

With no TTY the CLI skips its login TUI, opens the authorize page in the user's
browser, and prints the same URL to stderr. Two things follow from that:

- **Tell the user to approve the request in their browser**, and pass the
  printed URL along in case the browser never came to the front. They are
  watching you, not your tool output.
- **It blocks until they approve**, for up to 5 minutes. Run it in the
  background, or with a timeout well above your default — not in a foreground
  call that gives up after a minute.

`--no-browser` prints the URL without opening anything. `CI` and
`CIRCLE_NO_INTERACTIVE` also suppress the browser, since there is no one there
to use it; supply `CIRCLE_TOKEN` in those environments instead.

## Parsing JSON

Human output from `circleci` is markdown-formatted. If you want structured data:

- Add `--json` for structured output.
- Run a command with `--json` once to print the data, then analyze and pick what you need.
- Use `--jq '<expr>'` for filtering without piping through a separate `jq`.

## Project and organization targeting

`circleci` infers the project from the cwd's git remotes.

Pass `--org <VCS>/<ORG>` to override the resolved CWD repo, where VCS is `gh` / `bb` / `circleci`.

## Vocabulary: a pipeline execution is a "run"

One execution of a pipeline is a **run**, matching `/api/v3/runs`, so recent
pipelines come from `circleci run list`. `circleci pipeline` manages pipeline
*definitions*: which repo to check out and where the config YAML lives. (The
older v2 API called an execution a pipeline. That is the one name collision to
watch for.)

Paths below are relative to `/api/v3`, which is what `circleci api` assumes when
you give no prefix. Every command supports `--json` and `--jq`, so there is no
output-shape reason to reach for raw REST:

| What you want | Command | v3 endpoint |
| --- | --- | --- |
| Recent runs for a project | `circleci run list [--branch <b>] [--project gh/org/repo]` | `GET /runs?filter[project_id]=` |
| Your own recent runs | `circleci my runs` | `GET /runs?filter[user_id]=me` |
| One run and its workflows | `circleci run get <run-id>` | `GET /runs/{id}` |
| Workflows of a run | `circleci workflow list <run-id>` | `GET /workflows?filter[run_id]=` |
| One workflow | `circleci workflow get <workflow-id>` | `GET /workflows/{id}` |
| Jobs of a workflow | `circleci workflow get <workflow-id> --json` (`jobs[]`) | `GET /jobs?filter[workflow_id]=` |
| One job | `circleci job get <job-id>` | `GET /jobs/{id}` |
| Job step output | `circleci job output list <job-id>`, `circleci job output get <job-id>` | `GET /jobs/{id}/stdout` |
| Job artifacts | `circleci artifact <job-id>` | `GET /jobs/{id}/artifacts` |
| Test results | `circleci testresult list <job-id>` | `GET /jobs/{id}/tests` |
| Who am I | `circleci auth me` | `GET /users` |
| Pipeline definitions | `circleci pipeline list` | `GET /pipelines` |
| Triggers | `circleci project trigger list` | `GET /triggers` |
| Project env vars | `circleci envvar list` | `GET /projects/{id}/environment-variables` |
| Contexts and their env vars | `circleci context list`, `circleci context secret list <ctx>` | `GET /contexts`, `GET /contexts/{id}/env-vars` |

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

`circleci api <path>` resolves relative to `/api/v3`, so `circleci api runs`
hits `/api/v3/runs`. Surfaces with no command yet: `usage/exports`, `analysis/*`
and `metric/*` (charge, usage, job and test analytics), `notification/*`,
`provider/*` and `audit/*`.

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
