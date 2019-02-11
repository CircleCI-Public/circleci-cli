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
grep -i "$(uname)" tarball_urls.txt | xargs curl --silent --retry 3 --fail --location --output packr.tgz

tar zxf packr.tgz --strip 1

echo "Installing to $DESTDIR"
mv packr2 "$DESTDIR"
chmod +x "$DESTDIR/packr2"

command -v packr2

# Delete the working directory when the install was successful.
rm -r "$SCRATCH"

exit 0
