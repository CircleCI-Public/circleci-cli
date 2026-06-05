# UUID-Only Identifiers: Impact Summary

Moving runs and jobs from project-scoped numbers to UUIDs. This covers
the human experience, the agent experience, and the case for interactive
TUI navigation to close the usability gap.

Numbers are a pain to implement on the back end - and are only still 
included - for compatibility. We have a chance to drop them.

!! Once we support numbers - we can never go back. !!

---

## The case for UUIDs

Project-scoped numbers (`#42`, `#101`) look friendly but carry hidden
costs. They only work inside a specific project context — outside that
context they're meaningless. UUIDs are globally unique, context-free, and
work everywhere: the CLI, the API, the web UI URL bar, Slack messages,
log files, MCP tool calls.

The v3 API is UUID-native. Supporting numbers means either keeping v2
alive or adding number fields to v3 that don't naturally belong there.
Dropping them aligns the CLI with where the platform is going.

---

## Human impact

### What gets better

**One identifier everywhere.** Today, "run 75" only works in the CLI
inside the right repo directory. The same run's UUID works in the CLI,
in `curl`, in a Slack message, in a CI script, and in the web UI URL.
No more "which project was that in?"

**Fewer API round-trips.** Every number-based command currently makes an
extra call to resolve the number to a UUID. Gone.

**Simpler mental model.** No more guessing whether the CLI wants a number
or UUID. One format, one code path.

**Web UI alignment.** The web UI will also move to UUID-based URLs. Once
both surfaces use the same identifiers, cross-referencing between
terminal and browser becomes copy-paste.

### What looks different

