# Errors

Error messages are one of the highest-value parts of a CLI. A good error message turns a frustrating dead-end into a clear path forward.

---

## Structured Error Properties

Treat errors as structured objects, not just formatted strings. A complete error has five components:

| Field | Type | Purpose |
|-------|------|---------|
| `code` | string | Machine-readable identifier (e.g. `DB_CONNECTION_FAILED`) |
| `title` | string | Short scannable label (3–8 words) — useful in logs and dashboards |
| `message` | string | Full human-readable explanation of what went wrong and why |
| `suggestions` | string[] | Ordered list of concrete next steps — one action per item |
| `ref` | URL | Link to documentation for this specific error |

The `title` field (from Thoughtworks) is distinct from the `message`: it's a short label that names the error type. This makes errors scannable in CI logs or monitoring dashboards without reading the full description.

Separating `suggestions` into an array (rather than embedding them in the message) means they display consistently, are easy to test, and can be included in JSON error output.

```
Error [DB_CONNECTION_FAILED]: Database connection refused
Could not connect to the database at localhost:5432. The database
server may not be running, or the connection URL may be incorrect.

Suggestions:
  → Check your DATABASE_URL environment variable
  → Run: myapp db start
  → Verify the database is accepting connections: myapp db status

Documentation: https://docs.myapp.com/errors/db-connection
```

---

## The Anatomy of a Good Error Message

A complete error message answers three questions:

1. **What happened?** — State clearly that an error occurred and what it was
2. **Why did it happen?** — Explain the cause in human terms, not technical jargon
3. **What should I do?** — Propose a concrete next step or fix

```
# Bad: vague, no direction
Error: failed

# Bad: technical, no direction  
Error: ECONNREFUSED 127.0.0.1:5432

# Good: specific, actionable
Error: Could not connect to the database at localhost:5432

The database server doesn't appear to be running.
Start it with: myapp db start

Or check the connection settings in: ~/.myapp/config.yml
```

---

## Rules for Error Output

### Use stderr for all errors
All error messages go to stderr. This ensures:
- Errors are visible to users even when stdout is piped
- Error messages don't corrupt piped data
- Scripts can separate data from errors

### Be explicit
Don't bury errors in verbose output. Make it clear an error occurred. Use a visual indicator when in a TTY:

```
✗ Deployment failed: build step returned exit code 1
```

### Keep stack traces hidden
Stack traces are for developers debugging, not for end users. By default, show only the human-readable error message. Provide a flag to enable full traces:

```
Error: Configuration file not found at ~/.myapp/config.yml

Run with --debug for full details.
```

With `--debug` or `DEBUG=1`:
```
Error: Configuration file not found at ~/.myapp/config.yml

FileNotFoundError: [Errno 2] No such file or directory: '/home/user/.myapp/config.yml'
  File "myapp/config.py", line 42, in load_config
    with open(path) as f:
  ...
```

---

## Machine-Readable Errors

When `--json` is passed, errors should also be machine-readable. Output a structured error object to stderr:

```json
{
  "error": true,
  "code": "CONNECTION_REFUSED",
  "message": "Could not connect to database at localhost:5432",
  "details": {
    "host": "localhost",
    "port": 5432
  }
}
```

---

## Exit Codes for Errors

Map your most important failure modes to specific exit codes and document them:

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General/unspecified error |
| `2` | Invalid arguments or usage |
| `3+` | Application-specific errors (document these) |

Example documentation:
```
Exit Codes:
  0  Success
  1  Unexpected error
  2  Invalid arguments
  3  Authentication failed
  4  Resource not found
  5  Network error
```

---

## Input Validation Errors

Catch bad input early — before doing any work — and fail fast with clear messages:

```
# Bad: cryptic after partial execution
Error: NoneType has no attribute 'id'

# Good: caught at input validation, before any work started
Error: --timeout must be a positive integer (got: "fast")
```

For typos in subcommands or flags, suggest corrections:
```
Error: unknown flag '--verbouse'

Did you mean '--verbose'?
```

---

## What Not to Do

- **Don't silently fail.** If something goes wrong, say so. Silent failures leave users confused and scripts broken.
- **Don't show raw exceptions to users.** Translate them to human-readable messages.
- **Don't output errors to stdout.** Always use stderr.
- **Don't return exit code `0` on failure.** Even if you print an error, returning `0` tells scripts everything is fine.
- **Don't be vague.** "Something went wrong" is not helpful. Be specific about what failed.

---

## Summary Checklist

- [ ] All errors output to stderr
- [ ] All failures return non-zero exit code
- [ ] Error messages answer: what, why, and what to do next
- [ ] Stack traces hidden by default; available via `--debug` or env var
- [ ] `--json` flag produces structured error output on stderr
- [ ] Important failure modes mapped to specific exit codes and documented
- [ ] Input validation happens early, before executing operations
- [ ] Typos in flags/subcommands suggest corrections
