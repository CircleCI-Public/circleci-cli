# circleci/cli

Summary

## Requirements

* Go 1.x
* Make
* ...

It's written in Go. If you are new to Go, we recommend the following resources:

* [A Tour of Go](https://tour.golang.org/welcome/1)
* [The Go documentation](https://golang.org/doc/)

## Development Workflow

### 1. Go Dependencies

```
$ go get github.com/spf13/cobra
$ go get github.com/spf13/viper
$ go get github.com/machinebox/graphql
$ go get ...
```

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
