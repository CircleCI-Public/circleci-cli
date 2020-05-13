package version

import (
	"fmt"
	"os"
)

// These vars set by `goreleaser`:
var (
	// Version is the current Git tag (the v prefix is stripped) or the name of
	// the snapshot, if you’re using the --snapshot flag
	Version = "0.0.0-dev"
	// Commit is the current git commit SHA
	Commit = "dirty-local-tree"
)

// PackageManager defines the package manager which was used to install the CLI.
// You can override this value using -X flag to the compiler ldflags. This is
// overridden when we build for Homebrew, but not for Snap. the binary that we
// ship with Snap is the same binary that we ship to the GitHub release.
var packageManager = "source"

func PackageManager() string {
	if runningInsideSnap() {
		return "snap"
	}
	return packageManager
}

// UserAgent returns the user agent that should be user for external requests
func UserAgent() string {
	return fmt.Sprintf("circleci-cli/%s+%s (%s)", Version, Commit, PackageManager())
}

func runningInsideSnap() bool {
	// Snap sets a bunch of env vars when apps are running inside the snap
	// containers. SNAP_NAME is the name of the snap as specified in the
	// `snapcraft.yaml` file.
	// https://snapcraft.io/docs/environment-variables
	return os.Getenv("SNAP_NAME") == "circleci"
}
