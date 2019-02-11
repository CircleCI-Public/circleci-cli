#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

PACKR_DEST=bin/packr2
PACKR_VERSION=2.0.1
PACKR_URL=https://github.com/gobuffalo/packr/releases/download/v"$PACKR_VERSION"/packr_"$PACKR_VERSION"_linux_amd64.tar.gz

export PACKR_DEST
export PACKR_VERSION
export PACKR_URL

TDIR=$(mktemp -d)
ORIGIN="$PWD"

cleanup() {
    test -n "$TDIR" && test -d "$TDIR" && rm -rf "$TDIR"
}

trap cleanup SIGINT
trap 'cleanup; exit 127' INT TERM

(
    cd "$TDIR"
    curl -SL "$PACKR_URL" | tar -xzv
)

(
    cd "$ORIGIN"
    mv "$TDIR"/packr2 "$PACKR_DEST"

    rm -rf "$TDIR"
)

exit 0
