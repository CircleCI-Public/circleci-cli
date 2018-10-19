#!/usr/bin/env bash
set -o errexit
set -o nounset

RELEASE_URL="https://api.github.com/repos/CircleCI-Public/circleci-cli/releases/latest"
DESTDIR="${DESTDIR:-/usr/local/bin}"

# Run the script in a temporary directory that we know is empty.
SCRATCH=$(mktemp -d)
cd "$SCRATCH"

function error {
  echo "An error occured installing the tool."
  echo "The contents of the directory $SCRATCH have been left in place to help to debug the issue."
}

trap error ERR

echo "Finding latest release."
curl --retry 3 --fail --location --silent --output release.json "$RELEASE_URL"
python -m json.tool release.json > formatted_release.json

STRIP_JSON_STRING='s/.*"([^"]+)".*/\1/'

echo -n 'Downloading CircleCI '
grep tag_name formatted_release.json | sed -E "$STRIP_JSON_STRING"

grep browser_download_url formatted_release.json | sed -E "$STRIP_JSON_STRING" > tarball_urls.txt
grep -i "$(uname)" tarball_urls.txt | xargs curl --retry 3 --fail --location --output circleci.tgz

tar zxvf circleci.tgz --strip 1

echo "Installing to $DESTDIR"
mv circleci "$DESTDIR"
chmod +x "$DESTDIR/circleci"

command -v circleci

# Delete the working directory when the install was successful.
rm -r "$SCRATCH"
