#!/bin/sh
set -e

# Install the latest circleci from homebrew
brew update

VERSION=$("$DESTDIR"/circleci version)
TAG="v$(ruby -e "puts '$VERSION'.split(/[ +]/)[0]")"
REVISION=$(git rev-parse "$(ruby -e "puts '$VERSION'.split(/[ +]/)[1]")")
echo "Bumping circleci to $TAG+$REVISION"
brew bump-formula-pr --strict --tag="$TAG" --revision="$REVISION" circleci
