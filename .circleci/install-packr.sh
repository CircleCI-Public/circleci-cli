#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

RELEASE_URL="https://api.github.com/repos/gobuffalo/packr/releases/latest"
DESTDIR="${DESTDIR:-$PWD/bin}"

SCRATCH=$(mktemp -d)
cd "$SCRATCH"

function error() {
    echo "An error occured installing the tool."
    echo "The contents of the directory $SCRATCH have been left in place to help to debug the issue."
}

trap error SIGINT

echo "Finding latest release of packr."
curl --retry 3 --fail --location --silent --output release.json "$RELEASE_URL"
python -m json.tool release.json > formatted_release.json

STRIP_JSON_STRING='s/.*"([^"]+)".*/\1/'

echo -n 'Downloading packr '
grep tag_name formatted_release.json | sed -E "$STRIP_JSON_STRING"

grep browser_download_url formatted_release.json | sed -E "$STRIP_JSON_STRING" > tarball_urls.txt

if [[ $(uname -i) == "i686" ]]; then
    ARCH_TYPE="386"
elif [[ $(uname -i) == "x86_64" ]]; then
    ARCH_TYPE="amd64"
fi

if [[ "$OSTYPE" == "linux-gnu" ]]; then
    ARCH_BASE="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    ARCH_BASE="darwin"
fi

grep -i "$ARCH_BASE"_"$ARCH_TYPE" tarball_urls.txt | xargs curl --retry 3 --fail --location | tar -xz

echo "Installing packr for $ARCH_BASE-$ARCH_TYPE to $DESTDIR"
mv packr2 "$DESTDIR"
chmod +x "$DESTDIR/packr2"

command -v packr2

# Delete the working directory when the install was successful.
rm -r "$SCRATCH"

exit 0
