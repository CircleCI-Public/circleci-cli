#!/bin/sh

set -e

# Remove the original circleci-agent from /usr/local/bin for homebrew
rm /usr/local/bin/circleci
# Install the latest circleci from homebrew
brew install circleci

VERSION=$("$DESTDIR"/circleci version)
TAG="v$(ruby -e "puts '$VERSION'.split('+')[0]")"
REVISION=$(git rev-parse "$(ruby -e "puts '$VERSION'.split('+')[1]")")
echo "Bumping circleci to $TAG+$REVISION"
brew bump-formula-pr --strict \
  --tag="$TAG" \
  --revision="$REVISION" \
  circleci
