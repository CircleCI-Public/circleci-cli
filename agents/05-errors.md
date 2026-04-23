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

## Implementation Patterns

### Two-tier error model

Every error returned from a command carries two perspectives:

1. **Developer error** — the wrapped `error` chain for logs and debugging.
2. **User-facing message** — a plain-English sentence shown on stderr.

Implement with a struct that satisfies both `error` and a set of
display interfaces:

```go
type userError struct {
    msg        string // brief user-facing headline
    detail     string // optional clarification
    suggestion string // optional actionable hint
    err        error  // underlying Go error (for errors.Is / As)
}

func (e *userError) Error() string       { return e.err.Error() }
func (e *userError) UserMessage() string  { return e.msg }
func (e *userError) Detail() string       { return e.detail }
func (e *userError) Suggestion() string   { return e.suggestion }
func (e *userError) Unwrap() error        { return e.err }
```

The display interfaces (`UserMessage`, `Detail`, `Suggestion`) are checked
via type assertion at the top-level error handler — they are not imported
as a named interface. This keeps the error type private to the `cmd` package.

### Single error-rendering boundary

All formatting happens in `main()`, never inside command handlers:

```go
func main() {
    if err := rootCmd.Execute(); err != nil {
        msg, detail, suggestion := errorDetails(err)
        fmt.Fprint(os.Stderr, ui.FormatError(msg, detail, suggestion))
        os.Exit(1)
    }
}
```

`errorDetails` probes the error for the three display interfaces via duck
typing, then falls back to sensible defaults (the raw `.Error()` string
as the detail, pattern-matched hints as the suggestion).

**Rules:**
- Command handlers must never call `ui.FormatError` or print styled error
  text themselves. Return the error; let the boundary format it.
- Never use a sentinel "silent" error to suppress output. Every non-nil
  error produces output through the single boundary.
- Helpers like `notAuthorized(action, err)` can inspect an error and
  return a `*userError` (or nil to signal "not my error"). The caller
  chains them:
  ```go
  if ue := notAuthorized("sync files", err); ue != nil { return ue }
  ```

### Typed package-level errors instead of string matching

API client packages export sentinel errors and typed error structs so
callers use `errors.Is` / `errors.As` instead of string matching:

```go
var ErrTokenNotFound = errors.New("api token not found")

type StatusError struct {
    Op         string
    StatusCode int
}
```

HTTP client packages must not leak a shared HTTP error type to callers.
Instead they wrap it into their own `StatusError` via a `mapErr` helper.

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
- [ ] Two-tier error model: developer chain + user-facing message
- [ ] Error formatting happens at a single boundary (main), not in handlers
- [ ] API packages export sentinel errors and typed error structs
