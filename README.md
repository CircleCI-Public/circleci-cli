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

### From Scratch

#### Install script

If you're installing the new `circleci` CLI for the first time, run the following command:

```
bash -c "$(curl -fSl https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/master/install.sh)"
```

This will install the CLI into the `/usr/local/bin` directory.

If you do not have write permissions to `/usr/local/bin`, you may need to run the above command with `sudo`.

Optionally, you can install in a arbitrary directory by using `DESTDIR` environment variable.

```
export DESTDIR=/path/to/install/ && bash -c "$(curl -fSl https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/master/install.sh)"
```

#### Homebrew

```
brew install circleci
```

#### Snapcraft

```
sudo snap install circleci
```

### Upgrade from existing CLI

If you installed the old CLI before, and you're on version less than `0.1.6`, you need to run the following commands:

```
circleci update
circleci switch
```

This command may prompt you for `sudo` if your user doesn't have write permissions to the install directory, `/usr/local/bin`.

### Updating after install

The CLI comes with a built in version managment system. You can check if there any updates pending and update if so using the following commands:
```
circleci update check
circleci update install
```

## Configure the CLI

You may first need to generate a CircleCI API Token from the [Personal API Token tab](https://circleci.com/account/api).

```
$ circleci setup
```

If you are using this tool on `circleci.com`. accept the provided default `CircleCI Host`.

Server users will have to change the default value to your custom address (i.e. `circleci.my-org.com`).

**Note**: Server does not yet support config processing and orbs, you will only be able to use `circleci local execute` (previously `circleci build`) for now.


## Validate A Build Config

To ensure that the tool is installed, you can use it to validate a build config file.

```
$ circleci config validate
Config file at .circleci/config.yml is valid
```

## Contributing

Development instructions for the CircleCI CLI can be found in [HACKING.md](HACKING.md).

## More

Please see the [documentation](https://circleci-public.github.io/circleci-cli) or `circleci help` for more.
