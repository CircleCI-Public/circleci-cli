# Output

**Rules**

1. **Detect the TTY, and check stdout and stderr independently.** Human formatting
   when a terminal is attached; plain, unanimated output when it is not. In this
   repo that decision belongs to `iostream.Streams`, never a raw `os.Stdout` check.
2. **Every data-returning command gets `--json`**, pretty-printed, on stdout.
3. **`--json` suppresses all human output**, errors included — the only thing on
   stdout is the JSON.
4. **Report state changes.** An action command that changes something must say what
   changed; silence is only golden for scripts.
5. **Colour carries meaning, never decoration**, and is disabled when there is no
   TTY, when `NO_COLOR` is set, when `TERM=dumb`, or when `--no-color` is passed.
6. **Progress indicators go to stderr and only inside a TTY** — a re-rendering bar
   turns a CI log into a Christmas tree.
7. **Don't go silent for 10+ seconds.** Periodic status beats an unexplained pause.
8. **Make cross-boundary actions explicit** — file writes and network calls the
   user didn't ask for.
9. **Keep output grep-parseable** for commands whose job is to display data.
10. **Timestamps: RFC 3339 in UTC for JSON, local time for humans.** A `--json`
    field holds a `time.Time` (normalised with `.UTC()`) or a
    `cmdutil.JSONTime` string — never a display string. Human output goes through
    `cmdutil.DisplayTime` / `DisplayTimeSeconds` / `DisplayTimeString`, which honour
    `TZ` and name the zone. Never write a layout with a literal `UTC` or `Z` in it.

---

Output design is where the human-first principle becomes most visible. The goal is output that's clear to humans by default, but degrades gracefully for machines.

---

## The Core Rule: Detect the TTY

The most important output decision is whether a human is reading the output. Check whether stdout is a TTY (interactive terminal) and adapt accordingly:

- **TTY present:** Format for humans — colors, tables, progress bars, friendly text
- **No TTY (piped/redirected):** Format for machines — plain text, no color, no animations

Most languages have a standard way to check this:
```python
import sys
is_tty = sys.stdout.isatty()
```
```go
import "golang.org/x/term"
isTTY := term.IsTerminal(int(os.Stdout.Fd()))
```
```js
const isTTY = process.stdout.isTTY
```

Check stdout and stderr **independently** — a user might pipe stdout while still wanting colored errors on stderr.

---

## Output Formats

### Default: Human-readable
By default, output should be formatted for humans. Use tables, aligned columns, friendly labels.

```
NAME          STATUS     LAST DEPLOYED
production    running    2 hours ago
staging       stopped    3 days ago
preview       running    10 minutes ago
```

### `--json`: Machine-readable structured data
Provide `--json` for structured output when the data is complex. Pretty-print it (indented). This is widely supported by tooling like `jq` and web services via `curl`.

```sh
myapp list --json --jq '.[] | select(.status == "running") | .name'
```

Output:
```json
[
  {
    "name": "production",
    "status": "running",
    "lastDeployed": "2024-01-15T14:30:00Z"
  }
]
```

### Complete log suppression with `--json`

When `--json` is active, **all human-readable output should be suppressed** — not just reformatted. The only thing on stdout should be the JSON object. No progress messages, no status lines, no confirmations.

This makes the output reliably parseable: a script consuming JSON never has to strip noise. Errors should also be structured as JSON when `--json` is active:

```json
{
  "error": true,
  "code": "CONNECTION_REFUSED",
  "message": "Could not connect to database",
  "suggestions": ["Check DATABASE_URL", "Run: myapp db start"]
}
```

### `--plain` / `--terse`: Plain tabular output
If human-readable formatting (colors, special characters, table borders) breaks machine parsing but JSON is overkill, offer `--plain` (general convention) or `--terse` (Heroku ecosystem convention) for simple tab- or space-delimited output that's easy to pass to `grep`, `awk`, or `cut`.

```sh
myapp list --plain
# production  running  2024-01-15T14:30:00Z
# staging     stopped  2024-01-12T09:00:00Z
```

### `-q` / `--quiet`: Suppress non-essential output
Allow users to suppress informational messages (progress, status) without redirecting stderr to `/dev/null`. Output that matters (the actual result) should still appear.

```sh
myapp deploy production -q
# No "Deploying..." progress — just the result or error
```

---

## What to Output on Success

Traditionally, successful UNIX commands produce no output ("silence is golden"). This optimizes for scripting but leaves humans confused.

**Modern approach:** Output something meaningful, but keep it brief:
- Confirm what happened: `Deployed to production in 23s`
- Report any important state changes
- Suggest what to do next when it's helpful

Don't output nothing for operations that change state — users need confirmation.

---

## Communicating State Changes

When your command modifies state, report what changed so the user can build a mental model:

```
# Bad — silent success
$ myapp deploy production
$

# Good — confirms what happened
$ myapp deploy production
✓ Deployed v1.4.2 to production (23s)
  → https://app.example.com
```

If your command manages complex state that isn't visible in the filesystem, provide an easy way to inspect it:
```sh
myapp status       # like `git status`
myapp show <name>  # inspect a specific resource
```

---

## Suggesting Next Steps

After completing an action, suggest the natural next steps. This helps users learn and discover functionality:

```
✓ Created project "my-app"

Next steps:
  cd my-app
  myapp dev       # start development server
  myapp deploy    # deploy when ready
```

Use judgment — don't suggest next steps for every command, only where it adds real value.

---

## Color

### Use color intentionally
Color should highlight important information or indicate status — not decorate:

