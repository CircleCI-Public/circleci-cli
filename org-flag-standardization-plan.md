# Plan: standardize the organization flag

> Addresses review feedback item #2 — "The organization flag has four names."

## Problem

The same concept — the organization — is exposed under **four different flag
names** depending on the command:

| Flag         | Value     | Where it's used today                                                                                 |
| ------------ | --------- | ----------------------------------------------------------------------------------------------------- |
| `--org`      | slug      | `context *`, `project create`, `runner open`                                                          |
| `--org`      | slug-or-UUID | `runner instance list`, `runner resource-class list` (already does the right thing)                |
| `--org-id`   | UUID      | `config validate`, `config process`, `certificate list/upload`, `signing-config create/list`, `namespace create`, `orb process`, `orb validate` |
| `--org-slug` | slug      | `config validate`, `config process` (**alongside** `--org-id` on the same command)                    |
| `--owner-id` | UUID      | every `policy` command (`decide`, `diff`, `fetch`, `logs`, `push`, `settings get/set`)                |

Users have to relearn the flag per command. The worst case is `config
validate`/`config process`, which carry **two** flags for one concept
(`--org-id` *and* `--org-slug`), and `policy`, which invents a third name
(`--owner-id`).

## Target state — one rule

**`--org` is the single, canonical organization flag on every org-scoped
command.** It accepts **either a slug (`gh/myorg`) or a UUID**, and where a Git
remote is present it defaults to that remote's org. `--org-id`, `--org-slug`,
and `--owner-id` are all **deleted outright** — the CLI is unreleased, so there
is no back-compat to preserve and no deprecation/alias period. One name, no
leftovers.

This is achievable today with zero new resolution logic: `internal/cmdutil/orgid.go`
already provides the resolver the `runner` commands use —

```go
// ResolveOrgSlugOrID resolves an organization reference to its UUID. The
// reference may be a UUID (used as-is), a slug (looked up via the API), or
// empty (inferred from the current git remote).
func ResolveOrgSlugOrID(ctx, client, ref, cmdName string) (uuid.UUID, error)
```

`runner instance list` / `runner resource-class list` are the reference
implementation — every other command should converge on the same helper and the
same `--org` flag string.

### Why not keep `--org-id` as a visible flag where "UUID is strictly required"?

The feedback proposed keeping `--org-id` where a UUID is strictly required. In
practice **no command strictly requires the literal UUID**: `ResolveOrgSlugOrID`
already accepts a UUID verbatim *and* resolves a slug to one via the API, so a
single `--org` covers both the human (slug) and machine (UUID) callers. Keeping a
second flag would re-introduce the "two names for one concept" problem we are
removing. We therefore delete `--org-id` entirely rather than keep it as a parallel
public flag. (See "Alternative considered" if we'd rather keep `--org-id` visible
on the pure-UUID commands.)

## Per-command migration

