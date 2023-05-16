# circleci-cli

This is CircleCI's command-line application.

[Documentation](https://circleci-public.github.io/circleci-cli) |
[Code of Conduct](./CODE_OF_CONDUCT.md) |
[Contribution Guidelines](./CONTRIBUTING.md) |
[Hacking](./HACKING.md)

[![CircleCI](https://circleci.com/gh/CircleCI-Public/circleci-cli.svg?style=shield)](https://circleci.com/gh/CircleCI-Public/circleci-cli)
[![GitHub release](https://img.shields.io/github/tag/CircleCI-Public/circleci-cli.svg?label=latest)](https://github.com/CircleCI-Public/circleci-cli/releases)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/CircleCI-Public/circleci-cli)
[![License](https://img.shields.io/badge/license-MIT-red.svg)](./LICENSE)

## Getting Started

### Installation

CircleCI CLI is available on the following package managers:

#### Homebrew

```
brew install circleci
```

#### Snap

```
sudo snap install circleci
```

#### Chocolatey

```
choco install circleci-cli -y
```

### Install script

You can also install the CLI binary by running our install script on most Unix platforms:

```
curl -fLSs https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/main/install.sh | bash
```

By default, the `circleci` app will be installed to the ``/usr/local/bin`` directory. If you do not have write permissions to `/usr/local/bin`, you may need to run the above command with `sudo`:

```
curl -fLSs https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/main/install.sh | sudo bash
```

Alternatively, you can install to an alternate location by defining the `DESTDIR` environment variable when invoking `bash`:

```
curl -fLSs https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/main/install.sh | DESTDIR=/opt/bin bash
```

You can also set a specific version of the CLI to install with the `VERSION` environment variable:

```
curl -fLSs https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/main/install.sh | sudo VERSION=0.1.5222 bash
```

Take note that additional environment variables should be passed between sudo and invoking bash.

#### Checksum verification

If you would like to verify the checksum yourself, you can download the checksum file from the [GitHub releases page](https://github.com/CircleCI-Public/circleci-cli/releases) and verify the checksum of the archive using the `circleci-cli_<version>_checksums.txt` inside the assets of the release you'd like to install:

On macOS and Linux:
```sh
shasum -a 256 circleci-cli_<version>_<os>.tar.gz
```

and on Windows:
```powershell
Get-FileHash .\circleci-cli_<version>_<os>.tar.gz -Algorithm SHA256 | Format-List
```

And compare it to the right checksum depending on the downloaded version in the `circleci-cli_<version>_checksums.txt` file.

### Updating

If you installed the CLI without a package manager, you can use its built-in update command to check for pending updates and download them:

```
circleci update check
circleci update install
```

## Configure the CLI

After installing the CLI, you must run setup to configure the tool.

```
$ circleci setup
```

You should be prompted to enter the _CircleCI API Token_ you generated from the [Personal API Token tab](https://circleci.com/account/api)


```
✔ CircleCI API Token:

API token has been set.

✔ CircleCI Host: https://circleci.com

CircleCI host has been set.

Setup complete. Your configuration has been saved.
```

If you are using this tool on `circleci.com`, accept the provided default `CircleCI Host`.

Server users will have to change the default value to your custom address (e.g., `circleci.my-org.com`).

**Note**: Server does not yet support config processing and orbs, you will only be able to use `circleci local execute` (previously `circleci build`) for now.


## Validate A Build Config

To ensure that the tool is installed, you can use it to validate a build config file.

```
$ circleci config validate

Config file at .circleci/config.yml is valid
```


## Docker

The CLI may also be used without installation by using Docker.

```
docker run --rm -v $(pwd):/data -w /data circleci/circleci-cli:alpine config validate /data/.circleci/config.yml --token $TOKEN
```

## circleci-agent

In order to maintain backwards compatibility with the `circleci` binary present in builds, some commands are proxied to a program called `circleci-agent`.

This program must exist in your `$PATH` as is the case inside of a job.

The following commands are affected:

* `circleci tests split`
* `circleci step halt`
* `circleci config migrate`

## Platforms, Deployment and Package Managers

The tool is deployed through a number of channels. The primary release channel is through [GitHub Releases](https://github.com/CircleCI-Public/circleci-cli/releases). Green builds on the `main` branch will publish a new GitHub release. These releases contain binaries for macOS, Linux and Windows. These releases are published from (CircleCI)[https://app.circleci.com/pipelines/github/CircleCI-Public/circleci-cli] using [GoReleaser](https://goreleaser.com/).

### Homebrew

We publish the tool to [Homebrew](https://brew.sh/). The tool is [part of `homebrew-core`](https://github.com/Homebrew/homebrew-core/blob/main/Formula/circleci.rb), and therefore the maintainers of the tool are obligated to follow the guidelines for acceptable Homebrew formulae. You should [familairise yourself with the guidelines](https://docs.brew.sh/Acceptable-Formulae#we-dont-like-tools-that-upgrade-themselves) before making changes to the Homebrew deployment system.

The particular considerations that we make are:


1. Since Homebrew [doesn't "like tools that upgrade themselves"](https://docs.brew.sh/Acceptable-Formulae#we-dont-like-tools-that-upgrade-themselves), we disable the `circleci update` command when the tool is released through homebrew. We do this by [defining the PackageManager](https://github.com/Homebrew/homebrew-core/blob/eb1fdb84e2924289bcc8c85ee45081bf83dc024d/Formula/circleci.rb#L28) constant to `homebrew`, which allows us to [disable the `update` command at runtime](https://github.com/CircleCI-Public/circleci-cli/blob/67c7d52bace63846f87a1ed79f67f257c94a55b4/cmd/root.go#L119-L123).
1. We want to avoid every push to `main` from creating a Pull Request to the `circleci` formula on Homebrew. We want to avoid overloading the Homebrew team with pull requests to update our formula for small changes (changes to docs or other files that don't change functionality in the tool).

### Snap

We publish Linux builds of the tool to the Snap package manager.

Further [package information is available on Snap website](https://snapcraft.io/circleci).

## Contributing

Development instructions for the CircleCI CLI can be found in [HACKING.md](HACKING.md).

## More

Please see the [documentation](https://circleci-public.github.io/circleci-cli) or `circleci help` for more.

## Server compatibility

There are some difference of behavior depending on the version you use:
 - config validation will use the GraphQL API until **Server v4.0.5, v4.1.3, v4.2.0**. The above versions will use the new route `compile-config-with-defaults`
 - `circleci orb validate` will only allow you to validate orbs using other private orbs with the option `--org-slug` from version **Server v4.2.0**
