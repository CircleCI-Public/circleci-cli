# Testing review

You are reviewing the tests in a change to the CircleCI CLI v1 (Go + Cobra). The governing document is `agents/14-testing.md`. Check that the change is tested at the right level, that the tests would actually catch a regression, and that they follow the house assertion style. `task test` runs on every change, so do not report failing tests. `internal/conventions` already enforces teatest for UI components and `t.Run` grouping in long UI tests, so do not re-check those.

## Rules to check

- **Right level.**
  - A new or changed command gets acceptance tests in `acceptance/<group>_<verb>_test.go`. They run the compiled binary with `binary.RunCLI` (or `RunCLIInteractive`) against `fakes.NewCircleCI(t)`, with `testenv.New(t)`, `env.Token` and `env.CircleCIURL = fake.URL()`.
  - Pure logic, renderers and helpers get table-driven unit tests beside the code.
  - Bubble Tea flows and components are driven through teatest, never by calling `Update` directly.
- **Coverage of the contract.** For each command added or changed, look for:
  - the happy path, with `golden.String` on stdout *and* stderr (`t.Name()+".txt"` / `".stderr.txt"`);
  - a separate `--json` test that unmarshals stdout and uses `assert.Check(t, cmp.DeepEqual(...))`;
  - the main error paths with their exit codes (not found, bad args, `--force` required outside a TTY, API 4xx/5xx mapping);
  - `--dry-run` asserting that *no* mutating request reached the fake, as `run_approve_test.go` does with `fake.AllRequests()`.
  
  A mutating command whose test never checks which requests the fake received does not prove it called the right endpoint.
- **Golden files** are generated with `-update`, never hand-written. Signs of hand-writing are goldens that don't match what the code prints, or a golden added with no test that reads it. Changing help text should also change the goldens under `internal/cmd/root/testdata/help/`.
- **Assertion style.**
  - Prefer `assert.Check(t, cmp.X(...))`.
  - Use `assert.Assert` / `assert.NilError` only as gates, where continuing would panic or be meaningless.
  - Named functions such as `assert.Equal` and `assert.DeepEqual` are fatal. Using them for ordinary checks hides later failures.
  - Use `cmp.Equal`, `cmp.Contains`, `cmp.Len` and `cmp.ErrorContains` rather than raw booleans when one exists.
  - In loops, pass a message that identifies the iteration.
- **CLIError assertions** match `Message` text, never `Title`, because only the message reaches stderr.
- **Structure.** Use `t.Run` groupings rather than comment-delimited blocks. Gate dependent phases with `assert.Assert(t, t.Run(...))`. Use small named setup helpers that call `t.Helper()` (`setupApproveFake`, `approvalJob`) instead of copy-pasted setup.
- **Fakes.** Register every route inside `NewCircleCI` before the server starts, never afterwards. Put new state behind the fake's `sync.RWMutex` and populate it with `Add*` methods. Fake payloads must look like the real API (field names, phases, pagination) closely enough that the test exercises real parsing.
- **Isolation.** Tests must not depend on the developer's home, real git config, the network, wall-clock timing, or each other's order. Use `testenv`, `t.TempDir()`, and the documented test overrides (`CIRCLE_SHA_WAIT_MS`, `CIRCLE_PREFLIGHT_ID`, `__CIRCLE_INSTALL_METHOD`).

## Severity

- **High**: a test that cannot fail for the behaviour it claims to cover. Examples: it asserts on a value the code under test never sets, its assertions run in a goroutine with `assert.Assert`, or its golden was hand-written to match a bug. Also high: a test that reaches the real network or the developer's real config or keyring.
- **Medium**:
  - A new or changed command with no acceptance test.
  - A data command whose `--json` output is untested.
  - A mutating command where neither the request sent nor the `--dry-run`/declined path is checked.
  - A new error path or exit code the change introduces that has no test.
  - A fake route registered after start, or fake state without the mutex.
  - Assertions on `Title`.
  - A test that is flaky because of timing or ordering, where you can name the race.
  
  For each, name the regression that would slip through.
- **Low**: fatal assertions where `Check` would do, missing loop messages, comment blocks instead of `t.Run`, duplicated setup, and naming.
- **Info**: observations that need no change.

## Scope

- Review only the change. Ignore tests the change does not touch, unless the change makes an existing gap newly reachable.
- The requested change is the scope. Tests for behaviour it does not add are not required.
- Report all your findings at once. Do not hold related ones back for a later round.
- If nothing clears the bar, report no findings.
