# Rebuilding the CircleCI CLI from scratch

Every developer knows the moment: CI goes red, and you face a choice. Open the browser
and click through the web UI to the run, the workflow, the job, the step, the log line.
Or stay in the terminal, where the fix is going to happen anyway.

The new CircleCI CLI exists so you can stay. It's 1.0, it's in beta, and it's a
ground-up rewrite in Go, not an iteration on the CLI we've shipped for years. I wrote
most of it, so I'm biased, but the goal from day one was a genuinely high-quality
experience built for how developers, and now their agents, actually work.

You can install it now from [cli.circleci.com](https://cli.circleci.com) if you want to
follow along.

Here's why we started over, and what we built.

## Why a rewrite, not a refactor

The existing CLI has served developers for years, and like most long-lived tools it had
picked up its share of tech debt and usability rough edges along the way: `--json` in
some commands but not others, token-paste authentication, errors that told you what
failed but not what to do next.

We could have kept patching. What changed the calculus was a new set of tools arriving
across the whole industry: coding agents. An agent leans on exactly the things that had
grown inconsistent (predictable JSON, stable exit codes, help text it can read), and it
can't guess its way around a rough edge the way a person can. We found that making the
CLI genuinely great for agents meant rethinking the foundations it stood on, and once
you're doing that, you do it properly.

So we started clean: a new CLI written from scratch in Go, with its design rules
enforced from the first commit. That brings me to those rules.

## Design principles first, code second

We wrote the design guide before the code: a set of principle documents (output, errors,
flags, interactivity, robustness) that every command is checked against. In a sign of
the times, the guide lives in the repo's `agents/` directory, because it doubles as
instructions to the coding agents that helped build the CLI.

The philosophy in one line: **human-first, with composability opt-in**. Beautiful in a
terminal, plain and pipeable the moment you redirect output. On top of that sit a few
non-negotiable rules every command must satisfy:

1. Every data-returning command gets `--json`. No exceptions.
2. Errors are structured: what happened, why, what to try next, and where the docs are.
   Exit codes are stable and documented.
3. `circleci config` is your pipeline YAML; `circleci setting` is the CLI's own
   configuration. Never mixed again.
4. Commands nest at most two levels deep; anything that would go deeper gets a top-level
   alias.
5. Every command ships real help text and at least three real examples.
6. Telemetry is disclosed, opt-out, and disabled automatically in CI.

None of these are exciting on their own. All of them together are the difference between
a tool you fight and a tool that feels like it's on your side.

## Built for agents

Those same rules are what make the CLI legible to agents: consistent JSON, stable exit
codes, errors that say what to do next, and help text an agent can read (there's a
generated `llms.txt` too). Everything in the tour below works as well for an agent as it
does for you.

But it goes further: an MCP server is built into the binary, and one command wires it
into your editor or assistant of choice:

```shell
circleci mcp claude enable   # Claude Desktop
circleci mcp cursor enable   # Cursor
circleci mcp vscode enable   # VS Code
```

Your agent gets tools to inspect runs, jobs, step output and test results. No separate
install, no glue code.

Now Claude can find a failed job, read the logs, and push a fix, without being told
where to look.

## Login without the token paste

First contact with the old CLI was its worst moment: go to the web UI, generate a
personal API token, paste it into the terminal. The new flow is one command: pick your
host, and the CLI hands you off to the browser for OAuth. The token that comes back is
stored in your system keyring, never in a plaintext file. (Prefer a personal access
token? That flow is still there, one arrow key away.)

![circleci auth login](../demos/auth-login.gif)

## Your runs, without the browser

`circleci run list` shows recent runs for the project it infers from your git remote.
`circleci run get` gives you the full picture of a run (workflows, jobs, outcomes,
timings), and `circleci job get` drills into a single job, down to per-step durations
and exit codes.

Notice the output: it's not one flat table. Every command renders a markdown report,
with headings, lists and tables styled in the terminal, so a command can show you
structured information without pretending everything is tabular.

![circleci run list, run get and job get](../demos/run-list-get.gif)

## The TUI: day-to-day debugging, no browser tab

Run `circleci run get` with no arguments and you get the interactive experience: a run
picker with filtering by trigger, status and age, then drill straight into a run's
workflows, jobs, and steps. Page through step output and jump to what failed. The loop
that used to mean six clicks in the web UI is a few keystrokes, entirely in the
terminal.

It's built on the charm/bubbletea stack, and it replaces the web UI for most of my own
day-to-day debugging.

![circleci run get interactive TUI](../demos/run-get-tui.gif)

## `--json` and `--jq`, everywhere

Every data-returning command supports `--json`. New in this CLI: `--jq` is built in, so
your scripts don't need jq installed, and the output is colorized when you're at a
terminal and plain when you pipe it.

The names of every job that didn't succeed, in one line:

```shell
circleci run get --json --jq '.workflows[].jobs[] | select(.outcome != "success").name'
```

![circleci run get with --json and --jq](../demos/run-get-jq.gif)

## Make it yours

The rendered output is themed, and `circleci setting set theme` opens a live-preview
picker: arrow through the options and watch headings, tables and inline code restyle in
real time. Themes apply to all rendered output, not just the interactive flows.

![circleci setting set theme picker](../demos/setting-theme.gif)

## The unglamorous stuff

The details you only notice when they're missing, all present and accounted for:
`NO_COLOR` and the `PAGER` you already configured are respected; `DO_NOT_TRACK` disables
telemetry; TTY detection means output degrades sensibly when piped; colors downsample
for basic terminals; and `CI=true` implies non-interactive mode with no spinners and no
update nags.

Discovery works the same way: `circleci help getting-started` orients you,
`circleci help environment` documents every environment variable, and every error
suggests a next step. You should never need to leave the terminal to find out what the
terminal can do.

## Try it

The CLI is in beta now; head to [cli.circleci.com](https://cli.circleci.com) for install
options for every platform. If something feels rough (an error that didn't help you, a
command missing `--json`, output that broke in a pipe), that's exactly the feedback we
want; those are the standards this CLI is setting for itself. Open an issue and hold us
to them.
