# circleci-cli

This project is the seed for CircleCI's new command-line application.

[Documentation](https://circleci-public.github.io/circleci-cli) |
[Code of Conduct](./CODE_OF_CONDUCT.md) |
[Contribution Guidelines](./CONTRIBUTING.md) |
[Hacking](./HACKING.md)

[![CircleCI](https://circleci.com/gh/CircleCI-Public/circleci-cli.svg?style=svg)](https://circleci.com/gh/CircleCI-Public/circleci-cli)
[![GitHub release](https://img.shields.io/github/tag/CircleCI-Public/circleci-cli.svg?label=latest)](https://github.com/CircleCI-Public/circleci-cli/releases)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/CircleCI-Public/circleci-cli)
[![Codecov](https://codecov.io/gh/CircleCI-Public/circleci-cli/branch/master/graph/badge.svg)](https://codecov.io/gh/CircleCI-Public/circleci-cli)
[![License](https://img.shields.io/badge/license-MIT-red.svg)](./LICENSE)

## Getting Started

### Upgrade from existing CLI

If you installed the old CLI before, and you're on version less than `0.1.6`, you need to run the following commands:

```
circleci update
circleci switch
```

This command may prompt you for `sudo` if your user doesn't have write permissions to the install directory, `/usr/local/bin`.

### From Scratch

If you're installing the new `circleci` CLI for the first time, run the following command:

```
bash -c "$(curl -fSl https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/master/install.sh)"
```

This will install the CLI into the `/usr/local/bin` directory.

If you do not have write permissions to `/usr/local/bin`, you may need to run the above command with `sudo`.

## Configure the CLI

You may first need to generate a CircleCI API Token from the [Personal API Token tab](https://circleci.com/account/api).

```
$ circleci setup
```

If you are using this tool on `.com`. accept the provided default `CircleCI Host`.

Server users will have to change the default value to your custom address (i.e. `circleci.my-org.com`).


## Validate A Build Config

To ensure that the tool is installed, you can use it to validate a build config file.

```
$ circleci config validate
Config file at .circleci/config.yml is valid
```

## More

Please see the [documentation](https://circleci-public.github.io/circleci-cli) or `circleci help` for more.
