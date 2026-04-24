# CircleCI CLI v2

A new CircleCI CLI built from scratch in Go + Cobra, targeting exemplary CLI design.

Full architecture, command surface, and phased roadmap: `docs/build-plan.md`

---

## Critical rules — read before writing any command

These are the six design decisions that must not be violated. They exist because the
current circleci CLI got all six wrong, and this project exists to fix them.

**1. Every data-returning command gets `--json` with field enumeration in `--help`.**
No exceptions. Consistent JSON coverage is the #1 differentiator between a scripting
tool and an interactive-only tool. Use the output helper in `internal/output`.

**2. Use the structured error type in `internal/errors`. Never `fmt.Errorf` in handlers.**
Every error must have: `code`, `title`, `message`, `suggestions[]`, `ref` (doc URL).
Exit code constants live in `internal/errors/exitcodes.go` — always use those, never raw integers.

**3. Never import from the existing circleci-cli.**
This project is a clean rewrite. Importing from the old CLI would carry forward the design
debt we are explicitly replacing. If you need similar functionality, reimplement it here.

**4. `circleci config` = pipeline YAML. `circleci settings` = CLI tool config.**
This naming is non-negotiable. `circleci config validate` validates pipeline YAML.
`circleci settings set token <value>` manages the API token. Never mix these.

**5. Maximum 2 levels of command nesting. If you go to 3, add an alias.**
`circleci context secret set` = fine (2 levels under root).
`circleci job artifacts <n>` = 3 levels → `circleci artifacts` exists as the top-level alias.
The alias is the *primary* user-facing command; the deep path is a thin wrapper that calls into
the same business logic. The alias lives in `internal/cmd/<alias>/` as a full command, not a
Cobra alias string. Four levels must never occur — restructure or alias down to 2.

**6. Every command needs `Use`, `Short`, `Long` (heredoc), and `Example` (heredoc, 3+ examples).**
Examples are "by far the most-read section of help text." Use `github.com/MakeNowJust/heredoc`
for all multi-line strings. No blank `Long` descriptions.

**7. Telemetry must be disclosed, opt-out, and auto-disabled in CI.**
On first run, print a one-time notice. Respect `CIRCLECI_NO_TELEMETRY`, `NO_ANALYTICS`,
`DO_NOT_TRACK`. When `CI=true` is set, skip the notice and disable telemetry automatically.

---

## Design guidelines

Full guidelines are in `agents/`. Start with the checklist:

```
agents/checklist.md          ← run through this before any PR
agents/01-philosophy.md      ← the 9 core principles
agents/04-output.md          ← --json, color, TTY detection
agents/05-errors.md          ← error format, exit codes
agents/06-arguments-and-flags.md ← flag naming, short forms, env vars
```

---

## Package structure

