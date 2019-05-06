#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

PACKR_VERSION="2.2.0"
RELEASE_URL="https://github.com/gobuffalo/packr/releases/download"
DESTDIR="${DESTDIR:-$PWD/bin}"

SCRATCH=$(mktemp -d)
cd "$SCRATCH"

function error() {
    echo "An error occured installing the tool."
    echo "The contents of the directory $SCRATCH have been left in place to help to debug the issue."
}

trap error SIGINT

SUPPORTED_ARCHS=(darwin_386 darwin_amd64 linux_386 linux_amd64)

function install_arch() {
    ARCH=$1
    PACKR_RELEASE_URL="${RELEASE_URL}/v${PACKR_VERSION}/packr_${PACKR_VERSION}_${ARCH}.tar.gz"

    echo "Fetching packr from $PACKR_RELEASE_URL"

    curl --retry 3 --fail --location "$PACKR_RELEASE_URL" | tar -xz

    echo "Installing packr for $ARCH to $DESTDIR"
    mkdir "$DESTDIR/$ARCH"
    mv packr2 "$DESTDIR/$ARCH"
    chmod +x "$DESTDIR/$ARCH/packr2"

    command -v "$DESTDIR/$ARCH/packr2"
}

for ARCH in "${SUPPORTED_ARCHS[@]}"; do
    install_arch "$ARCH"
done

# Delete the working directory when the install was successful.
rm -r "$SCRATCH"

exit 0
