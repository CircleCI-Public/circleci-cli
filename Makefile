VERSION=0.1

GOFILES = $(shell find . -name '*.go' -not -path './vendor/*')

OS = $(shell uname)

CLIPATH=github.com/CircleCI-Public/circleci-cli

EXECUTABLE=circleci-cli
BUILD_DIR=build

.PHONY: build
build:
ifeq ($(OS), Darwin)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/darwin/amd64/$(EXECUTABLE)
else ifeq ($(OS), Linux)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/linux/amd64/$(EXECUTABLE)
endif

.PHONY: build/*
$(BUILD_DIR)/%/amd64/$(EXECUTABLE): $(GOFILES)
	GOOS=$* GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$*/amd64/$(EXECUTABLE) .

.PHONY: build-all
build-all: $(BUILD_DIR)/darwin/amd64/$(EXECUTABLE) $(BUILD_DIR)/linux/amd64/$(EXECUTABLE)

.PHONY: clean
clean:
	go clean
	rm -rf $(BUILD_DIR)

.PHONY: test
test:
	go test -v -short ./...

.PHONY: cover
cover:
	go test -race -coverprofile=coverage.txt ./...

.PHONY: lint
lint:
	gometalinter --deadline 60s --vendor ./...

.PHONY: doc
doc:
	godoc -http=:6060

.PHONY: dev
dev:
	go get golang.org/x/tools/cmd/godoc
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install
