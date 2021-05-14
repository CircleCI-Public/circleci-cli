#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

export GO111MODULE=on

function error() {
    echo "An error occured running packr."
}

trap error SIGINT
CMD="bin/packr2"

command -v "$CMD"

./"$CMD" build

exit 0
