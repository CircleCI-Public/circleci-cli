# rails-orb

Reusable configuration for Rails projects on CircleCI.  Commands and jobs within this orb should abstract away common build configuration and stay up to date with best practices.

## Getting started

This repo can be downloaded or sub-tree'd into your `.circleci/orbs` folder in your project. See the [config compiler](https://github.com/circleci/config-compilation) during the internal pre-release period to understand usage in a `build.yml` file that will generate a `config.yml` file.

See [the rails-postgres build.yml](usage/build.yml) for an example of current syntax.

## Usage

Take a look at [rails-orb-demo](https://github.com/circleci/rails-orb-demo) for example how to use this orb in a CircleCI project.

## Versioning

Orbs should adhere to a versioning scheme.  We recommend using SemVer with [Github Releases](https://help.github.com/articles/creating-releases/). Orb consumers will then be able to download a zip of multiple files, when we add multi-file orb processing to the config compiler.
