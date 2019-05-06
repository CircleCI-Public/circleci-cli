#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

export GO111MODULE=on

function error() {
    echo "An error occured running packr."
}

trap error SIGINT

function get_arch_type() {
    if [[ $(uname -m) == "i686" ]]; then
        echo "386"
    elif [[ $(uname -m) == "x86_64" ]]; then
        echo "amd64"
    fi
}

function get_arch_base() {
    if [[ "$OSTYPE" == "linux-gnu" ]]; then
        echo "linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        echo "darwin"
    fi
}

ARCH="$(get_arch_base)_$(get_arch_type)"
CMD="bin/$ARCH/packr2"

command -v "$CMD"

./"$CMD" build

exit 0
