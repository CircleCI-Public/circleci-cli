#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

function error() {
    echo "An error occured installing golangci-lint."
}

trap error SIGINT

curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s v1.17.1

command -v ./bin/golangci-lint
