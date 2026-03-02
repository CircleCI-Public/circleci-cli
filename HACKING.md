# Contributing to the CLI

If you're looking to contribute to this project, there's a few things you should know.

First, make sure you go through the [README](README.md).

Second, it's written in Go. If you are new to Go, we recommend the following resources:

* [A Tour of Go](https://tour.golang.org/welcome/1)
* [The Go documentation](https://golang.org/doc/)

## Requirements

* Go 1.12+
* [Task](https://taskfile.dev/) (task runner)
* ...

## Getting setup

You should already have [installed Go](https://golang.org/doc/install).

### 1. Get the repo

Clone the repo.

```
$ git clone git@github.com:CircleCI-Public/circleci-cli.git
$ cd circleci-cli
```

If you cloned the repo inside of your `$GOPATH`, you can use `GO111MODULE=on` in order to use Go modules. We recommend cloning the repo outside of `$GOPATH` as you would any other source code project, for example `~/code/circleci-cli`.

### 2. Build the binary

```
$ task build
```

### 3. Run tests

```
$ task test
```

### 4. Cleaning up installed binary

If you ran `go install` at some point, you will have a development version of `circleci-cli` lingering around.  You should clean this up with `task clean`.

```bash
$ which circleci-cli
/Users/erichu/go/bin/circleci-cli
$ task clean
$ which circleci-cli
$
```

## Managing Dependencies

We use Go 1.11 Modules for managing our dependencies.

You can read more about it on the wiki:
https://github.com/golang/go/wiki/Modules

## Linting your code

We use [`golangci-lint`](https://golangci-lint.run/) for linting.

You can run the linter locally with:

```
$ task lint
```

For CI-style output (JUnit XML), use `task ci:lint`.

## Code coverage

There is also a `coverage` job as part of the build which will lint any code you commit.

This can be run manually with `$ task cover`.

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

After installing it via `go install golang.org/x/tools/cmd/godoc@latest`.

1. Run `task doc`.
2. Access http://localhost:6060/pkg/github.com/CircleCI-Public/circleci-cli/
