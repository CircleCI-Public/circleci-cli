default: build

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

build: always
	go build -o build/$(GOOS)/$(GOARCH)/circleci

build-all: build/linux/amd64/circleci build/darwin/amd64/circleci

build/%/amd64/circleci: always
	GOOS=$* GOARCH=amd64 go build -v -o $@ .

.PHONY: clean
clean:
	go clean
	rm -rf build

.PHONY: test
test:
	go test -v -short ./...

.PHONY: cover
cover:
	go test -race -coverprofile=coverage.txt ./...

.PHONY: lint
lint:
	gometalinter ./...

.PHONY: doc
doc:
	godoc -http=:6060

.PHONY: dev
dev:
	go get golang.org/x/tools/cmd/godoc
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install

.PHONY: always
always:
