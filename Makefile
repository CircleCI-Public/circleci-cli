default: build

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

build: always
	GO111MODULE=on .circleci/pack.sh
	go build -o build/$(GOOS)/$(GOARCH)/circleci

build-all: build/linux/amd64/circleci build/darwin/amd64/circleci

build/%/amd64/circleci: always
	GOOS=$* GOARCH=amd64 go build -v -o $@ .

.PHONY: clean
clean:
	GO111MODULE=off go clean -i
	rm -rf build out docs dist
	.circleci/pack.sh clean

.PHONY: test
test:
	go test -v ./...

.PHONY: cover
cover:
	go test -race -coverprofile=coverage.txt ./...

.PHONY: lint
lint:
	bash .circleci/lint.sh

.PHONY: doc
doc:
	godoc -http=:6060

.PHONY: install-packr
install-packr:
	bash .circleci/install-packr.sh

.PHONY: pack
pack:
	bash .circleci/pack.sh

.PHONY: install-lint
install-lint:
	bash .circleci/install-lint.sh


.PHONY: always
always:
