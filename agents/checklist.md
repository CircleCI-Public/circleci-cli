# CLI Design Checklist

A quick-reference checklist for agents writing a new CLI. Work through each section during implementation. For deeper guidance on any item, see the linked topic file.

---

## Foundations → [02-basics.md](./02-basics.md)

- [ ] Using an argument parsing library (not hand-rolled)
- [ ] Exit code `0` on success, non-zero on all failures
- [ ] Primary output (data) goes to **stdout**
- [ ] Errors and messages go to **stderr**
- [ ] stdin handled gracefully — no hanging when stdin is a TTY with no input

---

## Help and Documentation → [03-help-and-documentation.md](./03-help-and-documentation.md)

- [ ] Concise help shown automatically when required args are missing
- [ ] Full help available via `-h` and `--help`
- [ ] `-h` is *only* used for help — never repurposed for another flag
- [ ] Help text leads with practical examples
- [ ] All flags documented with their default values
- [ ] Support path (issue tracker URL) included in top-level help
- [ ] Typos in subcommands/flags suggest corrections
- [ ] Link to web docs included in help

---

## Output → [04-output.md](./04-output.md)

- [ ] TTY detection used — format differs for human vs. piped output
- [ ] `--json` flag for machine-readable structured output on stdout
- [ ] `--plain` flag if human formatting would break machine parsing
- [ ] `-q` / `--quiet` flag to suppress non-essential output
- [ ] State changes reported (don't silently succeed without confirmation)
- [ ] All implicit steps (file writes, network calls, config changes) surfaced to user
- [ ] Long-running operations provide progress feedback — silence creates user anxiety
- [ ] Color disabled when: no TTY, `NO_COLOR` set, `COLOR=false` set, `TERM=dumb`, `--no-color` passed
- [ ] No animations/progress bars outside of a TTY
- [ ] Spinners and progress indicators routed to stderr (not stdout)
- [ ] Network calls and non-obvious file writes are disclosed to the user

---

## Errors → [05-errors.md](./05-errors.md)

- [ ] All errors output to **stderr**
- [ ] All failures return non-zero exit code
- [ ] Error messages answer: what happened, why, and what to do next
- [ ] Errors carry structured fields: `code`, `title`, `message`, `suggestions[]`, `ref` (doc URL)
- [ ] Stack traces hidden by default; enabled via `--debug` or env var
- [ ] `--json` produces structured error output when requested
- [ ] Key failure modes mapped to specific exit codes and documented
- [ ] Input validation happens *before* starting operations (fail fast)

---

## Arguments and Flags → [06-arguments-and-flags.md](./06-arguments-and-flags.md)

- [ ] Positional args for primary inputs; flags for modifiers/options
- [ ] Argument count: 1 is fine, 2 is questionable, 3+ is a redesign signal
- [ ] Both short (`-v`) and long (`--verbose`) forms for common flags
- [ ] Short flags can be grouped: `-abc` = `-a -b -c`
- [ ] Both `--flag=value` and `--flag value` syntax work
- [ ] Flag description text: lowercase, concise, no trailing period
- [ ] Flag types validated at parse time (integers, URLs, file paths, enums)
- [ ] Flag relationships declared explicitly (exclusive, dependsOn, exactlyOne)
- [ ] Sensitive flags (tokens, passwords) support stdin reading (`--flag -`)
- [ ] `--` passthrough convention supported for commands that wrap child processes
- [ ] Env var mapping declared alongside each flag definition
- [ ] Standard flag names followed:
  - `-h` / `--help`
  - `-V` / `--version`
  - `-v` / `--verbose`
  - `-q` / `--quiet`
  - `-f` / `--force`
  - `-n` / `--dry-run`
  - `-o` / `--output`
- [ ] All flag defaults documented in help text
- [ ] Flag values validated early with clear error messages
- [ ] Basic functionality works without flags
- [ ] Boolean flags use `--no-flag` to disable defaults

---

## Interactivity → [07-interactivity.md](./07-interactivity.md)

- [ ] Destructive operations ask for confirmation in a TTY
- [ ] Confirmation defaults to "No" — `[y/N]`
- [ ] `--force` / `-f` bypasses confirmation for scripting
- [ ] `--dry-run` available for previewing destructive operations
- [ ] Missing required inputs: prompt in TTY, error with clear message when not
- [ ] Sensitive inputs (passwords, tokens) are masked
- [ ] Non-interactive mode achievable (`--force`, `CI=true`, etc.)
- [ ] Prompts treated as a first-time-user affordance; every prompt has a flag equivalent

---

## Subcommands → [08-subcommands.md](./08-subcommands.md)

- [ ] Root command lists available subcommands
- [ ] Each subcommand has its own `--help` text
- [ ] Consistent naming convention (e.g., all verbs, or all nouns)
- [ ] Common aliases provided (`ls` for `list`, `rm` for `delete`)
- [ ] Most-used subcommands listed first in help
- [ ] Nesting kept shallow (one level preferred, two levels max)
- [ ] `help <subcommand>`, `<subcommand> --help`, and `<subcommand> -h` all work

---

## Robustness → [09-robustness.md](./09-robustness.md)

- [ ] Input validated before any work begins
- [ ] Operations don't continue after encountering errors that leave partial state
- [ ] SIGINT (Ctrl+C) handled — cleans up and exits with non-zero
- [ ] SIGTERM handled gracefully
- [ ] SIGPIPE handled silently (no "broken pipe" error when piped to `head`)
- [ ] Operations are idempotent where possible
- [ ] Sensible defaults for all common configuration
- [ ] Deprecation warnings added before removing features
- [ ] Deprecated commands/flags carry explicit state marker with version + migration guidance

---

## Configuration and Environment → [10-configuration-and-env.md](./10-configuration-and-env.md)

- [ ] Config priority: flags > env vars > project config > user config > defaults
- [ ] Config file location(s) documented
- [ ] `--config` flag to specify alternate config file
- [ ] `myapp config` subcommand for viewing/setting configuration
- [ ] Env vars use `MYAPP_SETTING` naming (uppercase, prefixed)
- [ ] Standard env vars respected: `NO_COLOR`, `TERM`, `CI`, `HOME`, `EDITOR`
- [ ] All env vars documented in help text
- [ ] Credentials never in plain config files in VCS

---

## Naming and Distribution → [11-naming-and-distribution.md](./11-naming-and-distribution.md)

- [ ] Command name is lowercase
- [ ] Name uses hyphens for word separation (`my-tool`, not `myTool`)
- [ ] Name doesn't shadow common UNIX commands
- [ ] `--version` / `-V` implemented
- [ ] Semantic versioning used
- [ ] Multiple installation methods documented
- [ ] Direct binary downloads available for major platforms
- [ ] Shell completion scripts provided (bash, zsh, fish)

---

## Analytics → [12-analytics.md](./12-analytics.md)

- [ ] Telemetry clearly documented (what, how, where)
- [ ] `MYAPP_NO_TELEMETRY=1` disables all collection
- [ ] `NO_ANALYTICS=1` and `DO_NOT_TRACK=1` also respected
- [ ] No PII collected (no file paths, arg values, IPs)
- [ ] Telemetry is asynchronous and non-blocking
- [ ] Telemetry errors silently ignored

---

## Quick Self-Test

Before shipping, test these scenarios manually:

```sh
# Does help work?
myapp --help
myapp -h
myapp help
myapp <subcommand> --help

# Does it handle no args gracefully?
myapp

# Does piping work?
myapp list | grep foo
myapp export --json | jq '.'

# Does it handle bad input?
myapp --invalid-flag
myapp unknownsubcommand
myapp <subcommand> --invalid-value foo

# Does color/animation turn off?
myapp list | cat           # should have no ANSI codes
NO_COLOR=1 myapp list      # should have no color

# Do exit codes work?
myapp do-thing; echo $?    # 0 on success
myapp bad-thing; echo $?   # non-zero on failure

# Does Ctrl+C work cleanly?
myapp long-operation        # interrupt with Ctrl+C — should clean up
```
