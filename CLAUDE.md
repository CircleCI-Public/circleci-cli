# CircleCI CLI v1

A new CircleCI CLI built from scratch in Go + Cobra, targeting exemplary CLI design.

> **Branch context:** `main` is the active v1 rewrite. `v0` is the legacy CLI that ships
> today. These are independent codebases — `main` does not import from `v0`. All new feature
> work happens on `main`; `v0` receives only critical fixes.

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

**4. `circleci config` = pipeline YAML. `circleci setting` = CLI tool config.**
This naming is non-negotiable. `circleci config validate` validates pipeline YAML.
`circleci setting set token <value>` manages the API token. Never mix these.

**5. Maximum 2 levels of command nesting. If you go to 3, add an alias.**
`circleci context secret set` = fine (2 levels under root).
`circleci job artifacts <n>` = 3 levels → `circleci artifacts` exists as the top-level alias.
The alias is the *primary* user-facing command; the deep path is a thin wrapper that calls into
the same business logic. The alias lives in `internal/cmd/<alias>/` as a full command, not a
Cobra alias string. Four levels must never occur — restructure or alias down to 2.

**6. Every command needs `Use`, `Short`, `Long` (heredoc), and `Example` (heredoc, 3+ examples).**
Examples are "by far the most-read section of help text." Use `github.com/MakeNowJust/heredoc`
for all multi-line strings. No blank `Long` descriptions.

---

## Design guidelines

The normative design guidelines live in [`agents/`](agents/README.md). **Read the linked file
before you write the code it governs** — not after, and not only at review time. Each line below
is a trigger: if you are about to do the thing on the left, open the file on the right first.

| If you are about to… | Read first |
|---|---|
| Open a PR / finish any command | [agents/checklist.md](agents/checklist.md) |
| Write any command at all | [agents/02-basics.md](agents/02-basics.md) — the non-negotiable basics: arg parsing, exit codes, stdout vs stderr |
| Name a command, or weigh a UX trade-off | [agents/01-philosophy.md](agents/01-philosophy.md) — the 9 core principles |
| Write `Short`, `Long`, or `Example` help text | [agents/03-help-and-documentation.md](agents/03-help-and-documentation.md) |
| Print to stdout/stderr, add `--json`, use color | [agents/04-output.md](agents/04-output.md) |
| Return an error or pick an exit code | [agents/05-errors.md](agents/05-errors.md) |
| Add a flag or positional argument | [agents/06-arguments-and-flags.md](agents/06-arguments-and-flags.md) |
| Add a prompt, spinner, or TUI flow | [agents/07-interactivity.md](agents/07-interactivity.md) |
| Add or restructure a subcommand | [agents/08-subcommands.md](agents/08-subcommands.md) |
| Write or change tests | [agents/14-testing.md](agents/14-testing.md) |

Consult when the topic comes up: [agents/09-robustness.md](agents/09-robustness.md) ·
[agents/10-configuration-and-env.md](agents/10-configuration-and-env.md) ·
[agents/11-naming-and-distribution.md](agents/11-naming-and-distribution.md) ·
[agents/12-analytics.md](agents/12-analytics.md) ·
[agents/13-extensibility.md](agents/13-extensibility.md)

---

## Package structure

### Repository layout

```
cmd/circleci/main.go      Entry point. Cobra bootstrap + top-level error handling.
                          (Lives under cmd/circleci/ so `go install .../cmd/circleci`
                          produces a binary named `circleci`, not `circleci-cli`.)
acceptance/               Acceptance tests — exec the compiled binary against fake servers.
agents/                   Design guidelines (normative — see above).
share/                    Shipped man pages + bash/zsh completions.
skills/circleci/          Agent skill shipped with the CLI.
packaging/                Distribution packaging assets (deb).
tools/                    Standalone build tooling — its own Go module.
docs/                     Hugo website, blog posts, terminal demo recordings.
internal/                 Everything else (below).
```

### `internal/cmd/` — one package per top-level command

Thin Cobra wrappers only. Parse flags, get `iostream.Streams` from the command, call into a
business logic package, return errors. **No business logic here.** No global state, no
`os.Stdout` writes in production code.

Some package names are prefixed to avoid colliding with a stdlib or domain package of the same
name (`internal/cmd/config` is `package cmdconfig`; likewise `cmdauth`, `cmdenv`, `cmdonboard`).

```
internal/cmd/
├── root/                 Root command, global flags, help topics, `help reference`, man pages.
├── api/                  circleci api <endpoint> — raw API escape hatch.
├── artifacts/            circleci artifacts — top-level alias, primary user-facing command.
├── certificate/          circleci certificate upload/list/delete — iOS code signing certs.
├── cmdauth/              circleci auth login/logout/me/id/signup.
├── completion/           circleci completion <shell>.
├── config/               circleci config validate/process/pack/generate.
├── context/              circleci context create/delete/get/list/open + secret + restriction.
├── deploy/               circleci deploy init/list/open.
├── dlc/                  circleci dlc purge — top-level alias for project dlc purge.
├── env/                  circleci env subst.
├── envvar/               circleci envvar list/set/delete — alias for project env.
├── extension/            circleci extension install/remove + dispatch to circleci-* binaries.
├── job/                  circleci job get/open/output/artifacts (deep path for artifacts).
├── my/                   circleci my runs.
├── namespace/            circleci namespace create/delete/get/rename.
├── onboard/              circleci onboard — guided first-run project setup.
├── orb/                  circleci orb init/create/list/get/source/pack/process/validate/
│                         publish/diff/unlist/*-category.
├── org/                  circleci org list/setting.
├── pipeline/             circleci pipeline create/list/run.
├── policy/               circleci policy push/diff/fetch/decide/eval/logs/settings/test.
├── project/              circleci project create/get/list/follow/link/open + env/dlc/
│                         setting/trigger.
├── run/                  circleci run trigger/get/list/watch/cancel/open.
├── runner/               circleci runner resource-class/token/instance/task/config/open.
├── setting/              circleci setting list/set/unset — CLI tool config (see rule 4).
├── setup/                circleci setup — hidden, legacy v0 compatibility only.
├── signingconfig/        circleci signing-config create/list/delete.
├── step/                 circleci step halt.
├── testresult/           circleci testresult get/list.
├── version/              circleci version.
└── receivetelemetry/     Hidden. Background subprocess that forwards events to Segment.
```

`circleci mcp` is **not** in this tree — it is generated from the Cobra command tree by
`ophis` and wired up in `root.go`. Adding a command automatically exposes it as an MCP tool.

### `internal/<domain>/` — business logic

Non-trivial logic lives here, not in `internal/cmd/`. Commands import these packages; never
the reverse.

```
internal/
├── artifacts/            Listing and downloading job artifacts.
├── configcmd/            Backs circleci config validate/process/pack.
├── configgen/            Renders pipeline YAML from a reposcan.Result and writes it to disk.
├── deployinit/           Scans and patches .circleci/config.yml to add deploy marker steps.
├── extension/            Plugin mechanism — discovers circleci-* binaries on PATH.
├── githubapp/            CircleCI GitHub App install detection + browser install flow.
├── gitremote/            Resolves project slug + branch. Prefers .circleci/info.yml, then
│                         falls back to parsing the git remote URL.
├── iossigning/           Reads and base64-encodes certs / provisioning profiles.
├── onboarder/            Orchestrates the local onboarding flow.
├── orbinit/              Scaffolds a new orb project from the Orb-Template repo.
├── org/                  Shared organization operations (slug/ID resolution).
├── pack/                 Merges a directory tree of YAML files into one document.
├── projectref/           Reads/writes .circleci/info.yml — the per-checkout project record
│                         that survives repository renames and standalone projects.
├── reposcan/             Detects language stack, container image, and setup commands.
├── run/                  Run-level operations (watch, failure context).
└── testrunner/           Runs the test command detected by reposcan.
```

### `internal/` — platform and shared infrastructure

