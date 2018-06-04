VERSION=0.1
DATE = $(shell date "+%FT%T%z")
SHA=$(shell git rev-parse --short HEAD)

GOFILES = $(shell find . -name '*.go' -not -path './vendor/*')

CLIPATH=github.com/circleci/circleci-cli

EXECUTABLE=circleci-cli
BUILD_DIR=build

.PHONY: build/linux
$(BUILD_DIR)/%/amd64/$(EXECUTABLE): $(GOFILES)
	GOOS=$* GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$*/amd64/$(EXECUTABLE) .

.PHONY: build
build: $(BUILD_DIR)/darwin/amd64/$(EXECUTABLE) $(BUILD_DIR)/linux/amd64/$(EXECUTABLE)

.PHONY: clean
clean:
	go clean
	rm -rf $(BUILD_DIR)

.PHONY: test
test:
	go test -short ./...

.PHONY: cover
coverage:
	go test -coverprofile=coverage.txt -covermode=count ./...
