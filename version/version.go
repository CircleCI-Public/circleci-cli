package version

// These vars set by `goreleaser`:
var (
	// Version is the current Git tag (the v prefix is stripped) or the name of the snapshot, if you’re using the --snapshot flag
	Version = "local-dev-build"
	// Commit is the current git commit SHA
	Commit = "dirty-local-tree"
)
