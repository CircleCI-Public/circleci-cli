# circleci-cli

This project is the seed for CircleCI's new command-line application.

## Getting Started

### Upgrade from existing CLI

If you already have installed the `circleci` CLI previously, run the following commands:

```
circleci update
circleci switch
```

This command may prompt you for `sudo` if your user doesn't have write permissions to the install directory, `/usr/local/bin`.

### From Scratch

If you haven't already installed `circleci` on your machine, run the following command:

```
curl https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/master/install.sh \
	--fail --show-error | bash
```

The CLI, `circleci`, is downloaded to the `/usr/local/bin` directory.

If you do not have write permissions for `/usr/local/bin`, you might need to run the above commands with `sudo`.

## Configure the CLI

You may first need to generate a CircleCI API Token from the [Personal API Token tab](https://circleci.com/account/api).

```
$ circleci setup
```

If you are using this tool on `.com`. accept the provided default `CircleCI API End Point`. If you are using it on Server, change the value to your Server address (i.e. `circleci.my-org.com`).


## Validate A Build Config

To ensure that the tool is installed, you can use it to validate a build config file.

```
$ circleci config validate
Config file at .circleci/config.yml is valid
```