```
internal/
├── cmdutil/              Shared command helpers: AddJSONFlag/WriteJSON, API client
│                         construction, project resolution, app URLs, GroupRunE, telemetry.
├── iostream/             TTY detection, color, themes, spinner, stdout/stderr wiring.
│                         NEVER call os.Getenv("NO_COLOR") in a command — ask IOStreams.
├── errors/               Structured error type + exit code constants.
│                         exitcodes.go: ExitSuccess=0, ExitAuthError=3, ExitAPIError=4,
│                         ExitNotFound=5, ExitValidationFail=7, ExitTimeout=8
├── config/               Read/write ~/.config/circleci/config.yml (XDG standard).
├── keyring/              OS keychain storage for the API token.
├── apiclient/            CircleCI REST API client. Injected via constructor; tests pass
│                         a custom http.RoundTripper to intercept requests.
│                         v3 endpoints type id fields (format: uuid in the spec) as
│                         uuid.UUID, not string — e.g. RunV3.ID, ProjectRef.ID/OrgID.
│                         Stringify with .String() at call sites that need a string.
├── httpcl/               Minimal HTTP client with JSON defaults and retries.
├── oauth/                Client side of the CircleCI OAuth 2.0 Authorization Code + PKCE
│                         flow, with Pushed Authorization Requests.
├── browser/              Opens a URL in the user's browser, or prints it as a fallback.
├── ui/                   Bubble Tea prompts, selects, and flows (login, orb init, run get,
│                         run filter, theme picker). components/ = reusable widgets;
│                         theme/ = colour tokens.
├── jsoncolor/            ANSI-colorized, indented JSON writer.
├── mdtable/              GitHub-Flavored Markdown table builder.
├── termrender/           Replays captured terminal output for docs/demos.
├── jq/                   jq expression evaluation over JSON strings.
├── telemetry/            Segment event sender + background delegate + receiver/.
├── agent/                Detect() — identifies the calling AI agent / MCP host so telemetry
│                         attributes tool-call subprocesses correctly.
├── bulkhead/             Runs a slice of work with bounded parallelism.
└── closer/               io.Closer error-handling helper for deferred Close().
```

### `internal/testing/` — test helpers

Not compiled into the production binary.

```
internal/testing/
├── binary/               BuildBinary() + RunCLI() for acceptance tests.
├── env/                  TestEnv: isolated home dir + environment for each test.
├── fakes/                Fake HTTP servers (Chi-based) for API endpoints.
├── fakesegment/          Fake Segment service for telemetry assertions.
├── httprecorder/         Records HTTP requests for later assertion.
│   ├── chirecorder/      Middleware wiring a recorder into Chi routers used in fakes.
│   └── httpnetrecorder/  Same, for plain net/http handlers.
└── logger/               Request-logging middleware for fake servers.
```

---

## Environment variables

**Run `circleci help environment` for the authoritative user-facing list.** That topic — defined
in `internal/cmd/root/help_topic.go` — is the source of truth, so adding a user-facing variable
means updating it, not just this table. Own variables use the `CIRCLE_` prefix; where a
cross-tool standard already exists (`NO_COLOR`, `DO_NOT_TRACK`, `PAGER`, XDG) we honour it.

Never read these with `os.Getenv` from a command. Auth/host/telemetry go through
`internal/config`; anything about the terminal (color, interactivity, spinner, pager) goes
through `iostream.Streams`.

**User-facing** — this table mirrors `circleci help environment`, in the same order:

| Variable | Purpose | Read in |
|---|---|---|
| `CIRCLE_TOKEN` | API token. Takes precedence over the stored token. (`CIRCLE_CLI_TOKEN` = legacy alias, checked second) | `config.EffectiveToken()` |
| `CIRCLE_HOST` | CircleCI host (default `https://circleci.com`). Takes precedence over the stored host | `config.EffectiveHost()` |
| `NO_COLOR` | no-color.org standard — always respected. Same effect as `--no-color` | `iostream` |
| `CIRCLE_NO_COLOR` | Disable ANSI color, same as `NO_COLOR` | `iostream` |
| `CIRCLE_NO_INTERACTIVE` | Suppress all interactive prompts | `iostream` |
| `CI` | Set by CI systems. Implies non-interactive (so: no prompts, no spinner) **and** disables telemetry | `iostream`, `config` |
| `CIRCLE_SPINNER_DISABLED` | Replace the animated spinner with plain text | `iostream` |
| `CIRCLE_NO_PAGER` | Print long output inline instead of through a pager | `iostream` |
| `PAGER` | Pager program for long output. Unset → built-in scrollable viewer; `cat` or empty → paging off | `iostream` |
| `CIRCLE_NO_TELEMETRY` | Disable telemetry | `config` |
| `NO_ANALYTICS` | Disable telemetry (cross-tool standard) | `config` |
| `DO_NOT_TRACK` | Disable telemetry (cross-tool standard) | `config` |

**Read from the environment but not documented in the help topic:**

| Variable | Purpose | Read in |
|---|---|---|
| `CIRCLECI` | Set inside a CircleCI job. Suppresses the `TERM=dumb` color opt-out, because CircleCI sets `TERM=dumb` on every job but its log viewer renders ANSI fine | `iostream` |
| `TERM` | `TERM=dumb` disables color — except inside CircleCI (see `CIRCLECI` above) | `iostream` |
| `XDG_CONFIG_HOME` | Config dir base (default `~/.config`) → `<base>/circleci/config.yml` | `config` |
| `XDG_DATA_HOME` | Data dir base (default `~/.local/share`) → `<base>/circleci/extensions` | `config` |