```
main.go                   Entry point. Cobra bootstrap + top-level error handling.

internal/
├── cmd/                  One package per top-level command (group or alias). Thin Cobra
│   │                     wrappers only — no business logic here.
│   ├── root/             Root command, help topics, global flags.
│   ├── artifacts/        circleci artifacts (top-level alias; primary user-facing command)
│   ├── auth/             circleci auth login/logout/status/token
│   ├── config/           circleci config validate/process/pack/generate
│   ├── context/          circleci context + circleci context secret
│   ├── job/              circleci job artifacts (deep path; wraps internal/artifacts)
│   ├── open/             circleci open (opens current project in the CircleCI web UI)
│   ├── pipeline/         circleci pipeline list/get/trigger
│   ├── workflow/         circleci workflow list/get/cancel/rerun
│   ├── orb/              circleci orb list/info/validate/publish/...
│   ├── project/          circleci project list/follow + project env
│   ├── runner/           circleci runner resource-class/token/instance
│   ├── policy/           circleci policy push/diff/fetch/...
│   ├── settings/         circleci settings list/get/set
│   └── api/              circleci api <endpoint> (raw API escape hatch)
│
├── artifacts/            Business logic for artifact listing and downloading.
│                         Pattern: non-trivial logic lives in internal/<domain>/, not in
│                         internal/cmd/. Commands import from here; never the reverse.
│
├── iostream/             TTY detection, color, stdout/stderr wiring.
│                         NEVER call os.Getenv("NO_COLOR") in a command — ask IOStreams.
│
├── errors/               Structured error type + exit code constants.
│                         exitcodes.go: ExitSuccess=0, ExitAuthError=3, ExitAPIError=4,
│                         ExitNotFound=5, ExitValidationFail=7, ExitTimeout=8
│
├── config/               Read/write ~/.config/circleci/config.yml (XDG standard).
│
├── apiclient/            CircleCI REST API client. Injected via constructor; tests pass
│                         a custom http.RoundTripper to intercept requests.
│
├── gitremote/            Detect project slug + branch from git remote URL.
│
└── testing/              Test helpers (not compiled into production binary).
    ├── binary/           BuildBinary() + RunCLI() for acceptance tests.
    ├── env/              TestEnv: isolated home dir + environment for each test.
    └── fakes/            Fake HTTP servers (Gin-based) for API endpoints.
```

Commands are thin wrappers: parse flags, get `iostream.Streams` from the command,
call into a business logic package, return errors. No global state, no `os.Stdout` writes
in production code.

---

## Environment variables

All documented at `circleci help environment`. Variables use `CIRCLECI_` prefix:

| Variable | Purpose |
|---|---|
| `CIRCLECI_TOKEN` | API token (also: `CIRCLECI_CLI_TOKEN` legacy alias) |
| `CIRCLECI_HOST` | CircleCI server host (default: `https://circleci.com`) |
| `CIRCLECI_NO_INTERACTIVE` | Suppress all prompts |
| `CIRCLECI_NO_COLOR` | Disable ANSI color |
| `CIRCLECI_SPINNER_DISABLED` | Replace animated spinner with plain text |
| `CIRCLECI_NO_UPDATE_NOTIFIER` | Suppress version update messages |
| `CIRCLECI_DEBUG` | Log HTTP requests to stderr |
| `CIRCLECI_NO_TELEMETRY` | Disable telemetry |
| `CI` | When set, implies NO_INTERACTIVE + disables spinner + update notifications |
| `NO_COLOR` | no-color.org standard — always respected |

---

## Exit codes

Defined in `internal/errors/exitcodes.go`. Document new codes there before using them.

| Code | Constant | Meaning |
|---|---|---|
| 0 | `ExitSuccess` | Command succeeded |
| 1 | `ExitGeneralError` | Unclassified error |
| 2 | `ExitBadArguments` | Invalid arguments or flags |
| 3 | `ExitAuthError` | Missing or invalid API token |
| 4 | `ExitAPIError` | CircleCI API returned 4xx/5xx |
| 5 | `ExitNotFound` | Requested resource does not exist |
| 6 | `ExitCancelled` | Operation cancelled by user (Ctrl+C) |
| 7 | `ExitValidationFail` | Config or policy validation failed |
| 8 | `ExitTimeout` | Operation timed out |

---

## Common commands

```sh
task build                             # build binary → dist/circleci
task test                              # all tests including acceptance, with -race
task lint                              # golangci-lint
task fmt                               # gosimports formatting
task mod-tidy                          # go mod tidy
goreleaser build --snapshot --clean    # test multi-platform release builds
./dist/circleci --help                 # smoke test
NO_COLOR=1 ./dist/circleci --help      # verify color is disabled
CI=true ./dist/circleci --help         # verify CI mode
```

`task test` runs unit tests (cached) then acceptance tests with `-count=1` (never cached).
Acceptance tests exec the compiled binary as a subprocess, so `go test` cannot invalidate their
cache when source files change — a stale green result is possible if caching is allowed. The
`-count=1` flag on the acceptance run prevents this.

