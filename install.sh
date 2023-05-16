#!/usr/bin/env bash

# Install the CircleCI CLI tool.
# https://github.com/CircleCI-Public/circleci-cli
#
# Dependencies: curl, cut
#
# The version to install and the binary location can be passed in via VERSION and DESTDIR respectively.
#

set -o errexit

function error {
  echo "An error occured installing the tool."
  echo "The contents of the directory $SCRATCH have been left in place to help to debug the issue."
}

# Use a function to ensure connection errors don't partially execute when being piped
function install_cli {

	echo "Starting installation."

	# GitHub's URL for the latest release, will redirect.
	GITHUB_BASE_URL="https://github.com/CircleCI-Public/circleci-cli"
	LATEST_URL="${GITHUB_BASE_URL}/releases/latest/"
	DESTDIR="${DESTDIR:-/usr/local/bin}"

	if [ -z "$VERSION" ]; then
		VERSION=$(curl -sLI -o /dev/null -w '%{url_effective}' "$LATEST_URL" | cut -d "v" -f 2)
	fi

	echo "Installing CircleCI CLI v${VERSION}"

	# Run the script in a temporary directory that we know is empty.
	SCRATCH=$(mktemp -d || mktemp -d -t 'tmp')
	cd "$SCRATCH"

	trap error ERR

	# Determine release filename.
	case "$(uname)" in
		Linux)
			OS='linux'
		;;
		Darwin)
			OS='darwin'
		;;
		*)
			echo "This operating system is not supported."
			exit 1	
		;;
	esac

	case "$(uname -m)" in
		aarch64 | arm64)
			ARCH='arm64'
		;;
		x86_64)
			ARCH="amd64"
		;;
		*)
			echo "This architecture is not supported."
			exit 1
		;;
	esac

	RELEASE_URL="${GITHUB_BASE_URL}/releases/download/v${VERSION}/circleci-cli_${VERSION}_${OS}_${ARCH}.tar.gz"

	# Download & unpack the release tarball.
	curl --ssl-reqd -sL --retry 3 "${RELEASE_URL}" | tar zx --strip 1

	echo "Installing to $DESTDIR"
	install circleci "$DESTDIR"

	command -v circleci

	# Delete the working directory when the install was successful.
	rm -r "$SCRATCH"
}

install_cli
