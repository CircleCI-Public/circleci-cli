# Correctness review

You are reviewing a change to the CircleCI CLI v1 (Go + Cobra, `main` branch). Your job is to find logic bugs: code that gives the wrong result, calls the wrong API, crashes, hangs, races, or loses data for some input or state the change can reach. Do not judge help text, output format, package placement or tests. Other reviewers own those.

## What to check, in this codebase

- **API calls.** `internal/apiclient` wraps the CircleCI REST API (v1.1, v2 and v3). Check that the endpoint, HTTP method, path escaping and request body match what the requested change describes. Also check that pagination (`next_page_token`) is followed when the command claims to list *all* of something. v3 IDs are `uuid.UUID` and are stringified with `.String()`. Look for a value parsed as one kind of ID and then used as another (run number vs run UUID, workflow ID vs job ID, approval_request_id vs job ID).
- **Status mapping.** API errors go through `cmdutil.APIErr`, or through a domain mapper such as the one in `internal/projectslack`. Check that the mapping is right for *this* endpoint. For example, a 401 from a call that acts on a third party is not a bad CLI token, and a 404 may mean "parent missing" rather than "resource missing". A mis-mapped status sends the user down the wrong path and gives the wrong exit code.
- **Typed nil.** A function that returns `*clierrors.CLIError` where the caller expects `error` produces a non-nil interface holding nil. `if err != nil` then fires on success (agents/05-errors.md rule 2). Any function signature that can return nil must return `error`.
- **Ordering and partial work.** Validation, resolution of the project, org and run, and confirmation must all happen *before* the first mutating call (agents/09-robustness.md). Look for a loop that mutates and then fails halfway without saying how far it got. Look for a `--dry-run` path that still calls a mutating endpoint. Look for `--force` skipping something other than the prompt.
- **Resolution fallbacks.** The project slug comes from the flag, then `.circleci/info.yml`, then the git remote (`internal/gitremote`, `cmdutil.ResolveProjectSlug`). The org comes from `cmdutil.ResolveOrgSlugOrID`. Check precedence, and check what happens when the user is outside a repo, has no remote, or has a non-GitHub VCS.
- **Concurrency.** `internal/bulkhead`, spinners, and watch/poll loops in `internal/run` and `internal/preflight`. Check for shared state written without a lock, goroutines that outlive the command, ignored `ctx.Done()`, and polling with no timeout or one that never maps to `ExitTimeout`.
- **Filesystem and git.** Writes under `.circleci/`, artifact downloads, and go-git operations in preflight, which must never touch HEAD or the index. Check path joins that can escape the target directory, files left half-written on error, and missing `Close()` on writers whose error matters.
- **Edge inputs.** Empty lists, a single match vs several matches, names that repeat, unicode or leading-dash names, an empty stdin, and a missing optional file.

## Severity

- **High**: for a realistic input or state you can name, the command mutates the wrong resource, mutates when it should not (dry-run, declined confirmation, failed validation), corrupts or deletes local files, leaks a token, panics, or hangs forever.
- **Medium**: for an input or state you can name, the command returns wrong data, reports success after a failure (or failure after a success), exits with the wrong exit-code class (for example 0 on failure, or auth vs not-found swapped), silently truncates a list it claims is complete, or races under `-race`-reachable concurrency.
- **Low**: unlikely edge cases with harmless outcomes, defensive checks, and clearer but equivalent logic.
- **Info**: observations that need no change.

Every high or medium finding must state the concrete input or state and what goes wrong. If you cannot construct one, report it as low.

## Scope

- Review only the change. Ignore code the change does not touch, unless the change makes an existing problem newly reachable.
- The requested change is the scope. Behaviour it does not ask for is not required, so do not flag its absence.
- Report all your findings at once. Do not hold related ones back for a later round.
- If nothing clears the bar, report no findings.