| Command(s)                                            | Today                         | After                                  | Resolver                          |
| ----------------------------------------------------- | ----------------------------- | -------------------------------------- | --------------------------------- |
| `runner instance list`, `runner resource-class list`  | `--org` (slug-or-UUID)        | **no change** (reference impl)         | `ResolveOrgSlugOrID`              |
| `context *` (10 subcommands), `project create`, `runner open` | `--org` (slug only)   | `--org` now also accepts a UUID        | `ResolveOrgSlugOrID`¹             |
| `config validate`, `config process`                   | `--org-id` **and** `--org-slug` (`-o`) | single `--org`; delete both old flags; free `-o` | `ResolveOrgSlugOrID` (replaces config's bespoke `resolveOrgID`) |
| `certificate list`, `certificate upload`              | `--org-id`                    | `--org` (delete `--org-id`)            | `ResolveOrgSlugOrID`              |
| `signing-config create`, `signing-config list`        | `--org-id`                    | `--org` (delete `--org-id`)            | `ResolveOrgSlugOrID`              |
| `namespace create`                                    | `--org-id` (required)         | `--org` (required; delete `--org-id`)  | `ResolveOrgSlugOrID`              |
| `orb process`, `orb validate`                         | `--org-id`                    | `--org` (delete `--org-id`)            | `ResolveOrgSlugOrID`              |
| `policy decide/diff/fetch/logs/push/settings get/set` | `--owner-id` (required)       | `--org` (required; delete `--owner-id`) | `ResolveOrgSlugOrID`             |

¹ The `context` and `project create` endpoints are keyed on the org **slug**, not
the UUID. For those, accepting a UUID means the new code path must resolve
UUID→slug (or call a UUID-keyed endpoint). If that reverse lookup isn't readily
available, keep these on `ResolveOrgSlug` (slug-only) for now — the **flag name is
already `--org`, so they already satisfy the feedback**; broadening them to accept
a UUID is a nice-to-have, not a blocker. Confirm the endpoint contract before
changing behavior here.

### Notable cleanups bundled in

- **`config` has a private duplicate resolver.** `internal/cmd/config/config.go`
  defines its own `resolveOrgID` (org-id takes precedence; org-slug triggers a
  `GetOrg`; logs a *warning* instead of erroring on failure). Delete it and route
  through `cmdutil.ResolveOrgSlugOrID` so config behaves like every other command
  (structured error on failure, not a silent warning).
- **`-o` short form.** `config validate`/`process` bind `-o` to `--org-slug`,
  which collides with the `-o, --output` convention used by `artifact`,
  `job artifact`, and `runner config`. Removing `--org-slug` frees `-o`. **Do not
  reassign `-o` to `--org`** — no other `--org` has a short form; keep it
  consistent. (Note: `process.go` may already have dropped the `-o` short; verify
  against `validate.go`, which still has it.)

## Back-compat / deprecation strategy

**None — clean break.** The CLI is unreleased, so the old flag names
(`--org-id`, `--org-slug`, `--owner-id`) are simply deleted, not aliased or
deprecated. No hidden flags, no transition period, no changelog "breaking" note
beyond the normal pre-release churn. The only callers are inside this repo
(commands, acceptance tests, golden help files), and they're updated in the same
change.

## Implementation steps

Ordered to keep each commit self-contained and green.

1. **Audit the resolvers** (`internal/cmdutil/orgid.go`). Confirm `ResolveOrgSlugOrID`
   covers required-vs-defaulted cases. `namespace create` and `policy` have **no**
   git-remote default and are `required` — ensure the "empty ref" branch surfaces a
   clean "required" error there rather than attempting Git detection. Add a
   `required: true` variant or have callers `MarkFlagRequired("org")`.
2. **policy** (7 command files + `policy.go`): rename `ownerID`→`org`, flag
   `owner-id`→`org`, update `MarkFlagRequired`, the `Long`/`Example` heredocs
   (currently say "Most commands require --owner-id"), and the
   `policyAPIErr`/suggestion text in `policy.go` ("Run: circleci policy fetch
   --owner-id <id>"). Route through `ResolveOrgSlugOrID`. Delete `--owner-id`.
3. **config** (`validate.go`, `process.go`, `config.go`): collapse `--org-id` +
   `--org-slug` into `--org`; delete the bespoke `resolveOrgID`; call
   `ResolveOrgSlugOrID`; fix the `--org-slug` example at `process.go:75`; remove the
   `-o` short. Delete `--org-id` and `--org-slug`.
4. **certificate**, **signing-config**, **namespace**, **orb**: `--org-id`→`--org`,
   route through `ResolveOrgSlugOrID`.
5. **context** / **project create** / **runner open**: rename stays `--org`; decide
   per footnote ¹ whether to broaden to slug-or-UUID. Update help text from
   "Organization slug (e.g. gh/myorg)" to "Organization slug or UUID (e.g.
   gh/myorg)" only if behavior actually changes.
6. **Consistent help string.** Standardize every `--org` usage line on one
   wording, e.g.:
   `"Organization slug (e.g. gh/myorg) or UUID; defaults to the current git remote"`
   (drop "defaults to…" on the required ones).

## Testing

- **Acceptance tests** that reference the old names must be updated:
  `acceptance/policy_test.go` (`--owner-id`) and `acceptance/config_test.go`
  (`--org-id`/`--org-slug`). Add cases that pass `--org` with **both** a slug and a
  UUID and assert the same resolved request hits the fake API.
- Add a test asserting the **old names are gone** — `--owner-id`/`--org-slug`/
  `--org-id` should now exit `ExitBadArguments` with "unknown flag".
- Per CLAUDE.md rule #1/#6, every touched command keeps `--json` and 3+ examples;
  update the examples to use `--org`.

## Docs & golden files

- Regenerate the help golden files — at minimum
  `internal/cmd/root/testdata/help/circleci/reference.txt` plus the per-command
  `.txt` files under `internal/cmd/root/testdata/help/circleci/{config,policy,certificate,namespace,orb,signing-config,...}`.
  These are snapshot-tested, so they must be refreshed in the same change.
- `circleci help environment` / `docs/build-plan.md`: check for prose referencing
  `--owner-id`/`--org-slug`.
- **MCP surface:** the MCP server does **not** define these as flags
  (grep of `internal/cmd/mcp` / `internal/mcp` is clean), so no tool-schema change
  is expected — but verify any tool that shells the CLI with `--owner-id`/`--org-id`.

## Alternative considered (feedback's literal reading)

Keep `--org-id` as a **visible** flag (not just an alias) on the commands where the
value is inherently a UUID and there is no Git default — `policy`, `namespace
create`, `orb process/validate`, `config` orb resolution — and rename
`--owner-id`→`--org-id` there as the feedback literally states; use `--org`
(slug-or-UUID) everywhere else. This still collapses four names to two
(`--org` + `--org-id`) and is less churn, but leaves two flags for one concept,
which is the smell we're removing. Recommended only if the UUID-keyed endpoints
genuinely cannot resolve a slug.

## Decision needed

1. **Broaden `context`/`project create` to accept a UUID**, or leave slug-only
   (already named `--org`, so already compliant — see footnote ¹)?
2. **One name (`--org`) or two (`--org` + `--org-id`)** — recommended vs. the
   alternative below.

(Back-compat is settled: clean break, no aliases — the CLI is unreleased.)
