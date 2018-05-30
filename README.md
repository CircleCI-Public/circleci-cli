# circleci/cli

Summary

## Requirements

* Go 1.9+
* Make
* ...

It's written in Go. If you are new to Go, we recommend the following resources:

* [A Tour of Go](https://tour.golang.org/welcome/1)
* [The Go documentation](https://golang.org/doc/)

## Development Workflow

### 1. Go Dependencies

Install `dep`:
https://github.com/golang/dep

On MacOS:

```
$ brew install dep
$ brew upgrade dep
```

On Linux, etc:

```
$ curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
```

Ensure dependencies are installed:

```
$ dep ensure
```

TODO: we want `make` to check vendor? They are committed...

### 2. Build it

```
$ make
```

### 3. Run Check

```
$ cli check
# some output
```

## Known Issues

* ...

## Doc

You can view godoc of cli in your browser.

1. Run `godoc -http=:6060`
2. Access http://localhost:6060/pkg/github.com/circleci/cli/

## Editor support

Go has great tooling for editors.

```
$ go get golang.org/x/tools/cmd/goimports
```

More [here](https://blog.golang.org/go-fmt-your-code).
