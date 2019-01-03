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
	go clean -i
	rm -rf build

.PHONY: test
test:
	go test -v ./...

.PHONY: cover
cover:
	go test -race -coverprofile=coverage.txt ./...

.PHONY: lint
lint:
	@echo Executing local build of lint job until gometalinter supports Go 1.11 modules...
	@echo This requires Docker to run, so it may fail if docker cannot be found.
	@echo 
	@echo Generating tmp/processed.yml from .circleci/config.yml 2.1 version
	go run main.go config process .circleci/config.yml > .circleci/processed.yml
	@echo 
	@echo Running local build..
	go run main.go local execute -c .circleci/processed.yml --job lint

.PHONY: doc
doc:
	godoc -http=:6060

.PHONY: dev
dev:
	go get golang.org/x/tools/cmd/godoc

.PHONY: always
always:
