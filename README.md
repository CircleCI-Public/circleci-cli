# circleci-cli

This project is the seed for CircleCI's new command-line application.

[Documentation](https://circleci-public.github.io/circleci-cli) |
[Code of Conduct](./CODE_OF_CONDUCT.md) |
[Contribution Guidelines](./CONTRIBUTING.md) |
[Hacking](./HACKING.md)

[![CircleCI](https://circleci.com/gh/CircleCI-Public/circleci-cli.svg?style=shield)](https://circleci.com/gh/CircleCI-Public/circleci-cli)
[![GitHub release](https://img.shields.io/github/tag/CircleCI-Public/circleci-cli.svg?label=latest)](https://github.com/CircleCI-Public/circleci-cli/releases)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/CircleCI-Public/circleci-cli)
[![Codecov](https://codecov.io/gh/CircleCI-Public/circleci-cli/branch/master/graph/badge.svg)](https://codecov.io/gh/CircleCI-Public/circleci-cli)
[![License](https://img.shields.io/badge/license-MIT-red.svg)](./LICENSE)

## Getting Started

### From Scratch

#### Install script

If you're installing the new `circleci` CLI for the first time, run the following command:

```
curl -fLSs https://circle.ci/cli | bash
```

By default, the `circleci` app will be installed to the ``/usr/local/bin`` directory. If you do not have write permissions to `/usr/local/bin`, you may need to run the above command with `sudo`. Alternatively, you can install to an alternate location by defining the `DESTDIR` environment variable when invoking `bash`:

```
curl -fLSs https://circle.ci/cli | DESTDIR=/opt/bin bash
```

You can also set a specific version of the CLI to install with the `VERSION` environment variable:

```
curl -fLSs https://circle.ci/cli | VERSION=0.1.5222 sudo bash
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

This command may require `sudo` if your user doesn't have write permissions to the install directory, `/usr/local/bin`. Otherwise, you may see the following error:

```
mv: cannot move 'circleci' to '/usr/local/bin/circleci': Permission denied
```


### Updating after install

The CLI comes with a built in version managment system. You can check if there any updates pending and update if so using the following commands:
```
circleci update check
circleci update install
```

## Configure the CLI

After installing the latest version of our CLI, you must run setup to configure the tool.

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


If you are using this tool on `circleci.com`. accept the provided default `CircleCI Host`.

Server users will have to change the default value to your custom address (i.e. `circleci.my-org.com`).

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
docker run --rm -v $(pwd):/data circleci/circleci-cli:alpine config validate /data/.circleci/config.yml --token $TOKEN
```

## circleci-agent

In order to maintain backwards compatibility with the `circleci` binary present in builds, some commands are proxied to a program called `circleci-agent`.

This program must exist in your `$PATH` as is the case inside of a job.

The following commands are affected:

* `circleci tests split`
* `circleci step halt`
* `circleci config migrate`

## Contributing

Development instructions for the CircleCI CLI can be found in [HACKING.md](HACKING.md).

## More

Please see the [documentation](https://circleci-public.github.io/circleci-cli) or `circleci help` for more.
