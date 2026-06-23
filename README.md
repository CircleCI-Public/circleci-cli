# circleci-cli

This is CircleCI's command-line application.

[Documentation](https://cli.circleci.com/reference/) |
[Code of Conduct](./CODE_OF_CONDUCT.md) |
[Contribution Guidelines](./CONTRIBUTING.md) |

[![CircleCI](https://circleci.com/gh/CircleCI-Public/circleci-cli.svg?style=shield)](https://circleci.com/gh/CircleCI-Public/circleci-cli)
[![GitHub release](https://img.shields.io/github/tag/CircleCI-Public/circleci-cli.svg?label=latest)](https://github.com/CircleCI-Public/circleci-cli/releases)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/CircleCI-Public/circleci-cli)
[![License](https://img.shields.io/badge/license-MIT-red.svg)](./LICENSE)

`circleci` is CircleCI's official command line tool. It is an agent-friedly CLI that brings CI runs, jobs,
configuration, and other CircleCI features to the terminal right where you're already working.

<p align="center">
  <a href="./docs/demos/run-get.svg">
    <img alt="output of freeze command, Haskell code block" src="./docs/demos/run-get.svg" width="800" />
  </a>
</p>

The CLI is supported for users on [circleci.com](https://circleci.com) and CircleCI server; with support for macOS,
Windows, and Linux.

## Installation

The CircleCI CLI is available on the following package managers:

Homebrew (preview):
```shell
brew tap circleci-public/homebrew-circleci
brew install circleci@next
```

Homebrew (stable):
```shell
brew install circleci
```

Snap:
```shell
sudo snap install circleci
```

WinGet:
```shell
winget install CircleCI.CLI
```

Chocolatey:
```shell
choco install circleci-cli -y
```

## Setup

### Login
Run the following command to login to the CircleCI CLI:
```shell
circleci auth login
```

### Model Context Protocol (MCP)
The CLI supports the MCP protocol. To enable it, run:

Claude:
```shell
circleci mcp claude enable # Enable in Claude desktop
claude mcp add-from-claude-desktop -s user # Add with current user scope
```

Cursor:
```shell
circleci mcp cursor enable
```

VS Code:
```shell
circleci mcp vscode enable
```

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