```
✓ Build succeeded       ← green checkmark
✗ Tests failed (3)      ← red X
⚠ 2 warnings           ← yellow warning
```

Overusing color dilutes its meaning. If everything is colored, nothing is highlighted.

### When to disable color

Disable color automatically when:

| Condition | Action |
|-----------|--------|
| stdout/stderr is not a TTY | Disable |
| `NO_COLOR` env var is set (any non-empty value) | Disable |
| `COLOR=false` env var is set | Disable (Heroku ecosystem convention — honor for compatibility) |
| `TERM=dumb` | Disable |
| `--no-color` flag is passed | Disable |

Optionally support a program-specific variable: `MYAPP_NO_COLOR=1`.

The `NO_COLOR` standard ([no-color.org](https://no-color.org)) is widely respected — always honor it. `COLOR=false` is a Heroku-specific convention; supporting it improves compatibility in Heroku-adjacent environments.

---

## Timestamps

A timestamp has two readers, and they want different things.

**JSON is for programs.** Emit RFC 3339 in UTC (`2026-01-15T14:30:45Z`) — the
format every language parses without a custom layout. Keep the full precision the
API gave you; don't round to the minute because the table does. In Go, put a
`time.Time` in the output struct and assign it with `.UTC()`, or use
`*time.Time` with `omitempty` for a time that may be absent. Where the field is a
string — a zero time must render as `""`, or the API handed the value over as a
string — use `cmdutil.JSONTime`.

**Human output is for people**, who think in their own zone. Render through the
`cmdutil.Display*` helpers: they convert to `time.Local`, which Go sets from `TZ`
(on Unix natively; on Windows, which Go would otherwise leave on the registry zone,
`cmd/circleci/tz_windows.go` applies it), and they print the zone abbreviation (`2026-01-15 09:30 EST`), so a reader never
has to guess which zone a column is in. Use `DisplayTime` (minutes) in tables and
`DisplayTimeSeconds` in single-resource detail views.

**Don't share one formatted string between the two.** The bug this rule exists for:
a struct field held `t.Format("2006-01-02 15:04 UTC")`, the table printed it, and
`--json` printed it too. JSON got a string no parser accepts, and the table ignored
`TZ`. Keep the struct typed and format at the point of rendering.

**Never hard-code the zone in a layout.** `"...15:04 UTC"` or `"...15:04:05Z"` is
only correct while the value happens to be in UTC; the moment it isn't, the output
is wrong and says it's right.

Acceptance tests pin `TZ=UTC` in `internal/testing/env`, so goldens don't depend on
the machine. To test a zone, set `env.Extra["TZ"]`.

---

## Output Command vs. Action Command

It helps to think about your commands in two categories:

**Output commands** exist to display data — `list`, `show`, `status`. Prioritize clean, consistently structured output that's grep-parseable. Users will pipe this to `grep`, `awk`, or `jq`.

**Action commands** exist to change state — `deploy`, `create`, `delete`. Prioritize confirming what happened: what changed, the result, any relevant URLs or next steps. The output is less about data and more about communication.

As a rule of thumb: if a user would pipe the output, it's an output command. If a user reads it and moves on, it's an action command.

> **Grep-parseability** is a useful design target for output commands: a user should be able to extract what they need with a simple `grep foo`, even from human-formatted output. Full awk-parseability (perfectly aligned columns) is a nice-to-have, not a requirement.

---

## Animations and Progress Bars

Progress indicators are valuable for long operations, but must be disabled outside a TTY, and should always be routed to **stderr** (not stdout). Spinners and progress bars are "out-of-band" information about a running task — sending them to stderr ensures they don't corrupt piped data on stdout.

A critical but often overlooked point: **silence during a long operation creates user anxiety.** When a command runs for 10+ seconds with no output, users can't tell if it's working or hung. Even a simple periodic status message — `Still working...` or `Building (step 2/4)...` — transforms the experience from uncertain to reassuring:

```python
if sys.stdout.isatty():
    show_progress_bar()    # → stderr, animated
else:
    # In CI or piped output, print periodic status lines (no animation)
    print("Processing...", file=sys.stderr)
    print("Still working (this may take a few minutes)...", file=sys.stderr)
```

In CI logs, a progress bar that re-renders in place creates a "Christmas tree" of output — hundreds of lines of partial progress bars. Always check for TTY before animating.

---

## Symbols and Emoji

Symbols and emoji can increase clarity when used sparingly:
- `✓` or `✔` for success
- `✗` or `✘` for failure  
- `⚠` for warnings
- `→` for redirects or next steps

Keep usage consistent and minimal. Symbol-heavy output looks cluttered and loses its communicative value.

---

## Cross-Program Actions

When your command takes actions that cross program or system boundaries, make them explicit:

- Reading or writing files that weren't explicitly passed as arguments (beyond internal caches)
- Making network requests to remote services

Users should never be surprised that their tool silently wrote to a file or called an external API.

---

## Summary Checklist

- [ ] TTY detection used to adapt output format
- [ ] Human-readable output by default
- [ ] `--json` flag for machine-readable output
- [ ] `--plain` flag if human formatting would break scripting
- [ ] `-q` / `--quiet` flag for suppressing non-essential output
- [ ] State changes reported clearly
- [ ] Color disabled when: not a TTY, `NO_COLOR` set, `TERM=dumb`, `--no-color` passed
- [ ] No animations outside of a TTY
- [ ] Cross-program actions (file writes, network calls) made explicit to user
- [ ] Timestamps: RFC 3339 UTC in JSON, `cmdutil.Display*` (local, zone named) in human output
