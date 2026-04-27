# circleci-cli-v2
Use CircleCI from the command line

## Development

### Local

This repository makes use [Task](https://taskfile.dev/#/) which can be installed (on MacOS) with:

```
$ brew install go-task/tap/go-task
```

Most other tools referenced in the `Taskfile.yml` are managed by the go.mod tool section.

TODO: add instructions to install other tools.

See the full list of available tasks by running `task -l`, or, see the [Taskfile.yml](./Taskfile.yml) script.

```sh
# Run all static checks
task check
# Format all the code
task fmt
# Apply licence headers
task licence

# Run all the tests
task test
# Run the quick tests
task test -- -short ./...
# Run the quick tests for one package
task test -- -short ./api/...
