// Package buildinfo exposes version information stamped into the binary at
// build time.
package buildinfo

// Version is the binary version. It defaults to "dev" for un-stamped builds and
// is overridden at link time via:
//
//	-X github.com/chinmina/kms-import/internal/buildinfo.Version=<version>
//
// goreleaser sets the released tag; "just build" sets a dev prerelease. The CLI
// surfaces it through urfave/cli's built-in --version flag.
var Version = "dev"
