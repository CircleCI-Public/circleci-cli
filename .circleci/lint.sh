#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

function error() {
    echo "An error occured running golangci-lint."
    echo "Have you run \"make install-lint\"?"
}

trap error SIGINT

./bin/golangci-lint run -E gofmt
