#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

RELEASE_URL="https://api.github.com/repos/CircleCI-Public/circleci-cli/releases/latest"
DEST="/usr/local/bin/circleci"

# Run the script in a temporary directory that we know is empty.
SCRATCH=$(mktemp -d)
cd "$SCRATCH"

function finish {
  # Delete the working directory when the install was successful.
  rm -r "$SCRATCH"
}

function error {
  echo "An error occured installing the tool."
  echo "The contents of the directory $SCRATCH have been left in place to help to debug the issue."
}

trap finish EXIT
trap error ERR

echo "Finding latest release."
curl --retry 3 --fail --location --silent --output release.json "$RELEASE_URL" 
echo "$(python -m json.tool release.json)" > release.json

STRIP_JSON_STRING='s/.*"([^"]+)".*/\1/'

echo -n 'Downloading CircleCI '
grep tag_name release.json | sed -E "$STRIP_JSON_STRING"

grep browser_download_url release.json | sed -E "$STRIP_JSON_STRING" > tarball_urls.txt
grep -i "$(uname)" tarball_urls.txt | xargs curl --retry 3 --fail --location --output circleci.tgz

tar zxvf circleci.tgz --strip 1

echo "Installing to $DEST"
mv circleci $DEST
chmod +x $DEST
command -v circleci
