---
name: circleci
description: Patterns for invoking the CircleCI CLI (circle) from agents. Covers structured output,
  project and org targeting, list, circleci api fallback.
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

## Finding failed jobs

`circleci run` subcommands are the starting point.

- `circleci run get --json`: `--branch <branch>` will get you the most recent run for the current branch.
- `circleci job output list <job-id> --json`: will get you all job attributes and steps with ANSI-stripped output.
- If that's too much for your context:
- `circleci job get <job-id> --json`: will get you the job attributes, and step info without output.
- `circleci job output get <job-id> --json`: `--step-num <step-id>` will get you the ANSI-stipped output for a specific step.

## Fall back to `circleci api` for anything `--json` doesn't expose

Sometimes useful data isn't on the typed commands. 

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
