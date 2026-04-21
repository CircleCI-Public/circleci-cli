# Robustness

Robustness is both objective (the tool actually handles edge cases correctly) and subjective (the tool *feels* solid). Users should never be surprised by unexpected behavior.

---

## Validate Input Early

Check arguments and flags before beginning any work. Don't start a long operation only to fail on something that could have been caught upfront.

```sh
# Bad: fails halfway through after doing work
$ myapp migrate --database foo --output /read-only-dir
Migrating 1,842 records...
Error: permission denied: /read-only-dir/output.sql

# Good: catches the problem before starting
$ myapp migrate --database foo --output /read-only-dir
Error: cannot write to /read-only-dir — permission denied
```

**Validate:**
- Required arguments are present
- Flags have valid values (correct type, within range, from allowed set)
- Referenced files/directories exist and have correct permissions
- External dependencies (connections, credentials) are available

---

## Fail Fast and Clearly

When something is wrong, stop early. A partial operation is often worse than no operation — it can leave the system in an inconsistent state.

```sh
# Bad: continues after error, leaves partial state
Processing file 1 of 10...
Processing file 2 of 10...
Error: file 3 not found
Processing file 4 of 10...  ← why continue?

# Good: stop and report
Processing file 1 of 10...
Processing file 2 of 10...
Error: file 3 not found — aborting

2 of 10 files processed. Run with --skip-missing to continue past missing files.
```

---

## Make Operations Idempotent

Where possible, running the same command twice should be safe:

```sh
myapp deploy production    # deploys
myapp deploy production    # checks current state, redeploys only if needed (or is a no-op)
```

Idempotency is especially important for:
- Setup and initialization commands (`myapp init`)
- Install/configure operations
- Any operation that might be retried after a failure

If an operation can't be made idempotent, document this clearly and require confirmation.

---

## Handle Signals Properly

### SIGINT (Ctrl+C)
Allow users to interrupt long operations. When interrupted:
1. Stop the operation cleanly
2. Clean up any partial state (temp files, incomplete writes)
3. Exit with a non-zero exit code

```sh
$ myapp import large-file.csv
Importing 50,000 records...
[████████░░░░░░░░░░░░] 4,231/50,000
^C
Interrupted. 4,231 records were imported before cancellation.
To resume, run: myapp import large-file.csv --skip 4231
```

### SIGTERM
Handle graceful shutdown for containerized or orchestrated environments. SIGTERM is the polite "please stop" signal — respond by finishing in-flight work and cleaning up.

### SIGPIPE
Handle broken pipes gracefully. When output is piped to a command that exits early (`head`, `grep -m 1`), don't print an error — just exit cleanly.

```sh
myapp list | head -5   # should work cleanly, not print "broken pipe" error
```

---

## Sensible Defaults

The command should work well with minimal configuration. Every required configuration that can have a sensible default should have one.

```sh
# Requires no configuration to try out
myapp init        # uses sensible defaults for everything
myapp serve       # serves on localhost:8080 by default
```

Document defaults clearly in help text so users understand what they're getting without explicitly configuring.

---

## Handle Edge Cases

Don't crash on unusual but valid inputs:
- Empty files or empty directories
- Files with unusual names (spaces, unicode, leading dashes)
- Missing optional files (fall back to defaults gracefully)
- Large inputs (don't load everything into memory if streaming is possible)
- Read-only file systems

Test these explicitly — they're easy to overlook and frustrating when they cause failures in production.

---

## Configuration Files

Support configuration files so users can set persistent defaults. See [10-configuration-and-env.md](./10-configuration-and-env.md) for full details.

The priority order should be (highest to lowest):
1. CLI flags (most specific, most intentional)
2. Environment variables
3. Project-level config file (e.g., `.myapp.yml` in current directory)
4. User-level config file (e.g., `~/.myapp/config.yml`)
5. Built-in defaults

---

## Formal Command and Flag State

Use explicit state markers rather than informal deprecation warnings embedded in description text. This creates consistent behavior that's easy for users to recognise and for tooling to detect:

**Command state:**
- `beta` — shows `[BETA]` in help; signals the API may change
- `deprecated` — shows `[DEPRECATED]` in help; warns on every use

**Flag deprecation:**
```
Warning: --old-flag is deprecated and will be removed in v3.0.
Use --new-flag instead.
```

Deprecation options should always include:
- **What replaces it** — the new flag or command to use
- **When it will be removed** — the version number
- **Migration command** — a concrete example of the new syntax

```
# Old
myapp export --format csv

# Deprecated flag warning
Warning: --format is deprecated (removed in v3.0). Use --output-format instead.
  Example: myapp export --output-format csv
  Docs: https://docs.myapp.com/migration/v3
```

---

## Don't Break Backward Compatibility Lightly

Scripts and pipelines depend on your CLI's behavior. Breaking changes should be:

- **Intentional:** Don't accidentally change output format or flag semantics
- **Communicated:** Announce deprecations with warnings before removing
- **Gradual:** Deprecate → warn → eventually remove (over multiple versions)
- **Documented:** Maintain a changelog with explicit migration guidance

When you need to change behavior:
```sh
# Add deprecation warning before removing
$ myapp --old-flag value
Warning: --old-flag is deprecated and will be removed in v3.0.
Use --new-flag instead: myapp --new-flag value

Documentation: https://docs.myapp.com/migration
```

---

## Test Error Paths

Most testing focuses on the happy path. Robustness comes from testing failures:

- Missing required files
- Permission errors
- Network failures and timeouts
- Invalid input values
- Partial failures mid-operation
- Concurrent invocations

---

## Summary Checklist

- [ ] Input validated before any work begins
- [ ] Fail fast on errors — don't continue after failures leave partial state
- [ ] SIGINT (Ctrl+C) handled — cleans up and exits with non-zero code
- [ ] SIGTERM handled for graceful shutdown
- [ ] SIGPIPE handled — no error message when piped to early-exiting command
- [ ] Operations idempotent where possible
- [ ] Sensible defaults for all required configuration
- [ ] Edge cases handled: empty files, unicode filenames, large inputs
- [ ] Flag precedence order: flags > env vars > config file > defaults
- [ ] Deprecation warnings added before removing features
- [ ] Error paths explicitly tested
