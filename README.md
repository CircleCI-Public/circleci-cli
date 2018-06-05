# circleci-cli [![CircleCI](https://circleci.com/gh/circleci/circleci-cli.svg?style=svg&circle-token=8adb850cb9110a49ab0990f198dd78966f9e6073)](https://circleci.com/gh/circleci/circleci-cli) [![codecov](https://codecov.io/gh/circleci/circleci-cli/branch/master/graph/badge.svg?token=VJMF7kG1Om)](https://codecov.io/gh/circleci/circleci-cli)

This project is the seed for CircleCI's new command-line application.

## Requirements

* Go 1.9+
* Make
* ...

It's written in Go. If you are new to Go, we recommend the following resources:

* [A Tour of Go](https://tour.golang.org/welcome/1)
* [The Go documentation](https://golang.org/doc/)

## Development

You should already have [installed Go](https://golang.org/doc/install) and setup your [workspace](https://golang.org/doc/code.html#Workspaces).

This includes setting a valid `$GOPATH`.

### 1. Get the repo

```
$ go get -u github.com/circleci/circleci-cli
$ cd $GOPATH/src/github.com/circleci/circleci-cli
```

### 2. Build it

```
$ make
```

### 3. Run Diagnostic check

```
$ ./build/target/linux/amd64/circleci-cli diagnostic

Please enter your CircleCI API token:
OK.
Your configuration has been created in `/home/zzak/.circleci/cli.yml`.
It can edited manually for advanced settings.

---
CircleCI CLI Diagnostics
---

Config found: `/home/zzak/.circleci/cli.yml`
Host is: https://circleci.com
OK, got a token.
```

## Dependencies

We use `dep` for vendoring our depencencies:
https://github.com/golang/dep

You can install it on MacOS:

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

## Known Issues

* ...

## Doc

You can view `godoc` of cli in your browser.

1. Run `godoc -http=:6060`
2. Access http://localhost:6060/pkg/github.com/circleci/circleci-cli/

## Editor support

Go has great tooling such as [`gofmt`](https://golang.org/cmd/gofmt/) and [`goimports`](https://godoc.org/golang.org/x/tools/cmd/goimports).

```
$ go get golang.org/x/tools/cmd/goimports
```

You can read about `gofmt` [here](https://blog.golang.org/go-fmt-your-code). In particular, you can set it up with [vim](https://github.com/fatih/vim-go) or [emacs](https://github.com/dominikh/go-mode.el).

I've the following in my `.emacs.d/init.el`:

```
(setq gofmt-command "goimports")
(require 'go-mode)
(add-hook 'before-save-hook 'gofmt-before-save)
(require 'go-rename)
```