**Set by the CLI for child processes — read these, don't set them:**

| Variable | Purpose |
|---|---|
| `CIRCLE_MCP` | Set to `1` on the `mcp start`/`mcp stream` server so `agent.Detect()` reports `mcp/<agent>` in every tool-call subprocess. Scoped to the server commands so `mcp <editor> enable` isn't misattributed |
| `CIRCLE_TOKEN`, `CIRCLE_HOST`, `CIRCLE_TELEMETRY_ENABLED`, `CIRCLE_VCS_TYPE`, `CIRCLE_PROJECT_USERNAME`, `CIRCLE_PROJECT_REPONAME`, `CIRCLE_PROJECT_ID`, `CIRCLE_BRANCH`, `CIRCLE_DEFAULT_BRANCH` | Passed to extension subprocesses (`internal/extension/manifest.go`). Project/branch values are best-effort — absent outside a linked checkout |
| `__CIRCLE_TELEMETRY_WRITE_KEY`, `__CIRCLE_TELEMETRY_ENDPOINT` | Passed to the hidden `receive-telemetry` subprocess. Double-underscore = internal, never user-set |

**Development and test overrides — not documented to users:**

| Variable | Purpose |
|---|---|
| `CIRCLE_DEBUG` | `CIRCLE_DEBUG=1` logs HTTP requests to stderr (`internal/apiclient`) |
| `CIRCLE_TELEMETRY_ENDPOINT` | Redirect telemetry to a fake Segment server |
| `CIRCLE_EXTENSION_HOST` | Override the extension registry host |
| `CIRCLE_ORB_TEMPLATE_URL` | Override the Orb-Template source for `orb init` |
| `CIRCLE_LOGIN_TIMEOUT` | Duration string overriding the `auth login` browser-flow timeout |
| `CIRCLE_SHA_WAIT_MS` | Shortens how long `run watch` waits for a SHA to appear |

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

## Commit messages

Write plain imperative subject lines. Do **not** use conventional commit prefixes (`feat:`, `fix:`, `chore:`, etc.).

Good: `Implement config validate/process/pack`
Bad: `feat(config): implement config validate/process/pack`

---

## Common commands

`task <name>` runs a Taskfile target. Run `task --list-all` to see every target; the ones you
will use day to day:

```sh
task test        # all tests (unit + acceptance) across ./..., with -race -count=1, via gotestsum
task check       # all static checks: lint, license headers, mod-tidy, release-check
task fix         # auto-fix what `task check` flags: fmt, license, mod-tidy, lint --fix
task build       # build the circleci binary to dist/circleci
```

`task test` passes `-count=1`, so results are never cached. This matters for acceptance tests:
they exec a freshly-built binary as a subprocess (`internal/testing/binary` runs `go build` at
test time), and `go test` only cache-keys on the acceptance package's direct Go imports — not on
everything that went into that binary — so without `-count=1` a change to a package the binary
uses but the test does not import could leave a stale green result. `task test` avoids that
for you; no need to add the flag yourself.

**Running a subset.** Everything after `--` is passed through to `go test` (the `CLI_ARGS`
var). Use this to target one package or one test instead of the whole tree:

```sh
task test -- ./internal/config/...             # one package
task test -- -run TestValidate ./...           # one test by name
task test -- -update ./acceptance/...          # regenerate golden files (never hand-write them)
task test -- -update ./internal/cmd/root/...   # refresh the help/usage goldens after changing any help text
```

That last one matters here: `internal/cmd/root/testdata/help/` holds a golden `--help` capture
for every command in the tree, so touching a `Short`/`Long`/`Example` or adding a flag updates
dozens of files. Run it with `-update` and commit the regenerated goldens alongside the change.

**Smoke tests** (compile-and-run without building a binary):

```sh
go run ./cmd/circleci --help             # basic smoke test
NO_COLOR=1 go run ./cmd/circleci --help  # verify color is disabled
CI=true go run ./cmd/circleci --help     # verify CI mode (non-interactive, no telemetry)
```

Dev tools (golangci-lint, gotestsum, gosimports) are pinned via the `tool` directive in
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

	// soft assertions for maximum discovery with assert.Check
    assert.Check(t, cmp.Equal(result.ExitCode, 0))
    assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
    assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
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
