# Runner smoke tests

These tests run the compiled `circleci` binary against the **live** CircleCI API,
covering the management cycle for a self-hosted runner: create a resource class,
generate an agent config, claim as an agent, read everything back, tear it all
down.

They complement `acceptance/runner_test.go`, which covers the same commands
against a fake server. The fake proves the CLI's own wiring; these prove the CLI
and the runner API still agree.

Excluded from `task test` by the `smoke` build tag, and skipped unless the
credentials below are set, so nothing here runs by accident.

## Running them

Export three variables and run one task:

```sh
export CIRCLE_TOKEN=<personal API token>
export CIRCLE_SMOKE_NAMESPACE=<runner namespace>
export CIRCLE_SMOKE_PROJECT=<project slug to trigger, e.g. gh/my-org/my-repo>

task test:smoke
```

| Variable | What it is |
|---|---|
| `CIRCLE_TOKEN` | Your personal API token, from `app.circleci.com/settings/user/tokens`. It must be able to **administer** runners in the target organization, not merely view them: the suite creates and deletes resource classes and tokens. |
| `CIRCLE_SMOKE_NAMESPACE` | The runner namespace, with no VCS prefix (`my-org`, not `gh/my-org`). Every resource class is created inside it. |
| `CIRCLE_SMOKE_PROJECT` | The project whose pipeline the queued-task test triggers, as `gh/org/repo`. Its committed `.circleci/config.yml` must declare a `runner_resource_class` pipeline parameter (see below). |

Anything unset makes the tests that need it skip, with a message naming the
variable, rather than fail:

- No `CIRCLE_TOKEN` or `CIRCLE_SMOKE_NAMESPACE`: the whole suite skips.
- No `CIRCLE_SMOKE_PROJECT`: the queued-task claim test skips; everything else runs.

### One optional variable

| Variable | When you need it |
|---|---|
| `CIRCLE_SMOKE_READONLY_NAMESPACE` | A namespace this token can **read but not administer**. It covers the insufficient-permissions failure path, which is otherwise the one acceptance criterion the suite leaves untested. Needs no second account: for an engineer whose own organization is the target, `circleci-runner` usually works, since viewing runners there is generally allowed and administering them is not. The subtest asserts both that the namespace is readable and that the create is denied, so if your grants differ the failure is telling you the variable is wrong rather than reporting a product bug. Without it, that subtest skips. |

Nothing is read from a config file or a git remote: every command is given its
namespace or resource class explicitly, and each run uses an isolated temporary
home directory.

To run one group:

```sh
task test:smoke -- -run TestRunnerSmokeFailures ./smoke/...
```

### Which namespace to point it at

Any namespace you are willing to have `cli-smoke-*` resource classes created and
deleted in, **and in which your token can administer runners**. Read access is not
enough: `resource-class create` needs the admin permission, and a denial arrives as
a 404 that the CLI reports as "No runner resource found", which reads like a
missing resource rather than a missing permission.

A personal organization is fine and is what this suite was developed against.
Pointing it at a shared namespace such as `circleci-runner` requires the runner
admin permission on that organization for every person expected to run it --
being a member is not sufficient.

Two things to know before choosing one:

- The orphan sweep deletes **any** `cli-smoke-*` resource class in the namespace older
  than an hour, not only ones it can prove it created. Do not point it at a
  namespace where a real runner's resource class might be named `cli-smoke-...`.
- Concurrent runs are safe: every resource class is named
  `cli-smoke-<unix-seconds>-<pid>`, and the sweep never deletes one younger than an
  hour, so it cannot reclaim a running suite's fixtures.

If this suite is ever wired into CI, a dedicated organization with a bot token
becomes worth it.

## Cleanup

Resource classes are named `cli-smoke-<unix-seconds>-<pid>`. Every run:

1. Sweeps `cli-smoke-*` resource classes in the namespace older than an hour, left
   behind by an interrupted earlier run.
2. Registers deletion of its own resource class with `t.Cleanup` before creating
   it, so an assertion failure or `t.Fatal` still reaps it.

`resource-class delete` always calls the API's force endpoint, which deletes the
class's tokens before the class, so tokens need no separate cleanup. The non-force
endpoint refuses while any token exists, so that is load-bearing rather than a
convenience. `t.Cleanup` does not run on `Ctrl-C` or a `go test -timeout`
kill; step 1 is what covers those, on the next run.

## The queued-task claim test

`TestRunnerSmoke` proves a token authenticates as an agent, which is a
`204 No Content` claim against a resource class with no work.
`TestRunnerSmokeClaimQueuedTask` proves the whole dispatch path: it queues a real
job **on the resource class it just created** and claims it, which is a `200` with
a task token.

The resource class is per-run, so the project's config cannot name it. It arrives
as a pipeline parameter instead, and the config interpolates it:

```yaml
version: 2.1

parameters:
  runner_resource_class:
    type: string
    default: ""

jobs:
  hello:
    machine: true
    resource_class: << pipeline.parameters.runner_resource_class >>
    steps:
      - run: echo "hello world"

workflows:
  say-hello:
    jobs:
      - hello
```

Point `CIRCLE_SMOKE_PROJECT` at that project and the test needs nothing else. It
creates `<namespace>/cli-smoke-<unix-seconds>-<pid>`, triggers a run on `main`
passing that name as the parameter, and claims the task the job queues.

Because the resource class is the same per-run name the rest of the suite uses,
concurrent runs cannot collide and the orphan sweep can reclaim it if a run is
interrupted.

If the config does not declare the parameter, the run ends immediately with a
config error naming the undeclared parameter, and the test reports it within
seconds rather than waiting out its claim timeout.

The full agent lifecycle it exercises:

| Step | Endpoint | Credential |
|---|---|---|
| Claim the task | `POST /api/v3/runner/claim` | resource-class token from the generated config |
| Read the task id | `GET /api/v2/task/config` | the task token from the claim |
| Hand the task back | `POST /api/v3/runner/unclaim` | task id + task token |

**Unclaiming matters, and the order matters.** Nothing executes the task, so
without an unclaim the claim holds the job queued until it times out, and
cancelling the run reports success while the job sits there for minutes.
Unclaiming first releases the task and lets the cancel take effect immediately.
Cancelling before unclaiming drops the task from the distributor, after which
unclaim answers 404.

## What these tests deliberately do not do

- **They do not install or run a real runner agent.** The agent part is
  simulated: the tests make the same requests an agent makes, using the token from
  the generated config. The API registers the runner before it looks for work, so
  the instance appears in `circleci runner instance list` with no queued job
  needed.
- **They never execute a task.** The queued-task test claims one and hands it
  straight back. No step in that `.circleci/config.yml` ever runs.
- **They do not cover `circleci runner open`**, which makes no API call and would
  try to launch a browser. See ONP-3562.
- **They do not cover task counts.** `circleci runner task` was removed in
  CircleCI-Public/circleci-cli#1723; the functionality returns with the
  consolidated v3 runner command.
