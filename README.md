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
 
Accept the provided default `CircleCI API End Point`.


## Validate A Build Config

To ensure that the tool is installed, you can use it to validate a build config file.

```
$ circleci config validate
Config file at .circleci/config.yml is valid
```
