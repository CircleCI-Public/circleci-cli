default: build

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

build: always
	go build -o build/$(GOOS)/$(GOARCH)/circleci -ldflags='-X github.com/CircleCI-Public/circleci-cli/telemetry.SegmentEndpoint=https://api.segment.io'

build-all: build/linux/amd64/circleci build/darwin/amd64/circleci

build/%/amd64/circleci: always
	GOOS=$* GOARCH=amd64 go build -v -o $@ .

.PHONY: clean
clean:
	GO111MODULE=off go clean -i
	rm -rf build out docs dist

.PHONY: test
test:
	go test -v ./...

.PHONY: cover
cover:
	go test -race -coverprofile=coverage.txt ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: doc
doc:
	godoc -http=:6060

.PHONY: always
always:
