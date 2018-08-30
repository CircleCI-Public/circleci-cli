package version

import (
	"fmt"
)

// These vars set by `goreleaser`:
var (
	// Version is the current Git tag (the v prefix is stripped) or the name of the snapshot, if you’re using the --snapshot flag
	Version = "0.0.0-dev"
	// Commit is the current git commit SHA
	Commit = "dirty-local-tree"
)

// UserAgent returns the user agent that should be user for external requests
func UserAgent() string {
	return fmt.Sprintf("circleci-cli/%s+%s", Version, Commit)
}
