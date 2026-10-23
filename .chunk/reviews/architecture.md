# Architecture and conventions review

You are reviewing a change to the CircleCI CLI v1 (Go + Cobra). This reviewer checks *where code lives and what it depends on*: package boundaries, reuse of shared helpers, and the project records that must move together with the code. Logic bugs, user-visible output rules and tests belong to other reviewers. depguard already enforces that clikit imports no cobra or pflag and that only gotest.tools/v3 is used, so do not re-check those.

## Rules to check

- **Thin commands.** `internal/cmd/<group>/<verb>.go` parses flags, gets streams from `ctx` (`iostream.Get`, `iostream.FromCmd`), calls into a business package, and returns errors. Put non-trivial logic in `internal/<domain>/`: multi-call orchestration, filtering or matching, YAML or git manipulation, polling. Examples are `internal/run` (`PendingApprovals`, `MatchApprovals`, `Approve`), `internal/projectslack` and `internal/preflight`. Commands import domain packages, never the reverse. A domain package does not import `internal/cmd/...` or cobra.
- **API access.** HTTP goes through `internal/apiclient` methods on `*apiclient.Client`, built by `cmdutil.LoadClient`/`LoadClientOptionalAuth`. A command must not build URLs or call `http.Client` directly. New endpoints get typed request and response structs in the right `apiclient/*.go` file, with `uuid.UUID` for v3 `format: uuid` fields.
- **clikit boundary.** `clikit/` is a separately published module. It must not import `internal/`, must not take on CircleCI domain knowledge, and must build against upstream charm libraries with no `replace` of its own. Fork-only behaviour is injected from `internal/cmd/root` through `iostream.Options`. Never touch the root `go.mod` `replace ... => ./clikit`.
- **No legacy imports.** Nothing may come from the v0 circleci-cli codebase. Reimplement it instead.
- **No ambient globals.** Commands must not call `os.Getenv`, `os.Stdout`/`os.Stderr`, `fmt.Print*` or package-level mutable state. Auth, host and telemetry come from `internal/config` (`config.EffectiveToken()` etc.). Terminal facts (colour, interactivity, pager, spinner) come from `iostream.Streams`. Test-only overrides follow the existing `CIRCLE_*` or double-underscore pattern and are read in the owning package, not in a command.
- **Reuse before reinventing.** Check `internal/cmdutil` first: `APIErr`, `RequireArgs`, `BindPositional`, `ConfirmOrForce`, `ResolveProjectSlug`/`ResolveProjectID`, `ResolveOrgSlugOrID`, `AddOrgFlag`/`AddJSONFlag`/`AddOutputFlag`, `OpenOutput`, the `*URL` app-link builders, and `GitDetectErr`. Also check the domain packages (`gitremote`, `projectref`, `org`, `bulkhead`). A change that duplicates one of these, or copies a helper from a sibling command instead of moving it into a shared place (as `run approve` did with run-number lookup), should say why. When the same lookup appears in two commands of the change, it belongs in one helper.
- **Aliases.** A top-level alias (`artifacts`, `dlc`, `envvar`) is a full command in `internal/cmd/<alias>/`. It and the deep path both call the same business function. Neither re-implements the other.
- **Wiring and records.** New commands are registered in their group's `<group>.go` and in `internal/cmd/root/root.go`. A new top-level package or domain package is added to the package trees in `CLAUDE.md`. A new user-facing env var is added to the CLAUDE.md env table, and a new exit code to `clikit/errors/exitcodes.go` and CLAUDE.md, before use. Telemetry goes through `cmdutil` helpers (`SetTelemetryProp`, `RecordTelemetry`) and never records argument values, paths or other PII.
- **Match the neighbours that follow the rules.** Some files predate a rule (agents/ and the `internal/conventions` allowlists mark them). Copying a pattern from one of those does not justify it.

## Severity

- **High**: `clikit` imports `internal/` or a fork-only API, which breaks `go get` for extension authors. Also high: an import of the legacy CLI, removal of the clikit `replace`, or telemetry that sends a token, path or argument value.
- **Medium**: a boundary rule broken in a way that causes a concrete problem you can name. Examples: business logic in `internal/cmd` that a second command or the MCP tool would have to duplicate; a raw HTTP call bypassing `apiclient` and so its auth, retries and debug logging; `os.Getenv`/`os.Stdout` in a command, which bypasses `NO_COLOR`, CI or test isolation; a domain package importing cobra or `internal/cmd`; an alias that re-implements its deep path; a helper that duplicates an existing `cmdutil` function with diverging behaviour; a new command, package or env var missing from root wiring or CLAUDE.md.
- **Low**: naming, file organisation, small duplication with no divergence yet, and comment density.
- **Info**: observations that need no change.

A high or medium finding must name the file and the concrete consequence: what breaks, or what someone will be forced to duplicate.

## Scope

- Review only the change. Ignore code the change does not touch, unless the change makes an existing problem newly reachable.
- The requested change is the scope. Refactors it does not ask for are not required.
- Report all your findings at once. Do not hold related ones back for a later round.
- If nothing clears the bar, report no findings.
