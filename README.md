# circleci-cli

This project is the seed for CircleCI's new command-line application.

## Getting Started

### 1. Get the latest binary

Download the [latest release](https://github.com/CircleCI-Public/circleci-cli/releases/latest) from GitHub for your operating system. If you're on a Mac, this would be `circleci-cli_0.1.XXX_darwin_amd64.tar.gz`.

### 2. Put the binary in your $PATH

```
$ tar -xvzf circleci-cli_0.1.XXX_darwin_amd64.tar.gz
$ mv circleci /usr/local/bin
$ circleci help
```

### 3. Add a Token
You may first need to generate a CircleCI API Token from the [Personal API Token tab](https://circleci.com/account/api).

```
$ circleci configure
```

If you are using this tool on `.com`. accept the provided default `CircleCI API End Point`. If you are using it on Server, change the value to your Server address (i.e. `circleci.my-org.com`).


## Validate A Build Config

To ensure that the tool is installed, you can use it to validate a build config file.

```
$ circleci config validate
Config file at .circleci/config.yml is valid
```

## Development

If you developed on this project and ran `go install` at some point, you will have a development version of `circleci-cli` lingering around.  You should clean this up with `make clean`.

```bash
$ which circleci-cli
/Users/erichu/go/bin/circleci-cli
$ make clean
$ which circleci-cli
$
```