Tools (golangci-lint, gotestsum, gosimports) are pinned via the `tool` directive in
`go.mod` and invoked as `go tool <name>` — no separate install step needed.

---

## When adding a new command

1. If business logic is non-trivial, create `internal/<domain>/` first and put it there.
   Commands import business logic packages; never the reverse.
2. Create `internal/cmd/<group>/<verb>.go` — thin Cobra wrapper:
   - `Use`, `Short`, `Long` (heredoc), `Example` (heredoc, 3+ examples)
   - Get `iostream.Streams` via `iostream.FromCmd(cmd)`
   - Parse flags, call into the business logic package, return errors
3. If the command returns data: declare a typed output struct, enumerate JSON fields in
   `Long`. Wire `--json` using `encoding/json` directly for now.
4. If the command mutates state: add `--force` for destructive ops; `--dry-run` where
   preview is useful.
5. All errors via `internal/errors` — never raw strings or `fmt.Errorf` in handlers.
6. Wire the command into `internal/cmd/<group>/<group>.go` and into `internal/cmd/root/root.go`.
7. If nesting would reach 3 levels, create a top-level alias command in `internal/cmd/<alias>/`
   that is the primary user-facing entry point. The deep path becomes a thin wrapper.
8. Add acceptance tests in `acceptance/<verb>_test.go`:
   - `TestMain` in `acceptance/acceptance_test.go` builds the binary once
   - Use `binary.RunCLI(t, args, env.Environ(), dir)` to invoke it
   - Use `fakes.NewCircleCI(t)` for a fake API server; set `env.CircleCIURL = fake.URL()`
   - Assert on `result.ExitCode`, `result.Stdout`, `result.Stderr`

## When adding a new command group

1. Create `internal/cmd/<group>/` with `<group>.go` (the group parent command).
2. Add individual verb files alongside it.
3. Register the group in `internal/cmd/root/root.go`.
4. Add `internal/<domain>/` for any shared business logic.
5. Set `RunE: cmdutil.GroupRunE` and `FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true}`
   on the group command. Without this, unknown subcommands silently show help and exit 0 (looks like
   success), and unknown flags after an unknown subcommand produce a misleading "unknown flag" error
   instead of "unknown command".

---

## Testing conventions

Acceptance tests live in `acceptance/` and run the real compiled binary against fake HTTP
servers. This catches integration issues that unit tests miss (flag wiring, exit codes,
output formatting).

**Structure of an acceptance test:**
```go
func TestXxx(t *testing.T) {
    fake := fakes.NewCircleCI(t)       // starts fake server, registers t.Cleanup
    fake.AddPipeline(id, payload)      // populate before any requests
    fake.AddStaticFile("/path", body)  // ditto for download tests

    env := testenv.New(t)              // isolated home dir
    env.Token = "testtoken"
    env.CircleCIURL = fake.URL()       // point CLI at fake

    result := binary.RunCLI(t, []string{"pipeline", "get", id}, env.Environ(), t.TempDir())

    assert.Equal(t, result.ExitCode, 0)
    assert.Assert(t, strings.Contains(result.Stdout, "expected text"))
}
```

**Fake server rules:**
- All routes are registered in `NewCircleCI` before the server starts. Never add routes
  after the server is running — gin's router tree is not safe for concurrent modification.
- `AddStaticFile` populates an in-memory map served by a pre-registered wildcard route.
- All mutable fake state is protected by `sync.RWMutex` — reads in handlers use RLock,
  writes in Add* methods use Lock.
- `BuildBinary()` returns `(string, error)`; on error `TestMain` exits 0 (skip) not 1
  (fail), so a broken build doesn't mask unrelated test failures.

---

## References

```
docs/build-plan.md                   Full architecture + phased roadmap
docs/assessments/circleci-cli.md     Assessment of the existing CLI — gaps this rewrite addresses
```
