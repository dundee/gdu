package build

// Version stores the current version of the app
var Version = "development"

// Time of the build
var Time string

// User who built it
var User string

// SelfUpdate is set to "true" only in binaries attached to GitHub releases,
// so that builds made by package managers never replace themselves.
var SelfUpdate = "false"

// RootPathPrefix stores path to be prepended to given absolute path
// e.g. /var/lib/snapd/hostfs for snap
var RootPathPrefix = ""
