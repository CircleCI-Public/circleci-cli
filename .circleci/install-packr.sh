#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

PACKR_VERSION="v2.2.0"
DESTDIR="${DESTDIR:-$PWD/bin}"

function error() {
    echo "An error occured installing the tool."
    echo "The contents of the directory $SCRATCH have been left in place to help to debug the issue."
}

trap error SIGINT

GOBIN=${DESTDIR} go install github.com/gobuffalo/packr/v2/packr2@${PACKR_VERSION}

exit 0
