# Contributing to the CLI

If you're looking to contribute to this project, there's a few things you should know.

First, make sure you go through the [README](README.md).

Second, it's written in Go. If you are new to Go, we recommend the following resources:

* [A Tour of Go](https://tour.golang.org/welcome/1)
* [The Go documentation](https://golang.org/doc/)

## Requirements

* Go 1.10+
* Make
* ...

## Getting setup

You should already have [installed Go](https://golang.org/doc/install) and setup your [workspace](https://golang.org/doc/code.html#Workspaces).

This includes setting a valid `$GOPATH`.

### 1. Get the repo

```
$ go get -u github.com/CircleCI-Public/circleci-cli
$ cd $GOPATH/src/github.com/CircleCI-Public/circleci-cli
```

### 2. Build the binary

```
$ make
```

### 3. Run tests

```
$ make test
```

## Managing Dependencies

We use `dep` for vendoring our depencencies:
https://github.com/golang/dep

If you want to update or modify any dependencies you will need to install it.

You can do so on MacOS:

```
$ brew install dep
$ brew upgrade dep
```

Or on Linux, etc:

```
$ curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
```

To make sure dependencies are installed:

```
$ dep ensure
```

## Linting your code

We use [`gometalinter`](github.com/alecthomas/gometalinter) for linting.

You can install it via `$ make dev` or manually with:

```
$ go get -u github.com/alecthomas/gometalinter
$ gometalinter --install
```

Then you can run it with `$ make lint`.

There is also a `coverage` job as part of the build which will lint any code you commit.

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
