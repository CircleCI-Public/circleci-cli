#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

PACKR_VERSION="2.0.1"
RELEASE_URL="https://github.com/gobuffalo/packr/releases/download"
DESTDIR="${DESTDIR:-$PWD/bin}"

SCRATCH=$(mktemp -d)
cd "$SCRATCH"

function error() {
    echo "An error occured installing the tool."
    echo "The contents of the directory $SCRATCH have been left in place to help to debug the issue."
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
PACKR_RELEASE_URL="${RELEASE_URL}/v${PACKR_VERSION}/packr_${PACKR_VERSION}_${ARCH}.tar.gz"

echo "Fetching packr from $PACKR_RELEASE_URL"

curl --retry 3 --fail --location "$PACKR_RELEASE_URL" | tar -xz

echo "Installing packr for $ARCH to $DESTDIR"
mv packr2 "$DESTDIR"
chmod +x "$DESTDIR/packr2"

command -v "$DESTDIR/packr2"

# Delete the working directory when the install was successful.
rm -r "$SCRATCH"

exit 0
