VERSION=0.1
DATE = $(shell date "+%FT%T%z")
SHA=$(shell git rev-parse --short HEAD)

GOPACKAGES = $(shell go list ./... 2>/dev/null )
GOFILES = $(shell find . -name '*.go' -not -path './vendor/*')

CLIPATH=github.com/circleci/cli

EXECUTABLE=cli
BUILD_DIR=build

.PHONY: build deps vendor clean test release release/darwin release/linux

$(BUILD_DIR)/%/amd64/$(EXECUTABLE): $(GOFILES)
	GOOS=$* GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$*/amd64/$(EXECUTABLE) .

build: $(BUILD_DIR)/darwin/amd64/$(EXECUTABLE) $(BUILD_DIR)/linux/amd64/$(EXECUTABLE)

clean:
	go clean
	rm -rf $(BUILD_DIR)

test:
	@echo go test -short ./...
	@go test -short $(GOPACKAGES)

coverage:
	go test -coverprofile=coverage.txt -covermode=count ./...
