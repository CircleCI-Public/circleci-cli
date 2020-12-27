# Contributing to the CLI

If you're looking to contribute to this project, there's a few things you should know.

First, make sure you go through the [README](README.md).

Second, it's written in Go. If you are new to Go, we recommend the following resources:

* [A Tour of Go](https://tour.golang.org/welcome/1)
* [The Go documentation](https://golang.org/doc/)

## Requirements

* Go 1.12+
* Make
* ...

## Getting setup

You should already have [installed Go](https://golang.org/doc/install).

### 1. Get the repo

Clone the repo.

```
$ git clone github.com/CircleCI-Public/circleci-cli
$ cd circleci-cli
```

If you cloned the repo inside of your `$GOPATH`, you can use `GO111MODULE=on` in order to use Go modules. We recommend cloning the repo outside of `$GOPATH` as you would any other source code project, for example `~/code/circleci-cli`.

### 2. Build the binary

```
$ make
```

### 3. Run tests

```
$ make test
```

### 4. Cleaning up installed binary

If you ran `go install` at some point, you will have a development version of `circleci-cli` lingering around.  You should clean this up with `make clean`.

```bash
$ which circleci-cli
/Users/erichu/go/bin/circleci-cli
$ make clean
$ which circleci-cli
$
```

## Managing Dependencies

We use Go 1.11 Modules for managing our dependencies.

You can read more about it on the wiki:
https://github.com/golang/go/wiki/Modules

## Linting your code

We use [`gometalinter`](github.com/alecthomas/gometalinter) for linting.

However, since we updated to Go 1.11 modules this doesn't work.

You will have to run our `lint` job inside a local build in order to lint your code changes.

This requires docker and may fail if the docker is not available on your machine.

Once you have installed Docker, you can execute the `lint` job locally with `$ make lint`.

## Code coverage

There is also a `coverage` job as part of the build which will lint any code you commit.

This can be run manually with `$ make cover`.

## Editor support

Go has great tooling such as [`gofmt`](https://golang.org/cmd/gofmt/) and [`goimports`](https://godoc.org/golang.org/x/tools/cmd/goimports).

In particular, **please be sure to `gofmt` your code before committing**.

You can install `goimport` via:

```
$ go get golang.org/x/tools/cmd/goimports
```

The golang blog post ["go fmt your code"](https://blog.golang.org/go-fmt-your-code) has a lot more info `gofmt`. To get it setup with [vim](https://github.com/fatih/vim-go) or [emacs](https://github.com/dominikh/go-mode.el).

For example, I've the following in my `.emacs.d/init.el`:

```
(setq gofmt-command "goimports")
(require 'go-mode)
(add-hook 'before-save-hook 'gofmt-before-save)
(require 'go-rename)
```

## Viewing API Documentation

You can view the documentation for this project in your browser using `godoc`.

After installing it via `make dev`.

1. Run `make doc`.
2. Access http://localhost:6060/pkg/github.com/CircleCI-Public/circleci-cli/
