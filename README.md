# circleci-cli-v2
Use CircleCI from the command line

## Development

### Local

This repository makes use [Task](https://taskfile.dev/#/) which can be installed (on MacOS) with:

```
$ brew install go-task/tap/go-task
```

Most other tools referenced in the `Taskfile.yml` are managed by the go.mod tool section.

See the full list of available tasks by running `task -l`, or, see the [Taskfile.yml](./Taskfile.yml) script.

```sh
# Run all static checks
task check
# Auto-fix static checks
task fix
# Run all the tests
task test

# Run the quick tests
task test -- -short ./...
# Run the quick tests for one package
task test -- -short ./internal/...

# Format all the code
task fmt
# Apply license headers
task license
# Tidy go.mod
task mod-tidy
```