The table below shows before/after for every affected command. Where
numbers disappear, the replacement is either a short UUID prefix (8
chars, like git's short SHA) or a contextual default that avoids needing
an identifier at all.

#### Run commands

```
BEFORE                                    AFTER
─────                                     ─────

$ circleci run list                       $ circleci run list

# Runs                                    # Runs
#   Branch   Revision  ID        ...      ID        Branch   Revision  ...
75  main     a1b2c3d   5034460f  ...      5034460f  main     a1b2c3d   ...
74  feature  e4f5678   8a2b3c4d  ...      8a2b3c4d  feature  e4f5678   ...

$ circleci run get 75                     $ circleci run get 5034460f-c7c4-4c43-9457-de07e2029e7b
                                          $ circleci run get          # ← latest run, no ID needed

$ circleci run cancel 75                  $ circleci run cancel 5034460f-c7c4-4c43-9457-de07e2029e7b

$ circleci run watch 75                   $ circleci run watch 5034460f-c7c4-4c43-9457-de07e2029e7b
                                          $ circleci run watch         # ← latest run
                                          $ circleci run watch --sha $(git rev-parse HEAD)

$ circleci run trigger                    $ circleci run trigger
Triggered run #76 (9f8e7d6c...)           Triggered run 9f8e7d6c on main
```

#### Job commands

```
BEFORE                                    AFTER
─────                                     ─────

$ circleci logs 101                       $ circleci logs b7a9c3d2-5678-4abc-def0-123456789abc
                                          $ circleci logs --last-failed   # ← no ID needed

$ circleci job artifacts 101              $ circleci job artifacts b7a9c3d2-...
                                          $ circleci artifacts            # ← latest run's artifacts

$ circleci run get                        $ circleci run get
  ...                                       ...
  Workflows:                                Workflows:
    build (success)                           build (success)
    - test    success  #101                   - test    success  b7a9c3d2
    - deploy  success  #102                   - deploy  success  c4e5f6a7
```

#### Workflow commands

```
BEFORE                                    AFTER
─────                                     ─────

$ circleci workflow list 75               $ circleci workflow list 5034460f-c7c4-4c43-9457-...
                                          $ circleci workflow list    # ← recent runs, no ID needed

$ circleci workflow get <uuid>            $ circleci workflow get <uuid>
  Run: #75                                  Run: 5034460f
```

#### Scripting

```
BEFORE                                    AFTER
─────                                     ─────

ID=$(circleci run trigger --json \        ID=$(circleci run trigger --json \
  | jq -r .id)                              | jq -r .id)
circleci run watch "$ID"                  circleci run watch "$ID"

# The scripting path is identical — scripts already use UUIDs.
```

### Addressing each concern

**"Numbers are easier to type."** True, but how often do you actually
type a run number from memory? The common flows don't need one:
- "What's my latest run?" → `circleci run get` (no args)
- "Watch my push" → `circleci run watch` or `circleci run watch --sha HEAD`
- "Why did CI fail?" → `circleci logs --last-failed`
- "Cancel the current run" → pipe from `run list`: `circleci run list --json --jq '.[0].id' | xargs circleci run cancel --force`

For the rare case where you do need a specific run, you copy-paste
from `run list` output or the web UI — which works the same for short
UUIDs as for numbers.

**"Numbers are easier to say aloud."** In conversation you'd say
"the latest run on main" or "the failing run" — not "run
5034460f." The same is true today: "run 75" only means something
if the listener knows which project and can infer recency. Short
UUID prefixes (`5034460f`) are no worse for verbal reference than
git SHAs, which developers already use daily.

**"Numbers convey ordering."** Run 76 is newer than run 75 — UUIDs
don't tell you that. But `run list` output is sorted by time, and
`run get` shows `created_at`. Ordering is a display concern, not an
identifier concern.

**"Existing scripts parse the `number` field."** This is a breaking
change for scripts that use `--json | jq .number`. The `id` field
has always been present and is the more robust choice. A migration
period where `number` is deprecated but still emitted (with a
warning) is reasonable.

**"The web UI shows `Pipeline #75`."** The web UI will be updated
to use short UUID prefixes too. This is a coordinated change, not
CLI-only.

---

## Agent and MCP impact

AI agents and MCP tool callers are the **primary beneficiaries** of
UUID-only identifiers.

### The urgency: agents are learning numbers right now

The MCP tool descriptions generated from the CLI's help text actively
teach agents to use numbers. Today, an LLM reading the tool schema for
`circleci_run_get` sees:

> "Pass a run number (e.g. 75) or UUID to look up a specific run."
> Usage pattern: `[<run-id-or-number>]`

And `circleci_run_cancel`:

> "Cancel a running CircleCI run by number or UUID."
> Usage pattern: `<run-number-or-id>`

And `circleci_job_logs`:

> "Fetch the log output for a specific job number."
> Usage pattern: `<job-number>`

And `circleci_workflow_list`:

> "Pass a run UUID or run number to list workflows for a single run.
> Run numbers are shown in 'circleci run list'."

Every agent that connects to the CircleCI MCP server today reads these
descriptions, learns that numbers are a valid input format, and starts
using them. The JSON output reinforces this — `run list --json` returns
`number` fields alongside `id` fields, and `workflow get --json` returns
`run_number`.

This is a compounding problem. As more agents are built on top of the
MCP server (Claude Code, Cursor, Windsurf, custom agent frameworks),
they'll encode number-based patterns into their tool-use behaviors,
prompt templates, and cached examples. Some will be hardcoded in
application logic. The longer numbers remain in the tool descriptions
and JSON output, the larger the installed base of agent code that
depends on them.

Removing numbers later means breaking those agents. Removing them now,
while the `next` CLI and its MCP server are pre-release, breaks nothing.
This is the window.

### How agents work today

1. Call `circleci run list --json` or use MCP tool `circleci_run_list`
2. Parse the `id` field from the JSON response
3. Pass it to a follow-up command or tool

An agent never "remembers" that run 75 is important — it parses
structured output and chains identifiers programmatically. The number
is noise; the UUID is the actual key.

### What improves for agents

**No project context needed.** Today, `circleci run get 75` requires
the agent to also determine the project slug (from git remote or
`--project` flag). With `circleci run get <uuid>`, the identifier is
self-contained. One fewer inference step, one fewer failure mode.
This matters especially for desktop agents and tools like Claude Code
agents, Cursor, Windsurf, and similar AI-powered development
environments — these often operate outside the context of a specific
repo directory. A Claude agent asked to "check why CI is failing on
the auth service" may not have the auth service repo checked out
locally. With number-based identifiers, the agent would need to clone
or navigate to the repo just to resolve `--project`. With UUIDs, the
agent can work with run and job identifiers directly, regardless of
its working directory.

**No argument ambiguity.** The current `looksLikeNumber()` heuristic
decides whether an arg is a number or UUID. An agent that passes a
numeric string gets different behavior than one that passes a UUID.
With UUID-only, there's one code path.

**MCP alignment.** The MCP server tools currently accept both numbers
and UUIDs (see [MCP tool audit](#mcp-tool-audit-what-agents-see-today)
below). Dropping numbers from the CLI means MCP descriptions and JSON
output become UUID-only — giving agents a single, unambiguous interface
instead of the current dual-format that teaches them to use numbers.

**Better MCP backend.** The CLI is used as an MCP server
(`circleci mcp start`). Simpler argument parsing and no
project-context requirement make it a cleaner tool interface for
any LLM caller.

### Agent workflow: before and after

```
BEFORE (agent must resolve project context for numbers):

  1. detect git remote → gh/org/repo
  2. circleci run list --project gh/org/repo --json
  3. extract run number 75
  4. circleci run get 75 --project gh/org/repo --json
  5. extract workflow ID
  6. circleci workflow get <workflow-uuid> --json

AFTER (UUIDs are self-contained):

  1. circleci run list --json          # project inferred or passed once
  2. extract run UUID
  3. circleci run get <run-uuid> --json
  4. extract workflow UUID
  5. circleci workflow get <workflow-uuid> --json
```

Step 4 no longer needs `--project`. The agent chains UUIDs without
maintaining project context across calls.

### MCP tool audit: what agents see today

The table below is a complete audit of MCP tool descriptions and JSON
output fields that reference numbers. These descriptions are what an
LLM reads when it connects to the CircleCI MCP server — they are the
agent's entire understanding of how to use the tool.

| MCP tool | What agents read | Problem |
|---|---|---|
| `circleci_run_get` | "Pass a run number (e.g. 75) or UUID"; arg pattern `<run-id-or-number>` | Teaches agents that numbers are valid input |
| `circleci_run_cancel` | "Cancel a running CircleCI run by number or UUID"; arg `<run-number-or-id>` | Same — number listed first |
| `circleci_run_watch` | "Pass a run number or UUID"; arg `<run-number-or-id>` | Same |
| `circleci_run_list` | JSON output includes `number` field | Agents will parse and chain `number` |
| `circleci_run_trigger` | JSON output includes `number` field; success message prints `#N` | Agents learn the number from trigger output |
| `circleci_workflow_list` | "Pass a run UUID or run number"; example `circleci workflow list 75` | Number used in the example |
| `circleci_workflow_get` | JSON output includes `run_number` field | Agents correlate workflows to runs via number |
| `circleci_job_logs` | "Fetch the log output for a specific job number"; arg `<job-number>` | Job number is the *only* accepted identifier |
| `circleci_job_artifacts` | arg `<job-number>` | Same |
| `circleci_logs` | arg `[<job-number>]` | Same |

**Tools already UUID-only (no changes needed):**
`circleci_workflow_cancel`, `circleci_workflow_rerun`,
`circleci_workflow_get` (input) — all accept `<workflow-id>` (UUID).

### What the MCP descriptions should say instead

After the UUID migration, tool descriptions should:

1. **Only mention UUIDs.** `circleci_run_get` description becomes
   "Look up a specific run by UUID" with arg pattern `<run-id>`.
2. **Remove number fields from JSON output.** `run list --json` emits
   `id` but not `number`. `workflow get --json` emits `run_id` but
   not `run_number`.
3. **Use UUID in examples.** Replace `circleci workflow list 75` with
   `circleci workflow list <run-uuid>` or show a realistic UUID prefix.
4. **Job tools accept UUIDs.** `circleci_job_logs` becomes
   "Fetch log output for a specific job by UUID" with arg `<job-id>`.

This is not cosmetic. LLMs use tool descriptions as their primary
signal for how to call a tool. If the description says "number," the
agent will use a number. If it says "UUID," the agent will use a UUID.
The MCP schema is the API contract with every AI agent that connects.

---

## Why not a TUI for human navigation?

The bubbletea/lipgloss/glamour stack is already in the CLI's
dependencies. `internal/ui/` has working components: select lists,
confirm prompts, spinners, text input, secret input. The foundation
is there.

An interactive TUI would close the usability gap that dropping numbers
creates. Instead of asking humans to type or copy UUIDs, give them
an interactive picker:

```
$ circleci run list --interactive    # or just detect TTY

  ┌─ Recent runs (main) ──────────────────────────────────────┐
  │                                                            │
  │  ▸ 5034460f  main     a1b2c3d  2m ago   success    3m12s  │
  │    8a2b3c4d  feature  e4f5678  15m ago  failed     1m44s  │
  │    c7d8e9f0  main     9876543  1h ago   success    4m02s  │
  │                                                            │
  │  ↑/↓ navigate  enter select  q quit  / filter              │
  └────────────────────────────────────────────────────────────┘

  → opens `circleci run get 5034460f-c7c4-...` for the selected run
```

### What a TUI would solve

- **No need to type identifiers at all.** Navigate with arrow keys,
  press enter. The UUID is an implementation detail the user never sees
  or types.
- **Drill-down navigation.** Select a run → see its workflows → select
  a workflow → see its jobs → select a job → see its logs. Each step
  is a bubbletea view, not a separate command invocation.
- **Filtering and search.** Type `/` to filter by branch, status, or
  author. Faster than remembering flag names.
- **Real-time updates.** `run watch` already polls — a TUI version
  could update the job status table in-place with color-coded state
  changes.

### Why the CLI doesn't have one yet

The design philosophy in `agents/01-philosophy.md` prioritises
"human-first design" and "composability as opt-in." The CLI was
built commands-first: each verb does one thing, composes via pipes
and `--json`. TUI views are a layer on top, not a replacement.

The interactivity guide (`agents/07-interactivity.md`) frames prompts
as "a first-time-user affordance" — helpful for discovery but never
required. The same principle applies to TUI navigation: it should be
an optional mode that enhances the experience, not a gate that blocks
scripting.

The practical reason is sequencing. The CLI team built the command
surface, output formatting, error handling, and JSON/JQ support first.
Those are the foundation that agents, scripts, and power users depend
on. TUI navigation is the polish layer — high value for human
experience, but not on the critical path for API parity or agent
support.

### What it would take

The components already exist in `internal/ui/`:
- `SelectModel` — arrow-key list picker (used by `auth login`)
- `SpinnerModel` — animated progress (used by `run get`, `run watch`)
- `ConfirmModel` — y/N prompts (used by `run cancel`)

A TUI navigation mode would add:
- A `list` bubbletea model that fetches runs and renders a selectable
  table (reusing `SelectModel` patterns)
- A `detail` model that shows run → workflow → job drill-down
- A `logs` model that streams step output in a scrollable view
- Wiring in the run/workflow/job commands to launch the TUI when
  stdout is a TTY and no `--json` flag is set

This is additive — no existing commands change. The TUI is an
alternative presentation of the same data, triggered by TTY
detection or an explicit `--interactive` flag.

### Recommendation

Build the TUI navigation as a follow-up to the UUID migration. The
combination of UUID-only identifiers + interactive TUI means:
- **Agents** get clean, unambiguous UUID-based commands
- **Humans** get arrow-key navigation and never need to touch a UUID
- **Scripts** get `--json` with stable UUID fields
- **Everyone** uses the same underlying identifiers

The TUI makes the UUID transition invisible to humans — they navigate
by selecting, not by typing identifiers. The short UUID prefix in
non-interactive output handles the remaining cases where a human sees
an identifier in plain text.
