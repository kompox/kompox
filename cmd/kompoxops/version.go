package main

// Build-time variables set via ldflags during releases
var (
	version = "latest"  // version is the application version shown by --version
	commit  = "unknown" // commit is the git commit hash
	date    = "unknown" // date is the build date
)
