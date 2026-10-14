package version

// Version is set at build time via -ldflags.
var Version = "dev"

// SnapshotTag is the GitHub pre-release tag for snapshot builds (e.g. "snapshot-feat-foo").
// Set at build time via -ldflags. Empty for release and local dev builds.
var SnapshotTag = ""

// Commit is the git commit hash (set at build time)
var Commit = "unknown"

// Date is the build date (set at build time)
var Date = "unknown"
