#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

function error() {
    echo "An error occured installing golangci-lint."
}

trap error SIGINT

curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s v1.16.0

command -v ./bin/golangci-lint
