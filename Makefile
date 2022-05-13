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
	GO111MODULE=off go clean -i
	rm -rf build out docs dist

.PHONY: test
test:
	TESTING=true go test -v ./...

.PHONY: cover
cover:
	go test -race -coverprofile=coverage.txt ./...

.PHONY: lint
lint:
	bash .circleci/lint.sh

.PHONY: doc
doc:
	godoc -http=:6060

.PHONY: install-lint
install-lint:
	bash .circleci/install-lint.sh


.PHONY: always
always:
